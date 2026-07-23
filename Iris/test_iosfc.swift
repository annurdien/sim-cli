import Foundation
import CoreVideo
import IOSurface

let iosurfaceProps: [String: Any] = [
    "IOSurfaceIsGlobal": true
]

let attrs: [String: Any] = [
    kCVPixelBufferWidthKey as String: 1920,
    kCVPixelBufferHeightKey as String: 1080,
    kCVPixelBufferPixelFormatTypeKey as String: kCVPixelFormatType_32BGRA,
    kCVPixelBufferIOSurfacePropertiesKey as String: iosurfaceProps
]

var pixBuf: CVPixelBuffer?
CVPixelBufferCreate(kCFAllocatorDefault, 1920, 1080, kCVPixelFormatType_32BGRA, attrs as CFDictionary, &pixBuf)

if let pixBuf = pixBuf {
    let ioSurface = CVPixelBufferGetIOSurface(pixBuf)!
    let id = IOSurfaceGetID(ioSurface.takeUnretainedValue())
    print("Created IOSurface ID: \(id)")
    
    let task = Process()
    task.executableURL = URL(fileURLWithPath: "./test_lookup")
    task.arguments = ["\(id)"]
    try! task.run()
    task.waitUntilExit()
}
