// SampleBufferFactory.h
// Objective-C header for MSCSampleBufferFactory.
// Uses #ifdef __cplusplus so this can be included from both .mm and pure C contexts.

#pragma once
#import <Foundation/Foundation.h>
#import <CoreMedia/CoreMedia.h>

#ifdef __cplusplus
#include "SharedFrameReader.hpp"
#endif

NS_ASSUME_NONNULL_BEGIN

@interface MSCSampleBufferFactory : NSObject

- (instancetype)initWithFPS:(int32_t)fps NS_DESIGNATED_INITIALIZER;
- (instancetype)init NS_UNAVAILABLE;

#ifdef __cplusplus
/// Creates a CMSampleBuffer from a FrameSnapshot read via SharedFrameReader.
/// Returns a +1 CF reference on success; the caller is responsible for CFRelease.
/// Returns nil if the snapshot is invalid or buffer creation fails.

/// Optimised single-copy path: reads frame metadata from the shm header, allocates
/// a CVPixelBuffer, and copies the frame data directly (one copy vs two).
/// Returns a +1 CF reference on success; the caller is responsible for CFRelease.
- (nullable CMSampleBufferRef)sampleBufferFromReader:(SharedFrameReader *)reader CF_RETURNS_RETAINED;
#endif

@end

NS_ASSUME_NONNULL_END
