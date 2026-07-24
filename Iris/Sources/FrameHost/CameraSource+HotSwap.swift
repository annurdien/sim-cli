// CameraSource+HotSwap.swift
import Foundation
import AVFoundation

extension CameraSource {

    func registerHotSwapObservers() {
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

    func unregisterHotSwapObservers() {
        let nc = NotificationCenter.default
        if let obs = disconnectObserver { nc.removeObserver(obs) }
        if let obs = connectObserver    { nc.removeObserver(obs) }
        disconnectObserver = nil
        connectObserver    = nil
    }

    private func handleDisconnect(notification: Notification) {
        guard let device = notification.object as? AVCaptureDevice else { return }

        let metadata = activeCameraMetadata()
        let isOurDevice = device.uniqueID == activeDeviceUniqueID()
                       || device.localizedName == metadata.name
        guard isOurDevice, isConnected, !isStopped else { return }

        print("[CameraSource] Camera disconnected: '\(metadata.name)' — waiting for reconnect.")
        isConnected = false
        lastDisconnectedAt = Date()
        reconnectAttempts = 0
        session.stopRunning()
        writeStatus(running: false)
    }

    private func handleConnect(notification: Notification) {
        guard !isConnected, !isStopped else { return }
        guard reconnectAttempts < maxReconnectAttempts else {
            print("[CameraSource] Max reconnect attempts (\(maxReconnectAttempts)) reached. Run 'sim cam start --camera' to try again.")
            return
        }

        guard let candidate = notification.object as? AVCaptureDevice else { return }
        guard candidate.hasMediaType(.video) else { return }

        let isMatch: Bool
        let metadata = activeCameraMetadata()
        if let id = activeDeviceUniqueID() { // fallback if cameraID wasn't used
            let nameLower = id.lowercased()
            isMatch = candidate.uniqueID == id
                   || candidate.localizedName.lowercased().contains(nameLower)
                   || candidate.localizedName == metadata.name
        } else {
            isMatch = candidate.localizedName == metadata.name
        }

        guard isMatch else { return }

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
            writeStatus(running: true)
        } catch {
            print("[CameraSource] Reconnect failed: \(error)")
            let backoff = min(4.0, Double(reconnectAttempts))
            scheduleReconnect(candidate: nil, delay: backoff)
        }
    }

    private func reconnectCandidate() throws -> AVCaptureDevice {
        let name = activeCameraMetadata().name
        guard let device = CameraDiscovery.allDevices().first(where: { $0.localizedName == name })?.device else {
            throw CameraError.noCameraFound
        }
        return device
    }
}
