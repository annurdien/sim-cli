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

/// Metadata objects synthesised from Core Image detector results run over the
/// injected frames. AVFoundation's metadata classes have no public initialiser,
/// so these are built with class_createInstance and answer every accessor from
/// their own ivars — the same approach MSCFakeCaptureDevice takes. Real
/// subclasses, not duck types, because apps commonly downcast with
/// `as? AVMetadataMachineReadableCodeObject`.
///
/// Instances are pooled and checked out; every object obtained from the
/// factories below must be handed back with MSCReleasePooledMetadataObjects
/// once the delegate callback that carried it has returned.
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

/// Returns pooled metadata objects for reuse. Call once the delegate callback
/// they were delivered in has returned; objects not handed back are leaked from
/// the pool's point of view and reduce the slots available to later detections.
#ifdef __cplusplus
extern "C" {
#endif
void MSCReleasePooledMetadataObjects(NSArray *objects);
#ifdef __cplusplus
}
#endif
