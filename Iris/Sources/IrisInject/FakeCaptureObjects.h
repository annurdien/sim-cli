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

/// Metadata objects synthesised from Vision results run over the injected
/// frames. AVFoundation's metadata classes have no public initialiser, so
/// these are built with class_createInstance and answer every accessor from
/// their own ivars — the same approach MSCFakeCaptureDevice takes. Real
/// subclasses, not duck types, because apps commonly downcast with
/// `as? AVMetadataMachineReadableCodeObject`.
@interface MSCFakeMachineReadableCodeObject : AVMetadataMachineReadableCodeObject
+ (instancetype)objectWithType:(AVMetadataObjectType)type
                   stringValue:(NSString *)stringValue
                        bounds:(CGRect)bounds
                       corners:(NSArray<NSDictionary *> *)corners
                          time:(CMTime)time;
@end

@interface MSCFakeFaceObject : AVMetadataFaceObject
+ (instancetype)objectWithFaceID:(NSInteger)faceID
                          bounds:(CGRect)bounds
                            time:(CMTime)time;
@end
