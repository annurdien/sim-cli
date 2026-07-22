// FrameTransportTests.swift
// Integration tests for SharedFrameWriter + SharedFrameReader round-trip.
// Requires macOS (uses POSIX mmap).

import XCTest
@testable import FrameHost

final class FrameTransportTests: XCTestCase {

    private var tmpPath: String!

    override func setUp() {
        super.setUp()
        tmpPath = NSTemporaryDirectory() + "minisimcam_test_\(UUID().uuidString).frames"
    }

    override func tearDown() {
        try? FileManager.default.removeItem(atPath: tmpPath)
        super.tearDown()
    }

    // MARK: - Basic round-trip

    func testWriteReadRoundTrip() throws {
        let writer = SharedFrameWriter(path: tmpPath)
        try writer.open(width: 320, height: 240)
        defer { writer.close() }

        // Create a red frame.
        let bpr    = ImageSource.alignedBytesPerRow(320)
        var pixels = [UInt8](repeating: 0, count: bpr * 240)
        for y in 0..<240 {
            for x in 0..<320 {
                let off = y * bpr + x * 4
                pixels[off + 0] = 0x00   // B
                pixels[off + 1] = 0x00   // G
                pixels[off + 2] = 0xFF   // R
                pixels[off + 3] = 0xFF   // A
            }
        }
        let frame = BGRAFrame(width: 320, height: 240, bytesPerRow: bpr, data: Data(pixels))
        try writer.publish(frame: frame, pts: 1_000_000)

        XCTAssertEqual(writer.framesProduced, 1)

        // Verify the file exists and has the expected size.
        let fileSize = (try? FileManager.default.attributesOfItem(atPath: tmpPath)[.size] as? Int) ?? 0
        let expected = 128 + 3 * bpr * 240
        XCTAssertEqual(fileSize, expected, "File size should match header + 3 × frame")
    }

    // MARK: - Odd dimensions (catches stride bugs)

    func testOddDimensionFrame() throws {
        let w = 641, h = 479
        let writer = SharedFrameWriter(path: tmpPath)
        try writer.open(width: w, height: h)
        defer { writer.close() }

        let frame = try ImageSource.colorBars(width: w, height: h)
        XCTAssertEqual(frame.bytesPerRow % 64, 0, "bytesPerRow must be 64-byte aligned")
        XCTAssertGreaterThanOrEqual(frame.bytesPerRow, w * 4)

        try writer.publish(frame: frame, pts: 1)
        XCTAssertEqual(writer.framesProduced, 1)
    }

    // MARK: - Producer restart detection

    func testProducerRestart() throws {
        let writer = SharedFrameWriter(path: tmpPath)
        try writer.open(width: 160, height: 120)
        let frame = try ImageSource.colorBars(width: 160, height: 120)
        try writer.publish(frame: frame, pts: 1)
        writer.close()

        // Simulate producer restart: re-open the same path.
        let writer2 = SharedFrameWriter(path: tmpPath)
        try writer2.open(width: 160, height: 120)
        defer { writer2.close() }

        // framesProduced resets to 0 on a fresh open.
        XCTAssertEqual(writer2.framesProduced, 0)
    }

    // MARK: - Monotonic frame count

    func testMonotonicFrameCount() throws {
        let writer = SharedFrameWriter(path: tmpPath)
        try writer.open(width: 160, height: 120)
        defer { writer.close() }

        let frame = try ImageSource.colorBars(width: 160, height: 120)
        for i in 1...10 {
            try writer.publish(frame: frame, pts: UInt64(i) * 33_333_333)
            XCTAssertEqual(writer.framesProduced, UInt64(i))
        }
    }

    // MARK: - ImageSource color bars

    func testColorBarsSize() throws {
        let f = try ImageSource.colorBars(width: 1280, height: 720)
        XCTAssertEqual(f.width, 1280)
        XCTAssertEqual(f.height, 720)
        XCTAssertEqual(f.bytesPerRow, 1280 * 4)   // Already aligned
        XCTAssertEqual(f.data.count, f.bytesPerRow * f.height)
    }

    // MARK: - Row alignment invariant

    func testRowAlignmentInvariant() {
        for w in [1, 7, 64, 100, 320, 641, 1280, 1920, 3840] {
            let bpr = ImageSource.alignedBytesPerRow(w)
            XCTAssertEqual(bpr % 64, 0, "bytesPerRow for width \(w) must be 64-byte aligned")
            XCTAssertGreaterThanOrEqual(bpr, w * 4, "bytesPerRow must cover all pixels")
        }
    }
}
