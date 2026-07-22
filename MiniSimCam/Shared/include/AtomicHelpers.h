// AtomicHelpers.h
// Thin C wrappers around stdatomic operations on the MSCStreamHeader fields.
// These use void* / byte-offsets so Swift can call them without ever touching
// _Atomic fields directly (which Swift cannot import).
//
// IMPORTANT: Offsets must match MSCStreamHeader exactly.
// Verified by the static assertion in MiniCamProtocol.h.

#pragma once
#include <stdint.h>

#ifdef __cplusplus
extern "C" {
#endif

// ---- Sequence (offset 32 in MSCStreamHeader, uint64_t _Atomic) -------------
// See MSC_OFF_SEQUENCE in MiniCamProtocol.h.

/// Acquire-load the sequence field.
uint64_t msc_seq_load_acquire(const void *header);

/// Store to the sequence field with release ordering.
void msc_seq_store_release(void *header, uint64_t value);

// ---- PublishedIndex (offset 40, uint32_t _Atomic) -------------------------
// See MSC_OFF_PUBLISHED_INDEX in MiniCamProtocol.h.

/// Acquire-load the publishedIndex field.
uint32_t msc_idx_load_acquire(const void *header);

/// Relaxed-store to the publishedIndex field.
void msc_idx_store_relaxed(void *header, uint32_t value);

// ---- FramesProduced (offset 56, uint64_t _Atomic) -------------------------
// See MSC_OFF_FRAMES_PRODUCED in MiniCamProtocol.h.

/// Relaxed-load the framesProduced field.
uint64_t msc_fp_load_relaxed(const void *header);

/// Relaxed fetch-and-add on framesProduced; returns the old value.
uint64_t msc_fp_fetch_add(void *header, uint64_t delta);

// ---- Control Channel (offsets 16 and 24, uint32_t _Atomic) ----------------
// See MSC_OFF_CONTROL_HEAD / MSC_OFF_CONTROL_TAIL.

/// Acquire-load the controlHead field.
uint32_t msc_ctl_head_load_acquire(const void *header);

/// Release-store the controlHead field.
void msc_ctl_head_store_release(void *header, uint32_t value);

/// Acquire-load the controlTail field.
uint32_t msc_ctl_tail_load_acquire(const void *header);

/// Release-store the controlTail field.
void msc_ctl_tail_store_release(void *header, uint32_t value);

#ifdef __cplusplus
}
#endif
