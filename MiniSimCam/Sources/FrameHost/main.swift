// main.swift — FrameHost CLI entry point
// NOTE: This file is named main.swift so Swift treats it as top-level code.
//       We cannot use @main here — instead call FrameHostCommand.main() directly.
import Foundation
import ArgumentParser
import MiniSimCamShared

struct FrameHostCommand: ParsableCommand {

    static let configuration = CommandConfiguration(
        commandName: "FrameHost",
        abstract: "MiniSimCam frame producer — writes BGRA frames to a shared-memory triple buffer.",
        version: "1.0.0"
    )

    // MARK: - Arguments

    @Option(name: .long, help: "Simulator UDID (used to derive shared-memory and status file paths).")
    var udid: String

    @Option(name: .long, help: "Path to a PNG or JPEG source image.")
    var image: String?

    @Flag(name: .long, help: "Use a synthetic SMPTE color-bar image instead of a file.")
    var bars: Bool = false

    @Option(name: .long, help: "Output frame width in pixels.")
    var width: Int = 1280

    @Option(name: .long, help: "Output frame height in pixels.")
    var height: Int = 720

    @Option(name: .long, help: "Frames per second.")
    var fps: Int = 30

    // MARK: - Validation

    func validate() throws {
        guard image != nil || bars else {
            throw ValidationError("Provide either --image <path> or --bars.")
        }
        guard width > 0, height > 0 else { throw ValidationError("Width and height must be > 0.") }
        guard fps > 0, fps <= 120 else { throw ValidationError("FPS must be between 1 and 120.") }
        guard udid.count >= 8 else { throw ValidationError("UDID looks invalid: \(udid)") }
    }

    // MARK: - Run

    func run() throws {
        let shmPath    = "/tmp/minisimcam.\(udid).frames"
        let statusPath = "/tmp/minisimcam.\(udid).status"
        let pidPath    = "/tmp/minisimcam.\(udid).pid"

        // Write PID file so `sim cam stop` can signal this process.
        let pid = ProcessInfo.processInfo.processIdentifier
        try String(pid).write(toFile: pidPath, atomically: false, encoding: .utf8)
        defer { try? FileManager.default.removeItem(atPath: pidPath) }

        // Load or generate the source frame.
        let sourceName: String
        let frame: BGRAFrame

        if bars {
            sourceName = "color-bars"
            frame = try ImageSource.colorBars(width: width, height: height)
        } else {
            let url = URL(fileURLWithPath: image!)
            sourceName = url.lastPathComponent
            frame = try ImageSource.load(url: url, targetWidth: width, targetHeight: height)
        }

        // Open shared memory.
        let writer = SharedFrameWriter(path: shmPath)
        try writer.open(width: width, height: height)
        defer { writer.close() }

        // Start the frame loop.
        let loop = FrameLoop(
            writer:     writer,
            frame:      frame,
            fps:        fps,
            udid:       udid,
            sourceName: sourceName,
            statusPath: statusPath
        )
        loop.start()

        print("[FrameHost] started — source=\(sourceName) \(width)×\(height) @ \(fps) fps")
        print("[FrameHost] shared memory: \(shmPath)")
        print("[FrameHost] status file:   \(statusPath)")
        print("[FrameHost] PID: \(pid)")

        // Install signal handlers for clean shutdown.
        signal(SIGTERM, SIG_IGN)
        signal(SIGINT, SIG_IGN)

        let sigTerm = DispatchSource.makeSignalSource(signal: SIGTERM, queue: .main)
        sigTerm.setEventHandler {
            print("[FrameHost] received SIGTERM — shutting down.")
            loop.stop()
            Foundation.exit(0)
        }
        sigTerm.resume()

        let sigInt = DispatchSource.makeSignalSource(signal: SIGINT, queue: .main)
        sigInt.setEventHandler {
            print("[FrameHost] received SIGINT — shutting down.")
            loop.stop()
            Foundation.exit(0)
        }
        sigInt.resume()

        // Park the main thread.
        dispatchMain()
    }
}

// Top-level entry — required when file is named main.swift (cannot use @main).
FrameHostCommand.main()
