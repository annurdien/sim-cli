// AtomicHelpers.h
// Thin C wrappers around stdatomic operations on the IRISStreamHeader fields.
// These use void* / byte-offsets so Swift can call them without ever touching
// _Atomic fields directly (which Swift cannot import).
//
// IMPORTANT: Offsets must match IRISStreamHeader exactly.
// Verified by the static assertion in IrisProtocol.h.

#pragma once
#include <stdint.h>

#ifdef __cplusplus
extern "C" {
#endif

// ---- Sequence (offset 32 in IRISStreamHeader, uint64_t _Atomic) ------------
// See IRIS_OFF_SEQUENCE in IrisProtocol.h.

/// Acquire-load the sequence field.
uint64_t iris_seq_load_acquire(const void *header);

/// Store to the sequence field with release ordering.
void iris_seq_store_release(void *header, uint64_t value);

// ---- PublishedIndex (offset 40, uint32_t _Atomic) -------------------------
// See IRIS_OFF_PUBLISHED_INDEX in IrisProtocol.h.

/// Acquire-load the publishedIndex field.
uint32_t iris_idx_load_acquire(const void *header);

/// Relaxed-store to the publishedIndex field.
void iris_idx_store_relaxed(void *header, uint32_t value);

// ---- FramesProduced (offset 56, uint64_t _Atomic) -------------------------
// See IRIS_OFF_FRAMES_PRODUCED in IrisProtocol.h.

/// Relaxed-load the framesProduced field.
uint64_t iris_fp_load_relaxed(const void *header);

/// Relaxed fetch-and-add on framesProduced; returns the old value.
uint64_t iris_fp_fetch_add(void *header, uint64_t delta);

#ifdef __cplusplus
}
#endif
