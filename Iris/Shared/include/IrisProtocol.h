#pragma once
// IrisProtocol.h
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

#define IRIS_MAGIC         0x4D534343u   // "MSCC"
#define IRIS_VERSION       1u
#define IRIS_BUFFER_COUNT  3u

// kCVPixelFormatType_32BGRA = 0x42475241 = 'BGRA'
#define IRIS_PIXEL_FORMAT  0x42475241u

// Align frame rows to 64 bytes (cache-line sized).
#define IRIS_ROW_ALIGNMENT 64u

// ----------------------------------------------------------------------------
// Stream header lives at offset 0 in the shared-memory file.
// The three frame buffers follow immediately after sizeof(IRISStreamHeader).
//
// Field layout (all offsets from struct start, verified by static assert):
//   [  0] uint32_t magic
//   [  4] uint32_t version
//   [  8] uint32_t width
//   [ 12] uint32_t height
//   [ 16] uint32_t bytesPerRow
//   [ 20] uint32_t pixelFormat
//   [ 24] uint32_t bufferCount
//   [ 28] uint32_t bufferSize
//   [ 32] uint64_t sequence        ← ATOMIC, use iris_seq_*()
//   [ 40] uint32_t publishedIndex  ← ATOMIC, use iris_idx_*()
//   [ 44] uint32_t _pad0           ← explicit padding for alignment
//   [ 48] uint64_t presentationTimeNs  (plain, written under seq lock)
//   [ 56] uint64_t framesProduced  ← ATOMIC, use iris_fp_*()
//   [ 64] uint8_t  reserved[64]
//   [128] = total
// ----------------------------------------------------------------------------
typedef struct {
    uint32_t magic;                // [  0]
    uint32_t version;              // [  4]
    uint32_t width;                // [  8]
    uint32_t height;               // [ 12]
    uint32_t bytesPerRow;          // [ 16]
    uint32_t pixelFormat;          // [ 20]
    uint32_t bufferCount;          // [ 24]
    uint32_t bufferSize;           // [ 28]
    uint64_t sequence;             // [ 32] ATOMIC — use iris_seq_*()
    uint32_t publishedIndex;       // [ 40] ATOMIC — use iris_idx_*()
    uint32_t _pad0;                // [ 44] explicit pad
    uint64_t presentationTimeNs;   // [ 48]
    uint64_t framesProduced;       // [ 56] ATOMIC — use iris_fp_*()
    uint8_t  reserved[64];         // [ 64]
} IRISStreamHeader;                // [128] total

// Byte offsets used by AtomicHelpers.c must match the layout above.
#define IRIS_OFF_SEQUENCE        32u
#define IRIS_OFF_PUBLISHED_INDEX 40u
#define IRIS_OFF_FRAMES_PRODUCED 56u

// Compile-time size check (C11 _Static_assert).
_Static_assert(sizeof(IRISStreamHeader) == 128, "IRISStreamHeader must be exactly 128 bytes");

#define IRIS_HEADER_EXPECTED_SIZE 128u

// Helpers

// Compute aligned bytes-per-row for a given pixel width.
static inline uint32_t iris_bytes_per_row(uint32_t width) {
    uint32_t raw = width * 4u;
    return (raw + IRIS_ROW_ALIGNMENT - 1u) & ~(IRIS_ROW_ALIGNMENT - 1u);
}

// Total mmap size: header + 3 * bufferSize.
static inline uint64_t iris_mapping_size(uint32_t bufferSize) {
    return (uint64_t)IRIS_HEADER_EXPECTED_SIZE + (uint64_t)IRIS_BUFFER_COUNT * bufferSize;
}

// Pointer to frame buffer N inside a mapped region.
static inline void *iris_frame_ptr(void *base, uint32_t bufferSize, uint32_t index) {
    uint8_t *p = (uint8_t *)base;
    return p + IRIS_HEADER_EXPECTED_SIZE + (uint64_t)bufferSize * index;
}
