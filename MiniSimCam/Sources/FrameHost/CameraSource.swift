// CameraSource.swift
// Captures live frames from the Mac's camera using AVFoundation.
// Supports explicit camera selection by name/ID and hot-swap reconnect.

import Foundation
import AVFoundation
import CoreVideo
import CoreImage
import CoreGraphics

// MARK: - ScaleMode

/// Controls how camera frames are scaled to match the target resolution.
enum ScaleMode: String {
    /// Crop to fill: scales proportionally, then crops equally from opposite edges.
    case fill = "fill"
    /// Fit with letterbox: scales uniformly to fit inside the target, pads remaining
    /// area with black. Preserves the full camera frame.
    case fit  = "fit"
}

final class CameraSource: NSObject, AVCaptureVideoDataOutputSampleBufferDelegate {

    // MARK: - Configuration

    private let writer: SharedFrameWriter
    private let targetWidth: Int
    private let targetHeight: Int
    private let fps: Int
    private let udid: String
    private let statusPath: String
    /// Optional explicit camera identifier (uniqueID or localizedName substring).
    private let cameraID: String?
    private let scaleMode: ScaleMode

    // MARK: - Session state

    private let session = AVCaptureSession()
    private let captureQueue = DispatchQueue(label: "com.minisimcam.cameraCapture")
    /// AVFoundation requires all session configuration and lifecycle operations
    /// to be serialized on one queue.
    private let sessionQueue = DispatchQueue(label: "com.minisimcam.cameraSession")
    private let stateLock = NSLock()
    private let statusLock = NSLock()

    /// GPU-accelerated CoreImage context for fit-mode scaling.
    private lazy var ciContext: CIContext = CIContext(options: [
        .useSoftwareRenderer: false,
        .cacheIntermediates: false
    ])

    /// The device currently in use; set after successful setup.
    private var activeDevice: AVCaptureDevice?
    /// Human-readable device name; used in status and logging.
    private var activeCameraName: String = "mac-camera"
    /// Device type label (Built-in / Continuity Camera / External).
    private var activeCameraType: String = ""

    // MARK: - Hot-swap state

    private var isConnected: Bool = false
    private var isStopped: Bool = false
    private var reconnectAttempts: Int = 0
    private let maxReconnectAttempts: Int = 5
    private var lastDisconnectedAt: Date?
    private var disconnectObserver: NSObjectProtocol?
    private var connectObserver: NSObjectProtocol?

    // MARK: - Status tracking

    private let startedAt = Date()
    private var lastStatusWrite = Date.distantPast
    private let _timebaseInfo: mach_timebase_info = {
        var info = mach_timebase_info(numer: 0, denom: 0)
        mach_timebase_info(&info)
        return info
    }()

    // MARK: - Init

    init(writer: SharedFrameWriter,
         width: Int,
         height: Int,
         fps: Int,
         udid: String,
         statusPath: String,
         cameraID: String? = nil,
         scaleMode: ScaleMode = .fill) {
        self.writer = writer
        self.targetWidth = width
        self.targetHeight = height
        self.fps = fps
        self.udid = udid
        self.statusPath = statusPath
        self.cameraID = cameraID
        self.scaleMode = scaleMode
        super.init()
    }

    // MARK: - Public lifecycle

    func start() throws {
        var startError: Error?
        sessionQueue.sync {
            do {
                let device = try resolveDevice()
                try configureSession(with: device)
                updateActiveDevice(device)

                isStopped = false
                isConnected = true
                reconnectAttempts = 0

                session.startRunning()
                registerHotSwapObservers()
                let metadata = activeCameraMetadata()
                writeStatus(nowNs: clock_gettime_nsec_np(CLOCK_MONOTONIC_RAW))
                print("[CameraSource] started — device='\(metadata.name)' (\(metadata.type))")
            } catch {
                startError = error
            }
        }
        if let startError { throw startError }
    }

    func stop() {
        sessionQueue.sync {
            isStopped = true
            isConnected = false
            unregisterHotSwapObservers()
            session.stopRunning()
        }
        // Clear status
        try? FileManager.default.removeItem(atPath: statusPath)
    }

    // MARK: - AVCaptureVideoDataOutputSampleBufferDelegate

    func captureOutput(_ output: AVCaptureOutput,
                       didOutput sampleBuffer: CMSampleBuffer,
                       from connection: AVCaptureConnection) {
        guard let pixelBuffer = CMSampleBufferGetImageBuffer(sampleBuffer) else { return }

        let nowNs = clock_gettime_nsec_np(CLOCK_MONOTONIC_RAW)

        do {
            switch scaleMode {
            case .fill:
                if CVPixelBufferGetWidth(pixelBuffer) == targetWidth,
                   CVPixelBufferGetHeight(pixelBuffer) == targetHeight {
                    try writer.publish(pixelBuffer: pixelBuffer, pts: nowNs)
                } else if let scaled = scale(pixelBuffer, mode: .fill) {
                    try writer.publish(pixelBuffer: scaled, pts: nowNs)
                } else {
                    throw CameraError.scaleFailed
                }
            case .fit:
                // Letterbox path: scale to fit target, pad remainder with black.
                if let scaled = scale(pixelBuffer, mode: .fit) {
                    try writer.publish(pixelBuffer: scaled, pts: nowNs)
                } else {
                    throw CameraError.scaleFailed
                }
            }
            writeStatus(nowNs: nowNs)
        } catch {
            print("[CameraSource] Error publishing frame: \(error)")
        }
    }

    // MARK: - Private: fit scaling

    /// Scales `src` to fit within targetWidth × targetHeight while preserving
    /// aspect ratio. The remaining area is filled with black (letterbox / pillarbox).
    private func scale(_ src: CVPixelBuffer, mode: ScaleMode) -> CVPixelBuffer? {
        let srcW = CVPixelBufferGetWidth(src)
        let srcH = CVPixelBufferGetHeight(src)

        // Compute uniform scale and centering offset. Fit leaves letterbox
        // space; fill overflows evenly and is cropped by the output bounds.
        let scaleX = CGFloat(targetWidth)  / CGFloat(srcW)
        let scaleY = CGFloat(targetHeight) / CGFloat(srcH)
        let scale  = mode == .fit ? min(scaleX, scaleY) : max(scaleX, scaleY)

        let scaledW = Int((CGFloat(srcW) * scale).rounded())
        let scaledH = Int((CGFloat(srcH) * scale).rounded())
        let padX    = (targetWidth  - scaledW) / 2
        let padY    = (targetHeight - scaledH) / 2

        // Allocate a black output CVPixelBuffer at target dimensions.
        let attrs: [CFString: Any] = [
            kCVPixelBufferCGImageCompatibilityKey:          true,
            kCVPixelBufferCGBitmapContextCompatibilityKey:  true,
            kCVPixelBufferIOSurfacePropertiesKey:           [:] as [CFString: Any],
        ]
        var outBuf: CVPixelBuffer?
        guard CVPixelBufferCreate(
            kCFAllocatorDefault,
            targetWidth, targetHeight,
            kCVPixelFormatType_32BGRA,
            attrs as CFDictionary,
            &outBuf
        ) == kCVReturnSuccess, let out = outBuf else { return nil }

        // Clear to black.
        CVPixelBufferLockBaseAddress(out, [])
        if let base = CVPixelBufferGetBaseAddress(out) {
            memset(base, 0, CVPixelBufferGetDataSize(out))
        }
        CVPixelBufferUnlockBaseAddress(out, [])

        // Build scaled + translated CIImage and render into the output buffer.
        // CoreImage uses bottom-left origin; the translation puts the image
        // in the vertical center of the output.
        let ciSrc = CIImage(cvPixelBuffer: src)
        let scaled = ciSrc
            .transformed(by: CGAffineTransform(scaleX: scale, y: scale))
            .transformed(by: CGAffineTransform(translationX: CGFloat(padX), y: CGFloat(padY)))

        ciContext.render(
            scaled,
            to: out,
            bounds: CGRect(x: 0, y: 0, width: targetWidth, height: targetHeight),
            colorSpace: CGColorSpaceCreateDeviceRGB()
        )

        return out
    }

    // MARK: - Private: session setup

    private func resolveDevice() throws -> AVCaptureDevice {
        if let id = cameraID {
            guard let device = try CameraDiscovery.findDevice(byNameOrID: id) else {
                throw CameraError.deviceNotFound(id)
            }
            return device
        }
        guard let device = AVCaptureDevice.default(for: .video) else {
            throw CameraError.noCameraFound
        }
        return device
    }

    private func configureSession(with device: AVCaptureDevice) throws {
        // Remove existing inputs/outputs for hot-swap restart.
        session.beginConfiguration()
        defer { session.commitConfiguration() }
        session.inputs.forEach  { session.removeInput($0) }
        session.outputs.forEach { session.removeOutput($0) }

        switch scaleMode {
        case .fill:
            // Use a native/high-quality source and perform aspect-fill scaling
            // below. A fixed 720p preset cannot satisfy arbitrary targets.
            if session.canSetSessionPreset(.photo) {
                session.sessionPreset = .photo
            } else if session.canSetSessionPreset(.high) {
                session.sessionPreset = .high
            } else if session.canSetSessionPreset(.hd1280x720) {
                session.sessionPreset = .hd1280x720
            }
        case .fit:
            // .photo requests the camera's highest-quality native format on macOS,
            // avoiding the 16:9 center-crop that hd presets apply.
            // We scale to the target dimensions ourselves in the callback.
            if session.canSetSessionPreset(.photo) {
                session.sessionPreset = .photo
            } else {
                session.sessionPreset = .high
            }
        }

        let input = try AVCaptureDeviceInput(device: device)
        guard session.canAddInput(input) else { throw CameraError.cannotAddInput }
        session.addInput(input)

        let videoOutput = AVCaptureVideoDataOutput()
        videoOutput.alwaysDiscardsLateVideoFrames = true
        videoOutput.videoSettings = [
            kCVPixelBufferPixelFormatTypeKey as String: Int(kCVPixelFormatType_32BGRA)
        ]
        videoOutput.setSampleBufferDelegate(self, queue: captureQueue)
        guard session.canAddOutput(videoOutput) else { throw CameraError.cannotAddOutput }
        session.addOutput(videoOutput)

        // Attempt to set frame rate.
        do {
            try device.lockForConfiguration()
            device.activeVideoMinFrameDuration = CMTimeMake(value: 1, timescale: Int32(fps))
            device.activeVideoMaxFrameDuration = CMTimeMake(value: 1, timescale: Int32(fps))
            device.unlockForConfiguration()
        } catch {
            print("[CameraSource] Warning: could not set framerate to \(fps): \(error)")
        }

    }

    // MARK: - Private: hot-swap

    private func registerHotSwapObservers() {
        let nc = NotificationCenter.default

        disconnectObserver = nc.addObserver(
            forName: AVCaptureDevice.wasDisconnectedNotification,
            object: nil,
            queue: nil
        ) { [weak self] notification in
            self?.sessionQueue.async { self?.handleDisconnect(notification: notification) }
        }

        connectObserver = nc.addObserver(
            forName: AVCaptureDevice.wasConnectedNotification,
            object: nil,
            queue: nil
        ) { [weak self] notification in
            self?.sessionQueue.async { self?.handleConnect(notification: notification) }
        }
    }

    private func unregisterHotSwapObservers() {
        let nc = NotificationCenter.default
        if let obs = disconnectObserver { nc.removeObserver(obs) }
        if let obs = connectObserver    { nc.removeObserver(obs) }
        disconnectObserver = nil
        connectObserver    = nil
    }

    private func handleDisconnect(notification: Notification) {
        guard let device = notification.object as? AVCaptureDevice else { return }

        // Match by localizedName — Continuity Camera gets a new uniqueID on every
        // connection, so uniqueID is not a stable identifier.
        let metadata = activeCameraMetadata()
        let isOurDevice = device.uniqueID == activeDevice?.uniqueID
                       || device.localizedName == metadata.name
        guard isOurDevice, isConnected, !isStopped else { return }

        print("[CameraSource] Camera disconnected: '\(metadata.name)' — waiting for reconnect.")
        isConnected = false
        lastDisconnectedAt = Date()
        reconnectAttempts = 0  // Reset so a fresh reconnect gets the full budget.
        session.stopRunning()
        writeStatusDisconnected()
    }

    private func handleConnect(notification: Notification) {
        guard !isConnected, !isStopped else { return }
        guard reconnectAttempts < maxReconnectAttempts else {
            print("[CameraSource] Max reconnect attempts (\(maxReconnectAttempts)) reached. Run 'sim cam start --camera' to try again.")
            return
        }

        guard let candidate = notification.object as? AVCaptureDevice else { return }
        // Only consider video devices.
        guard candidate.hasMediaType(.video) else { return }

        // Determine if this is the device we want to reconnect to.
        // Primary match: the explicit --camera-id query the user passed.
        // Fallback match: localizedName of the device we were running before.
        // We intentionally do NOT match by uniqueID because Continuity Camera
        // is reassigned a new uniqueID on each connection.
        let isMatch: Bool
        if let id = cameraID {
            let nameLower = id.lowercased()
            isMatch = candidate.uniqueID == id
                   || candidate.localizedName.lowercased().contains(nameLower)
        } else {
            // No explicit camera ID — reconnect if the device name matches what we had.
            isMatch = candidate.localizedName == activeCameraMetadata().name
        }

        guard isMatch else { return }

        // Let the OS settle before reopening. Subsequent failures schedule a
        // bounded retry that rediscovers the current device.
        scheduleReconnect(candidate: candidate, delay: 0.8)
    }

    private func scheduleReconnect(candidate: AVCaptureDevice?, delay: TimeInterval) {
        guard !isConnected, !isStopped, reconnectAttempts < maxReconnectAttempts else { return }
        sessionQueue.asyncAfter(deadline: .now() + delay) { [weak self] in
            self?.attemptReconnect(candidate: candidate)
        }
    }

    private func attemptReconnect(candidate: AVCaptureDevice?) {
        guard !isConnected, !isStopped, reconnectAttempts < maxReconnectAttempts else { return }
        reconnectAttempts += 1

        do {
            let device = try candidate ?? reconnectCandidate()
            print("[CameraSource] Reconnecting to '\(device.localizedName)' (attempt \(reconnectAttempts)/\(maxReconnectAttempts))…")
            try configureSession(with: device)
            updateActiveDevice(device)
            session.startRunning()
            isConnected = true
            reconnectAttempts = 0
            lastDisconnectedAt = nil
            print("[CameraSource] Reconnected successfully to '\(activeCameraMetadata().name)'.")
        } catch {
            print("[CameraSource] Reconnect failed: \(error)")
            let backoff = min(4.0, Double(reconnectAttempts))
            scheduleReconnect(candidate: nil, delay: backoff)
        }
    }

    private func reconnectCandidate() throws -> AVCaptureDevice {
        if let id = cameraID {
            guard let device = try CameraDiscovery.findDevice(byNameOrID: id) else {
                throw CameraError.deviceNotFound(id)
            }
            return device
        }
        let name = activeCameraMetadata().name
        guard let device = CameraDiscovery.allDevices().first(where: { $0.localizedName == name })?.device else {
            throw CameraError.noCameraFound
        }
        return device
    }

    // MARK: - Status

    private func updateActiveDevice(_ device: AVCaptureDevice) {
        let type = CameraInfo(
            uniqueID: device.uniqueID,
            localizedName: device.localizedName,
            deviceType: device.deviceType,
            device: device
        ).typeLabel
        stateLock.lock()
        activeDevice = device
        activeCameraName = device.localizedName
        activeCameraType = type
        stateLock.unlock()
    }

    private func activeCameraMetadata() -> (name: String, type: String) {
        stateLock.lock()
        defer { stateLock.unlock() }
        return (activeCameraName, activeCameraType)
    }

    private func writeStatus(nowNs: UInt64) {
        statusLock.lock()
        defer { statusLock.unlock() }
        let now = Date()
        guard now.timeIntervalSince(lastStatusWrite) >= 1.0 else { return }
        lastStatusWrite = now

        let hdrAgeNs = clock_gettime_nsec_np(CLOCK_MONOTONIC_RAW) - nowNs
        let ageMs = Double(hdrAgeNs) * Double(_timebaseInfo.numer) / Double(_timebaseInfo.denom) / 1_000_000.0

        let metadata = activeCameraMetadata()
        let isoFormatter = ISO8601DateFormatter()
        let status = """
        {
            "udid": "\(udid)",
            "source": "\(metadata.name)",
            "cameraName": "\(metadata.name)",
            "cameraType": "\(metadata.type)",
            "width": \(targetWidth),
            "height": \(targetHeight),
            "fps": \(fps),
            "framesProduced": \(writer.framesProduced),
            "hostPID": \(ProcessInfo.processInfo.processIdentifier),
            "startedAt": "\(isoFormatter.string(from: startedAt))",
            "lastFrameAgeMs": \(ageMs),
            "running": true
        }
        """
        do {
            try status.write(toFile: statusPath, atomically: true, encoding: .utf8)
        } catch {
            print("[CameraSource] Warning: Failed to write status file: \(error)")
        }
    }

    private func writeStatusDisconnected() {
        statusLock.lock()
        defer { statusLock.unlock() }
        let metadata = activeCameraMetadata()
        let isoFormatter = ISO8601DateFormatter()
        let disconnectedAt = lastDisconnectedAt.map { isoFormatter.string(from: $0) } ?? ""
        let status = """
        {
            "udid": "\(udid)",
            "source": "disconnected",
            "cameraName": "\(metadata.name)",
            "cameraType": "\(metadata.type)",
            "width": \(targetWidth),
            "height": \(targetHeight),
            "fps": \(fps),
            "framesProduced": \(writer.framesProduced),
            "hostPID": \(ProcessInfo.processInfo.processIdentifier),
            "startedAt": "\(isoFormatter.string(from: startedAt))",
            "lastFrameAgeMs": 0,
            "lastDisconnectedAt": "\(disconnectedAt)",
            "running": false
        }
        """
        do {
            try status.write(toFile: statusPath, atomically: true, encoding: .utf8)
        } catch {
            print("[CameraSource] Warning: Failed to write disconnected status: \(error)")
        }
    }
}

// MARK: - Errors

enum CameraError: Error, CustomStringConvertible {
    case noCameraFound
    case cannotAddInput
    case cannotAddOutput
    case deviceNotFound(String)
    case ambiguousDevice(String)
    case scaleFailed

    var description: String {
        switch self {
        case .noCameraFound:          return "No video capture device found"
        case .cannotAddInput:         return "Could not add video input to capture session"
        case .cannotAddOutput:        return "Could not add video output to capture session"
        case .deviceNotFound(let id): return "No camera found matching '\(id)' — run 'sim cam list' to see available devices"
        case .ambiguousDevice(let m): return m
        case .scaleFailed:             return "Could not scale camera frame to the target resolution"
        }
    }
}
