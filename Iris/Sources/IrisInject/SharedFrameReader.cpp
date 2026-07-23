// SharedFrameReader.cpp
// Sequence-lock consumer for the shared-memory header.
// Retrieves the IOSurfaceID instead of raw pixels.

#include "SharedFrameReader.hpp"
#include <sys/mman.h>
#include <sys/stat.h>
#include <fcntl.h>
#include <unistd.h>
#include <atomic>
#include <cstring>
#include <mach/mach_time.h>
#include <mutex>

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

// Offsets matching IRISStreamHeader in IrisProtocol.h.
constexpr size_t kOffSequence       = IRIS_OFF_SEQUENCE;         // 32
constexpr size_t kOffIOSurfaceID    = IRIS_OFF_IOSURFACE_ID;     // 40
constexpr size_t kOffFramesProduced = IRIS_OFF_FRAMES_PRODUCED;  // 56

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
    IRISStreamHeader hdr = {};
    if (::read(fd_, &hdr, sizeof(hdr)) != (ssize_t)sizeof(hdr)) {
        ::close(fd_); fd_ = -1;
        return false;
    }

    if (hdr.magic != IRIS_MAGIC || hdr.version != IRIS_VERSION) {
        ::close(fd_); fd_ = -1;
        return false;
    }

    size_t total = iris_mapping_size();

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
    if (hdr->magic != IRIS_MAGIC || hdr->version != IRIS_VERSION) return snap;

    const uint32_t bpr = hdr->bytesPerRow;
    const uint32_t w   = hdr->width;
    const uint32_t h   = hdr->height;

    auto* seqAtomic = atomicAt<uint64_t>(mapping_, kOffSequence);
    auto* sfcAtomic = atomicAt<uint32_t>(mapping_, kOffIOSurfaceID);
    auto* fpAtomic  = atomicAt<uint64_t>(mapping_, kOffFramesProduced);

    // Sequence-lock read: retry up to 64 times.
    for (int attempt = 0; attempt < 64; ++attempt) {
        uint64_t seqA = seqAtomic->load(std::memory_order_acquire);
        if (seqA & 1u) {
            sched_yield();
            continue;
        }

        uint32_t sfcID = sfcAtomic->load(std::memory_order_acquire);
        uint64_t pts   = hdr->presentationTimeNs;
        uint64_t fp    = fpAtomic->load(std::memory_order_acquire);

        uint64_t seqB = seqAtomic->load(std::memory_order_acquire);
        if (seqA != seqB) continue;   // Torn read — retry.

        snap.valid          = true;
        snap.width          = w;
        snap.height         = h;
        snap.bytesPerRow    = bpr;
        snap.ptsNs          = pts;
        snap.framesProduced = fp;
        snap.ioSurfaceID    = sfcID;
        return snap;
    }

    return snap;
}

bool SharedFrameReader::isProducerStale() const {
    if (!mapping_) return true;
    auto* hdr = header();
    return (monoNs() - hdr->presentationTimeNs) > IRIS_STALE_THRESHOLD_NS;
}
