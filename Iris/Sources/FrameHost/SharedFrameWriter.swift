// SharedFrameWriter.swift
// Creates and manages the shared-memory file and IOSurface-backed pixel buffer pool.

import Foundation
import Darwin
import CoreVideo
import IOSurface
import IrisShared

/// Writes IOSurfaceIDs into a memory-mapped header.
final class SharedFrameWriter {

    // MARK: - State

    private let path: String
    private var mapping: UnsafeMutableRawPointer?
    private var mappingSize: Int = 0

    private var pool: CVPixelBufferPool?
    private var poolWidth: Int = 0
    private var poolHeight: Int = 0
    
    // Retain buffers we create so they stay alive long enough to be read.
    private var ourRetainedBuffers: [CVPixelBuffer] = []
    
    // MARK: - Init / Teardown

    init(path: String) {
        self.path = path
    }

    deinit {
        close()
    }

    func open(width: Int, height: Int) throws {
        let totalSize = Int(IRIS_HEADER_EXPECTED_SIZE) // Just 128 bytes

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

        let hdr = ptr.bindMemory(to: IRISStreamHeader.self, capacity: 1)
        if needsInit {
            hdr.pointee.magic        = IRIS_MAGIC
            hdr.pointee.version      = IRIS_VERSION
            hdr.pointee.width        = UInt32(width)
            hdr.pointee.height       = UInt32(height)
            let bytesPerRow = ImageSource.alignedBytesPerRow(width)
            hdr.pointee.bytesPerRow  = UInt32(bytesPerRow)
            hdr.pointee.pixelFormat  = IRIS_PIXEL_FORMAT
        } else {
            // If we reused the file, repair sequence lock if odd
            let seq = iris_seq_load_acquire(ptr)
            if seq % 2 != 0 {
                iris_seq_store_release(ptr, seq &+ 1)
            }
        }
        ensurePool(width: width, height: height, pixelFormat: IRIS_PIXEL_FORMAT)
    }

    private var poolPixelFormat: OSType = 0

    private func ensurePool(width: Int, height: Int, pixelFormat: OSType) {
        if pool != nil && poolWidth == width && poolHeight == height && poolPixelFormat == pixelFormat { return }
        pool = nil
        ourRetainedBuffers.removeAll()
        poolWidth = width
        poolHeight = height
        poolPixelFormat = pixelFormat

        let poolAttributes: [String: Any] = [
            kCVPixelBufferPoolMinimumBufferCountKey as String: 6
        ]
        
        let bufferAttributes: [String: Any] = [
            kCVPixelBufferPixelFormatTypeKey as String: Int(pixelFormat),
            kCVPixelBufferWidthKey as String: width,
            kCVPixelBufferHeightKey as String: height,
            kCVPixelBufferBytesPerRowAlignmentKey as String: Int(IRIS_ROW_ALIGNMENT),
            kCVPixelBufferIOSurfacePropertiesKey as String: ["IOSurfaceIsGlobal": true]
        ]

        var newPool: CVPixelBufferPool?
        CVPixelBufferPoolCreate(kCFAllocatorDefault, poolAttributes as CFDictionary, bufferAttributes as CFDictionary, &newPool)
        self.pool = newPool
    }

    /// Closes the mapping and removes the shared file.
    func close() {
        if let mapping {
            munmap(mapping, mappingSize)
        }
        mapping = nil
        pool = nil
        ourRetainedBuffers.removeAll()
        try? FileManager.default.removeItem(atPath: path)
    }

    // MARK: - Publishing

    /// Publishes one BGRA frame using the sequence-lock algorithm.
    func publish(frame: BGRAFrame, pts: UInt64) throws {
        guard let _ = mapping, let pool = pool else { throw WriterError.notOpen }
        
        var pixelBuffer: CVPixelBuffer?
        guard CVPixelBufferPoolCreatePixelBuffer(kCFAllocatorDefault, pool, &pixelBuffer) == kCVReturnSuccess,
              let pixBuf = pixelBuffer else { return }

        CVPixelBufferLockBaseAddress(pixBuf, [])
        if let dest = CVPixelBufferGetBaseAddress(pixBuf) {
            _ = frame.data.withUnsafeBytes { src in
                memcpy(dest, src.baseAddress!, frame.data.count)
            }
        }
        CVPixelBufferUnlockBaseAddress(pixBuf, [])

        try publish(poolBuffer: pixBuf, pts: pts)
        
        ourRetainedBuffers.append(pixBuf)
        if ourRetainedBuffers.count > 3 {
            ourRetainedBuffers.removeFirst()
        }
    }

    /// Publishes one frame directly from a CVPixelBuffer by copying it into our global pool.
    func publish(pixelBuffer: CVPixelBuffer, pts: UInt64) throws {
        // We MUST copy incoming AVFoundation buffers into our pool because AVFoundation's
        // buffers are not marked as global IOSurfaces, so the simulator cannot map them!
        let width = CVPixelBufferGetWidth(pixelBuffer)
        let height = CVPixelBufferGetHeight(pixelBuffer)
        let fmt = CVPixelBufferGetPixelFormatType(pixelBuffer)
        
        ensurePool(width: width, height: height, pixelFormat: fmt)
        guard let pool = pool else { throw WriterError.notOpen }
        
        var newBuffer: CVPixelBuffer?
        guard CVPixelBufferPoolCreatePixelBuffer(kCFAllocatorDefault, pool, &newBuffer) == kCVReturnSuccess,
              let pixBuf = newBuffer else { return }

        pixelBuffer.copy(to: pixBuf)

        try publish(poolBuffer: pixBuf, pts: pts)
        
        ourRetainedBuffers.append(pixBuf)
        if ourRetainedBuffers.count > 3 {
            ourRetainedBuffers.removeFirst()
        }
    }

    private func publish(poolBuffer: CVPixelBuffer, pts: UInt64) throws {
        guard let base = mapping else { throw WriterError.notOpen }

        guard let ioSurface = CVPixelBufferGetIOSurface(poolBuffer) else { return }
        let surfaceID = IOSurfaceGetID(ioSurface.takeUnretainedValue())

        let seqBefore = iris_seq_load_acquire(base)
        iris_seq_store_release(base, seqBefore &+ 1)

        let hdr = base.bindMemory(to: IRISStreamHeader.self, capacity: 1)
        hdr.pointee.presentationTimeNs = pts
        hdr.pointee.width = UInt32(CVPixelBufferGetWidth(poolBuffer))
        hdr.pointee.height = UInt32(CVPixelBufferGetHeight(poolBuffer))
        hdr.pointee.bytesPerRow = UInt32(CVPixelBufferGetBytesPerRow(poolBuffer))
        hdr.pointee.pixelFormat = CVPixelBufferGetPixelFormatType(poolBuffer)

        iris_iosfc_store_relaxed(base, surfaceID)

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
