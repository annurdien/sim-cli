// SampleBufferFactory.mm
// Creates CVPixelBuffers and CMSampleBuffers from shared-memory frames.
// Uses IOSurface for true zero-copy frame delivery.

#import <AVFoundation/AVFoundation.h>
#import <CoreMedia/CoreMedia.h>
#import <CoreVideo/CoreVideo.h>
#import <IOSurface/IOSurfaceRef.h>
#import <mach/mach_time.h>
#import "SampleBufferFactory.h"
#import "SharedFrameReader.hpp"

// ---------------------------------------------------------------------------
// Factory Implementation
// ---------------------------------------------------------------------------

@implementation MSCSampleBufferFactory {
    CMVideoFormatDescriptionRef _formatDesc;
    uint32_t _descWidth;
    uint32_t _descHeight;
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

    FrameSnapshot snap = reader->copyLatestFrame();
    if (!snap.valid || snap.ioSurfaceID == 0) return nil;

    // Lookup the IOSurface across process boundaries
    IOSurfaceRef ioSurface = IOSurfaceLookup(snap.ioSurfaceID);
    if (!ioSurface) return nil;

    // Wrap the IOSurface into a CoreVideo buffer
    CVPixelBufferRef pixBuf = nullptr;
    NSDictionary *attrs = @{
        (NSString *)kCVPixelBufferPixelFormatTypeKey: @(kCVPixelFormatType_32BGRA),
        (NSString *)kCVPixelBufferWidthKey: @(snap.width),
        (NSString *)kCVPixelBufferHeightKey: @(snap.height),
    };

    CVReturn ret = CVPixelBufferCreateWithIOSurface(
        kCFAllocatorDefault,
        ioSurface,
        (__bridge CFDictionaryRef)attrs,
        &pixBuf
    );
    
    // CFRelease the IOSurfaceRef because IOSurfaceLookup returns a +1 retain
    CFRelease(ioSurface);

    if (ret != kCVReturnSuccess || !pixBuf) {
        if (pixBuf) CVPixelBufferRelease(pixBuf);
        return nil;
    }

    return [self wrapPixelBuffer:pixBuf pts:snap.ptsNs width:snap.width height:snap.height];
}

/// Internal helper: wraps a CVPixelBuffer into a CMSampleBuffer.
/// Takes ownership of pixBuf (calls CVPixelBufferRelease).
/// Returns a +1 CF reference; caller must CFRelease.
- (nullable CMSampleBufferRef)wrapPixelBuffer:(CVPixelBufferRef)pixBuf pts:(uint64_t)ptsNs width:(uint32_t)w height:(uint32_t)h {
    if (!_formatDesc || _descWidth != w || _descHeight != h) {
        if (_formatDesc) { CFRelease(_formatDesc); _formatDesc = nullptr; }
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
    // Return a +1 CF reference. The caller is responsible for CFRelease.
    return sampleBuf;
}

@end
