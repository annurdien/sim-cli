// ImageSource.swift
// Loads a PNG or JPEG and produces raw BGRA frames aligned to IRIS_ROW_ALIGNMENT.

import CoreGraphics
import ImageIO
import Foundation

/// A decoded, BGRA-converted image ready for shared-memory publishing.
struct BGRAFrame {
    let width: Int
    let height: Int
    let bytesPerRow: Int
    let data: Data        // Owned copy; safe to share across threads after init.
}

struct ImageSource {

    // MARK: - Public

    /// Loads an image file and converts it to a BGRA frame at the given dimensions.
    /// Throws `ImageSourceError` on failure.
    static func load(url: URL, targetWidth: Int, targetHeight: Int) throws -> BGRAFrame {
        guard let imageSource = CGImageSourceCreateWithURL(url as CFURL, nil),
              let cgImage = CGImageSourceCreateImageAtIndex(imageSource, 0, nil) else {
            throw ImageSourceError.cannotLoadImage(url)
        }
        return try render(cgImage: cgImage, width: targetWidth, height: targetHeight)
    }

    /// Generates a synthetic color-bar test card at the given dimensions.
    static func colorBars(width: Int, height: Int) throws -> BGRAFrame {
        let bytesPerRow = alignedBytesPerRow(width)
        let totalBytes = bytesPerRow * height

        var pixels = [UInt8](repeating: 0, count: totalBytes)

        // SMPTE-style color bars: 7 vertical stripes in BGRA order.
        let barColors: [(b: UInt8, g: UInt8, r: UInt8)] = [
            (192, 192, 192),   // White
            (192, 192, 0),     // Yellow
            (0, 192, 192),     // Cyan
            (0, 192, 0),       // Green
            (192, 0, 192),     // Magenta
            (192, 0, 0),       // Red
            (0, 0, 192),       // Blue
        ]
        let barWidth = width / barColors.count

        for y in 0..<height {
            for x in 0..<width {
                let barIndex = min(x / barWidth, barColors.count - 1)
                let color = barColors[barIndex]
                let offset = y * bytesPerRow + x * 4
                pixels[offset + 0] = color.b
                pixels[offset + 1] = color.g
                pixels[offset + 2] = color.r
                pixels[offset + 3] = 0xFF      // Alpha = fully opaque
            }
        }

        return BGRAFrame(
            width: width,
            height: height,
            bytesPerRow: bytesPerRow,
            data: Data(pixels)
        )
    }

    // MARK: - Private

    private static func render(cgImage: CGImage, width: Int, height: Int) throws -> BGRAFrame {
        let bytesPerRow = alignedBytesPerRow(width)
        let totalBytes = bytesPerRow * height

        guard totalBytes > 0 else {
            throw ImageSourceError.zeroDimension
        }

        // BGRA with premultiplied alpha matches kCVPixelFormatType_32BGRA.
        let colorSpace = CGColorSpaceCreateDeviceRGB()
        let bitmapInfo = CGBitmapInfo.byteOrder32Little.rawValue
                       | CGImageAlphaInfo.premultipliedFirst.rawValue

        var pixelBuffer = [UInt8](repeating: 0, count: totalBytes)
        try pixelBuffer.withUnsafeMutableBytes { rawBuf in
            guard let ctx = CGContext(
                data: rawBuf.baseAddress,
                width: width,
                height: height,
                bitsPerComponent: 8,
                bytesPerRow: bytesPerRow,
                space: colorSpace,
                bitmapInfo: bitmapInfo
            ) else {
                throw ImageSourceError.cannotCreateContext
            }
            ctx.draw(cgImage, in: CGRect(x: 0, y: 0, width: width, height: height))
        }

        return BGRAFrame(
            width: width,
            height: height,
            bytesPerRow: bytesPerRow,
            data: Data(pixelBuffer)
        )
    }

    /// Round `width * 4` up to the nearest multiple of `IRIS_ROW_ALIGNMENT` (64).
    static func alignedBytesPerRow(_ width: Int) -> Int {
        let raw = width * 4
        return (raw + 63) & ~63
    }
}

// MARK: - Errors

enum ImageSourceError: Error, CustomStringConvertible {
    case cannotLoadImage(URL)
    case cannotCreateContext
    case zeroDimension

    var description: String {
        switch self {
        case .cannotLoadImage(let url): return "Cannot load image at \(url.path)"
        case .cannotCreateContext:      return "Cannot create BGRA CGContext"
        case .zeroDimension:            return "Width or height is zero"
        }
    }
}
