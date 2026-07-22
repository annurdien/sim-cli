// SharedFrameReader.hpp
// C++ consumer for the shared-memory triple-buffer.
// Implements the sequence-lock read algorithm defined in MiniCamProtocol.h.

#pragma once
#include "MiniCamProtocol.h"
#include "MiniCamConstants.h"
#include <cstdint>
#include <string>
#include <vector>

/// Metadata returned by the zero-copy path.
struct FrameInfo {
    bool      valid        = false;
    uint32_t  width        = 0;
    uint32_t  height       = 0;
    uint32_t  ioSurfaceID  = 0;
    uint64_t  ptsNs        = 0;
    uint64_t  framesProduced = 0;
};

class SharedFrameReader {
public:
    explicit SharedFrameReader(const std::string& path);
    ~SharedFrameReader();

    // Non-copyable, non-moveable.
    SharedFrameReader(const SharedFrameReader&)            = delete;
    SharedFrameReader& operator=(const SharedFrameReader&) = delete;

    /// Open and memory-map the shared file.
    /// Returns false if the file does not exist or the header is invalid.
    bool open();

    /// Close the mapping (idempotent).
    void close();

    bool isOpen() const { return mapping_ != nullptr; }

    /// Read-only pointer to the stream header. Returns nullptr if not open.
    /// Valid only while the mapping is open; do NOT cache across open/close.
    const MSCStreamHeader* peekHeader() const {
        return mapping_ ? static_cast<const MSCStreamHeader*>(mapping_) : nullptr;
    }

    /// Gets the latest frame metadata, including the IOSurfaceID.
    /// Uses the sequence-lock algorithm to ensure a consistent read.
    FrameInfo getLatestFrameInfo();

    /// Check whether the producer has gone stale (no update within MSC_STALE_THRESHOLD_NS).
    bool isProducerStale() const;

    /// Enqueue a control event to the shared memory ring buffer.
    /// Returns true if enqueued successfully, false if the buffer is full or disconnected.
    bool enqueueControlEvent(const MSCControlEvent& event);

private:
    void setupVnodeMonitor();
    void teardownVnodeMonitor();

    std::string  path_;
    void*        mapping_     = nullptr;
    size_t       mappingSize_ = 0;
    int          fd_          = -1;
    
    void*        vnodeSource_ = nullptr; // dispatch_source_t

    MSCStreamHeader* header() const {
        return static_cast<MSCStreamHeader*>(mapping_);
    }
};
