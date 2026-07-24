// CameraSource.swift
// Captures live frames from the Mac's camera using AVFoundation.
// Supports explicit camera selection by name/ID and hot-swap reconnect.

import Foundation
import AVFoundation
import CoreVideo

final class CameraSource: NSObject, AVCaptureVideoDataOutputSampleBufferDelegate {

    // MARK: - Configuration

    private let writer: SharedFrameWriter
    private let targetWidth: Int
    private let targetHeight: Int
    private let fps: Int
    private let udid: String
    private let statusPath: String
    private let cameraID: String?

    // MARK: - Session state

    let session = AVCaptureSession()
    let sessionQueue = DispatchQueue(label: "com.iris.cameraSession")
    private let captureQueue = DispatchQueue(label: "com.iris.cameraCapture")
    private let stateLock = NSLock()
    private let statusLock = NSLock()

    private var activeDevice: AVCaptureDevice?
    private var activeCameraName: String = "mac-camera"
    private var activeCameraType: String = ""

    // MARK: - Hot-swap state

    var isConnected: Bool = false
    var isStopped: Bool = false
    var reconnectAttempts: Int = 0
    let maxReconnectAttempts: Int = 5
    var lastDisconnectedAt: Date?
    var disconnectObserver: NSObjectProtocol?
    var connectObserver: NSObjectProtocol?

    // MARK: - Status tracking

    private let startedAt = Date()
    private var lastStatusWrite = Date.distantPast
    private var lastPublishNs: UInt64 = 0

    // MARK: - Init

    init(writer: SharedFrameWriter,
         width: Int,
         height: Int,
         fps: Int,
         udid: String,
         statusPath: String,
         cameraID: String? = nil) {
        self.writer = writer
        self.targetWidth = width
        self.targetHeight = height
        self.fps = fps
        self.udid = udid
        self.statusPath = statusPath
        self.cameraID = cameraID
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
                writeStatus(running: true)
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
        try? FileManager.default.removeItem(atPath: statusPath)
    }

    // MARK: - AVCaptureVideoDataOutputSampleBufferDelegate

    func captureOutput(_ output: AVCaptureOutput,
                       didOutput sampleBuffer: CMSampleBuffer,
                       from connection: AVCaptureConnection) {
        guard let pixelBuffer = CMSampleBufferGetImageBuffer(sampleBuffer) else { return }

        let nowNs = currentMonotonicNs()

        do {
            try writer.publish(pixelBuffer: pixelBuffer, pts: nowNs)
            lastPublishNs = nowNs
            writeStatus(running: true)
        } catch {
            print("[CameraSource] Error publishing frame: \(error)")
        }
    }

    // MARK: - Session setup

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

    func configureSession(with device: AVCaptureDevice) throws {
        session.beginConfiguration()
        defer { session.commitConfiguration() }
        session.inputs.forEach  { session.removeInput($0) }
        session.outputs.forEach { session.removeOutput($0) }

        if session.canSetSessionPreset(.photo) {
            session.sessionPreset = .photo
        } else if session.canSetSessionPreset(.high) {
            session.sessionPreset = .high
        } else if session.canSetSessionPreset(.hd1280x720) {
            session.sessionPreset = .hd1280x720
        }

        let input = try AVCaptureDeviceInput(device: device)
        guard session.canAddInput(input) else { throw CameraError.cannotAddInput }
        session.addInput(input)

        let videoOutput = AVCaptureVideoDataOutput()
        videoOutput.alwaysDiscardsLateVideoFrames = true
        videoOutput.videoSettings = [
            kCVPixelBufferPixelFormatTypeKey as String: Int(kCVPixelFormatType_420YpCbCr8BiPlanarVideoRange),
            kCVPixelBufferIOSurfacePropertiesKey as String: ["IOSurfaceIsGlobal": true]
        ]
        videoOutput.setSampleBufferDelegate(self, queue: captureQueue)
        guard session.canAddOutput(videoOutput) else { throw CameraError.cannotAddOutput }
        session.addOutput(videoOutput)

        do {
            try device.lockForConfiguration()
            device.activeVideoMinFrameDuration = CMTimeMake(value: 1, timescale: Int32(fps))
            device.activeVideoMaxFrameDuration = CMTimeMake(value: 1, timescale: Int32(fps))
            device.unlockForConfiguration()
        } catch {
            print("[CameraSource] Warning: could not set framerate to \(fps): \(error)")
        }
    }

    // MARK: - Status

    func updateActiveDevice(_ device: AVCaptureDevice) {
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

    func activeCameraMetadata() -> (name: String, type: String) {
        stateLock.lock()
        defer { stateLock.unlock() }
        return (activeCameraName, activeCameraType)
    }
    
    func activeDeviceUniqueID() -> String? {
        stateLock.lock()
        defer { stateLock.unlock() }
        return activeDevice?.uniqueID
    }

    func writeStatus(running: Bool) {
        statusLock.lock()
        defer { statusLock.unlock() }
        
        let now = Date()
        // Throttle status writes unless we are signaling a stopped state
        if running && now.timeIntervalSince(lastStatusWrite) < 1.0 { return }
        lastStatusWrite = now

        let metadata = activeCameraMetadata()
        let isoFormatter = ISO8601DateFormatter()
        let disconnectedAt = lastDisconnectedAt.map { isoFormatter.string(from: $0) }
        let ageMs = (lastPublishNs == 0 || !running) ? 0 : ageInMs(fromMonotonicNs: lastPublishNs)
        
        let status = HostStatus(
            udid: udid,
            source: running ? metadata.name : "disconnected",
            cameraName: metadata.name,
            cameraType: metadata.type,
            width: targetWidth,
            height: targetHeight,
            fps: fps,
            framesProduced: writer.framesProduced,
            hostPID: ProcessInfo.processInfo.processIdentifier,
            startedAt: isoFormatter.string(from: startedAt),
            lastFrameAgeMs: ageMs,
            lastDisconnectedAt: disconnectedAt,
            running: running
        )
        
        guard let data = try? JSONEncoder().encode(status) else { return }
        try? data.write(to: URL(fileURLWithPath: statusPath), options: .atomic)
    }
}

// MARK: - Errors

enum CameraError: Error, CustomStringConvertible {
    case noCameraFound
    case cannotAddInput
    case cannotAddOutput
    case deviceNotFound(String)
    case ambiguousDevice(String)

    var description: String {
        switch self {
        case .noCameraFound:          return "No video capture device found"
        case .cannotAddInput:         return "Could not add video input to capture session"
        case .cannotAddOutput:        return "Could not add video output to capture session"
        case .deviceNotFound(let id): return "No camera found matching '\(id)' — run 'sim cam list' to see available devices"
        case .ambiguousDevice(let m): return m
        }
    }
}
