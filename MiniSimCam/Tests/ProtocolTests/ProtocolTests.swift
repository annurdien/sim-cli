// ProtocolTests.swift
// Unit tests for MiniCamProtocol.h calculations.
// These do NOT require a running FrameHost; they validate pure math.

import XCTest
@testable import MiniSimCamShared   // Swift wrapper around the C header

final class ProtocolTests: XCTestCase {

    // MARK: - Header size

    func testHeaderSize() {
        // Must remain exactly 128 bytes so existing shared files stay valid.
        XCTAssertEqual(MemoryLayout<MSCStreamHeader>.size, 128)
    }

    // MARK: - Row stride alignment

    func testBytesPerRowExactMultiple() {
        // Width that already aligns: 64 pixels → 64*4 = 256 B, already 64-byte aligned.
        XCTAssertEqual(msc_bytes_per_row(64), 256)
    }

    func testBytesPerRowRoundsUp() {
        // 1280 pixels → 1280*4 = 5120 B → already aligned (5120 % 64 == 0).
        XCTAssertEqual(msc_bytes_per_row(1280), 5120)
    }

    func testBytesPerRowOddWidth() {
        // 641 pixels → 641*4 = 2564 B → rounded up to next multiple of 64 = 2624.
        let bpr = msc_bytes_per_row(641)
        XCTAssertEqual(bpr % 64, 0)
        XCTAssertGreaterThanOrEqual(bpr, 641 * 4)
    }

    func testBytesPerRowMinimalWidth() {
        // Width 1 → 4 bytes → rounded to 64.
        XCTAssertEqual(msc_bytes_per_row(1), 64)
    }

    // MARK: - Mapping size

    func testMappingSizeFormula() {
        let bpr     = msc_bytes_per_row(1280)
        let bufSize = bpr * 720
        let total   = msc_mapping_size(bufSize)
        XCTAssertEqual(total, UInt64(128) + UInt64(3) * UInt64(bufSize))
    }

    // MARK: - Frame pointer arithmetic

    func testFramePtrOffsets() {
        let bufSize: UInt32 = 100
        var fakeBase = [UInt8](repeating: 0, count: 128 + 3 * 100)
        fakeBase.withUnsafeMutableBytes { raw in
            let base = raw.baseAddress!
            let p0 = msc_frame_ptr(base, bufSize, 0)
            let p1 = msc_frame_ptr(base, bufSize, 1)
            let p2 = msc_frame_ptr(base, bufSize, 2)

            XCTAssertEqual(p0, base.advanced(by: 128))
            XCTAssertEqual(p1, base.advanced(by: 128 + 100))
            XCTAssertEqual(p2, base.advanced(by: 128 + 200))
        }
    }

    // MARK: - Magic / version constants

    func testMagicConstant() {
        XCTAssertEqual(MSC_MAGIC, 0x4D534343)
    }

    func testVersionConstant() {
        XCTAssertEqual(MSC_VERSION, 1)
    }

    func testBufferCount() {
        XCTAssertEqual(MSC_BUFFER_COUNT, 3)
    }

    // MARK: - Pixel format

    func testPixelFormat() {
        // kCVPixelFormatType_32BGRA = 'BGRA' = 0x42475241
        XCTAssertEqual(MSC_PIXEL_FORMAT, 0x42475241)
    }
}
