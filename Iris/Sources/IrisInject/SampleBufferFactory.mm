// SampleBufferFactory.mm
#import <AVFoundation/AVFoundation.h>
#import <CoreMedia/CoreMedia.h>
#import <CoreVideo/CoreVideo.h>
#import <IOSurface/IOSurfaceRef.h>
#import <mach/mach_time.h>
#import "SampleBufferFactory.h"
#import "SharedFrameReader.hpp"

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
    if (_formatDesc) {
        CFRelease(_formatDesc);
        _formatDesc = nullptr;
    }
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
