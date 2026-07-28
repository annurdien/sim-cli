// FakeCaptureObjects.mm
#import "FakeCaptureObjects.h"
#import <objc/runtime.h>
#import <os/lock.h>

@implementation MSCFakeConnection
@end

@implementation MSCFakeCaptureDeviceFormat
- (CMFormatDescriptionRef)formatDescription {
    CMVideoFormatDescriptionRef formatDesc = NULL;
    CMVideoFormatDescriptionCreate(kCFAllocatorDefault, kCVPixelFormatType_32BGRA, 1280, 720, NULL, &formatDesc);
    return formatDesc;
}
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

- (NSString *)uniqueID { return @"MSCFakeCamera_001"; }
- (NSString *)localizedName { return @"Iris Fake Device"; }
- (AVCaptureDevicePosition)position { return AVCaptureDevicePositionBack; }
- (BOOL)isConnected { return YES; }
- (BOOL)isSuspended { return NO; }

- (CMVideoDimensions)activeSensorLocation { return (CMVideoDimensions){1280, 720}; }
- (void)setActiveVideoMinFrameDuration:(CMTime)activeVideoMinFrameDuration {}
- (CMTime)activeVideoMinFrameDuration { return CMTimeMake(1, 30); }
- (void)setActiveVideoMaxFrameDuration:(CMTime)activeVideoMaxFrameDuration {}
- (CMTime)activeVideoMaxFrameDuration { return CMTimeMake(1, 30); }

- (AVCaptureDeviceType)deviceType { return AVCaptureDeviceTypeBuiltInWideAngleCamera; }

- (id)activeFormat {
    static MSCFakeCaptureDeviceFormat *fakeFormat = nil;
    static dispatch_once_t onceToken;
    dispatch_once(&onceToken, ^{
        fakeFormat = [[MSCFakeCaptureDeviceFormat alloc] init];
    });
    return fakeFormat;
}
@end

@implementation MSCFakeCaptureInputPort
- (AVMediaType)mediaType { return AVMediaTypeVideo; }
- (CMFormatDescriptionRef)formatDescription {
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

@implementation MSCFakeCaptureInput
- (AVCaptureDevice *)device {
    static MSCFakeCaptureDevice *fakeDevice = nil;
    static dispatch_once_t onceToken;
    dispatch_once(&onceToken, ^{
        fakeDevice = (MSCFakeCaptureDevice *)class_createInstance([MSCFakeCaptureDevice class], 0);
    });
    return fakeDevice;
}
- (NSArray<AVCaptureInputPort *> *)ports {
    static MSCFakeCaptureInputPort *fakePort = nil;
    static dispatch_once_t onceToken;
    dispatch_once(&onceToken, ^{
        fakePort = (MSCFakeCaptureInputPort *)class_createInstance([MSCFakeCaptureInputPort class], 0);
    });
    return @[fakePort];
}

// AVFoundation's own implementation walks internal state that class_createInstance
// never set up (-[AVCaptureDeviceInput multiCamPorts] dereferences it and
// segfaults), so answer typed port lookups ourselves. Only video is synthesised:
// callers asking for metadata/audio/depth ports get an empty array, which is the
// documented "no such port" answer and one every caller already handles.
- (NSArray<AVCaptureInputPort *> *)portsWithMediaType:(AVMediaType)mediaType
                                     sourceDeviceType:(AVCaptureDeviceType)sourceDeviceType
                                 sourceDevicePosition:(AVCaptureDevicePosition)sourceDevicePosition {
    if (mediaType == nil || [mediaType isEqualToString:AVMediaTypeVideo]) {
        return self.ports;
    }
    return @[];
}
@end

// MARK: - Synthesised metadata objects

/// Synthesised metadata objects are pooled: allocated once and reused, never
/// released.
///
/// AVMetadataObject's own -dealloc walks internals that class_createInstance
/// never set up and segfaults — reproducibly, even on an instance with nothing
/// assigned — so these must never be deallocated. Recycling a fixed set keeps
/// memory bounded instead.
///
/// Slots are checked out and back in rather than handed round a ring, because a
/// pooled object's ivars are written by the recognition queue and read by the
/// delegate on a queue we don't control. A ring makes overlap merely unlikely
/// (bounded by pool size × cadence); check-out makes it impossible — a slot is
/// never handed out again until the callback it was delivered in has returned,
/// which is exactly the lifetime the real API promises. Callers must therefore
/// pair every acquisition with MSCReleasePooledMetadataObjects.
///
/// If every slot is checked out the pool grows, up to a hard cap; past that,
/// acquisition returns nil and the caller drops that object. A pathological
/// delegate that never returns costs detections, not memory or correctness.
#define MSC_METADATA_POOL_SIZE 16
#define MSC_METADATA_POOL_MAX 64

static NSMutableDictionary<NSString *, NSMutableArray *> *sMetadataPools = nil;
static NSMutableDictionary<NSString *, NSMutableIndexSet *> *sMetadataInUse = nil;
static os_unfair_lock sMetadataPoolLock = OS_UNFAIR_LOCK_INIT;

static void MSCMetadataPoolInit(void) {
    static dispatch_once_t onceToken;
    dispatch_once(&onceToken, ^{
        sMetadataPools = [NSMutableDictionary dictionary];
        sMetadataInUse = [NSMutableDictionary dictionary];
    });
}

/// Never released — see the pool note above, and the +1 from
/// class_createInstance is deliberately abandoned.
static id MSCCreateUnreleasedInstance(Class cls) {
    return class_createInstance(cls, 0);
}

static id MSCAcquirePooledMetadataObject(Class cls) {
    MSCMetadataPoolInit();

    NSString *key = NSStringFromClass(cls);
    os_unfair_lock_lock(&sMetadataPoolLock);

    NSMutableArray *pool = sMetadataPools[key];
    if (!pool) {
        pool = [NSMutableArray arrayWithCapacity:MSC_METADATA_POOL_SIZE];
        for (NSUInteger i = 0; i < MSC_METADATA_POOL_SIZE; i++) {
            id instance = MSCCreateUnreleasedInstance(cls);
            if (instance) {
                [pool addObject:instance];
            }
        }
        sMetadataPools[key] = pool;
        sMetadataInUse[key] = [NSMutableIndexSet indexSet];
    }

    NSMutableIndexSet *inUse = sMetadataInUse[key];
    NSUInteger slot = NSNotFound;
    for (NSUInteger i = 0; i < pool.count; i++) {
        if (![inUse containsIndex:i]) {
            slot = i;
            break;
        }
    }
    if (slot == NSNotFound && pool.count < MSC_METADATA_POOL_MAX) {
        id instance = MSCCreateUnreleasedInstance(cls);
        if (instance) {
            [pool addObject:instance];
            slot = pool.count - 1;
        }
    }

    id object = nil;
    if (slot != NSNotFound) {
        [inUse addIndex:slot];
        object = pool[slot];
    }
    NSUInteger checkedOut = inUse.count;

    os_unfair_lock_unlock(&sMetadataPoolLock);

    if (!object) {
        // Only reachable if delegate callbacks are not returning: every slot is
        // still checked out at the cap. Say so — the alternative is detections
        // that quietly stop producing results.
        NSLog(@"[IrisInject] Metadata pool exhausted (%lu/%d checked out) — "
              @"dropping a %@. Is the metadata delegate blocking its queue?",
              (unsigned long)checkedOut, MSC_METADATA_POOL_MAX, key);
    }
    return object;
}

void MSCReleasePooledMetadataObjects(NSArray *objects) {
    if (objects.count == 0) {
        return;
    }

    os_unfair_lock_lock(&sMetadataPoolLock);
    for (id object in objects) {
        NSString *key = NSStringFromClass(object_getClass(object));
        NSMutableArray *pool = sMetadataPools[key];
        if (!pool) {
            continue;
        }
        NSUInteger slot = [pool indexOfObjectIdenticalTo:object];
        if (slot != NSNotFound) {
            [sMetadataInUse[key] removeIndex:slot];
        }
    }
    os_unfair_lock_unlock(&sMetadataPoolLock);
}

@implementation MSCFakeMachineReadableCodeObject {
    AVMetadataObjectType _irisType;
    NSString *_irisStringValue;
    CGRect _irisBounds;
    NSArray<NSDictionary *> *_irisCorners;
    CMTime _irisTime;
}

+ (instancetype)objectWithType:(AVMetadataObjectType)type
                   stringValue:(NSString *)stringValue
                        bounds:(CGRect)bounds
                       corners:(NSArray<NSDictionary *> *)corners
                          time:(CMTime)time {
    MSCFakeMachineReadableCodeObject *object =
        MSCAcquirePooledMetadataObject([MSCFakeMachineReadableCodeObject class]);
    if (object) {
        object->_irisType = [type copy];
        object->_irisStringValue = [stringValue copy];
        object->_irisBounds = bounds;
        object->_irisCorners = [corners copy];
        object->_irisTime = time;
    }
    return object;
}

- (AVMetadataObjectType)type { return _irisType; }
- (NSString *)stringValue { return _irisStringValue; }
- (CGRect)bounds { return _irisBounds; }
- (NSArray<NSDictionary *> *)corners { return _irisCorners ?: @[]; }
- (CMTime)time { return _irisTime; }
// CMTimeMake, not the deprecated kCMTimeZero. Detection is instantaneous from
// the consumer's point of view: one frame in, one result out.
- (CMTime)duration { return CMTimeMake(0, 1); }
@end

@implementation MSCFakeFaceObject {
    NSInteger _irisFaceID;
    CGRect _irisBounds;
    CMTime _irisTime;
}

+ (instancetype)objectWithFaceID:(NSInteger)faceID
                          bounds:(CGRect)bounds
                            time:(CMTime)time {
    MSCFakeFaceObject *object =
        MSCAcquirePooledMetadataObject([MSCFakeFaceObject class]);
    if (object) {
        object->_irisFaceID = faceID;
        object->_irisBounds = bounds;
        object->_irisTime = time;
    }
    return object;
}

- (AVMetadataObjectType)type { return AVMetadataObjectTypeFace; }
// Not a tracking id, unlike the real one: CIDetector offers no identity across
// frames, so this is only an index within a single detection pass. Apps that
// follow a face between frames by faceID won't work here.
- (NSInteger)faceID { return _irisFaceID; }
- (CGRect)bounds { return _irisBounds; }
- (CMTime)time { return _irisTime; }
// CMTimeMake, not the deprecated kCMTimeZero. Detection is instantaneous from
// the consumer's point of view: one frame in, one result out.
- (CMTime)duration { return CMTimeMake(0, 1); }
- (BOOL)hasRollAngle { return NO; }
- (BOOL)hasYawAngle { return NO; }
@end
