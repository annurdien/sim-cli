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
/// memory bounded instead. An object stays valid for the delegate callback it
/// is delivered in, which is the same lifetime the real API promises; it is
/// reused after MSC_METADATA_POOL_SIZE further detections.
#define MSC_METADATA_POOL_SIZE 16

static id MSCPooledMetadataObject(Class cls) {
    static NSMutableDictionary<NSString *, NSMutableArray *> *pools = nil;
    static NSMutableDictionary<NSString *, NSNumber *> *cursors = nil;
    static os_unfair_lock lock = OS_UNFAIR_LOCK_INIT;
    static dispatch_once_t onceToken;
    dispatch_once(&onceToken, ^{
        pools = [NSMutableDictionary dictionary];
        cursors = [NSMutableDictionary dictionary];
    });

    NSString *key = NSStringFromClass(cls);
    os_unfair_lock_lock(&lock);

    NSMutableArray *pool = pools[key];
    if (!pool) {
        pool = [NSMutableArray arrayWithCapacity:MSC_METADATA_POOL_SIZE];
        for (NSUInteger i = 0; i < MSC_METADATA_POOL_SIZE; i++) {
            // __bridge, not __bridge_transfer: the array's retain is the only
            // ownership these ever get, so nothing releases them back to zero.
            id instance = (__bridge id)(__bridge void *)class_createInstance(cls, 0);
            if (instance) {
                [pool addObject:instance];
            }
        }
        pools[key] = pool;
        cursors[key] = @0;
    }

    id object = nil;
    if (pool.count > 0) {
        NSUInteger cursor = cursors[key].unsignedIntegerValue % pool.count;
        object = pool[cursor];
        cursors[key] = @(cursor + 1);
    }

    os_unfair_lock_unlock(&lock);
    return object;
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
        MSCPooledMetadataObject([MSCFakeMachineReadableCodeObject class]);
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
- (CMTime)duration { return kCMTimeZero; }
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
        MSCPooledMetadataObject([MSCFakeFaceObject class]);
    if (object) {
        object->_irisFaceID = faceID;
        object->_irisBounds = bounds;
        object->_irisTime = time;
    }
    return object;
}

- (AVMetadataObjectType)type { return AVMetadataObjectTypeFace; }
- (NSInteger)faceID { return _irisFaceID; }
- (CGRect)bounds { return _irisBounds; }
- (CMTime)time { return _irisTime; }
- (CMTime)duration { return kCMTimeZero; }
- (BOOL)hasRollAngle { return NO; }
- (BOOL)hasYawAngle { return NO; }
@end
