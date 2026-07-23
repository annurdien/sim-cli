// CameraDiscovery.swift
// Enumerates all available video capture devices (built-in, Continuity Camera, external)
// using AVCaptureDevice.DiscoverySession, and provides fuzzy name/ID matching.

import Foundation
import AVFoundation

// MARK: - CameraInfo

struct CameraInfo {
    let uniqueID: String
    let localizedName: String
    let deviceType: AVCaptureDevice.DeviceType
    let device: AVCaptureDevice

    var typeLabel: String {
        switch deviceType {
        case .builtInWideAngleCamera: return "Built-in"
        default:
            if #available(macOS 14.0, *), deviceType == .continuityCamera {
                return "Continuity Camera"
            }
            return "External"
        }
    }
}

// MARK: - CameraDiscovery

enum CameraDiscovery {

    // Device types to discover. continuityCamera requires NSCameraUseContinuityCameraDeviceType
    // in Info.plist (macOS 14+). externalUnknown catches USB webcams.
    private static var deviceTypes: [AVCaptureDevice.DeviceType] {
        var types: [AVCaptureDevice.DeviceType] = [
            .builtInWideAngleCamera,
            .externalUnknown,
        ]
        if #available(macOS 14.0, *) {
            types.append(.continuityCamera)
        }
        return types
    }

    // MARK: - Enumerate

    /// Returns all discovered video devices.
    static func allDevices() -> [CameraInfo] {
        let session = AVCaptureDevice.DiscoverySession(
            deviceTypes: deviceTypes,
            mediaType: .video,
            position: .unspecified
        )
        return session.devices.map { device in
            CameraInfo(
                uniqueID: device.uniqueID,
                localizedName: device.localizedName,
                deviceType: device.deviceType,
                device: device
            )
        }
    }

    // MARK: - Find

    /// Find a device by exact uniqueID or case-insensitive substring of localizedName.
    /// - Returns: the matching `AVCaptureDevice`, or nil if not found.
    /// - Throws: `CameraError.ambiguousDevice` if multiple name matches exist.
    static func findDevice(byNameOrID query: String) throws -> AVCaptureDevice? {
        let all = allDevices()

        // 1. Exact uniqueID match takes absolute priority.
        if let exact = all.first(where: { $0.uniqueID == query }) {
            return exact.device
        }

        // 2. Case-insensitive substring match on localizedName.
        let needle = query.lowercased()
        let matches = all.filter { $0.localizedName.lowercased().contains(needle) }

        switch matches.count {
        case 0:  return nil
        case 1:  return matches[0].device
        default:
            let names = matches.map { "\"\($0.localizedName)\"" }.joined(separator: ", ")
            throw CameraError.ambiguousDevice("'\(query)' matches multiple cameras: \(names). Use a more specific name or the exact uniqueID (from 'sim cam list').")
        }
    }

    // MARK: - Print Table

    /// Prints a formatted table of all discovered cameras to stdout.
    static func printTable() {
        let all = allDevices()

        if all.isEmpty {
            print("[CameraDiscovery] No video devices found.")
            return
        }

        // Helper: left-pad a string to exactly `width` characters.
        func lpad(_ s: String, _ width: Int) -> String {
            if s.count >= width { return String(s.prefix(width)) }
            return s + String(repeating: " ", count: width - s.count)
        }

        // Column widths (minimum enforced).
        let idColW   = max(12, all.map { $0.uniqueID.count }.max() ?? 12)
        let nameColW = max(24, all.map { $0.localizedName.count }.max() ?? 24)
        let typeColW = max(18, all.map { $0.typeLabel.count }.max() ?? 18)
        let numColW  = 3

        // Total inner width: "  #   Name   Type   UniqueID  " + separators
        let innerW = numColW + 2 + nameColW + 2 + typeColW + 2 + idColW + 4
        let border = String(repeating: "─", count: innerW)

        let header = "│  Available Cameras"
            + String(repeating: " ", count: max(0, innerW - 18)) + "│"
        let colHeader = "│  "
            + lpad("#",        numColW) + "  "
            + lpad("Name",     nameColW) + "  "
            + lpad("Type",     typeColW) + "  "
            + lpad("UniqueID", idColW)   + "  │"

        print("┌\(border)┐")
        print(header)
        print("├\(border)┤")
        print(colHeader)
        print("├\(border)┤")

        for (i, cam) in all.enumerated() {
            let displayID = cam.uniqueID.count > idColW
                ? String(cam.uniqueID.prefix(idColW - 1)) + "…"
                : cam.uniqueID
            let row = "│  "
                + lpad("\(i + 1)", numColW) + "  "
                + lpad(cam.localizedName, nameColW) + "  "
                + lpad(cam.typeLabel,     typeColW) + "  "
                + lpad(displayID,         idColW)   + "  │"
            print(row)
        }

        print("└\(border)┘")
        print("\nUse: sim cam start --camera --camera-id \"<Name or UniqueID>\"")
    }

    /// Prints a JSON array of all discovered cameras to stdout.
    static func printJSON() {
        let all = allDevices()
        let mapped = all.map { [
            "uniqueID": $0.uniqueID,
            "localizedName": $0.localizedName,
            "typeLabel": $0.typeLabel
        ] }
        if let data = try? JSONSerialization.data(withJSONObject: mapped, options: .prettyPrinted),
           let jsonString = String(data: data, encoding: .utf8) {
            print(jsonString)
        } else {
            print("[]")
        }
    }
}
