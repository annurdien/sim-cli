// SharedFrameWriter.swift
// Creates and manages the shared-memory triple-buffer that the injector reads.

import Foundation
import Darwin
import CoreVideo
import IrisShared

/// Writes BGRA frames into a memory-mapped triple-buffer file.
///
/// Layout (offsets):
///   [0 ..< 128]           IRISStreamHeader
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

    func open(width: Int, height: Int) throws {
        let bytesPerRow = ImageSource.alignedBytesPerRow(width)
        let bufSize = bytesPerRow * height
        let totalSize = 128 + 3 * bufSize     // header (128 B) + 3 × frame

        var fd = Darwin.open(path, O_RDWR)
        var needsInit = false

        if fd != -1 {
            var statBuf = stat()
            if fstat(fd, &statBuf) == 0 && statBuf.st_size == totalSize {
                // File exists and is the exact size we need. Reuse it!
            } else {
                // Size mismatch. Close and recreate.
                Darwin.close(fd)
                fd = -1
            }
        }

        if fd == -1 {
            // Recreate file
            if FileManager.default.fileExists(atPath: path) {
                try? FileManager.default.removeItem(atPath: path)
            }
            guard FileManager.default.createFile(atPath: path, contents: nil) else {
                throw WriterError.cannotCreateFile(path)
            }
            fd = Darwin.open(path, O_RDWR)
            guard fd != -1 else { throw WriterError.openFailed(path, errno: errno) }
            
            guard ftruncate(fd, off_t(totalSize)) == 0 else {
                Darwin.close(fd)
                throw WriterError.truncateFailed(errno: errno)
            }
            needsInit = true
        }
        
        defer { Darwin.close(fd) }

        // mmap read-write shared
        let ptr = mmap(nil, totalSize, PROT_READ | PROT_WRITE, MAP_SHARED, fd, 0)
        guard let ptr, ptr != MAP_FAILED else {
            throw WriterError.mmapFailed(errno: errno)
        }

        mapping = ptr
        mappingSize = totalSize
        frameSize = bufSize

        let hdr = ptr.bindMemory(to: IRISStreamHeader.self, capacity: 1)
        if needsInit {
            hdr.pointee.magic        = IRIS_MAGIC
            hdr.pointee.version      = IRIS_VERSION
            hdr.pointee.width        = UInt32(width)
            hdr.pointee.height       = UInt32(height)
            hdr.pointee.bytesPerRow  = UInt32(bytesPerRow)
            hdr.pointee.pixelFormat  = IRIS_PIXEL_FORMAT
            hdr.pointee.bufferCount  = IRIS_BUFFER_COUNT
            hdr.pointee.bufferSize   = UInt32(bufSize)
            // sequence, publishedIndex, framesProduced remain 0 from ftruncate.
        } else {
            // If we reused the file, the previous FrameHost might have died in the middle of a write.
            // If the sequence lock is odd (write in progress), force it to even.
            let seq = iris_seq_load_acquire(ptr)
            if seq % 2 != 0 {
                iris_seq_store_release(ptr, seq &+ 1)
            }
        }
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
        let currentPublished = Int(iris_idx_load_acquire(base))
        let writeIndex = (currentPublished + 1) % 3

        // --- Sequence lock: begin write (seq → odd) ---
        // Canonical pattern: read seq, store seq+1 (odd = write in progress).
        // On completion store seq+2 (even, advanced). Never re-read in between.
        let seqBefore = iris_seq_load_acquire(base)
        iris_seq_store_release(base, seqBefore &+ 1)

        // --- Copy frame data ---
        let dest = base.advanced(by: 128 + writeIndex * frameSize)
        _ = frame.data.withUnsafeBytes { src in
            memcpy(dest, src.baseAddress!, frame.data.count)
        }

        // --- Update presentation timestamp (plain write, protected by seq lock) ---
        let hdr = base.bindMemory(to: IRISStreamHeader.self, capacity: 1)
        hdr.pointee.presentationTimeNs = pts

        // --- Publish buffer index ---
        iris_idx_store_relaxed(base, UInt32(writeIndex))

        // --- Sequence lock: end write (seq → even, advanced by 2 total) ---
        iris_seq_store_release(base, seqBefore &+ 2)

        // --- Increment frame counter ---
        _ = iris_fp_fetch_add(base, 1)
    }

    /// Publishes one BGRA frame directly from a CVPixelBuffer (zero-copy).
    /// - Parameter pixelBuffer: A CVPixelBuffer (must be kCVPixelFormatType_32BGRA).
    /// - Parameter pts:         Presentation timestamp in nanoseconds.
    func publish(pixelBuffer: CVPixelBuffer, pts: UInt64) throws {
        guard let base = mapping else {
            throw WriterError.notOpen
        }

        // Lock the base address for reading
        CVPixelBufferLockBaseAddress(pixelBuffer, .readOnly)
        defer { CVPixelBufferUnlockBaseAddress(pixelBuffer, .readOnly) }

        guard let srcBaseAddress = CVPixelBufferGetBaseAddress(pixelBuffer) else {
            return
        }

        let currentPublished = Int(iris_idx_load_acquire(base))
        let writeIndex = (currentPublished + 1) % 3

        let seqBefore = iris_seq_load_acquire(base)
        iris_seq_store_release(base, seqBefore &+ 1)

        // We must copy row-by-row to handle stride (bytesPerRow) differences
        // between the camera's CVPixelBuffer and our shared memory.
        let srcBPR = CVPixelBufferGetBytesPerRow(pixelBuffer)
        let srcWidth = CVPixelBufferGetWidth(pixelBuffer)
        let srcHeight = CVPixelBufferGetHeight(pixelBuffer)

        let hdr = base.bindMemory(to: IRISStreamHeader.self, capacity: 1)
        let dstWidth = Int(hdr.pointee.width)
        let dstHeight = Int(hdr.pointee.height)
        let dstBPR = Int(hdr.pointee.bytesPerRow)

        let copyWidth = min(srcWidth, dstWidth)
        let copyHeight = min(srcHeight, dstHeight)
        let bytesToCopy = copyWidth * 4

        let destBase = base.advanced(by: 128 + writeIndex * frameSize)

        for row in 0..<copyHeight {
            let srcRow = srcBaseAddress.advanced(by: row * srcBPR)
            let dstRow = destBase.advanced(by: row * dstBPR)
            memcpy(dstRow, srcRow, bytesToCopy)
        }

        hdr.pointee.presentationTimeNs = pts

        iris_idx_store_relaxed(base, UInt32(writeIndex))
        iris_seq_store_release(base, seqBefore &+ 2)

        _ = iris_fp_fetch_add(base, 1)
    }

    /// Returns the number of frames successfully published so far.
    var framesProduced: UInt64 {
        guard let base = mapping else { return 0 }
        return iris_fp_load_relaxed(base)
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
