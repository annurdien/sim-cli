// HostStatus.swift
import Foundation

struct HostStatus: Codable {
    var udid: String
    var source: String
    var cameraName: String?
    var cameraType: String?
    var width: Int
    var height: Int
    var fps: Int
    var framesProduced: UInt64
    var hostPID: Int32
    var startedAt: String // ISO-8601
    var lastFrameAgeMs: Double // milliseconds since last publish
    var lastDisconnectedAt: String? // ISO-8601
    var running: Bool
}
