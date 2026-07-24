// FakeCaptureObjects.h
#import <AVFoundation/AVFoundation.h>

@interface MSCFakeConnection : AVCaptureConnection
@end

@interface MSCFakeCaptureDeviceFormat : NSObject
- (CMFormatDescriptionRef)formatDescription;
@end

@interface MSCFakeCaptureDevice : AVCaptureDevice
@end

@interface MSCFakeCaptureInputPort : AVCaptureInputPort
@end

@interface MSCFakeCaptureInput : AVCaptureDeviceInput
@end
