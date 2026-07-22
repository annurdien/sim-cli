// SampleBufferFactory.mm
// Creates CMSampleBuffers from shared-memory frames using IOSurface zero-copy.

#import <AVFoundation/AVFoundation.h>
#import <CoreMedia/CoreMedia.h>
#import <CoreVideo/CoreVideo.h>
#import <IOSurface/IOSurfaceRef.h>
#import <mach/mach_time.h>
#import "SampleBufferFactory.h"
#import "SharedFrameReader.hpp"

extern "C" {
    IOSurfaceRef IOSurfaceLookup(uint32_t csid);
}

@implementation MSCSampleBufferFactory {
    CMVideoFormatDescriptionRef _formatDesc;
    int32_t  _fps;
    CMTime   _frameDuration;
    CMTime   _startCMTime;
}

- (instancetype)initWithFPS:(int32_t)fps {
    if (!(self = [super init])) return nil;
    _fps = fps;
    _frameDuration = CMTimeMake(1, fps);
    _startCMTime   = CMClockGetTime(CMClockGetHostTimeClock());
    return self;
}

- (void)dealloc {
    if (_formatDesc) { CFRelease(_formatDesc); _formatDesc = nullptr; }
}

- (nullable CMSampleBufferRef)sampleBufferFromReader:(SharedFrameReader *)reader {
    if (!reader || !reader->isOpen()) return nil;

    FrameInfo info = reader->getLatestFrameInfo();
    // Only log occasionally to prevent spam, or just log if it's invalid or 0
    if (!info.valid || info.ioSurfaceID == 0) {
        static int dropCount = 0;
        if (dropCount++ % 60 == 0) {
            NSLog(@"[MiniCamInject] Dropped frame (valid=%d, ioSurfaceID=%u, fp=%llu)", info.valid, info.ioSurfaceID, info.framesProduced);
        }
        return nil;
    }

    IOSurfaceRef surface = IOSurfaceLookup(info.ioSurfaceID);
    if (!surface) {
        NSLog(@"[MiniCamInject] IOSurfaceLookup failed for ID %u", info.ioSurfaceID);
        return nil;
    }

    CVPixelBufferRef pixBuf = nullptr;
    CVReturn ret = CVPixelBufferCreateWithIOSurface(
        kCFAllocatorDefault,
        surface,
        nullptr, // Let CoreVideo use default attributes
        &pixBuf
    );
    
    CFRelease(surface); // CVPixelBuffer retains it if needed
    
    if (ret != kCVReturnSuccess || !pixBuf) {
        NSLog(@"[MiniCamInject] CVPixelBufferCreateWithIOSurface failed with error %d", ret);
        return nil;
    }

    CMSampleBufferRef sbuf = [self wrapPixelBuffer:pixBuf pts:info.ptsNs];
    if (!sbuf) {
        NSLog(@"[MiniCamInject] wrapPixelBuffer failed");
    } else {
        static int okCount = 0;
        if (okCount++ % 60 == 0) {
            NSLog(@"[MiniCamInject] wrapPixelBuffer success (seq=%llu)", info.framesProduced);
        }
    }
    return sbuf;
}

/// Internal helper: wraps a CVPixelBuffer into a CMSampleBuffer.
/// Takes ownership of pixBuf (calls CVPixelBufferRelease).
/// Returns a +1 CF reference; caller must CFRelease.
- (nullable CMSampleBufferRef)wrapPixelBuffer:(CVPixelBufferRef)pixBuf pts:(uint64_t)ptsNs {
    if (!_formatDesc) {
        CMVideoFormatDescriptionCreateForImageBuffer(
            kCFAllocatorDefault, pixBuf, &_formatDesc
        );
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
    // Return a +1 CF reference. The caller is responsible for CFRelease.
    return sampleBuf;
}

@end
