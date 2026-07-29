// SampleBufferFactory.mm
#import <AVFoundation/AVFoundation.h>
#import <CoreImage/CoreImage.h>
#import <CoreMedia/CoreMedia.h>
#import <CoreVideo/CoreVideo.h>
#import <IOSurface/IOSurfaceRef.h>
#import <UIKit/UIKit.h>
#import <mach/mach_time.h>
#import <atomic>
#import "SampleBufferFactory.h"
#import "SharedFrameReader.hpp"

// MARK: - Device orientation
//
// A real camera is bolted into the device, so its buffers always arrive in the
// sensor's own orientation and the scene inside them turns as the device does.
// Consumers rely on that: they read the interface orientation and rotate by the
// matching quarter turns to get an upright picture.
//
// The host webcam is bolted to nothing. Delivered as-is, its frames are already
// upright, so the consumer's correction becomes the error — 90 degrees out in
// portrait, the other way upside down, 180 out in the opposite landscape.
//
// So counter-rotate by the correction the consumer is about to apply, and the
// two cancel. The buffer keeps the sensor's fixed dimensions throughout: the
// device advertises one format (FakeCaptureObjects), and a frame whose width
// and height swapped underneath it would contradict its own activeFormat.

/// Quarter turns clockwise a consumer applies for the current interface
/// orientation. Matches AVFoundation's convention for a back-facing sensor,
/// whose native orientation is landscape-right.
///
/// The device holding the sensor that way reports UIInterfaceOrientationLandscape
/// *Left*: UIKit's landscape constants are the mirror of the device orientations
/// that produce them. So the no-turn case below is the one whose name reads
/// wrong, and pairing these up by name is what breaks it.
static std::atomic<int> gConsumerQuarterTurns{0};

static int MSCQuarterTurnsForOrientation(UIInterfaceOrientation orientation) {
    switch (orientation) {
        case UIInterfaceOrientationLandscapeLeft:       return 0;
        case UIInterfaceOrientationPortrait:            return 1;
        case UIInterfaceOrientationLandscapeRight:      return 2;
        case UIInterfaceOrientationPortraitUpsideDown:  return 3;
        default:                                        return -1; // unknown
    }
}

/// Track the interface orientation on the main thread, where UIKit may be read,
/// and publish it for the delivery queue. An orientation we don't recognise
/// (face up, unknown) leaves the last good value in place rather than snapping
/// the picture to some default.
static void MSCRefreshOrientation(void) {
    UIWindowScene *scene = nil;
    for (UIScene *candidate in UIApplication.sharedApplication.connectedScenes) {
        if ([candidate isKindOfClass:[UIWindowScene class]]) {
            scene = (UIWindowScene *)candidate;
            break;
        }
    }
    if (!scene) return;
    int turns = MSCQuarterTurnsForOrientation(scene.interfaceOrientation);
    if (turns >= 0) gConsumerQuarterTurns.store(turns, std::memory_order_relaxed);
}

static void MSCStartOrientationTracking(void) {
    static dispatch_once_t onceToken;
    dispatch_once(&onceToken, ^{
        dispatch_async(dispatch_get_main_queue(), ^{
            [UIDevice.currentDevice beginGeneratingDeviceOrientationNotifications];
            [NSNotificationCenter.defaultCenter
                addObserverForName:UIDeviceOrientationDidChangeNotification
                            object:nil
                             queue:NSOperationQueue.mainQueue
                        usingBlock:^(NSNotification *_Nonnull note) {
                            MSCRefreshOrientation();
                        }];
            MSCRefreshOrientation();
        });
    });
}

/// The orientation to stamp on the frame so that the consumer's own rotation
/// lands upright — the inverse of what it is about to apply.
static CGImagePropertyOrientation MSCInverseOrientation(int consumerQuarterTurns) {
    switch (((consumerQuarterTurns % 4) + 4) % 4) {
        case 1:  return kCGImagePropertyOrientationLeft;  // undo 90 clockwise
        case 2:  return kCGImagePropertyOrientationDown;  // undo 180
        case 3:  return kCGImagePropertyOrientationRight; // undo 90 anticlockwise
        default: return kCGImagePropertyOrientationUp;
    }
}

@implementation MSCSampleBufferFactory {
    CMVideoFormatDescriptionRef _formatDesc;
    uint32_t _descWidth;
    uint32_t _descHeight;
    int32_t  _fps;
    CMTime   _frameDuration;
    CMTime   _startCMTime;
    CIContext *_ciContext;
    CVPixelBufferPoolRef _rotationPool;
    uint32_t _poolWidth;
    uint32_t _poolHeight;
}

- (instancetype)initWithFPS:(int32_t)fps {
    if (!(self = [super init])) return nil;
    _fps = fps;
    _frameDuration = CMTimeMake(1, fps);
    _startCMTime   = CMClockGetTime(CMClockGetHostTimeClock());
    MSCStartOrientationTracking();
    return self;
}

- (void)dealloc {
    if (_formatDesc) {
        CFRelease(_formatDesc);
        _formatDesc = nullptr;
    }
    if (_rotationPool) {
        CVPixelBufferPoolRelease(_rotationPool);
        _rotationPool = nullptr;
    }
}

/// Pool of sensor-sized buffers to rotate into. Recycled rather than allocated
/// per frame — at 30fps a fresh IOSurface each time is a lot of churn.
- (CVPixelBufferRef)rotationBufferOfWidth:(uint32_t)w height:(uint32_t)h {
    if (!_rotationPool || _poolWidth != w || _poolHeight != h) {
        if (_rotationPool) CVPixelBufferPoolRelease(_rotationPool);
        _rotationPool = nullptr;

        NSDictionary *attrs = @{
            (NSString *)kCVPixelBufferPixelFormatTypeKey: @(kCVPixelFormatType_32BGRA),
            (NSString *)kCVPixelBufferWidthKey: @(w),
            (NSString *)kCVPixelBufferHeightKey: @(h),
            (NSString *)kCVPixelBufferIOSurfacePropertiesKey: @{},
        };
        if (CVPixelBufferPoolCreate(kCFAllocatorDefault, NULL,
                                    (__bridge CFDictionaryRef)attrs,
                                    &_rotationPool) != kCVReturnSuccess) {
            _rotationPool = nullptr;
            return nullptr;
        }
        _poolWidth = w;
        _poolHeight = h;
    }

    CVPixelBufferRef out = nullptr;
    if (CVPixelBufferPoolCreatePixelBuffer(kCFAllocatorDefault, _rotationPool,
                                           &out) != kCVReturnSuccess) {
        return nullptr;
    }
    return out;
}

/// Rotate `src` so a consumer's orientation correction cancels out, returning a
/// buffer of the same dimensions. Rotating 90 degrees turns a landscape frame
/// portrait, so it is scaled to cover the sensor rectangle and centre-cropped —
/// which is what a real sensor shows in portrait too: the same optics, a
/// narrower slice of the world.
///
/// Returns NULL if anything fails, leaving the caller to send the frame
/// unrotated. A picture the wrong way up beats no picture at all.
- (nullable CVPixelBufferRef)rotatedBuffer:(CVPixelBufferRef)src
                                     turns:(int)turns
                                     width:(uint32_t)w
                                    height:(uint32_t)h {
    if (!_ciContext) {
        _ciContext = [CIContext contextWithOptions:@{
            kCIContextWorkingColorSpace: [NSNull null],
        }];
    }
    if (!_ciContext) return nullptr;

    CIImage *image = [CIImage imageWithCVPixelBuffer:src];
    image = [image imageByApplyingCGOrientation:MSCInverseOrientation(turns)];

    CGRect extent = image.extent;
    if (CGRectIsEmpty(extent)) return nullptr;

    CGFloat scale = MAX(w / extent.size.width, h / extent.size.height);
    image = [image imageByApplyingTransform:CGAffineTransformMakeScale(scale, scale)];

    extent = image.extent;
    image = [image imageByApplyingTransform:CGAffineTransformMakeTranslation(
                       (w - extent.size.width) / 2.0 - extent.origin.x,
                       (h - extent.size.height) / 2.0 - extent.origin.y)];

    CVPixelBufferRef dst = [self rotationBufferOfWidth:w height:h];
    if (!dst) return nullptr;

    [_ciContext render:image toCVPixelBuffer:dst];
    return dst;
}

- (nullable CMSampleBufferRef)sampleBufferFromReader:(SharedFrameReader *)reader {
    if (!reader || !reader->isOpen()) return nil;

    FrameSnapshot snap = reader->copyLatestFrame();
    if (!snap.valid || snap.ioSurfaceID == 0) return nil;

    IOSurfaceRef ioSurface = IOSurfaceLookup(snap.ioSurfaceID);
    if (!ioSurface) return nil;

    CVPixelBufferRef pixBuf = nullptr;
    NSDictionary *attrs = @{
        (NSString *)kCVPixelBufferPixelFormatTypeKey: @(snap.pixelFormat),
        (NSString *)kCVPixelBufferWidthKey: @(snap.width),
        (NSString *)kCVPixelBufferHeightKey: @(snap.height),
    };

    CVReturn ret = CVPixelBufferCreateWithIOSurface(
        kCFAllocatorDefault,
        ioSurface,
        (__bridge CFDictionaryRef)attrs,
        &pixBuf
    );
    
    CFRelease(ioSurface);

    if (ret != kCVReturnSuccess || !pixBuf) {
        if (pixBuf) CVPixelBufferRelease(pixBuf);
        return nil;
    }

    // The sensor's native landscape needs no turn, which is the common case and
    // stays zero-copy — the IOSurface goes to the app untouched.
    int turns = gConsumerQuarterTurns.load(std::memory_order_relaxed);
    if (turns != 0) {
        CVPixelBufferRef rotated = [self rotatedBuffer:pixBuf
                                                 turns:turns
                                                 width:snap.width
                                                height:snap.height];
        if (rotated) {
            CVPixelBufferRelease(pixBuf);
            pixBuf = rotated; // owned by us; wrapPixelBuffer releases it
        }
    }

    return [self wrapPixelBuffer:pixBuf pts:snap.ptsNs width:snap.width height:snap.height];
}

- (nullable CMSampleBufferRef)wrapPixelBuffer:(CVPixelBufferRef)pixBuf pts:(uint64_t)ptsNs width:(uint32_t)w height:(uint32_t)h {
    if (!_formatDesc || _descWidth != w || _descHeight != h) {
        if (_formatDesc) {
            CFRelease(_formatDesc);
            _formatDesc = nullptr;
        }
        CMVideoFormatDescriptionCreateForImageBuffer(
            kCFAllocatorDefault, pixBuf, &_formatDesc
        );
        _descWidth = w;
        _descHeight = h;
    }
    
    if (!_formatDesc) {
        CVPixelBufferRelease(pixBuf);
        return nil;
    }

    CMTime pts = CMTimeMake(ptsNs, 1000000000);

    CMSampleTimingInfo timing = {
        .duration              = _frameDuration,
        .presentationTimeStamp = pts,
        .decodeTimeStamp       = kCMTimeInvalid
    };

    CMSampleBufferRef sampleBuf = nullptr;
    OSStatus status = CMSampleBufferCreateReadyWithImageBuffer(
        kCFAllocatorDefault,
        pixBuf,
        _formatDesc,
        &timing,
        &sampleBuf
    );

    CVPixelBufferRelease(pixBuf);

    if (status != noErr || !sampleBuf) return nil;
    return sampleBuf;
}

@end
