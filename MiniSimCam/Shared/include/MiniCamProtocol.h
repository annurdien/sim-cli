#pragma once
// MiniCamProtocol.h
// Shared between the macOS FrameHost and the iOS Simulator injector dylib.
// Must stay C-compatible so it can be imported from Swift and ObjC++.
//
// ATOMIC FIELDS POLICY
// ====================
// The sequence, publishedIndex, and framesProduced fields carry atomic
// semantics but are declared as plain integer types here so Swift can
// import them. ALL access to these fields MUST go through the wrapper
// functions in AtomicHelpers.h. Never read/write them directly.
//
// The C++ injector (SharedFrameReader.cpp) casts these fields to
// std::atomic<T>* — valid under the C++ memory model when the underlying
// type has the same size and alignment.

#include <stdint.h>

#define MSC_MAGIC         0x4D534343u   // "MSCC"
#define MSC_VERSION       1u
#define MSC_BUFFER_COUNT  3u

// kCVPixelFormatType_32BGRA = 0x42475241 = 'BGRA'
#define MSC_PIXEL_FORMAT  0x42475241u

// Align frame rows to 64 bytes (cache-line sized).
#define MSC_ROW_ALIGNMENT 64u

// ----------------------------------------------------------------------------
// Stream header — lives at offset 0 in the shared-memory file.
// The control ring buffer follows immediately after sizeof(MSCStreamHeader).
//
// Field layout (all offsets from struct start, verified by static assert):
//   [  0] uint32_t magic
//   [  4] uint32_t version
//   [  8] uint32_t width
//   [ 12] uint32_t height
//   [ 16] uint32_t controlHead     ← ATOMIC
//   [ 20] uint32_t pixelFormat
//   [ 24] uint32_t controlTail     ← ATOMIC
//   [ 28] uint32_t ioSurfaceID     ← Single ID if static, or just unused
//   [ 32] uint64_t sequence        ← ATOMIC, use msc_seq_*()
//   [ 40] uint32_t publishedIndex  ← ATOMIC, use msc_idx_*()
//   [ 44] uint32_t _pad0           ← explicit padding for alignment
//   [ 48] uint64_t presentationTimeNs  (plain, written under seq lock)
//   [ 56] uint64_t framesProduced  ← ATOMIC, use msc_fp_*()
//   [ 64] uint32_t ioSurfaceIDs[3] ← The 3 IOSurface IDs for the triple buffer
//   [ 76] uint8_t  reserved[52]
//   [128] = total
// ----------------------------------------------------------------------------
typedef struct {
    uint32_t magic;                // [  0]
    uint32_t version;              // [  4]
    uint32_t width;                // [  8]
    uint32_t height;               // [ 12]
    uint32_t controlHead;          // [ 16] ATOMIC (Written by iOS, Read by macOS)
    uint32_t pixelFormat;          // [ 20]
    uint32_t controlTail;          // [ 24] ATOMIC (Written by macOS, Read by iOS)
    uint32_t ioSurfaceID;          // [ 28] (Legacy/Fallback)
    uint64_t sequence;             // [ 32] ATOMIC — use msc_seq_*()
    uint32_t publishedIndex;       // [ 40] ATOMIC — use msc_idx_*()
    uint32_t _pad0;                // [ 44] explicit pad
    uint64_t presentationTimeNs;   // [ 48]
    uint64_t framesProduced;       // [ 56] ATOMIC — use msc_fp_*()
    uint32_t ioSurfaceIDs[3];      // [ 64] 
    uint8_t  reserved[52];         // [ 76]
} MSCStreamHeader;                 // [128] total

// Byte offsets used by AtomicHelpers.c — must match the layout above.
#define MSC_OFF_SEQUENCE        32u
#define MSC_OFF_PUBLISHED_INDEX 40u
#define MSC_OFF_FRAMES_PRODUCED 56u
#define MSC_OFF_CONTROL_HEAD    16u
#define MSC_OFF_CONTROL_TAIL    24u

// Compile-time size check (C11 _Static_assert).
_Static_assert(sizeof(MSCStreamHeader) == 128, "MSCStreamHeader must be exactly 128 bytes");

#define MSC_HEADER_EXPECTED_SIZE 128u

// ----------------------------------------------------------------------------
// Control Channel Schema
// ----------------------------------------------------------------------------
typedef enum : uint32_t {
    MSC_CONTROL_EVENT_NONE          = 0,
    MSC_CONTROL_EVENT_FLIP_CAMERA   = 1,
    MSC_CONTROL_EVENT_FOCUS_POINT   = 2
} MSCControlEventType;

typedef struct {
    uint32_t type;
    float x;
    float y;
} MSCControlEvent;

#define MSC_CONTROL_RING_CAPACITY 16u

// ----------------------------------------------------------------------------
// Helpers
// ----------------------------------------------------------------------------

// Compute aligned bytes-per-row for a given pixel width.
static inline uint32_t msc_bytes_per_row(uint32_t width) {
    uint32_t raw = width * 4u;
    return (raw + MSC_ROW_ALIGNMENT - 1u) & ~(MSC_ROW_ALIGNMENT - 1u);
}

// Total mmap size: header + control ring buffer.
static inline uint64_t msc_mapping_size(void) {
    return (uint64_t)MSC_HEADER_EXPECTED_SIZE + (uint64_t)MSC_CONTROL_RING_CAPACITY * sizeof(MSCControlEvent);
}

// Pointer to the control ring buffer inside a mapped region.
static inline MSCControlEvent *msc_control_ring_ptr(void *base) {
    uint8_t *p = (uint8_t *)base;
    return (MSCControlEvent *)(p + MSC_HEADER_EXPECTED_SIZE);
}
