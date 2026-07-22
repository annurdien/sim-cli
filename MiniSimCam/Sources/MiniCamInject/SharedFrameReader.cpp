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
#include <dispatch/dispatch.h>
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

    fd_ = ::open(path_.c_str(), O_RDWR);
    if (fd_ == -1) {
        // Fallback to read-only if we can't open for writing
        fd_ = ::open(path_.c_str(), O_RDONLY);
        if (fd_ == -1) return false;
    }

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

    size_t total = msc_mapping_size();

    struct stat st;
    if (fstat(fd_, &st) != 0 || (size_t)st.st_size < total) {
        ::close(fd_); fd_ = -1;
        return false;
    }

    void* ptr = mmap(nullptr, total, PROT_READ | PROT_WRITE, MAP_SHARED, fd_, 0);
    if (ptr == MAP_FAILED) {
        ptr = mmap(nullptr, total, PROT_READ, MAP_SHARED, fd_, 0);
        if (ptr == MAP_FAILED) {
            ::close(fd_); fd_ = -1;
            return false;
        }
    }

    mapping_     = ptr;
    mappingSize_ = total;
    
    setupVnodeMonitor();
    
    return true;
}

void SharedFrameReader::close() {
    teardownVnodeMonitor();
    if (mapping_) { munmap(mapping_, mappingSize_); mapping_ = nullptr; }
    if (fd_ != -1) { ::close(fd_); fd_ = -1; }
    mappingSize_ = 0;
}

void SharedFrameReader::setupVnodeMonitor() {
    if (fd_ == -1) return;
    
    dispatch_queue_t queue = dispatch_get_global_queue(DISPATCH_QUEUE_PRIORITY_DEFAULT, 0);
    dispatch_source_t source = dispatch_source_create(DISPATCH_SOURCE_TYPE_VNODE, fd_, DISPATCH_VNODE_DELETE | DISPATCH_VNODE_RENAME, queue);
    
    if (source) {
        dispatch_source_set_event_handler(source, ^{
            // The file was deleted or renamed (e.g. Mac changed resolution).
            // This allows the injector to try re-opening the file.
            // For now, we rely on the SampleBufferFactory polling isProducerStale() 
            // and checking if open() fails/succeeds on the next iteration.
            // In a fully event-driven setup, we would signal the factory here.
        });
        
        dispatch_source_set_cancel_handler(source, ^{
            // Clean up if needed
        });
        
        dispatch_resume(source);
        vnodeSource_ = source;
    }
}

void SharedFrameReader::teardownVnodeMonitor() {
    if (vnodeSource_) {
        dispatch_source_cancel((dispatch_source_t)vnodeSource_);
        dispatch_release((dispatch_source_t)vnodeSource_);
        vnodeSource_ = nullptr;
    }
}

FrameInfo SharedFrameReader::getLatestFrameInfo() {
    FrameInfo info;
    if (!mapping_) return info;

    auto* hdr = header();
    if (hdr->magic != MSC_MAGIC || hdr->version != MSC_VERSION) return info;

    const uint32_t w = hdr->width;
    const uint32_t h = hdr->height;

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

        uint32_t ioSurfaceID = 0;
        if (idx < 3) {
            ioSurfaceID = hdr->ioSurfaceIDs[idx];
        }

        uint64_t seqB = seqAtomic->load(std::memory_order_acquire);
        if (seqA != seqB) continue;   // Torn read — retry.

        info.valid          = true;
        info.width          = w;
        info.height         = h;
        info.ioSurfaceID    = ioSurfaceID;
        info.ptsNs          = pts;
        info.framesProduced = fp;
        return info;
    }

    return info;
}

bool SharedFrameReader::isProducerStale() const {
    if (!mapping_) return true;
    auto* hdr = header();
    return (monoNs() - hdr->presentationTimeNs) > MSC_STALE_THRESHOLD_NS;
}

bool SharedFrameReader::enqueueControlEvent(const MSCControlEvent& event) {
    if (!mapping_) return false;
    
    auto* headAtomic = atomicAt<uint32_t>(mapping_, MSC_OFF_CONTROL_HEAD);
    auto* tailAtomic = atomicAt<uint32_t>(mapping_, MSC_OFF_CONTROL_TAIL);
    
    uint32_t head = headAtomic->load(std::memory_order_acquire);
    uint32_t tail = tailAtomic->load(std::memory_order_acquire);
    
    uint32_t nextHead = head + 1;
    if (nextHead - tail > MSC_CONTROL_RING_CAPACITY) {
        // Buffer full
        return false;
    }
    
    auto* ring = msc_control_ring_ptr(mapping_);
    ring[head % MSC_CONTROL_RING_CAPACITY] = event;
    
    headAtomic->store(nextHead, std::memory_order_release);
    return true;
}
