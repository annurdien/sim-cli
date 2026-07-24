// CaptureHooks.mm
#import <AVFoundation/AVFoundation.h>
#import <QuartzCore/QuartzCore.h>
#import <VideoToolbox/VideoToolbox.h>
#import <objc/runtime.h>
#import <dispatch/dispatch.h>
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
        SharedFrameReader* reader = nullptr;
        MSCSampleBufferFactory* factory = nullptr;
        
        id<AVCaptureVideoDataOutputSampleBufferDelegate> delegate = nil;
        dispatch_queue_t delegateQueue = nullptr;
        AVCaptureVideoDataOutput* output = nil;
        
        NSHashTable<AVCaptureVideoPreviewLayer *>* previewLayers = nil;
        
        int32_t fps = 30;
        bool running = false;
        bool avSessionStarted = false;
    };

    InjectorState gState;

    void startDelivery(void);
    void stopDelivery(void);
    void deliverFrame(void);

    void MSCAutoStartSessionIfNeeded(AVCaptureSession *session) {
        if (!session) return;
        
        dispatch_after(dispatch_time(DISPATCH_TIME_NOW, (int64_t)(500 * NSEC_PER_MSEC)),
                       dispatch_get_global_queue(QOS_CLASS_USER_INITIATED, 0), ^{
            os_unfair_lock_lock(&gState.lock);
            bool alreadyStarted = gState.avSessionStarted;
            os_unfair_lock_unlock(&gState.lock);
            
            if (!alreadyStarted) {
                NSLog(@"[IrisInject] Auto-starting AVCaptureSession");
                @try { [session startRunning]; }
                @catch (NSException *e) {
                    NSLog(@"[IrisInject] Auto-start failed: %@", e.reason);
                }
            }
        });
    }

    void swizzleInstance(Class cls, SEL orig, SEL repl) {
        Method origMethod = class_getInstanceMethod(cls, orig);
        Method replMethod = class_getInstanceMethod(cls, repl);
        if (!origMethod || !replMethod) {
            NSLog(@"[IrisInject] ⚠️ Cannot swizzle %@.%@", NSStringFromClass(cls), NSStringFromSelector(orig));
            return;
        }
        method_exchangeImplementations(origMethod, replMethod);
    }

    void swizzleClass(Class cls, SEL orig, SEL repl) {
        Class meta = object_getClass((id)cls);
        Method origMethod = class_getInstanceMethod(meta, orig);
        Method replMethod = class_getInstanceMethod(meta, repl);
        if (!origMethod || !replMethod) {
            NSLog(@"[IrisInject] ⚠️ Cannot swizzle class method %@.%@", NSStringFromClass(cls), NSStringFromSelector(orig));
            return;
        }
        method_exchangeImplementations(origMethod, replMethod);
    }
}

// MARK: - AVCaptureSession (IrisHook)

@implementation AVCaptureSession (IrisHook)

- (void)msc_startRunning {
    NSLog(@"[IrisInject] AVCaptureSession startRunning intercepted");
    os_unfair_lock_lock(&gState.lock);
    gState.avSessionStarted = true;
    os_unfair_lock_unlock(&gState.lock);
    
    [self msc_startRunning];
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
    os_unfair_lock_lock(&gState.lock);
    BOOL isRunning = gState.running;
    os_unfair_lock_unlock(&gState.lock);
    return isRunning;
}

@end

// MARK: - AVCaptureVideoDataOutput (IrisHook)

@implementation AVCaptureVideoDataOutput (IrisHook)

- (void)msc_setSampleBufferDelegate:(id<AVCaptureVideoDataOutputSampleBufferDelegate>)delegate
                              queue:(dispatch_queue_t)queue {
    os_unfair_lock_lock(&gState.lock);
    gState.delegate = delegate;
    gState.delegateQueue = queue ?: dispatch_get_main_queue();
    gState.output = self;
    os_unfair_lock_unlock(&gState.lock);

    NSLog(@"[IrisInject] Captured delegate=%@ queue=%@", delegate, queue);
    [self msc_setSampleBufferDelegate:delegate queue:queue];
}

@end

// Fix for AVCapturePhotoOutput crash on iOS Simulator
@interface AVCaptureOutput (IrisHookFix)
@end
@implementation AVCaptureOutput (IrisHookFix)
+ (NSArray *)availableVideoCodecTypesForSourceDevice:(id)arg1 sourceFormat:(id)arg2 outputDimensions:(CMVideoDimensions)arg3 fileType:(id)arg4 videoCodecTypesAllowList:(id)arg5 {
    return @[];
}
@end

// MARK: - AVCaptureDevice (IrisHook)

@implementation AVCaptureDevice (IrisHook)

+ (nullable AVCaptureDevice *)msc_defaultDeviceWithMediaType:(AVMediaType)mediaType {
    if ([mediaType isEqualToString:AVMediaTypeVideo]) {
        static MSCFakeCaptureDevice *fakeDevice = nil;
        static dispatch_once_t onceToken;
        dispatch_once(&onceToken, ^{
            fakeDevice = (MSCFakeCaptureDevice *)class_createInstance([MSCFakeCaptureDevice class], 0);
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
            fakeDevice = (MSCFakeCaptureDevice *)class_createInstance([MSCFakeCaptureDevice class], 0);
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

// MARK: - AVCaptureDeviceDiscoverySession (IrisHook)

@implementation AVCaptureDeviceDiscoverySession (IrisHook)

- (NSArray<AVCaptureDevice *> *)msc_devices {
    static MSCFakeCaptureDevice *fakeDevice = nil;
    static dispatch_once_t onceToken;
    dispatch_once(&onceToken, ^{
        fakeDevice = (MSCFakeCaptureDevice *)class_createInstance([MSCFakeCaptureDevice class], 0);
    });
    return @[fakeDevice];
}

@end

// MARK: - AVCaptureDeviceInput (IrisHook)

@implementation AVCaptureDeviceInput (IrisHook)

+ (instancetype)msc_deviceInputWithDevice:(AVCaptureDevice *)device error:(NSError **)outError {
    if ([device isKindOfClass:[MSCFakeCaptureDevice class]]) {
        static MSCFakeCaptureInput *fakeInput = nil;
        static dispatch_once_t onceToken;
        dispatch_once(&onceToken, ^{
            fakeInput = (MSCFakeCaptureInput *)class_createInstance([MSCFakeCaptureInput class], 0);
        });
        return fakeInput;
    }
    return [self msc_deviceInputWithDevice:device error:outError];
}

- (instancetype)init_mscWithDevice:(AVCaptureDevice *)device error:(NSError **)outError {
    if ([device isKindOfClass:[MSCFakeCaptureDevice class]]) {
        static MSCFakeCaptureInput *fakeInput = nil;
        static dispatch_once_t onceToken;
        dispatch_once(&onceToken, ^{
            fakeInput = (MSCFakeCaptureInput *)class_createInstance([MSCFakeCaptureInput class], 0);
        });
        return fakeInput; // Return directly as ARC handles +1/-1 lifecycle
    }
    return [self init_mscWithDevice:device error:outError];
}

@end

// MARK: - AVCaptureVideoPreviewLayer (IrisHook)

@implementation AVCaptureVideoPreviewLayer (IrisHook)

- (instancetype)init_mscWithSession:(AVCaptureSession *)session {
    id instance = [self init_mscWithSession:session];
    if (instance) {
        os_unfair_lock_lock(&gState.lock);
        [gState.previewLayers addObject:instance];
        os_unfair_lock_unlock(&gState.lock);
        
        ((AVCaptureVideoPreviewLayer *)instance).contentsGravity = kCAGravityResizeAspectFill;
        startDelivery();
        MSCAutoStartSessionIfNeeded(session);
    }
    return instance;
}

- (instancetype)init_mscWithSessionWithNoConnection:(AVCaptureSession *)session {
    id instance = [self init_mscWithSessionWithNoConnection:session];
    if (instance) {
        os_unfair_lock_lock(&gState.lock);
        [gState.previewLayers addObject:instance];
        os_unfair_lock_unlock(&gState.lock);
        
        ((AVCaptureVideoPreviewLayer *)instance).contentsGravity = kCAGravityResizeAspectFill;
        startDelivery();
        MSCAutoStartSessionIfNeeded(session);
    }
    return instance;
}

- (void)msc_setSession:(AVCaptureSession *)session {
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
    
    [self msc_setSession:session];
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
        uint64_t leewayNs   = intervalNs / 10;

        gState.deliveryTimer = dispatch_source_create(DISPATCH_SOURCE_TYPE_TIMER, 0, 0, gState.deliveryQueue);
        dispatch_source_set_timer(
            gState.deliveryTimer,
            DISPATCH_TIME_NOW,
            intervalNs,
            leewayNs
        );
        dispatch_source_set_event_handler(gState.deliveryTimer, ^{ deliverFrame(); });
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

    void deliverFrame(void) {
        if (!gState.reader) return;

        if (!gState.reader->isOpen()) {
            if (!gState.reader->open()) {
                return;
            }
        }

        CMSampleBufferRef rawBuf = [gState.factory sampleBufferFromReader:gState.reader];
        if (!rawBuf) return;
        
        id arcSampleBuf = CFBridgingRelease(rawBuf); // ARC now owns the sample buffer

        CVPixelBufferRef pixelBuffer = CMSampleBufferGetImageBuffer((__bridge CMSampleBufferRef)arcSampleBuf);
        if (pixelBuffer) {
            CGImageRef cgImage = NULL;
            if (VTCreateCGImageFromCVPixelBuffer(pixelBuffer, NULL, &cgImage) == noErr && cgImage) {
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

        if (!del || !q) return;

        dispatch_async(q, ^{
            if ([del respondsToSelector:@selector(captureOutput:didOutputSampleBuffer:fromConnection:)]) {
                CMSampleBufferRef sampleBuf = (__bridge CMSampleBufferRef)arcSampleBuf;
                AVCaptureConnection *conn = outObj.connections.firstObject;
                if (!conn) {
                    static MSCFakeConnection *fakeConn = nil;
                    static dispatch_once_t onceToken;
                    dispatch_once(&onceToken, ^{
                        fakeConn = (MSCFakeConnection *)class_createInstance([MSCFakeConnection class], 0);
                    });
                    conn = fakeConn;
                }
                [del captureOutput:outObj
               didOutputSampleBuffer:sampleBuf
                      fromConnection:conn];
            }
        });
    }
}

// MARK: - Lifecycle Setup

void MSCInstallHooks(SharedFrameReader* reader, int32_t fps) {
    gState.deliveryQueue = dispatch_queue_create("com.iris.delivery", DISPATCH_QUEUE_SERIAL);
    gState.reader        = reader;
    gState.fps           = fps;
    gState.factory       = [[MSCSampleBufferFactory alloc] initWithFPS:fps];
    gState.previewLayers = [NSHashTable weakObjectsHashTable];

    swizzleInstance([AVCaptureSession class], @selector(startRunning), @selector(msc_startRunning));
    swizzleInstance([AVCaptureSession class], @selector(stopRunning), @selector(msc_stopRunning));
    swizzleInstance([AVCaptureSession class], @selector(canAddInput:), @selector(msc_canAddInput:));
    swizzleInstance([AVCaptureSession class], @selector(addInput:), @selector(msc_addInput:));
    swizzleInstance([AVCaptureSession class], @selector(canAddOutput:), @selector(msc_canAddOutput:));
    swizzleInstance([AVCaptureSession class], @selector(addOutput:), @selector(msc_addOutput:));
    swizzleInstance([AVCaptureSession class], @selector(isRunning), @selector(msc_isRunning));

    swizzleInstance([AVCaptureVideoDataOutput class], @selector(setSampleBufferDelegate:queue:), @selector(msc_setSampleBufferDelegate:queue:));

    Class layerCls = NSClassFromString(@"AVCaptureVideoPreviewLayer");
    if (layerCls) {
        swizzleInstance(layerCls, @selector(setSession:), @selector(msc_setSession:));
        swizzleInstance(layerCls, @selector(initWithSession:), @selector(init_mscWithSession:));
        swizzleInstance(layerCls, @selector(initWithSessionWithNoConnection:), @selector(init_mscWithSessionWithNoConnection:));
    }

    swizzleClass([AVCaptureDevice class], @selector(defaultDeviceWithMediaType:), @selector(msc_defaultDeviceWithMediaType:));
    swizzleClass([AVCaptureDevice class], @selector(defaultDeviceWithDeviceType:mediaType:position:), @selector(msc_defaultDeviceWithDeviceType:mediaType:position:));
    swizzleClass([AVCaptureDevice class], @selector(authorizationStatusForMediaType:), @selector(msc_authorizationStatusForMediaType:));
    swizzleClass([AVCaptureDevice class], @selector(requestAccessForMediaType:completionHandler:), @selector(msc_requestAccessForMediaType:completionHandler:));

    swizzleClass([AVCaptureDeviceInput class], @selector(deviceInputWithDevice:error:), @selector(msc_deviceInputWithDevice:error:));
    swizzleInstance([AVCaptureDeviceInput class], @selector(initWithDevice:error:), @selector(init_mscWithDevice:error:));

    Class discoverySessionCls = NSClassFromString(@"AVCaptureDeviceDiscoverySession");
    if (discoverySessionCls) {
        swizzleInstance(discoverySessionCls, @selector(devices), @selector(msc_devices));
    }

    NSLog(@"[IrisInject] Hooks installed — fps=%d", fps);
}

void MSCUninstallHooks(void) {
    stopDelivery();
}
