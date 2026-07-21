// SharedFrameWriter.swift
// Creates and manages the shared-memory triple-buffer that the injector reads.

import Foundation
import Darwin
import MiniSimCamShared

/// Writes BGRA frames into a memory-mapped triple-buffer file.
///
/// Layout (offsets):
///   [0 ..< 128]           MSCStreamHeader
///   [128 ..< 128+N]       Frame buffer 0
///   [128+N ..< 128+2N]    Frame buffer 1
///   [128+2N ..< 128+3N]   Frame buffer 2
///
/// where N = header.bufferSize.
final class SharedFrameWriter {

    // MARK: - State

    private let path: String
    private var mapping: UnsafeMutableRawPointer?
    private var mappingSize: Int = 0
    private var frameSize: Int = 0

    // MARK: - Init / Teardown

    init(path: String) {
        self.path = path
    }

    deinit {
        close()
    }

    /// Opens (or re-creates) the shared file and writes the stream header.
    func open(width: Int, height: Int) throws {
        let bytesPerRow = ImageSource.alignedBytesPerRow(width)
        let bufSize = bytesPerRow * height
        let totalSize = 128 + 3 * bufSize     // header (128 B) + 3 × frame

        // Remove stale file if present.
        if FileManager.default.fileExists(atPath: path) {
            try FileManager.default.removeItem(atPath: path)
        }

        // Create and size the file.
        guard FileManager.default.createFile(atPath: path, contents: nil) else {
            throw WriterError.cannotCreateFile(path)
        }
        let fd = Darwin.open(path, O_RDWR)
        guard fd != -1 else { throw WriterError.openFailed(path, errno: errno) }
        defer { Darwin.close(fd) }

        guard ftruncate(fd, off_t(totalSize)) == 0 else {
            throw WriterError.truncateFailed(errno: errno)
        }

        // mmap read-write shared. ftruncate zeroes all bytes, so
        // sequence=0 (even) and publishedIndex=0 are correct initial values.
        let ptr = mmap(nil, totalSize, PROT_READ | PROT_WRITE, MAP_SHARED, fd, 0)
        guard let ptr, ptr != MAP_FAILED else {
            throw WriterError.mmapFailed(errno: errno)
        }

        mapping = ptr
        mappingSize = totalSize
        frameSize = bufSize

        // Write the non-atomic header fields directly (Swift can access these).
        let hdr = ptr.bindMemory(to: MSCStreamHeader.self, capacity: 1)
        hdr.pointee.magic        = MSC_MAGIC
        hdr.pointee.version      = MSC_VERSION
        hdr.pointee.width        = UInt32(width)
        hdr.pointee.height       = UInt32(height)
        hdr.pointee.bytesPerRow  = UInt32(bytesPerRow)
        hdr.pointee.pixelFormat  = MSC_PIXEL_FORMAT
        hdr.pointee.bufferCount  = MSC_BUFFER_COUNT
        hdr.pointee.bufferSize   = UInt32(bufSize)
        // sequence, publishedIndex, framesProduced remain 0 from ftruncate.
    }

    /// Closes the mapping and removes the shared file.
    func close() {
        if let mapping {
            munmap(mapping, mappingSize)
        }
        mapping = nil
        try? FileManager.default.removeItem(atPath: path)
    }

    // MARK: - Publishing

    /// Publishes one BGRA frame using the sequence-lock algorithm.
    /// - Parameter frame: The BGRA frame to publish.
    /// - Parameter pts:   Presentation timestamp in nanoseconds (monotonic).
    func publish(frame: BGRAFrame, pts: UInt64) throws {
        guard let base = mapping else {
            throw WriterError.notOpen
        }

        // Pick a buffer index that is NOT currently the published one.
        let currentPublished = Int(msc_idx_load_acquire(base))
        let writeIndex = (currentPublished + 1) % 3

        // --- Sequence lock: begin write (seq → odd) ---
        // Canonical pattern: read seq, store seq+1 (odd = write in progress).
        // On completion store seq+2 (even, advanced). Never re-read in between.
        let seqBefore = msc_seq_load_acquire(base)
        msc_seq_store_release(base, seqBefore &+ 1)

        // --- Copy frame data ---
        let dest = base.advanced(by: 128 + writeIndex * frameSize)
        _ = frame.data.withUnsafeBytes { src in
            memcpy(dest, src.baseAddress!, frame.data.count)
        }

        // --- Update presentation timestamp (plain write, protected by seq lock) ---
        let hdr = base.bindMemory(to: MSCStreamHeader.self, capacity: 1)
        hdr.pointee.presentationTimeNs = pts

        // --- Publish buffer index ---
        msc_idx_store_relaxed(base, UInt32(writeIndex))

        // --- Sequence lock: end write (seq → even, advanced by 2 total) ---
        msc_seq_store_release(base, seqBefore &+ 2)

        // --- Increment frame counter ---
        _ = msc_fp_fetch_add(base, 1)
    }

    /// Returns the number of frames successfully published so far.
    var framesProduced: UInt64 {
        guard let base = mapping else { return 0 }
        return msc_fp_load_relaxed(base)
    }
}

// MARK: - Errors

enum WriterError: Error, CustomStringConvertible {
    case cannotCreateFile(String)
    case openFailed(String, errno: Int32)
    case truncateFailed(errno: Int32)
    case mmapFailed(errno: Int32)
    case notOpen

    var description: String {
        switch self {
        case .cannotCreateFile(let p):    return "Cannot create file at \(p)"
        case .openFailed(let p, let e):   return "open(\(p)) failed: \(String(cString: strerror(e)))"
        case .truncateFailed(let e):      return "ftruncate failed: \(String(cString: strerror(e)))"
        case .mmapFailed(let e):          return "mmap failed: \(String(cString: strerror(e)))"
        case .notOpen:                    return "Writer is not open"
        }
    }
}
