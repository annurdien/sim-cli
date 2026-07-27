// CaptureHooks.mm
#import <AVFoundation/AVFoundation.h>
#import <QuartzCore/QuartzCore.h>
#import <VideoToolbox/VideoToolbox.h>
#import <CoreImage/CoreImage.h>
#import <dispatch/dispatch.h>
#import <objc/runtime.h>
#import <os/lock.h>

#import "CaptureHooks.h"
#import "FakeCaptureObjects.h"
#import "SampleBufferFactory.h"
#import "SharedFrameReader.hpp"

namespace {
struct InjectorState {
  os_unfair_lock lock = OS_UNFAIR_LOCK_INIT;
  dispatch_queue_t deliveryQueue = nullptr;
  dispatch_source_t deliveryTimer = nullptr;
  SharedFrameReader *reader = nullptr;
  MSCSampleBufferFactory *factory = nullptr;

  id<AVCaptureVideoDataOutputSampleBufferDelegate> delegate = nil;
  dispatch_queue_t delegateQueue = nullptr;
  AVCaptureVideoDataOutput *output = nil;

  NSHashTable<AVCaptureVideoPreviewLayer *> *previewLayers = nil;

  // Metadata (barcode/face) delivery. The app never sees a real capture
  // connection, so recognition runs here over the injected frames and the
  // results are handed straight to its AVCaptureMetadataOutput delegate.
  id<AVCaptureMetadataOutputObjectsDelegate> metadataDelegate = nil;
  dispatch_queue_t metadataDelegateQueue = nullptr;
  AVCaptureMetadataOutput *metadataOutput = nil;
  NSArray<AVMetadataObjectType> *requestedMetadataTypes = nil;
  dispatch_queue_t recognitionQueue = nullptr;
  bool recognitionInFlight = false;
  CFAbsoluteTime lastRecognition = 0;

  int32_t fps = 30;
  bool running = false;
  bool avSessionStarted = false;
};

InjectorState gState;

void startDelivery(void);
void stopDelivery(void);
void deliverFrame(void);
void runRecognition(CVPixelBufferRef pixelBuffer, CMTime time);

void MSCAutoStartSessionIfNeeded(AVCaptureSession *session) {
  if (!session)
    return;

  dispatch_after(
      dispatch_time(DISPATCH_TIME_NOW, (int64_t)(500 * NSEC_PER_MSEC)),
      dispatch_get_global_queue(QOS_CLASS_USER_INITIATED, 0), ^{
        os_unfair_lock_lock(&gState.lock);
        bool alreadyStarted = gState.avSessionStarted;
        os_unfair_lock_unlock(&gState.lock);

        if (!alreadyStarted) {
          NSLog(@"[IrisInject] Auto-starting AVCaptureSession");
          @try {
            [session startRunning];
          } @catch (NSException *e) {
            NSLog(@"[IrisInject] Auto-start failed: %@", e.reason);
          }
        }
      });
}

void swizzleInstance(Class cls, SEL orig, SEL repl) {
  Method origMethod = class_getInstanceMethod(cls, orig);
  Method replMethod = class_getInstanceMethod(cls, repl);
  if (!origMethod || !replMethod) {
    NSLog(@"[IrisInject] ⚠️ Cannot swizzle %@.%@", NSStringFromClass(cls),
          NSStringFromSelector(orig));
    return;
  }
  method_exchangeImplementations(origMethod, replMethod);
}

void swizzleClass(Class cls, SEL orig, SEL repl) {
  Class meta = object_getClass((id)cls);
  Method origMethod = class_getInstanceMethod(meta, orig);
  Method replMethod = class_getInstanceMethod(meta, repl);
  if (!origMethod || !replMethod) {
    NSLog(@"[IrisInject] ⚠️ Cannot swizzle class method %@.%@",
          NSStringFromClass(cls), NSStringFromSelector(orig));
    return;
  }
  method_exchangeImplementations(origMethod, replMethod);
}
} // namespace

// MARK: - AVCaptureSession (IrisHook)

@implementation AVCaptureSession (IrisHook)

- (void)iris_startRunning {
  NSLog(@"[IrisInject] AVCaptureSession startRunning intercepted");
  os_unfair_lock_lock(&gState.lock);
  gState.avSessionStarted = true;
  os_unfair_lock_unlock(&gState.lock);

  [self iris_startRunning];
  startDelivery();
}

- (void)iris_stopRunning {
  [self iris_stopRunning];
  stopDelivery();
}

- (BOOL)iris_canAddInput:(AVCaptureInput *)input {
  return YES;
}

- (void)iris_addInput:(AVCaptureInput *)input {
  NSLog(@"[IrisInject] AVCaptureSession addInput: silenced (no hardware)");
}

- (BOOL)iris_canAddOutput:(AVCaptureOutput *)output {
  return YES;
}

- (void)iris_addOutput:(AVCaptureOutput *)output {
  [self iris_addOutput:output];
}

- (BOOL)iris_isRunning {
  os_unfair_lock_lock(&gState.lock);
  BOOL isRunning = gState.running;
  os_unfair_lock_unlock(&gState.lock);
  return isRunning;
}

@end

// MARK: - AVCaptureVideoDataOutput (IrisHook)

@implementation AVCaptureVideoDataOutput (IrisHook)

- (void)iris_setSampleBufferDelegate:
            (id<AVCaptureVideoDataOutputSampleBufferDelegate>)delegate
                               queue:(dispatch_queue_t)queue {
  os_unfair_lock_lock(&gState.lock);
  gState.delegate = delegate;
  gState.delegateQueue = queue ?: dispatch_get_main_queue();
  gState.output = self;
  os_unfair_lock_unlock(&gState.lock);

  NSLog(@"[IrisInject] Captured delegate=%@ queue=%@", delegate, queue);
  [self iris_setSampleBufferDelegate:delegate queue:queue];
}

@end

// MARK: - AVCaptureMetadataOutput (IrisHook)

namespace {
/// What we can actually synthesise in the Simulator.
///
/// Vision is unavailable there — every request fails with "Could not create
/// inference context" (its ML models have no backing device), which rules out
/// VNDetectBarcodesRequest and the full symbology set. Core Image's older
/// CPU detectors do work, and they cover QR codes and faces. Advertise exactly
/// those: apps intersect their requested types against this list, so claiming
/// EAN/PDF417 here would just make them wait for callbacks that never come.
NSArray<AVMetadataObjectType> *MSCSupportedMetadataTypes(void) {
  static NSArray<AVMetadataObjectType> *types = nil;
  static dispatch_once_t onceToken;
  dispatch_once(&onceToken, ^{
    types = @[ AVMetadataObjectTypeQRCode, AVMetadataObjectTypeFace ];
  });
  return types;
}

/// CIDetector construction is expensive; -featuresInImage: is thread-safe, so
/// one detector per type is reused for the life of the process.
CIDetector *MSCDetector(NSString *type) {
  static NSMutableDictionary<NSString *, CIDetector *> *detectors = nil;
  static os_unfair_lock detectorLock = OS_UNFAIR_LOCK_INIT;
  static dispatch_once_t onceToken;
  dispatch_once(&onceToken, ^{
    detectors = [NSMutableDictionary dictionary];
  });

  os_unfair_lock_lock(&detectorLock);
  CIDetector *detector = detectors[type];
  if (!detector) {
    detector = [CIDetector
        detectorOfType:type
               context:nil
               options:@{CIDetectorAccuracy : CIDetectorAccuracyHigh}];
    if (detector)
      detectors[type] = detector;
  }
  os_unfair_lock_unlock(&detectorLock);
  return detector;
}

/// Core Image works in pixels with a bottom-left origin; AVMetadataObject uses
/// a 0-1 space with a top-left origin.
CGRect MSCNormalisedBounds(CGRect rect, CGRect extent) {
  if (CGRectIsEmpty(extent))
    return CGRectZero;
  return CGRectMake(rect.origin.x / extent.size.width,
                    1.0 - CGRectGetMaxY(rect) / extent.size.height,
                    rect.size.width / extent.size.width,
                    rect.size.height / extent.size.height);
}

NSDictionary *MSCNormalisedCorner(CGPoint point, CGRect extent) {
  if (CGRectIsEmpty(extent))
    return @{};
  CGPoint normalised = CGPointMake(point.x / extent.size.width,
                                   1.0 - point.y / extent.size.height);
  return (__bridge_transfer NSDictionary *)
      CGPointCreateDictionaryRepresentation(normalised);
}
} // namespace

@implementation AVCaptureMetadataOutput (IrisHook)

- (void)iris_setMetadataObjectsDelegate:
            (id<AVCaptureMetadataOutputObjectsDelegate>)delegate
                                  queue:(dispatch_queue_t)queue {
  os_unfair_lock_lock(&gState.lock);
  gState.metadataDelegate = delegate;
  gState.metadataDelegateQueue = queue ?: dispatch_get_main_queue();
  gState.metadataOutput = self;
  os_unfair_lock_unlock(&gState.lock);

  NSLog(@"[IrisInject] Captured metadata delegate=%@ queue=%@", delegate, queue);
  [self iris_setMetadataObjectsDelegate:delegate queue:queue];
  startDelivery();
}

- (NSArray<AVMetadataObjectType> *)iris_availableMetadataObjectTypes {
  return MSCSupportedMetadataTypes();
}

- (void)iris_setMetadataObjectTypes:(NSArray<AVMetadataObjectType> *)types {
  os_unfair_lock_lock(&gState.lock);
  gState.requestedMetadataTypes = [types copy];
  os_unfair_lock_unlock(&gState.lock);
  NSLog(@"[IrisInject] Metadata types requested: %lu (%@)",
        (unsigned long)types.count, [types componentsJoinedByString:@", "]);
  // Deliberately NOT forwarded: the real setter validates against its own
  // (empty) available list and raises NSInvalidArgumentException. The getter
  // below answers from what was recorded here instead.
}

- (NSArray<AVMetadataObjectType> *)iris_metadataObjectTypes {
  os_unfair_lock_lock(&gState.lock);
  NSArray<AVMetadataObjectType> *types = gState.requestedMetadataTypes;
  os_unfair_lock_unlock(&gState.lock);
  return types ?: @[];
}

@end

// Fix for AVCapturePhotoOutput crash on iOS Simulator
@interface AVCaptureOutput (IrisHookFix)
@end
@implementation AVCaptureOutput (IrisHookFix)
+ (NSArray *)availableVideoCodecTypesForSourceDevice:(id)arg1
                                        sourceFormat:(id)arg2
                                    outputDimensions:(CMVideoDimensions)arg3
                                            fileType:(id)arg4
                            videoCodecTypesAllowList:(id)arg5 {
  return @[];
}
@end

// MARK: - AVCaptureDevice (IrisHook)

@implementation AVCaptureDevice (IrisHook)

+ (nullable AVCaptureDevice *)iris_defaultDeviceWithMediaType:
    (AVMediaType)mediaType {
  if ([mediaType isEqualToString:AVMediaTypeVideo]) {
    static MSCFakeCaptureDevice *fakeDevice = nil;
    static dispatch_once_t onceToken;
    dispatch_once(&onceToken, ^{
      fakeDevice = (MSCFakeCaptureDevice *)class_createInstance(
          [MSCFakeCaptureDevice class], 0);
    });
    return fakeDevice;
  }
  return [self iris_defaultDeviceWithMediaType:mediaType];
}

+ (nullable AVCaptureDevice *)
    iris_defaultDeviceWithDeviceType:(AVCaptureDeviceType)deviceType
                           mediaType:(nullable AVMediaType)mediaType
                            position:(AVCaptureDevicePosition)position {
  if ([mediaType isEqualToString:AVMediaTypeVideo]) {
    static MSCFakeCaptureDevice *fakeDevice = nil;
    static dispatch_once_t onceToken;
    dispatch_once(&onceToken, ^{
      fakeDevice = (MSCFakeCaptureDevice *)class_createInstance(
          [MSCFakeCaptureDevice class], 0);
    });
    return fakeDevice;
  }
  return [self iris_defaultDeviceWithDeviceType:deviceType
                                      mediaType:mediaType
                                       position:position];
}

+ (AVAuthorizationStatus)iris_authorizationStatusForMediaType:
    (AVMediaType)mediaType {
  if ([mediaType isEqualToString:AVMediaTypeVideo]) {
    return AVAuthorizationStatusAuthorized;
  }
  return [self iris_authorizationStatusForMediaType:mediaType];
}

+ (void)iris_requestAccessForMediaType:(AVMediaType)mediaType
                     completionHandler:(void (^)(BOOL granted))handler {
  if ([mediaType isEqualToString:AVMediaTypeVideo]) {
    if (handler) {
      dispatch_async(dispatch_get_main_queue(), ^{
        handler(YES);
      });
    }
    return;
  }
  [self iris_requestAccessForMediaType:mediaType completionHandler:handler];
}

@end

// MARK: - AVCaptureDeviceDiscoverySession (IrisHook)

@implementation AVCaptureDeviceDiscoverySession (IrisHook)

- (NSArray<AVCaptureDevice *> *)iris_devices {
  static MSCFakeCaptureDevice *fakeDevice = nil;
  static dispatch_once_t onceToken;
  dispatch_once(&onceToken, ^{
    fakeDevice = (MSCFakeCaptureDevice *)class_createInstance(
        [MSCFakeCaptureDevice class], 0);
  });
  return @[ fakeDevice ];
}

@end

// MARK: - AVCaptureDeviceInput (IrisHook)

@implementation AVCaptureDeviceInput (IrisHook)

+ (instancetype)iris_deviceInputWithDevice:(AVCaptureDevice *)device
                                     error:(NSError **)outError {
  if ([device isKindOfClass:[MSCFakeCaptureDevice class]]) {
    static MSCFakeCaptureInput *fakeInput = nil;
    static dispatch_once_t onceToken;
    dispatch_once(&onceToken, ^{
      fakeInput = (MSCFakeCaptureInput *)class_createInstance(
          [MSCFakeCaptureInput class], 0);
    });
    return fakeInput;
  }
  return [self iris_deviceInputWithDevice:device error:outError];
}

- (instancetype)init_irisWithDevice:(AVCaptureDevice *)device
                              error:(NSError **)outError {
  if ([device isKindOfClass:[MSCFakeCaptureDevice class]]) {
    static MSCFakeCaptureInput *fakeInput = nil;
    static dispatch_once_t onceToken;
    dispatch_once(&onceToken, ^{
      fakeInput = (MSCFakeCaptureInput *)class_createInstance(
          [MSCFakeCaptureInput class], 0);
    });
    return fakeInput; // Return directly as ARC handles +1/-1 lifecycle
  }
  return [self init_irisWithDevice:device error:outError];
}

@end

// MARK: - AVCaptureVideoPreviewLayer (IrisHook)

@implementation AVCaptureVideoPreviewLayer (IrisHook)

- (instancetype)init_irisWithSession:(AVCaptureSession *)session {
  id instance = [self init_irisWithSession:session];
  if (instance) {
    os_unfair_lock_lock(&gState.lock);
    [gState.previewLayers addObject:instance];
    os_unfair_lock_unlock(&gState.lock);

    ((AVCaptureVideoPreviewLayer *)instance).contentsGravity =
        kCAGravityResizeAspectFill;
    startDelivery();
    MSCAutoStartSessionIfNeeded(session);
  }
  return instance;
}

- (instancetype)init_irisWithSessionWithNoConnection:
    (AVCaptureSession *)session {
  id instance = [self init_irisWithSessionWithNoConnection:session];
  if (instance) {
    os_unfair_lock_lock(&gState.lock);
    [gState.previewLayers addObject:instance];
    os_unfair_lock_unlock(&gState.lock);

    ((AVCaptureVideoPreviewLayer *)instance).contentsGravity =
        kCAGravityResizeAspectFill;
    startDelivery();
    MSCAutoStartSessionIfNeeded(session);
  }
  return instance;
}

- (void)iris_setSession:(AVCaptureSession *)session {
  os_unfair_lock_lock(&gState.lock);
  if (session) {
    [gState.previewLayers addObject:self];
  } else {
    [gState.previewLayers removeObject:self];
  }
  os_unfair_lock_unlock(&gState.lock);

  self.contentsGravity = kCAGravityResizeAspectFill;

  if (session) {
    startDelivery();
    MSCAutoStartSessionIfNeeded(session);
  }

  [self iris_setSession:session];
}

@end

// MARK: - Delivery Engine

namespace {
void startDelivery(void) {
  os_unfair_lock_lock(&gState.lock);
  if (gState.running) {
    os_unfair_lock_unlock(&gState.lock);
    return;
  }
  gState.running = true;
  os_unfair_lock_unlock(&gState.lock);

  uint64_t intervalNs = 1'000'000'000ULL / (uint64_t)gState.fps;
  uint64_t leewayNs = intervalNs / 10;

  gState.deliveryTimer = dispatch_source_create(DISPATCH_SOURCE_TYPE_TIMER, 0,
                                                0, gState.deliveryQueue);
  dispatch_source_set_timer(gState.deliveryTimer, DISPATCH_TIME_NOW, intervalNs,
                            leewayNs);
  dispatch_source_set_event_handler(gState.deliveryTimer, ^{
    deliverFrame();
  });
  dispatch_resume(gState.deliveryTimer);
  NSLog(@"[IrisInject] Delivery started @ %d fps", gState.fps);
}

void stopDelivery(void) {
  os_unfair_lock_lock(&gState.lock);
  if (!gState.running) {
    os_unfair_lock_unlock(&gState.lock);
    return;
  }
  gState.running = false;
  dispatch_source_t t = gState.deliveryTimer;
  gState.deliveryTimer = nullptr;
  os_unfair_lock_unlock(&gState.lock);

  if (t) {
    dispatch_source_cancel(t);
  }
  NSLog(@"[IrisInject] Delivery stopped");
}

/// Runs Core Image detectors over an injected frame and hands the results to
/// the app's AVCaptureMetadataOutput delegate as synthesised metadata objects.
///
/// Throttled and single-flight: frames arrive at up to 120fps while detection
/// costs milliseconds each, and nothing downstream benefits from scanning every
/// one.
void runRecognition(CVPixelBufferRef pixelBuffer, CMTime time) {
  os_unfair_lock_lock(&gState.lock);
  id<AVCaptureMetadataOutputObjectsDelegate> delegate = gState.metadataDelegate;
  dispatch_queue_t delegateQueue = gState.metadataDelegateQueue;
  AVCaptureMetadataOutput *output = gState.metadataOutput;
  NSArray<AVMetadataObjectType> *requested = gState.requestedMetadataTypes;
  bool inFlight = gState.recognitionInFlight;
  CFAbsoluteTime last = gState.lastRecognition;
  dispatch_queue_t queue = gState.recognitionQueue;
  os_unfair_lock_unlock(&gState.lock);

  if (!delegate || !delegateQueue || !queue || requested.count == 0)
    return;
  if (inFlight)
    return;

  CFAbsoluteTime now = CFAbsoluteTimeGetCurrent();
  if (now - last < IRIS_RECOGNITION_INTERVAL)
    return;

  bool wantsCodes = [requested containsObject:AVMetadataObjectTypeQRCode];
  bool wantsFaces = [requested containsObject:AVMetadataObjectTypeFace];
  if (!wantsCodes && !wantsFaces)
    return;

  os_unfair_lock_lock(&gState.lock);
  gState.recognitionInFlight = true;
  gState.lastRecognition = now;
  os_unfair_lock_unlock(&gState.lock);

  CVPixelBufferRetain(pixelBuffer);
  dispatch_async(queue, ^{
    NSMutableArray<AVMetadataObject *> *objects = [NSMutableArray array];

    @autoreleasepool {
      CIImage *image = [CIImage imageWithCVPixelBuffer:pixelBuffer];
      CGRect extent = image.extent;

      if (wantsCodes) {
        CIDetector *detector = MSCDetector(CIDetectorTypeQRCode);
        for (CIFeature *feature in [detector featuresInImage:image]) {
          if (![feature isKindOfClass:[CIQRCodeFeature class]])
            continue;
          CIQRCodeFeature *code = (CIQRCodeFeature *)feature;
          if (code.messageString.length == 0)
            continue;

          NSArray<NSDictionary *> *corners = @[
            MSCNormalisedCorner(code.topLeft, extent),
            MSCNormalisedCorner(code.topRight, extent),
            MSCNormalisedCorner(code.bottomRight, extent),
            MSCNormalisedCorner(code.bottomLeft, extent),
          ];
          MSCFakeMachineReadableCodeObject *object =
              [MSCFakeMachineReadableCodeObject
                  objectWithType:AVMetadataObjectTypeQRCode
                     stringValue:code.messageString
                          bounds:MSCNormalisedBounds(code.bounds, extent)
                         corners:corners
                            time:time];
          if (object)
            [objects addObject:object];
        }
      }

      if (wantsFaces) {
        CIDetector *detector = MSCDetector(CIDetectorTypeFace);
        NSInteger faceID = 1;
        for (CIFeature *feature in [detector featuresInImage:image]) {
          if (![feature isKindOfClass:[CIFaceFeature class]])
            continue;
          MSCFakeFaceObject *object = [MSCFakeFaceObject
              objectWithFaceID:faceID++
                        bounds:MSCNormalisedBounds(feature.bounds, extent)
                          time:time];
          if (object)
            [objects addObject:object];
        }
      }
    }

    CVPixelBufferRelease(pixelBuffer);

    os_unfair_lock_lock(&gState.lock);
    gState.recognitionInFlight = false;
    os_unfair_lock_unlock(&gState.lock);

    if (objects.count == 0)
      return;

    NSLog(@"[IrisInject] Recognised %lu metadata object(s)",
          (unsigned long)objects.count);

    dispatch_async(delegateQueue, ^{
      if ([delegate respondsToSelector:@selector(captureOutput:
                                       didOutputMetadataObjects:fromConnection:)]) {
        AVCaptureConnection *connection = output.connections.firstObject;
        if (!connection) {
          static MSCFakeConnection *fakeConnection = nil;
          static dispatch_once_t onceToken;
          dispatch_once(&onceToken, ^{
            fakeConnection = (MSCFakeConnection *)class_createInstance(
                [MSCFakeConnection class], 0);
          });
          connection = fakeConnection;
        }
        [delegate captureOutput:output
            didOutputMetadataObjects:objects
                      fromConnection:connection];
      }
    });
  });
}

void deliverFrame(void) {
  if (!gState.reader)
    return;

  if (!gState.reader->isOpen()) {
    if (!gState.reader->open()) {
      return;
    }
  }

  CMSampleBufferRef rawBuf =
      [gState.factory sampleBufferFromReader:gState.reader];
  if (!rawBuf)
    return;

  id arcSampleBuf = CFBridgingRelease(rawBuf); // ARC now owns the sample buffer

  CVPixelBufferRef pixelBuffer =
      CMSampleBufferGetImageBuffer((__bridge CMSampleBufferRef)arcSampleBuf);
  if (pixelBuffer) {
    runRecognition(pixelBuffer,
                   CMSampleBufferGetPresentationTimeStamp(
                       (__bridge CMSampleBufferRef)arcSampleBuf));

    CGImageRef cgImage = NULL;
    if (VTCreateCGImageFromCVPixelBuffer(pixelBuffer, NULL, &cgImage) ==
            noErr &&
        cgImage) {
      id arcImage = (__bridge_transfer id)cgImage;

      os_unfair_lock_lock(&gState.lock);
      NSArray *layers = [gState.previewLayers allObjects];
      os_unfair_lock_unlock(&gState.lock);

      if (layers.count > 0) {
        dispatch_async(dispatch_get_main_queue(), ^{
          for (AVCaptureVideoPreviewLayer *layer in layers) {
            layer.contents = arcImage;
          }
        });
      }
    }
  }

  os_unfair_lock_lock(&gState.lock);
  id<AVCaptureVideoDataOutputSampleBufferDelegate> del = gState.delegate;
  dispatch_queue_t q = gState.delegateQueue;
  AVCaptureVideoDataOutput *outObj = gState.output;
  os_unfair_lock_unlock(&gState.lock);

  if (!del || !q)
    return;

  dispatch_async(q, ^{
    if ([del respondsToSelector:@selector(captureOutput:
                                    didOutputSampleBuffer:fromConnection:)]) {
      CMSampleBufferRef sampleBuf = (__bridge CMSampleBufferRef)arcSampleBuf;
      AVCaptureConnection *conn = outObj.connections.firstObject;
      if (!conn) {
        static MSCFakeConnection *fakeConn = nil;
        static dispatch_once_t onceToken;
        dispatch_once(&onceToken, ^{
          fakeConn = (MSCFakeConnection *)class_createInstance(
              [MSCFakeConnection class], 0);
        });
        conn = fakeConn;
      }
      [del captureOutput:outObj
          didOutputSampleBuffer:sampleBuf
                 fromConnection:conn];
    }
  });
}
} // namespace

// MARK: - Lifecycle Setup

void MSCInstallHooks(SharedFrameReader *reader, int32_t fps) {
  gState.deliveryQueue =
      dispatch_queue_create("com.iris.delivery", DISPATCH_QUEUE_SERIAL);
  gState.recognitionQueue =
      dispatch_queue_create("com.iris.recognition", DISPATCH_QUEUE_SERIAL);
  gState.reader = reader;
  gState.fps = fps;
  gState.factory = [[MSCSampleBufferFactory alloc] initWithFPS:fps];
  gState.previewLayers = [NSHashTable weakObjectsHashTable];

  swizzleInstance([AVCaptureSession class], @selector(startRunning),
                  @selector(iris_startRunning));
  swizzleInstance([AVCaptureSession class], @selector(stopRunning),
                  @selector(iris_stopRunning));
  swizzleInstance([AVCaptureSession class], @selector(canAddInput:),
                  @selector(iris_canAddInput:));
  swizzleInstance([AVCaptureSession class], @selector(addInput:),
                  @selector(iris_addInput:));
  swizzleInstance([AVCaptureSession class], @selector(canAddOutput:),
                  @selector(iris_canAddOutput:));
  swizzleInstance([AVCaptureSession class], @selector(addOutput:),
                  @selector(iris_addOutput:));
  swizzleInstance([AVCaptureSession class], @selector(isRunning),
                  @selector(iris_isRunning));

  swizzleInstance([AVCaptureVideoDataOutput class],
                  @selector(setSampleBufferDelegate:queue:),
                  @selector(iris_setSampleBufferDelegate:queue:));

  swizzleInstance([AVCaptureMetadataOutput class],
                  @selector(setMetadataObjectsDelegate:queue:),
                  @selector(iris_setMetadataObjectsDelegate:queue:));
  swizzleInstance([AVCaptureMetadataOutput class],
                  @selector(availableMetadataObjectTypes),
                  @selector(iris_availableMetadataObjectTypes));
  swizzleInstance([AVCaptureMetadataOutput class],
                  @selector(setMetadataObjectTypes:),
                  @selector(iris_setMetadataObjectTypes:));
  swizzleInstance([AVCaptureMetadataOutput class],
                  @selector(metadataObjectTypes),
                  @selector(iris_metadataObjectTypes));

  Class layerCls = NSClassFromString(@"AVCaptureVideoPreviewLayer");
  if (layerCls) {
    swizzleInstance(layerCls, @selector(setSession:),
                    @selector(iris_setSession:));
    swizzleInstance(layerCls, @selector(initWithSession:),
                    @selector(init_irisWithSession:));
    swizzleInstance(layerCls, @selector(initWithSessionWithNoConnection:),
                    @selector(init_irisWithSessionWithNoConnection:));
  }

  swizzleClass([AVCaptureDevice class], @selector(defaultDeviceWithMediaType:),
               @selector(iris_defaultDeviceWithMediaType:));
  swizzleClass([AVCaptureDevice class],
               @selector(defaultDeviceWithDeviceType:mediaType:position:),
               @selector(iris_defaultDeviceWithDeviceType:mediaType:position:));
  swizzleClass([AVCaptureDevice class],
               @selector(authorizationStatusForMediaType:),
               @selector(iris_authorizationStatusForMediaType:));
  swizzleClass([AVCaptureDevice class],
               @selector(requestAccessForMediaType:completionHandler:),
               @selector(iris_requestAccessForMediaType:completionHandler:));

  swizzleClass([AVCaptureDeviceInput class],
               @selector(deviceInputWithDevice:error:),
               @selector(iris_deviceInputWithDevice:error:));
  swizzleInstance([AVCaptureDeviceInput class],
                  @selector(initWithDevice:error:),
                  @selector(init_irisWithDevice:error:));

  Class discoverySessionCls =
      NSClassFromString(@"AVCaptureDeviceDiscoverySession");
  if (discoverySessionCls) {
    swizzleInstance(discoverySessionCls, @selector(devices),
                    @selector(iris_devices));
  }

  NSLog(@"[IrisInject] Hooks installed — fps=%d", fps);
}

void MSCUninstallHooks(void) { stopDelivery(); }
