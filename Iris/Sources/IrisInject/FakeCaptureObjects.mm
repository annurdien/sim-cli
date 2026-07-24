// FakeCaptureObjects.mm
#import "FakeCaptureObjects.h"
#import <objc/runtime.h>

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
@end
