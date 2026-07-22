// SampleBufferFactory.mm
// Creates CVPixelBuffers and CMSampleBuffers from shared-memory frames.
// Two code paths:
//   1. sampleBufferFromSnapshot: — legacy, takes a pre-copied FrameSnapshot.
//   2. sampleBufferFromReader:   — optimised, single-copy directly into CVPixelBuffer.

#import <AVFoundation/AVFoundation.h>
#import <CoreMedia/CoreMedia.h>
#import <CoreVideo/CoreVideo.h>
#import <mach/mach_time.h>
#import "SampleBufferFactory.h"
#import "SharedFrameReader.hpp"

// ---------------------------------------------------------------------------
// CVPixelBufferPool wrapper
// ---------------------------------------------------------------------------

@implementation MSCSampleBufferFactory {
    CVPixelBufferPoolRef _pool;
    CMVideoFormatDescriptionRef _formatDesc;
    uint32_t _poolWidth;
    uint32_t _poolHeight;
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
    [self teardownPool];
}

- (void)teardownPool {
    if (_pool)       { CVPixelBufferPoolRelease(_pool); _pool = nullptr; }
    if (_formatDesc) { CFRelease(_formatDesc); _formatDesc = nullptr; }
    _poolWidth = 0; _poolHeight = 0;
}

/// Ensure the pixel-buffer pool matches the current frame dimensions.
- (BOOL)ensurePoolWidth:(uint32_t)w height:(uint32_t)h {
    if (_pool && _poolWidth == w && _poolHeight == h) return YES;
    [self teardownPool];
    _poolWidth  = w;
    _poolHeight = h;

    NSDictionary *attrs = @{
        (NSString *)kCVPixelBufferPixelFormatTypeKey: @(kCVPixelFormatType_32BGRA),
        (NSString *)kCVPixelBufferWidthKey:           @(w),
        (NSString *)kCVPixelBufferHeightKey:          @(h),
        (NSString *)kCVPixelBufferIOSurfacePropertiesKey: @{},
        (NSString *)kCVPixelBufferCGImageCompatibilityKey: @YES,
        (NSString *)kCVPixelBufferCGBitmapContextCompatibilityKey: @YES
    };

    CVReturn ret = CVPixelBufferPoolCreate(
        kCFAllocatorDefault, nullptr,
        (__bridge CFDictionaryRef)attrs,
        &_pool
    );
    return ret == kCVReturnSuccess;
}

- (nullable CMSampleBufferRef)sampleBufferFromSnapshot:(const FrameSnapshot &)snap {
    if (!snap.valid || snap.data.empty()) return nil;

    const uint32_t w   = snap.width;
    const uint32_t h   = snap.height;
    const uint32_t bpr = snap.bytesPerRow;

    if (![self ensurePoolWidth:w height:h]) return nil;

    // --- Allocate pixel buffer from pool ---
    CVPixelBufferRef pixBuf = nullptr;
    if (CVPixelBufferPoolCreatePixelBuffer(kCFAllocatorDefault, _pool, &pixBuf) != kCVReturnSuccess) {
        return nil;
    }

    // --- Copy frame data row-by-row (source and destination may have different strides) ---
    CVPixelBufferLockBaseAddress(pixBuf, 0);
    uint8_t *dst   = (uint8_t *)CVPixelBufferGetBaseAddress(pixBuf);
    size_t dstBPR  = CVPixelBufferGetBytesPerRow(pixBuf);
    const uint8_t *src = snap.data.data();

    for (uint32_t row = 0; row < h; ++row) {
        memcpy(dst + row * dstBPR, src + row * bpr, (size_t)w * 4);
    }
    CVPixelBufferUnlockBaseAddress(pixBuf, 0);

    return [self wrapPixelBuffer:pixBuf pts:snap.ptsNs];
}

- (nullable CMSampleBufferRef)sampleBufferFromReader:(SharedFrameReader *)reader {
    if (!reader || !reader->isOpen()) return nil;

    // Read dimensions directly from the shm header — no pixel copy yet.
    const MSCStreamHeader* hdr = reader->peekHeader();
    if (!hdr || hdr->magic != MSC_MAGIC) return nil;

    const uint32_t w = hdr->width;
    const uint32_t h = hdr->height;
    if (w == 0 || h == 0) return nil;

    if (![self ensurePoolWidth:w height:h]) return nil;

    CVPixelBufferRef pixBuf = nullptr;
    if (CVPixelBufferPoolCreatePixelBuffer(kCFAllocatorDefault, _pool, &pixBuf) != kCVReturnSuccess) {
        return nil;
    }

    // Single pass: shm → CVPixelBuffer (no intermediate vector). The
    // reader copies per row because its stream stride can differ from the
    // pixel buffer pool's stride.
    CVPixelBufferLockBaseAddress(pixBuf, 0);
    void *dst       = CVPixelBufferGetBaseAddress(pixBuf);
    size_t dstBPR   = CVPixelBufferGetBytesPerRow(pixBuf);
    size_t dstTotal = dstBPR * h;

    FrameInfo info = reader->copyLatestFrameInto(dst, dstBPR, dstTotal);
    CVPixelBufferUnlockBaseAddress(pixBuf, 0);

    if (!info.valid) {
        CVPixelBufferRelease(pixBuf);
        return nil;
    }

    return [self wrapPixelBuffer:pixBuf pts:info.ptsNs];
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
