// CaptureHooks.mm
// AVFoundation method swizzles that intercept just enough for the
// AVCaptureSession + AVCaptureVideoDataOutput path.
//
// Intercepted selectors:
//   -[AVCaptureSession startRunning]
//   -[AVCaptureSession stopRunning]
//   -[AVCaptureVideoDataOutput setSampleBufferDelegate:queue:]
//   +[AVCaptureDevice defaultDeviceWithMediaType:]
//   -[AVCaptureSession canAddInput:] (allow synthetic sessions)
//   -[AVCaptureSession addInput:]    (no-op for sessions without hardware)

#import <AVFoundation/AVFoundation.h>
#import <QuartzCore/QuartzCore.h>
#import <VideoToolbox/VideoToolbox.h>
#import <objc/runtime.h>
#import <dispatch/dispatch.h>
#import "CaptureHooks.h"
#import "SampleBufferFactory.h"
#import "SharedFrameReader.hpp"

@interface MSCFakeConnection : AVCaptureConnection
@end
@implementation MSCFakeConnection
@end

// ---------------------------------------------------------------------------
// Delivery state (process-global, guarded by a lock)
// ---------------------------------------------------------------------------

static dispatch_queue_t      gDeliveryQueue;
static dispatch_source_t     gDeliveryTimer;
static SharedFrameReader*    gReader;
static MSCSampleBufferFactory* gFactory;
static id<AVCaptureVideoDataOutputSampleBufferDelegate> gDelegate;
static dispatch_queue_t      gDelegateQueue;
static BOOL                  gRunning = NO;
// Tracks whether the app explicitly called startRunning on the AVCaptureSession.
// Used to avoid auto-starting the session when the app already did so, which
// would crash on Simulator by calling startRunning on an already-running session.
static BOOL                  gAVSessionStarted = NO;
static NSLock*               gLock;
static int32_t               gFPS = 30;
static NSHashTable<AVCaptureVideoPreviewLayer *> *gPreviewLayers;

// ---------------------------------------------------------------------------
// Forward declarations
// ---------------------------------------------------------------------------
static void startDelivery(void);
static void stopDelivery(void);
static void deliverFrame(void);

// ---------------------------------------------------------------------------
// Swizzle helpers
// ---------------------------------------------------------------------------

static void swizzleInstance(Class cls, SEL orig, SEL repl) {
    Method origMethod = class_getInstanceMethod(cls, orig);
    Method replMethod = class_getInstanceMethod(cls, repl);
    if (!origMethod || !replMethod) {
        NSLog(@"[IrisInject] ⚠️  Cannot swizzle %@.%@", NSStringFromClass(cls), NSStringFromSelector(orig));
        return;
    }
    method_exchangeImplementations(origMethod, replMethod);
}

static void swizzleClass(Class cls, SEL orig, SEL repl) {
    Class meta = object_getClass((id)cls);
    Method origMethod = class_getInstanceMethod(meta, orig);
    Method replMethod = class_getInstanceMethod(meta, repl);
    if (!origMethod || !replMethod) {
        NSLog(@"[IrisInject] ⚠️  Cannot swizzle class method %@.%@", NSStringFromClass(cls), NSStringFromSelector(orig));
        return;
    }
    method_exchangeImplementations(origMethod, replMethod);
}

// ---------------------------------------------------------------------------
// AVCaptureSession category — swizzled methods
// ---------------------------------------------------------------------------

@implementation AVCaptureSession (IrisHook)

- (void)msc_startRunning {
    NSLog(@"[IrisInject] AVCaptureSession startRunning intercepted");
    [gLock lock];
    gAVSessionStarted = YES;
    [gLock unlock];
    [self msc_startRunning]; // call original (if any)
    // gReader is initialised in EntryPoint — do not recreate here.
    startDelivery();
}

- (void)msc_stopRunning {
    [self msc_stopRunning];
    stopDelivery();
}

- (BOOL)msc_canAddInput:(AVCaptureInput *)input {
    return YES;
}

- (void)msc_addInput:(AVCaptureInput *)input {
    NSLog(@"[IrisInject] AVCaptureSession addInput: silenced (no hardware)");
}

- (BOOL)msc_canAddOutput:(AVCaptureOutput *)output {
    return YES;
}

- (void)msc_addOutput:(AVCaptureOutput *)output {
    [self msc_addOutput:output];
}

- (BOOL)msc_isRunning {
    [gLock lock];
    BOOL running = gRunning;
    [gLock unlock];
    return running;
}

@end

// ---------------------------------------------------------------------------
// AVCaptureVideoDataOutput category — swizzled methods
// ---------------------------------------------------------------------------

static AVCaptureVideoDataOutput* gOutput;

@implementation AVCaptureVideoDataOutput (IrisHook)

- (void)msc_setSampleBufferDelegate:(id<AVCaptureVideoDataOutputSampleBufferDelegate>)delegate
                              queue:(dispatch_queue_t)queue {
    [gLock lock];
    gDelegate      = delegate;
    gDelegateQueue = queue ?: dispatch_get_main_queue();
    gOutput        = self;
    [gLock unlock];

    NSLog(@"[IrisInject] captured delegate=%@ queue=%@", delegate, queue);
    // Also call original so AVFoundation internal state is consistent.
    [self msc_setSampleBufferDelegate:delegate queue:queue];
}

@end

// Fix for AVCapturePhotoOutput crash on iOS Simulator
@interface AVCaptureOutput (IrisHookFix)
@end
@implementation AVCaptureOutput (IrisHookFix)
+ (NSArray *)availableVideoCodecTypesForSourceDevice:(id)arg1 sourceFormat:(id)arg2 outputDimensions:(CMVideoDimensions)arg3 fileType:(id)arg4 videoCodecTypesAllowList:(id)arg5 {
    return @[]; // Return empty array to prevent crash
}
@end

// ---------------------------------------------------------------------------
// Fakes for AVCaptureDevice and AVCaptureDeviceInput
// ---------------------------------------------------------------------------

@interface MSCFakeCaptureDeviceFormat : NSObject
@end
@implementation MSCFakeCaptureDeviceFormat
- (CMFormatDescriptionRef)formatDescription {
    CMVideoFormatDescriptionRef formatDesc = NULL;
    CMVideoFormatDescriptionCreate(kCFAllocatorDefault, kCVPixelFormatType_32BGRA, 1280, 720, NULL, &formatDesc);
    return formatDesc;
}
@end

@interface MSCFakeCaptureDevice : AVCaptureDevice
@end
@implementation MSCFakeCaptureDevice
- (BOOL)hasMediaType:(AVMediaType)mediaType { return YES; }
- (BOOL)supportsAVCaptureSessionPreset:(AVCaptureSessionPreset)preset { return YES; }
- (BOOL)isFocusPointOfInterestSupported { return YES; }
- (BOOL)isFocusModeSupported:(AVCaptureFocusMode)focusMode { return YES; }
- (BOOL)isExposurePointOfInterestSupported { return YES; }
- (BOOL)isExposureModeSupported:(AVCaptureExposureMode)exposureMode { return YES; }
- (BOOL)lockForConfiguration:(NSError **)outError { return YES; }
- (void)unlockForConfiguration {}
- (void)setFocusPointOfInterest:(CGPoint)focusPointOfInterest {}
- (void)setFocusMode:(AVCaptureFocusMode)focusMode {}
- (void)setExposurePointOfInterest:(CGPoint)exposurePointOfInterest {}
- (void)setExposureMode:(AVCaptureExposureMode)exposureMode {}

// Basic device properties
- (NSString *)uniqueID { return @"MSCFakeCamera_001"; }
- (NSString *)localizedName { return @"Iris Fake Device"; }
- (AVCaptureDevicePosition)position { return AVCaptureDevicePositionBack; }
- (BOOL)isConnected { return YES; }
- (BOOL)isSuspended { return NO; }

// Active format mocking
- (CMVideoDimensions)activeSensorLocation { return (CMVideoDimensions){1280, 720}; }
- (void)setActiveVideoMinFrameDuration:(CMTime)activeVideoMinFrameDuration {}
- (CMTime)activeVideoMinFrameDuration { return CMTimeMake(1, 30); }
- (void)setActiveVideoMaxFrameDuration:(CMTime)activeVideoMaxFrameDuration {}
- (CMTime)activeVideoMaxFrameDuration { return CMTimeMake(1, 30); }

// Device type — needed so discovery-session filters (e.g. bestPossibleBackCamera)
// can match via their preferred-type loop rather than falling through to the last-resort .first.
- (AVCaptureDeviceType)deviceType { return AVCaptureDeviceTypeBuiltInWideAngleCamera; }

- (id)activeFormat {
    static MSCFakeCaptureDeviceFormat *fakeFormat = nil;
    static dispatch_once_t onceToken;
    dispatch_once(&onceToken, ^{
        fakeFormat = (MSCFakeCaptureDeviceFormat *)class_createInstance(NSClassFromString(@"MSCFakeCaptureDeviceFormat"), 0);
    });
    return fakeFormat;
}
@end

@interface MSCFakeCaptureInputPort : AVCaptureInputPort
@end
@implementation MSCFakeCaptureInputPort
- (AVMediaType)mediaType { return AVMediaTypeVideo; }
- (CMFormatDescriptionRef)formatDescription {
    // AVCaptureInputPort.formatDescription is declared as CMFormatDescriptionRef,
    // not id — return a real CF type to silence -Wmismatched-return-types.
    static CMVideoFormatDescriptionRef sFormatDesc = NULL;
    static dispatch_once_t onceToken;
    dispatch_once(&onceToken, ^{
        CMVideoFormatDescriptionCreate(
            kCFAllocatorDefault,
            kCVPixelFormatType_32BGRA,
            1280, 720,
            NULL,
            &sFormatDesc
        );
    });
    return sFormatDesc;
}
- (BOOL)isEnabled { return YES; }
@end

@interface MSCFakeCaptureInput : AVCaptureDeviceInput
@end
@implementation MSCFakeCaptureInput
- (AVCaptureDevice *)device {
    static MSCFakeCaptureDevice *fakeDevice = nil;
    static dispatch_once_t onceToken;
    dispatch_once(&onceToken, ^{
        fakeDevice = (MSCFakeCaptureDevice *)class_createInstance(NSClassFromString(@"MSCFakeCaptureDevice"), 0);
    });
    return fakeDevice;
}
- (NSArray<AVCaptureInputPort *> *)ports {
    static MSCFakeCaptureInputPort *fakePort = nil;
    static dispatch_once_t onceToken;
    dispatch_once(&onceToken, ^{
        fakePort = (MSCFakeCaptureInputPort *)class_createInstance(NSClassFromString(@"MSCFakeCaptureInputPort"), 0);
    });
    return @[fakePort];
}
@end

// ---------------------------------------------------------------------------
// AVCaptureDevice class-method swizzle
// ---------------------------------------------------------------------------

@implementation AVCaptureDevice (IrisHook)

+ (nullable AVCaptureDevice *)msc_defaultDeviceWithMediaType:(AVMediaType)mediaType {
    if ([mediaType isEqualToString:AVMediaTypeVideo]) {
        static MSCFakeCaptureDevice *fakeDevice = nil;
        static dispatch_once_t onceToken;
        dispatch_once(&onceToken, ^{
            fakeDevice = (MSCFakeCaptureDevice *)class_createInstance(NSClassFromString(@"MSCFakeCaptureDevice"), 0);
        });
        return fakeDevice;
    }
    return [self msc_defaultDeviceWithMediaType:mediaType];
}

+ (nullable AVCaptureDevice *)msc_defaultDeviceWithDeviceType:(AVCaptureDeviceType)deviceType mediaType:(nullable AVMediaType)mediaType position:(AVCaptureDevicePosition)position {
    if ([mediaType isEqualToString:AVMediaTypeVideo]) {
        static MSCFakeCaptureDevice *fakeDevice = nil;
        static dispatch_once_t onceToken;
        dispatch_once(&onceToken, ^{
            fakeDevice = (MSCFakeCaptureDevice *)class_createInstance(NSClassFromString(@"MSCFakeCaptureDevice"), 0);
        });
        return fakeDevice;
    }
    return [self msc_defaultDeviceWithDeviceType:deviceType mediaType:mediaType position:position];
}

+ (AVAuthorizationStatus)msc_authorizationStatusForMediaType:(AVMediaType)mediaType {
    if ([mediaType isEqualToString:AVMediaTypeVideo]) {
        return AVAuthorizationStatusAuthorized;
    }
    return [self msc_authorizationStatusForMediaType:mediaType];
}

+ (void)msc_requestAccessForMediaType:(AVMediaType)mediaType completionHandler:(void (^)(BOOL granted))handler {
    if ([mediaType isEqualToString:AVMediaTypeVideo]) {
        if (handler) {
            dispatch_async(dispatch_get_main_queue(), ^{
                handler(YES);
            });
        }
        return;
    }
    [self msc_requestAccessForMediaType:mediaType completionHandler:handler];
}

@end

// ---------------------------------------------------------------------------
// AVCaptureDeviceDiscoverySession swizzle
// ---------------------------------------------------------------------------

@implementation AVCaptureDeviceDiscoverySession (IrisHook)

- (NSArray<AVCaptureDevice *> *)msc_devices {
    static MSCFakeCaptureDevice *fakeDevice = nil;
    static dispatch_once_t onceToken;
    dispatch_once(&onceToken, ^{
        fakeDevice = (MSCFakeCaptureDevice *)class_createInstance(NSClassFromString(@"MSCFakeCaptureDevice"), 0);
    });
    return @[fakeDevice];
}

@end

// ---------------------------------------------------------------------------
// AVCaptureDeviceInput swizzle
// ---------------------------------------------------------------------------

@implementation AVCaptureDeviceInput (IrisHook)

+ (instancetype)msc_deviceInputWithDevice:(AVCaptureDevice *)device error:(NSError **)outError {
    if ([device isKindOfClass:NSClassFromString(@"MSCFakeCaptureDevice")]) {
        static MSCFakeCaptureInput *fakeInput = nil;
        static dispatch_once_t onceToken;
        dispatch_once(&onceToken, ^{
            fakeInput = (MSCFakeCaptureInput *)class_createInstance(NSClassFromString(@"MSCFakeCaptureInput"), 0);
        });
        return fakeInput;
    }
    return [self msc_deviceInputWithDevice:device error:outError];
}

- (instancetype)init_mscWithDevice:(AVCaptureDevice *)device error:(NSError **)outError {
    if ([device isKindOfClass:NSClassFromString(@"MSCFakeCaptureDevice")]) {
        static MSCFakeCaptureInput *fakeInput = nil;
        static dispatch_once_t onceToken;
        dispatch_once(&onceToken, ^{
            fakeInput = (MSCFakeCaptureInput *)class_createInstance(NSClassFromString(@"MSCFakeCaptureInput"), 0);
        });
        return (__bridge id)CFRetain((__bridge CFTypeRef)fakeInput);
    }
    return [self init_mscWithDevice:device error:outError];
}

@end

// Forward declaration
static void startDelivery(void);

// ---------------------------------------------------------------------------
// AVCaptureVideoPreviewLayer category — swizzled methods
// ---------------------------------------------------------------------------

@implementation AVCaptureVideoPreviewLayer (IrisHook)

- (instancetype)init_mscWithSession:(AVCaptureSession *)session {
    id instance = [self init_mscWithSession:session];
    if (instance) {
        [gLock lock];
        [gPreviewLayers addObject:instance];
        [gLock unlock];
        ((AVCaptureVideoPreviewLayer *)instance).contentsGravity = kCAGravityResizeAspectFill;
        // gReader is initialised in EntryPoint — do not recreate here.
        startDelivery();
        // Auto-start the session so AVCaptureVideoPreviewLayer activates its rendering
        // path. Delayed 500 ms so onAppear (and any explicit startRunning the app calls)
        // has already fired by the time the block runs. gAVSessionStarted gates the call
        // so apps that do call startRunning() are unaffected.
        if (session) {
            dispatch_after(dispatch_time(DISPATCH_TIME_NOW, (int64_t)(500 * NSEC_PER_MSEC)),
                           dispatch_get_global_queue(QOS_CLASS_USER_INITIATED, 0), ^{
                [gLock lock];
                BOOL alreadyStarted = gAVSessionStarted;
                [gLock unlock];
                if (!alreadyStarted) {
                    NSLog(@"[IrisInject] preview layer got session but app never called startRunning — auto-starting");
                    @try { [session startRunning]; }
                    @catch (NSException *e) {
                        NSLog(@"[IrisInject] auto-start failed: %@", e.reason);
                    }
                }
            });
        }
    }
    return instance;
}

- (instancetype)init_mscWithSessionWithNoConnection:(AVCaptureSession *)session {
    id instance = [self init_mscWithSessionWithNoConnection:session];
    if (instance) {
        [gLock lock];
        [gPreviewLayers addObject:instance];
        [gLock unlock];
        ((AVCaptureVideoPreviewLayer *)instance).contentsGravity = kCAGravityResizeAspectFill;
        // gReader is initialised in EntryPoint — do not recreate here.
        startDelivery();
        // Auto-start the session so AVCaptureVideoPreviewLayer activates its rendering
        // path. Delayed 500 ms so onAppear (and any explicit startRunning the app calls)
        // has already fired by the time the block runs. gAVSessionStarted gates the call
        // so apps that do call startRunning() are unaffected.
        if (session) {
            dispatch_after(dispatch_time(DISPATCH_TIME_NOW, (int64_t)(500 * NSEC_PER_MSEC)),
                           dispatch_get_global_queue(QOS_CLASS_USER_INITIATED, 0), ^{
                [gLock lock];
                BOOL alreadyStarted = gAVSessionStarted;
                [gLock unlock];
                if (!alreadyStarted) {
                    NSLog(@"[IrisInject] preview layer got session but app never called startRunning — auto-starting");
                    @try { [session startRunning]; }
                    @catch (NSException *e) {
                        NSLog(@"[IrisInject] auto-start failed: %@", e.reason);
                    }
                }
            });
        }
    }
    return instance;
}

- (void)msc_setSession:(AVCaptureSession *)session {
    [gLock lock];
    if (session) {
        [gPreviewLayers addObject:self];
    } else {
        [gPreviewLayers removeObject:self];
    }
    [gLock unlock];
    
    // Ensure the layer is flipped correctly for the generated CGImage
    self.contentsGravity = kCAGravityResizeAspectFill;
    
    if (session) {
        // gReader is initialised in EntryPoint — do not recreate here.
        startDelivery();
        // Auto-start the session so AVCaptureVideoPreviewLayer activates its rendering
        // path. Apps that configure inputs/outputs without calling startRunning() (e.g.
        // PermissionManager patterns) would otherwise show a frozen black frame because
        // the preview layer's internal renderer stays dormant until the session runs.
        // Delayed 500 ms so the app's own onAppear/startRunning (if any) fires first,
        // preventing a double-start crash on Simulator. @try/@catch for safety.
        dispatch_after(dispatch_time(DISPATCH_TIME_NOW, (int64_t)(500 * NSEC_PER_MSEC)),
                       dispatch_get_global_queue(QOS_CLASS_USER_INITIATED, 0), ^{
            [gLock lock];
            BOOL alreadyStarted = gAVSessionStarted;
            [gLock unlock];
            if (!alreadyStarted) {
                NSLog(@"[IrisInject] preview layer got session but app never called startRunning — auto-starting");
                @try { [session startRunning]; }
                @catch (NSException *e) {
                    NSLog(@"[IrisInject] auto-start failed: %@", e.reason);
                }
            }
        });
    }
    
    [self msc_setSession:session];
}

@end

// ---------------------------------------------------------------------------
// Delivery engine
// ---------------------------------------------------------------------------

static void startDelivery(void) {
    [gLock lock];
    if (gRunning) { [gLock unlock]; return; }
    gRunning = YES;
    [gLock unlock];

    uint64_t intervalNs = 1'000'000'000ULL / (uint64_t)gFPS;
    uint64_t leewayNs   = intervalNs / 10;

    gDeliveryTimer = dispatch_source_create(DISPATCH_SOURCE_TYPE_TIMER, 0, 0, gDeliveryQueue);
    dispatch_source_set_timer(
        gDeliveryTimer,
        DISPATCH_TIME_NOW,
        intervalNs,
        leewayNs
    );
    dispatch_source_set_event_handler(gDeliveryTimer, ^{ deliverFrame(); });
    dispatch_resume(gDeliveryTimer);
    NSLog(@"[IrisInject] delivery started @ %d fps", gFPS);
}

static void stopDelivery(void) {
    [gLock lock];
    if (!gRunning) { [gLock unlock]; return; }
    gRunning = NO;
    dispatch_source_t t = gDeliveryTimer;
    gDeliveryTimer = nullptr;
    [gLock unlock];

    if (t) dispatch_source_cancel(t);
    NSLog(@"[IrisInject] delivery stopped");
}

static void deliverFrame(void) {
    if (!gReader) return;

    if (!gReader->isOpen()) {
        if (!gReader->open()) {
            return; // Not created yet
        }
    }

    // Optimised single-copy path: factory reads from shm directly into CVPixelBuffer.
    // Factory returns a +1 CF reference; transfer ownership to ARC immediately
    // so it stays alive across the async dispatch below.
    CMSampleBufferRef rawBuf = [gFactory sampleBufferFromReader:gReader];
    if (!rawBuf) return;
    id arcSampleBuf = CFBridgingRelease(rawBuf); // ARC now owns it

    // --- Update preview layers ---
    CVPixelBufferRef pixelBuffer = CMSampleBufferGetImageBuffer((__bridge CMSampleBufferRef)arcSampleBuf);
    if (pixelBuffer) {
        CGImageRef cgImage = NULL;
        if (VTCreateCGImageFromCVPixelBuffer(pixelBuffer, NULL, &cgImage) == noErr && cgImage) {
            id arcImage = (__bridge_transfer id)cgImage;
            [gLock lock];
            NSArray *layers = [gPreviewLayers allObjects];
            [gLock unlock];
            
            if (layers.count > 0) {
                dispatch_async(dispatch_get_main_queue(), ^{
                    for (AVCaptureVideoPreviewLayer *layer in layers) {
                        layer.contents = arcImage;
                    }
                });
            }
        }
    }

    [gLock lock];
    id<AVCaptureVideoDataOutputSampleBufferDelegate> del = gDelegate;
    dispatch_queue_t q = gDelegateQueue;
    AVCaptureVideoDataOutput *outObj = gOutput;
    [gLock unlock];

    if (!del || !q) return;

    dispatch_async(q, ^{
        if ([del respondsToSelector:@selector(captureOutput:didOutputSampleBuffer:fromConnection:)]) {
#pragma clang diagnostic push
#pragma clang diagnostic ignored "-Wnonnull"
            // ARC captures arcSampleBuf, keeping the buffer alive for this block.
            CMSampleBufferRef sampleBuf = (__bridge CMSampleBufferRef)arcSampleBuf;
            AVCaptureConnection *conn = outObj.connections.firstObject;
            if (!conn) {
                static AVCaptureConnection *fakeConn = nil;
                static dispatch_once_t onceToken;
                dispatch_once(&onceToken, ^{
                    fakeConn = (AVCaptureConnection *)class_createInstance(NSClassFromString(@"MSCFakeConnection"), 0);
                });
                conn = fakeConn;
            }
            [del captureOutput:outObj
           didOutputSampleBuffer:sampleBuf
                  fromConnection:conn];
#pragma clang diagnostic pop
        }
    });
}

// ---------------------------------------------------------------------------
// Install / uninstall (called from EntryPoint)
// ---------------------------------------------------------------------------

void MSCInstallHooks(SharedFrameReader* reader, int32_t fps) {
    gLock          = [NSLock new];
    gDeliveryQueue = dispatch_queue_create("com.iris.delivery", DISPATCH_QUEUE_SERIAL);
    gReader        = reader;
    gFPS           = fps;
    gFactory       = [[MSCSampleBufferFactory alloc] initWithFPS:fps];
    gPreviewLayers = [NSHashTable weakObjectsHashTable];

    // AVCaptureSession
    swizzleInstance([AVCaptureSession class],
                    @selector(startRunning), @selector(msc_startRunning));
    swizzleInstance([AVCaptureSession class],
                    @selector(stopRunning),  @selector(msc_stopRunning));
    swizzleInstance([AVCaptureSession class],
                    @selector(canAddInput:), @selector(msc_canAddInput:));
    swizzleInstance([AVCaptureSession class],
                    @selector(addInput:),    @selector(msc_addInput:));
    swizzleInstance([AVCaptureSession class],
                    @selector(canAddOutput:), @selector(msc_canAddOutput:));
    swizzleInstance([AVCaptureSession class],
                    @selector(addOutput:),    @selector(msc_addOutput:));
    swizzleInstance([AVCaptureSession class],
                    @selector(isRunning),     @selector(msc_isRunning));

    // AVCaptureVideoDataOutput
    swizzleInstance([AVCaptureVideoDataOutput class],
                    @selector(setSampleBufferDelegate:queue:),
                    @selector(msc_setSampleBufferDelegate:queue:));

    // AVCaptureVideoPreviewLayer
    Class layerCls = NSClassFromString(@"AVCaptureVideoPreviewLayer");
    if (layerCls) {
        Method m1 = class_getInstanceMethod(layerCls, @selector(setSession:));
        Method m2 = class_getInstanceMethod(layerCls, @selector(msc_setSession:));
        if (m1 && m2) method_exchangeImplementations(m1, m2);

        Method m3 = class_getInstanceMethod(layerCls, @selector(initWithSession:));
        Method m4 = class_getInstanceMethod(layerCls, @selector(init_mscWithSession:));
        if (m3 && m4) method_exchangeImplementations(m3, m4);

        Method m5 = class_getInstanceMethod(layerCls, @selector(initWithSessionWithNoConnection:));
        Method m6 = class_getInstanceMethod(layerCls, @selector(init_mscWithSessionWithNoConnection:));
        if (m5 && m6) method_exchangeImplementations(m5, m6);
    }

    // AVCaptureDevice (class-level)
    swizzleClass([AVCaptureDevice class],
                 @selector(defaultDeviceWithMediaType:),
                 @selector(msc_defaultDeviceWithMediaType:));
    swizzleClass([AVCaptureDevice class],
                 @selector(defaultDeviceWithDeviceType:mediaType:position:),
                 @selector(msc_defaultDeviceWithDeviceType:mediaType:position:));
    swizzleClass([AVCaptureDevice class],
                 @selector(authorizationStatusForMediaType:),
                 @selector(msc_authorizationStatusForMediaType:));
    swizzleClass([AVCaptureDevice class],
                 @selector(requestAccessForMediaType:completionHandler:),
                 @selector(msc_requestAccessForMediaType:completionHandler:));

    // AVCaptureDeviceInput
    swizzleClass([AVCaptureDeviceInput class],
                 @selector(deviceInputWithDevice:error:),
                 @selector(msc_deviceInputWithDevice:error:));
    swizzleInstance([AVCaptureDeviceInput class],
                 @selector(initWithDevice:error:),
                 @selector(init_mscWithDevice:error:));

    // AVCaptureDeviceDiscoverySession
    Class discoverySessionCls = NSClassFromString(@"AVCaptureDeviceDiscoverySession");
    if (discoverySessionCls) {
        swizzleInstance(discoverySessionCls,
                     @selector(devices),
                     @selector(msc_devices));
    }

    NSLog(@"[IrisInject] hooks installed — fps=%d", fps);
}

void MSCUninstallHooks(void) {
    stopDelivery();
    // Note: method_exchangeImplementations is idempotent for paired calls,
    // but we cannot easily "un-swizzle" without re-exchanging.
    // In practice, the process exits when the app is terminated.
}
