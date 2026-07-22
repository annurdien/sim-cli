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
    var udid: String = ""

    @Option(name: .long, help: "Path to a PNG or JPEG source image.")
    var image: String?

    @Flag(name: .long, help: "Use a synthetic SMPTE color-bar image instead of a file.")
    var bars: Bool = false

    @Flag(name: .long, help: "Use the Mac's physical camera as a live source.")
    var camera: Bool = false

    @Option(name: .long, help: "Camera name (substring) or uniqueID to select. Use 'sim cam list' to enumerate devices. Only valid with --camera.")
    var cameraID: String?

    @Option(name: .long, help: "How to scale the camera frame to fit the target resolution: 'fill' (crop to fill, fast) or 'fit' (letterbox, preserves full frame). Only valid with --camera. (default: fill)")
    var scaleMode: ScaleMode = .fill

    @Flag(name: .long, help: "List all available cameras and exit. Does not require --udid.")
    var listCameras: Bool = false

    @Flag(name: .long, help: "Output camera list as JSON (use with --list-cameras).")
    var json: Bool = false

    @Option(name: .long, help: "Output frame width in pixels.")
    var width: Int = 1280

    @Option(name: .long, help: "Output frame height in pixels.")
    var height: Int = 720

    @Option(name: .long, help: "Frames per second.")
    var fps: Int = 30

    // MARK: - Validation

    func validate() throws {
        // --list-cameras is a standalone mode — skip all other checks.
        if listCameras { return }

        // Require exactly one source.
        let count = (image != nil ? 1 : 0) + (bars ? 1 : 0) + (camera ? 1 : 0)
        guard count == 1 else {
            throw ValidationError("Provide exactly one of: --image <path>, --bars, or --camera.")
        }
        // --camera-id only makes sense with --camera.
        if cameraID != nil && !camera {
            throw ValidationError("--camera-id requires --camera.")
        }
        guard width > 0, height > 0 else { throw ValidationError("Width and height must be > 0.") }
        let maxDimension = Int(MSC_MAX_DIMENSION)
        guard width <= maxDimension, height <= maxDimension else {
            throw ValidationError("Width and height must not exceed \(maxDimension).")
        }
        guard fps > 0, fps <= 120 else { throw ValidationError("FPS must be between 1 and 120.") }
        guard !listCameras else { return }
        guard udid.count >= 8 else { throw ValidationError("UDID looks invalid: \(udid)") }
    }

    // MARK: - Run

    func run() throws {
        // Handle --list-cameras first — no UDID, shared memory, or frame loop needed.
        if listCameras {
            if json {
                CameraDiscovery.printJSON()
            } else {
                CameraDiscovery.printTable()
            }
            return
        }

        let shmPath    = "/tmp/minisimcam.\(udid).frames"
        let statusPath = "/tmp/minisimcam.\(udid).status"
        let pidPath    = "/tmp/minisimcam.\(udid).pid"

        // Never let a previous crashed host satisfy the CLI's readiness check.
        try? FileManager.default.removeItem(atPath: statusPath)

        // Write PID file so `sim cam stop` can signal this process.
        let pid = ProcessInfo.processInfo.processIdentifier
        try String(pid).write(toFile: pidPath, atomically: false, encoding: .utf8)
        defer { try? FileManager.default.removeItem(atPath: pidPath) }

        // Open shared memory.
        let writer = SharedFrameWriter(path: shmPath)
        try writer.open(width: width, height: height)
        defer { writer.close() }

        // Start the appropriate source.
        let sourceName: String
        let loop: FrameLoop?
        let camSource: CameraSource?

        if camera {
            sourceName = cameraID ?? "mac-camera"
            loop = nil
            camSource = CameraSource(
                writer:     writer,
                width:      width,
                height:     height,
                fps:        fps,
                udid:       udid,
                statusPath: statusPath,
                cameraID:   cameraID,
                scaleMode:  scaleMode
            )
            try camSource?.start()
        } else {
            let frame: BGRAFrame
            if bars {
                sourceName = "color-bars"
                frame = try ImageSource.colorBars(width: width, height: height)
            } else {
                let url = URL(fileURLWithPath: image!)
                sourceName = url.lastPathComponent
                frame = try ImageSource.load(url: url, targetWidth: width, targetHeight: height)
            }
            camSource = nil
            loop = FrameLoop(
                writer:     writer,
                frame:      frame,
                fps:        fps,
                udid:       udid,
                sourceName: sourceName,
                statusPath: statusPath
            )
            loop?.start()
        }

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
            loop?.stop()
            camSource?.stop()
            Foundation.exit(0)
        }
        sigTerm.resume()

        let sigInt = DispatchSource.makeSignalSource(signal: SIGINT, queue: .main)
        sigInt.setEventHandler {
            print("[FrameHost] received SIGINT — shutting down.")
            loop?.stop()
            camSource?.stop()
            Foundation.exit(0)
        }
        sigInt.resume()

        // Park the main thread.
        dispatchMain()
    }
}

// Top-level entry — required when file is named main.swift (cannot use @main).
FrameHostCommand.main()

// MARK: - ArgumentParser conformance

extension ScaleMode: ExpressibleByArgument {
    init?(argument: String) { self.init(rawValue: argument) }
}
