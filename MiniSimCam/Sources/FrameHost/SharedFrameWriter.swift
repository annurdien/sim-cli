// SharedFrameWriter.swift
// Creates and manages the shared-memory triple-buffer that the injector reads.

import Foundation
import Darwin
import CoreVideo
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
    private var retainedBuffers: [CVPixelBuffer?] = [nil, nil, nil]

    // MARK: - Init / Teardown

    init(path: String) {
        self.path = path
    }

    deinit {
        close()
    }

    func open(width: Int, height: Int) throws {
        let totalSize = 128 + 16 * 12 // header (128 B) + 16 x MSCControlEvent (12B)

        var fd = Darwin.open(path, O_RDWR)
        var needsInit = false

        if fd != -1 {
            var statBuf = stat()
            if fstat(fd, &statBuf) == 0 && statBuf.st_size == totalSize {
                // File exists and is the exact size we need. Reuse it!
            } else {
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

        let ptr = mmap(nil, totalSize, PROT_READ | PROT_WRITE, MAP_SHARED, fd, 0)
        guard let ptr, ptr != MAP_FAILED else {
            throw WriterError.mmapFailed(errno: errno)
        }

        mapping = ptr
        mappingSize = totalSize
        frameSize = 0 // Not used anymore

        let hdr = ptr.bindMemory(to: MSCStreamHeader.self, capacity: 1)
        if needsInit {
            hdr.pointee.magic        = MSC_MAGIC
            hdr.pointee.version      = MSC_VERSION
            hdr.pointee.width        = UInt32(width)
            hdr.pointee.height       = UInt32(height)
            hdr.pointee.controlHead  = 0
            hdr.pointee.controlTail  = 0
            hdr.pointee.pixelFormat  = MSC_PIXEL_FORMAT
            hdr.pointee.ioSurfaceID  = 0
            hdr.pointee.sequence     = 0
            hdr.pointee.publishedIndex = 0
            hdr.pointee.framesProduced = 0
            memset(&hdr.pointee.ioSurfaceIDs, 0, MemoryLayout.size(ofValue: hdr.pointee.ioSurfaceIDs))
        } else {
            let seq = msc_seq_load_acquire(ptr)
            if seq % 2 != 0 {
                msc_seq_store_release(ptr, seq &+ 1)
            }
        }
    }

    func close() {
        if let mapping {
            munmap(mapping, mappingSize)
        }
        mapping = nil
        try? FileManager.default.removeItem(atPath: path)
    }

    // MARK: - Publishing

    func publish(frame: BGRAFrame, pts: UInt64) throws {
        // Create an IOSurface-backed CVPixelBuffer
        let attrs = [
            kCVPixelBufferIOSurfacePropertiesKey: [
                "IOSurfaceIsGlobal": true
            ] as [String: Any]
        ] as CFDictionary
        
        var pixelBuffer: CVPixelBuffer?
        CVPixelBufferCreate(
            kCFAllocatorDefault,
            frame.width,
            frame.height,
            kCVPixelFormatType_32BGRA,
            attrs,
            &pixelBuffer
        )
        
        guard let pixelBuffer else { return }
        
        CVPixelBufferLockBaseAddress(pixelBuffer, [])
        if let baseAddress = CVPixelBufferGetBaseAddress(pixelBuffer) {
            frame.data.withUnsafeBytes { ptr in
                if let src = ptr.baseAddress {
                    let dstBPR = CVPixelBufferGetBytesPerRow(pixelBuffer)
                    let srcBPR = frame.width * 4
                    if dstBPR == srcBPR {
                        memcpy(baseAddress, src, frame.data.count)
                    } else {
                        let h = frame.height
                        for row in 0..<h {
                            memcpy(baseAddress.advanced(by: row * dstBPR),
                                   src.advanced(by: row * srcBPR),
                                   srcBPR)
                        }
                    }
                }
            }
        }
        CVPixelBufferUnlockBaseAddress(pixelBuffer, [])
        
        try publish(pixelBuffer: pixelBuffer, pts: pts)
    }

    func publish(pixelBuffer: CVPixelBuffer, pts: UInt64) throws {
        guard let base = mapping else {
            throw WriterError.notOpen
        }

        let unmanagedSurface = CVPixelBufferGetIOSurface(pixelBuffer)
        guard let surface = unmanagedSurface?.takeUnretainedValue() else {
            return
        }
        let surfaceID = IOSurfaceGetID(surface)

        let currentPublished = Int(msc_idx_load_acquire(base))
        let writeIndex = (currentPublished + 1) % 3

        // Retain the CVPixelBuffer to keep the IOSurface alive while the reader consumes it.
        retainedBuffers[writeIndex] = pixelBuffer

        let seqBefore = msc_seq_load_acquire(base)
        msc_seq_store_release(base, seqBefore &+ 1)

        let hdr = base.bindMemory(to: MSCStreamHeader.self, capacity: 1)
        hdr.pointee.presentationTimeNs = pts
        
        // Write the IOSurfaceID to the array
        withUnsafeMutablePointer(to: &hdr.pointee.ioSurfaceIDs) { ptr in
            ptr.withMemoryRebound(to: UInt32.self, capacity: 3) { arrayPtr in
                arrayPtr[writeIndex] = surfaceID
            }
        }

        msc_idx_store_relaxed(base, UInt32(writeIndex))
        msc_seq_store_release(base, seqBefore &+ 2)

        _ = msc_fp_fetch_add(base, 1)
    }

    // MARK: - Control Channel

    /// Polls the control ring buffer for new events.
    func pollControlEvents() -> [MSCControlEvent] {
        guard let base = mapping else { return [] }
        var events = [MSCControlEvent]()
        
        let head = msc_ctl_head_load_acquire(base)
        var tail = msc_ctl_tail_load_acquire(base)
        
        let capacity = UInt32(16)
        let ringPtr = base.advanced(by: 128).bindMemory(to: MSCControlEvent.self, capacity: 16)
        
        while tail != head {
            let event = ringPtr[Int(tail % capacity)]
            events.append(event)
            tail &+= 1
        }
        
        msc_ctl_tail_store_release(base, tail)
        return events
    }

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
