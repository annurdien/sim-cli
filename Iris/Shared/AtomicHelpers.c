// AtomicHelpers.c
// C implementations of the atomic accessor wrappers.
// Uses GCC/Clang __atomic_* builtins which work on any integer at a pointer.
// Swift calls these via the module map; C++ uses std::atomic<T>* casts directly.

#include "AtomicHelpers.h"
#include "IrisProtocol.h"
#include <stddef.h>

// Byte-pointer helpers using the offsets defined in IrisProtocol.h.
#define SEQ_PTR(h)  ((volatile uint64_t *)((uint8_t *)(h) + IRIS_OFF_SEQUENCE))
#define IDX_PTR(h)  ((volatile uint32_t *)((uint8_t *)(h) + IRIS_OFF_PUBLISHED_INDEX))
#define FP_PTR(h)   ((volatile uint64_t *)((uint8_t *)(h) + IRIS_OFF_FRAMES_PRODUCED))

// Sequence

uint64_t iris_seq_load_acquire(const void *header) {
    return __atomic_load_n(SEQ_PTR(header), __ATOMIC_ACQUIRE);
}

void iris_seq_store_release(void *header, uint64_t value) {
    __atomic_store_n(SEQ_PTR(header), value, __ATOMIC_RELEASE);
}

// PublishedIndex

uint32_t iris_idx_load_acquire(const void *header) {
    return __atomic_load_n(IDX_PTR(header), __ATOMIC_ACQUIRE);
}

void iris_idx_store_relaxed(void *header, uint32_t value) {
    __atomic_store_n(IDX_PTR(header), value, __ATOMIC_RELAXED);
}

// FramesProduced

uint64_t iris_fp_load_relaxed(const void *header) {
    return __atomic_load_n(FP_PTR(header), __ATOMIC_RELAXED);
}

uint64_t iris_fp_fetch_add(void *header, uint64_t delta) {
    return __atomic_fetch_add(FP_PTR(header), delta, __ATOMIC_RELAXED);
}
