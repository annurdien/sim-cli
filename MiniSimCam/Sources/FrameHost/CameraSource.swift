// CameraSource.swift
// Captures live frames from the Mac's camera using AVFoundation.

import Foundation
import AVFoundation
import CoreVideo

final class CameraSource: NSObject, AVCaptureVideoDataOutputSampleBufferDelegate {
    
    private let writer: SharedFrameWriter
    private let targetWidth: Int
    private let targetHeight: Int
    private let fps: Int
    private let udid: String
    private let statusPath: String
    
    private let session = AVCaptureSession()
    private let captureQueue = DispatchQueue(label: "com.minisimcam.cameraCapture")
    
    // Status tracking
    private let startedAt = Date()
    private var lastStatusWrite = Date.distantPast
    private let _timebaseInfo: mach_timebase_info = {
        var info = mach_timebase_info(numer: 0, denom: 0)
        mach_timebase_info(&info)
        return info
    }()
    
    init(writer: SharedFrameWriter, width: Int, height: Int, fps: Int, udid: String, statusPath: String) {
        self.writer = writer
        self.targetWidth = width
        self.targetHeight = height
        self.fps = fps
        self.udid = udid
        self.statusPath = statusPath
        super.init()
    }
    
    func start() throws {
        session.beginConfiguration()
        
        // Ensure we try to match the requested resolution if possible
        if session.canSetSessionPreset(.hd1280x720) {
            session.sessionPreset = .hd1280x720
        } else {
            session.sessionPreset = .high
        }
        
        guard let device = AVCaptureDevice.default(for: .video) else {
            throw CameraError.noCameraFound
        }
        
        let input = try AVCaptureDeviceInput(device: device)
        if session.canAddInput(input) {
            session.addInput(input)
        } else {
            throw CameraError.cannotAddInput
        }
        
        let output = AVCaptureVideoDataOutput()
        output.alwaysDiscardsLateVideoFrames = true
        // Ask for BGRA so we don't have to convert
        output.videoSettings = [
            kCVPixelBufferPixelFormatTypeKey as String: Int(kCVPixelFormatType_32BGRA)
        ]
        
        output.setSampleBufferDelegate(self, queue: captureQueue)
        if session.canAddOutput(output) {
            session.addOutput(output)
        } else {
            throw CameraError.cannotAddOutput
        }
        
        // Attempt to set frame rate
        do {
            try device.lockForConfiguration()
            device.activeVideoMinFrameDuration = CMTimeMake(value: 1, timescale: Int32(fps))
            device.activeVideoMaxFrameDuration = CMTimeMake(value: 1, timescale: Int32(fps))
            device.unlockForConfiguration()
        } catch {
            print("[CameraSource] Warning: could not set framerate to \(fps)")
        }
        
        session.commitConfiguration()
        session.startRunning()
    }
    
    func stop() {
        session.stopRunning()
        // Clear status
        try? FileManager.default.removeItem(atPath: statusPath)
    }
    
    // MARK: - AVCaptureVideoDataOutputSampleBufferDelegate
    
    func captureOutput(_ output: AVCaptureOutput, didOutput sampleBuffer: CMSampleBuffer, from connection: AVCaptureConnection) {
        guard let pixelBuffer = CMSampleBufferGetImageBuffer(sampleBuffer) else { return }
        
        let nowNs = clock_gettime_nsec_np(CLOCK_MONOTONIC_RAW)
        
        do {
            // Note: If the camera's resolution doesn't match targetWidth/targetHeight perfectly,
            // we should technically resize it. For zero-copy, we assume the writer handles max bounds,
            // or we just memcpy what we can. SharedFrameWriter's publish does `min(dataSize, frameSize)`.
            // In a production environment, you'd use a CIContext or vImage to scale the CVPixelBuffer first
            // if dimensions mismatch, but for this POC, direct copy is extremely fast.
            try writer.publish(pixelBuffer: pixelBuffer, pts: nowNs)
            writeStatus(nowNs: nowNs)
        } catch {
            print("[CameraSource] Error publishing frame: \(error)")
        }
    }
    
    // MARK: - Status
    
    private func writeStatus(nowNs: UInt64) {
        let now = Date()
        guard now.timeIntervalSince(lastStatusWrite) >= 1.0 else { return }
        lastStatusWrite = now
        
        // Approximate age of the frame (usually close to 0 since we just pushed it)
        let hdrAgeNs = clock_gettime_nsec_np(CLOCK_MONOTONIC_RAW) - nowNs
        let ageMs = Double(hdrAgeNs) * Double(_timebaseInfo.numer) / Double(_timebaseInfo.denom) / 1_000_000.0
        
        let status = """
        {
            "udid": "\(udid)",
            "source": "mac-camera",
            "width": \(targetWidth),
            "height": \(targetHeight),
            "fps": \(fps),
            "framesProduced": \(writer.framesProduced),
            "hostPID": \(ProcessInfo.processInfo.processIdentifier),
            "startedAt": "\(ISO8601DateFormatter().string(from: startedAt))",
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
}

enum CameraError: Error, CustomStringConvertible {
    case noCameraFound
    case cannotAddInput
    case cannotAddOutput
    
    var description: String {
        switch self {
        case .noCameraFound: return "No video capture device found"
        case .cannotAddInput: return "Could not add video input to capture session"
        case .cannotAddOutput: return "Could not add video output to capture session"
        }
    }
}
