// SharedFrameReader.cpp
// Sequence-lock consumer for the shared-memory triple-buffer.
// The struct fields are plain integers; we cast their addresses to
// std::atomic<T>* for correct cross-process atomic semantics (C++20
// guarantees this is well-defined when the natural alignment matches).

#include "SharedFrameReader.hpp"
#include <sys/mman.h>
#include <sys/stat.h>
#include <fcntl.h>
#include <unistd.h>
#include <atomic>
#include <cstring>
#include <mach/mach_time.h>

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

static uint64_t monoNs() {
    static mach_timebase_info_data_t info;
    static std::once_flag flag;
    std::call_once(flag, []{ mach_timebase_info(&info); });
    return mach_absolute_time() * info.numer / info.denom;
}

// Reinterpret a plain integer field as a lock-free atomic reference.
template<typename T>
static std::atomic<T>* atomicAt(void* base, size_t offset) {
    return reinterpret_cast<std::atomic<T>*>(
        static_cast<uint8_t*>(base) + offset
    );
}
template<typename T>
static const std::atomic<T>* atomicAt(const void* base, size_t offset) {
    return reinterpret_cast<const std::atomic<T>*>(
        static_cast<const uint8_t*>(base) + offset
    );
}

// Offsets matching MSCStreamHeader in MiniCamProtocol.h.
constexpr size_t kOffSequence       = MSC_OFF_SEQUENCE;         // 32
constexpr size_t kOffPublishedIndex = MSC_OFF_PUBLISHED_INDEX;  // 40
constexpr size_t kOffFramesProduced = MSC_OFF_FRAMES_PRODUCED;  // 56

// ---------------------------------------------------------------------------
// SharedFrameReader
// ---------------------------------------------------------------------------

SharedFrameReader::SharedFrameReader(const std::string& path)
    : path_(path) {}

SharedFrameReader::~SharedFrameReader() {
    close();
}

bool SharedFrameReader::open() {
    close();

    fd_ = ::open(path_.c_str(), O_RDONLY);
    if (fd_ == -1) return false;

    // Read the header to discover the mapping size.
    MSCStreamHeader hdr = {};
    if (::read(fd_, &hdr, sizeof(hdr)) != (ssize_t)sizeof(hdr)) {
        ::close(fd_); fd_ = -1;
        return false;
    }

    if (hdr.magic != MSC_MAGIC || hdr.version != MSC_VERSION) {
        ::close(fd_); fd_ = -1;
        return false;
    }

    size_t total = msc_mapping_size(hdr.bufferSize);

    struct stat st;
    if (fstat(fd_, &st) != 0 || (size_t)st.st_size < total) {
        ::close(fd_); fd_ = -1;
        return false;
    }

    void* ptr = mmap(nullptr, total, PROT_READ, MAP_SHARED, fd_, 0);
    if (ptr == MAP_FAILED) {
        ::close(fd_); fd_ = -1;
        return false;
    }

    mapping_     = ptr;
    mappingSize_ = total;
    return true;
}

void SharedFrameReader::close() {
    if (mapping_) { munmap(mapping_, mappingSize_); mapping_ = nullptr; }
    if (fd_ != -1) { ::close(fd_); fd_ = -1; }
    mappingSize_ = 0;
}

FrameSnapshot SharedFrameReader::copyLatestFrame() {
    FrameSnapshot snap;
    if (!mapping_) return snap;

    auto* hdr = header();
    if (hdr->magic != MSC_MAGIC || hdr->version != MSC_VERSION) return snap;

    const uint32_t bufSize = hdr->bufferSize;
    const uint32_t bpr     = hdr->bytesPerRow;
    const uint32_t w       = hdr->width;
    const uint32_t h       = hdr->height;

    auto* seqAtomic = atomicAt<uint64_t>(mapping_, kOffSequence);
    auto* idxAtomic = atomicAt<uint32_t>(mapping_, kOffPublishedIndex);
    auto* fpAtomic  = atomicAt<uint64_t>(mapping_, kOffFramesProduced);

    // Sequence-lock read: retry up to 64 times.
    for (int attempt = 0; attempt < 64; ++attempt) {
        uint64_t seqA = seqAtomic->load(std::memory_order_acquire);
        if (seqA & 1u) {
            sched_yield();
            continue;
        }

        uint32_t idx  = idxAtomic->load(std::memory_order_acquire);
        uint64_t pts  = hdr->presentationTimeNs;
        uint64_t fp   = fpAtomic->load(std::memory_order_acquire);

        if (idx >= MSC_BUFFER_COUNT) break;

        const uint8_t* src = static_cast<const uint8_t*>(
            msc_frame_ptr(mapping_, bufSize, idx)
        );

        std::vector<uint8_t> pixels(bufSize);
        std::memcpy(pixels.data(), src, bufSize);

        uint64_t seqB = seqAtomic->load(std::memory_order_acquire);
        if (seqA != seqB) continue;   // Torn read — retry.

        snap.valid          = true;
        snap.width          = w;
        snap.height         = h;
        snap.bytesPerRow    = bpr;
        snap.ptsNs          = pts;
        snap.framesProduced = fp;
        snap.data           = std::move(pixels);
        return snap;
    }

    return snap;
}

bool SharedFrameReader::isProducerStale() const {
    if (!mapping_) return true;
    auto* hdr = header();
    return (monoNs() - hdr->presentationTimeNs) > MSC_STALE_THRESHOLD_NS;
}

FrameInfo SharedFrameReader::copyLatestFrameInto(void* dst, size_t dstBytesPerRow, size_t dstSize) {
    FrameInfo info;
    if (!mapping_ || !dst) return info;

    auto* hdr = header();
    if (hdr->magic != MSC_MAGIC || hdr->version != MSC_VERSION) return info;

    const uint32_t bufSize = hdr->bufferSize;
    const uint32_t bpr     = hdr->bytesPerRow;
    const uint32_t w       = hdr->width;
    const uint32_t h       = hdr->height;

    if (dstBytesPerRow < bpr || dstSize < dstBytesPerRow * h) return info;

    auto* seqAtomic = atomicAt<uint64_t>(mapping_, kOffSequence);
    auto* idxAtomic = atomicAt<uint32_t>(mapping_, kOffPublishedIndex);
    auto* fpAtomic  = atomicAt<uint64_t>(mapping_, kOffFramesProduced);

    // Sequence-lock read: retry up to 64 times.
    for (int attempt = 0; attempt < 64; ++attempt) {
        uint64_t seqA = seqAtomic->load(std::memory_order_acquire);
        if (seqA & 1u) {
            sched_yield();
            continue;
        }

        uint32_t idx = idxAtomic->load(std::memory_order_acquire);
        uint64_t pts = hdr->presentationTimeNs;
        uint64_t fp  = fpAtomic->load(std::memory_order_acquire);

        if (idx >= MSC_BUFFER_COUNT) break;

        const uint8_t* src = static_cast<const uint8_t*>(
            msc_frame_ptr(mapping_, bufSize, idx)
        );
        uint8_t* out = static_cast<uint8_t*>(dst);
        // The stream and CVPixelBuffer may use different strides. Copy each
        // source row into the destination row rather than treating both as a
        // contiguous image.
        for (uint32_t row = 0; row < h; ++row) {
            std::memcpy(out + row * dstBytesPerRow, src + row * bpr, bpr);
        }

        uint64_t seqB = seqAtomic->load(std::memory_order_acquire);
        if (seqA != seqB) continue;   // Torn read — retry.

        info.valid          = true;
        info.width          = w;
        info.height         = h;
        info.bytesPerRow    = bpr;
        info.ptsNs          = pts;
        info.framesProduced = fp;
        return info;
    }

    return info;
}
