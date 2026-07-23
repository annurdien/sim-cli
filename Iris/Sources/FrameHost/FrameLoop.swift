// FrameLoop.swift
// Drives frame publishing at a configured FPS using a DispatchSourceTimer.
// Also writes a status JSON file that the Go CLI reads for `sim cam status`.

import Foundation

struct FrameLoopStatus: Codable {
    var udid: String
    var source: String
    var width: Int
    var height: Int
    var fps: Int
    var framesProduced: UInt64
    var hostPID: Int32
    var startedAt: String         // ISO-8601
    var lastFrameAgeMs: Double    // milliseconds since last publish
    var running: Bool
}

// MARK: - Monotonic clock

/// Cached mach timebase info — initialised once, not on every call.
private let _timebaseInfo: mach_timebase_info_data_t = {
    var info = mach_timebase_info_data_t()
    mach_timebase_info(&info)
    return info
}()

/// Returns a monotonic nanosecond timestamp anchored to mach_absolute_time.
func currentMonotonicNs() -> UInt64 {
    let ticks = mach_absolute_time()
    return ticks * UInt64(_timebaseInfo.numer) / UInt64(_timebaseInfo.denom)
}

final class FrameLoop {

    // MARK: - State

    private let writer: SharedFrameWriter
    private let statusPath: String
    private let frame: BGRAFrame
    private let fps: Int
    private let udid: String
    private let sourceName: String

    private var timer: DispatchSourceTimer?
    private var startDate: Date = .now
    private var lastPublishNs: UInt64 = 0
    private var framesProducedLocal: UInt64 = 0
    private let queue = DispatchQueue(label: "com.minisimcam.frameloop", qos: .userInteractive)

    // Status throttle: write at most once per second.
    private var lastStatusWriteNs: UInt64 = 0
    private let statusWriteIntervalNs: UInt64 = 1_000_000_000

    // MARK: - Init

    init(
        writer: SharedFrameWriter,
        frame: BGRAFrame,
        fps: Int,
        udid: String,
        sourceName: String,
        statusPath: String
    ) {
        self.writer     = writer
        self.frame      = frame
        self.fps        = fps
        self.udid       = udid
        self.sourceName = sourceName
        self.statusPath = statusPath
    }

    // MARK: - Lifecycle

    func start() {
        startDate = .now
        let intervalNs = 1_000_000_000 / UInt64(fps)
        let leewayNs   = intervalNs / 10   // 10% leeway

        let src = DispatchSource.makeTimerSource(flags: .strict, queue: queue)
        src.schedule(deadline: .now(), repeating: .nanoseconds(Int(intervalNs)),
                     leeway: .nanoseconds(Int(leewayNs)))
        src.setEventHandler { [weak self] in self?.tick() }
        src.resume()
        timer = src
    }

    func stop() {
        timer?.cancel()
        timer = nil
        writeStatus(running: false, force: true)
    }

    // MARK: - Private

    private func tick() {
        let pts = currentMonotonicNs()
        do {
            try writer.publish(frame: frame, pts: pts)
            lastPublishNs = pts
            framesProducedLocal &+= 1
        } catch {
            fputs("[FrameLoop] publish error: \(error)\n", stderr)
        }
        // Throttle: write status at most once per second, not every frame.
        writeStatus(running: true, force: false)
    }

    /// Write status JSON, subject to throttle unless `force` is true.
    private func writeStatus(running: Bool, force: Bool) {
        let now = currentMonotonicNs()
        guard force || (now &- lastStatusWriteNs) >= statusWriteIntervalNs else { return }
        lastStatusWriteNs = now

        let ageMs: Double = lastPublishNs == 0 ? 0
                          : Double(now &- lastPublishNs) / 1_000_000.0

        let status = FrameLoopStatus(
            udid:           udid,
            source:         sourceName,
            width:          frame.width,
            height:         frame.height,
            fps:            fps,
            framesProduced: framesProducedLocal,
            hostPID:        ProcessInfo.processInfo.processIdentifier,
            startedAt:      ISO8601DateFormatter().string(from: startDate),
            lastFrameAgeMs: ageMs,
            running:        running
        )

        guard let data = try? JSONEncoder().encode(status) else { return }
        // Single atomic write — no need for a second replaceItem on top.
        try? data.write(to: URL(fileURLWithPath: statusPath), options: .atomic)
    }
}

// currentMonotonicNs() is defined above, before FrameLoop, using a cached timebase.
