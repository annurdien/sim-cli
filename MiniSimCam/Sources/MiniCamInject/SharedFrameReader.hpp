// SharedFrameReader.hpp
// C++ consumer for the shared-memory triple-buffer.
// Implements the sequence-lock read algorithm defined in MiniCamProtocol.h.

#pragma once
#include "MiniCamProtocol.h"
#include "MiniCamConstants.h"
#include <cstdint>
#include <string>
#include <vector>

/// Result returned by SharedFrameReader::copyLatestFrame.
struct FrameSnapshot {
    bool      valid        = false;
    uint32_t  width        = 0;
    uint32_t  height       = 0;
    uint32_t  bytesPerRow  = 0;
    uint64_t  ptsNs        = 0;    ///< Presentation timestamp (monotonic ns)
    uint64_t  framesProduced = 0;
    std::vector<uint8_t> data;     ///< Owned BGRA copy; length = bytesPerRow * height
};

/// Metadata returned by the zero-copy path (no pixel data).
struct FrameInfo {
    bool      valid        = false;
    uint32_t  width        = 0;
    uint32_t  height       = 0;
    uint32_t  bytesPerRow  = 0;
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

    /// Copy the latest published frame into a FrameSnapshot.
    /// Uses the sequence-lock algorithm to ensure a consistent read.
    /// Returns an invalid snapshot if the producer is stalled or the header is corrupt.
    FrameSnapshot copyLatestFrame();

    /// Zero-copy path: copy the latest frame directly into `dst` (caller-allocated).
    /// `dstBytesPerRow` describes the caller's row stride and must be at least
    /// the stream stride. `dstSize` must cover dstBytesPerRow * height.
    /// Returns metadata; returns invalid FrameInfo if the header is corrupt or
    /// the buffer is too small. Eliminates the intermediate std::vector allocation.
    FrameInfo copyLatestFrameInto(void* dst, size_t dstBytesPerRow, size_t dstSize);

    /// Check whether the producer has gone stale (no update within MSC_STALE_THRESHOLD_NS).
    bool isProducerStale() const;

private:
    std::string  path_;
    void*        mapping_     = nullptr;
    size_t       mappingSize_ = 0;
    int          fd_          = -1;

    MSCStreamHeader* header() const {
        return static_cast<MSCStreamHeader*>(mapping_);
    }
};
