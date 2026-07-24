// CVPixelBuffer+Copying.swift
import Foundation
import CoreVideo

extension CVPixelBuffer {
    
    /// Deep copies the contents of `self` into `destinationBuffer`.
    /// Handles both planar and non-planar pixel buffers.
    func copy(to destinationBuffer: CVPixelBuffer) {
        CVPixelBufferLockBaseAddress(self, .readOnly)
        CVPixelBufferLockBaseAddress(destinationBuffer, [])
        defer {
            CVPixelBufferUnlockBaseAddress(destinationBuffer, [])
            CVPixelBufferUnlockBaseAddress(self, .readOnly)
        }
        
        let planeCount = CVPixelBufferGetPlaneCount(self)
        if planeCount > 0 {
            for plane in 0..<planeCount {
                if let srcPlane = CVPixelBufferGetBaseAddressOfPlane(self, plane),
                   let dstPlane = CVPixelBufferGetBaseAddressOfPlane(destinationBuffer, plane) {
                    
                    let srcBPR = CVPixelBufferGetBytesPerRowOfPlane(self, plane)
                    let dstBPR = CVPixelBufferGetBytesPerRowOfPlane(destinationBuffer, plane)
                    let copyHeight = min(CVPixelBufferGetHeightOfPlane(self, plane), CVPixelBufferGetHeightOfPlane(destinationBuffer, plane))
                    let copyBytes = min(srcBPR, dstBPR)
                    
                    for row in 0..<copyHeight {
                        memcpy(dstPlane.advanced(by: row * dstBPR), srcPlane.advanced(by: row * srcBPR), copyBytes)
                    }
                }
            }
        } else {
            if let srcBase = CVPixelBufferGetBaseAddress(self), let dstBase = CVPixelBufferGetBaseAddress(destinationBuffer) {
                let srcBPR = CVPixelBufferGetBytesPerRow(self)
                let dstBPR = CVPixelBufferGetBytesPerRow(destinationBuffer)
                let copyHeight = min(CVPixelBufferGetHeight(self), CVPixelBufferGetHeight(destinationBuffer))
                let copyBytes = min(srcBPR, dstBPR)
                
                for row in 0..<copyHeight {
                    memcpy(dstBase.advanced(by: row * dstBPR), srcBase.advanced(by: row * srcBPR), copyBytes)
                }
            }
        }
    }
}
