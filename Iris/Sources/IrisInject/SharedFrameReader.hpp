// SharedFrameReader.hpp
// C++ consumer for the shared-memory.
// Implements the sequence-lock read algorithm to safely extract the IOSurfaceID.

#pragma once
#include "IrisProtocol.h"
#include "IrisConstants.h"
#include <cstdint>
#include <string>

/// Result returned by SharedFrameReader::copyLatestFrame.
struct FrameSnapshot {
    bool      valid        = false;
    uint32_t  width        = 0;
    uint32_t  height       = 0;
    uint32_t  bytesPerRow  = 0;
    uint64_t  ptsNs        = 0;    ///< Presentation timestamp (monotonic ns)
    uint64_t  framesProduced = 0;
    uint32_t  ioSurfaceID  = 0;    ///< The zero-copy IOSurface ID
};

class SharedFrameReader {
public:
    explicit SharedFrameReader(const std::string& path);
    ~SharedFrameReader();

    // Non-copyable, non-moveable.
    SharedFrameReader(const SharedFrameReader&)            = delete;
    SharedFrameReader& operator=(const SharedFrameReader&) = delete;

    /// Open and memory-map the shared file.
    bool open();

    /// Close the mapping (idempotent).
    void close();

    bool isOpen() const { return mapping_ != nullptr; }

    const IRISStreamHeader* peekHeader() const {
        return mapping_ ? static_cast<const IRISStreamHeader*>(mapping_) : nullptr;
    }

    /// Read the latest frame metadata and IOSurfaceID using the sequence-lock.
    FrameSnapshot copyLatestFrame();

    /// Check whether the producer has gone stale (no update within IRIS_STALE_THRESHOLD_NS).
    bool isProducerStale() const;

private:
    std::string  path_;
    void*        mapping_     = nullptr;
    size_t       mappingSize_ = 0;
    int          fd_          = -1;

    IRISStreamHeader* header() const {
        return static_cast<IRISStreamHeader*>(mapping_);
    }
};
