// swift-tools-version: 5.9
// Package.swift — MiniSimCam monorepo SPM manifest.

import PackageDescription

let package = Package(
    name: "MiniSimCam",
    platforms: [
        .macOS(.v13),
        .iOS(.v16),
    ],
    products: [
        // FrameHost: macOS command-line executable (frame producer).
        .executable(name: "FrameHost", targets: ["FrameHost"]),

        // MiniSimCamShared: C/Swift shared-memory protocol (importable in tests).
        .library(name: "MiniSimCamShared", targets: ["MiniSimCamShared"]),

        // MiniCamInject: iOS Simulator dylib (built via xcodebuild, not SPM).
        // Listed as a library target so Xcode can reference the sources.
        .library(name: "MiniCamInject", type: .dynamic, targets: ["MiniCamInject"]),
    ],
    dependencies: [
        .package(
            url: "https://github.com/apple/swift-argument-parser",
            from: "1.4.0"
        ),
    ],
    targets: [

        // ------------------------------------------------------------------
        // Shared C headers (system module map style).
        // ------------------------------------------------------------------
        .target(
            name: "MiniSimCamShared",
            path: "Shared",
            sources: ["AtomicHelpers.c"],
            publicHeadersPath: "include"
        ),

        // ------------------------------------------------------------------
        // FrameHost — macOS executable.
        // ------------------------------------------------------------------
        .executableTarget(
            name: "FrameHost",
            dependencies: [
                "MiniSimCamShared",
                .product(name: "ArgumentParser", package: "swift-argument-parser"),
            ],
            path: "Sources/FrameHost",
            exclude: ["Info.plist"],
            linkerSettings: [
                .unsafeFlags([
                    "-Xlinker", "-sectcreate",
                    "-Xlinker", "__TEXT",
                    "-Xlinker", "__info_plist",
                    "-Xlinker", "Sources/FrameHost/Info.plist"
                ])
            ]
        ),

        // ------------------------------------------------------------------
        // MiniCamInject — iOS Simulator dynamic library.
        // SPM can resolve types; actual build must use xcodebuild for the
        // simulator-sdk target.
        // ------------------------------------------------------------------
        .target(
            name: "MiniCamInject",
            dependencies: ["MiniSimCamShared"],
            path: "Sources/MiniCamInject",
            publicHeadersPath: ".",
            cxxSettings: [
                .headerSearchPath("../../Shared/include"),
                .unsafeFlags(["-std=c++17"]),
            ],
            linkerSettings: [
                .linkedFramework("AVFoundation"),
                .linkedFramework("CoreMedia"),
                .linkedFramework("CoreVideo"),
            ]
        ),

        // ------------------------------------------------------------------
        // Tests
        // ------------------------------------------------------------------
        .testTarget(
            name: "ProtocolTests",
            dependencies: ["MiniSimCamShared"],
            path: "Tests/ProtocolTests"
        ),

        .testTarget(
            name: "FrameTransportTests",
            dependencies: ["FrameHost", "MiniSimCamShared"],
            path: "Tests/FrameTransportTests"
        ),
    ],
    cxxLanguageStandard: .cxx17
)
