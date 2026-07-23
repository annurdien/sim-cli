// swift-tools-version: 5.9
// Package.swift — Iris monorepo SPM manifest.

import PackageDescription

let package = Package(
    name: "Iris",
    platforms: [
        .macOS(.v13),
        .iOS(.v16),
    ],
    products: [
        .executable(name: "FrameHost", targets: ["FrameHost"]),
        .library(name: "IrisShared", targets: ["IrisShared"]),
        .library(name: "IrisInject", type: .dynamic, targets: ["IrisInject"]),
    ],
    dependencies: [
        .package(
            url: "https://github.com/apple/swift-argument-parser",
            from: "1.4.0"
        ),
    ],
    targets: [
        .target(
            name: "IrisShared",
            path: "Shared",
            sources: ["AtomicHelpers.c"],
            publicHeadersPath: "include"
        ),
        .executableTarget(
            name: "FrameHost",
            dependencies: [
                "IrisShared",
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
        .target(
            name: "IrisInject",
            dependencies: ["IrisShared"],
            path: "Sources/IrisInject",
            publicHeadersPath: ".",
            cxxSettings: [
                .headerSearchPath("../../Shared/include"),
                .unsafeFlags(["-std=c++17"]),
            ],
            linkerSettings: [
                .linkedFramework("AVFoundation"),
                .linkedFramework("CoreMedia"),
                .linkedFramework("CoreVideo"),
                .linkedFramework("Foundation"),
                .linkedFramework("CoreGraphics"),
                .linkedFramework("VideoToolbox"),
                .linkedFramework("QuartzCore"),
                .linkedFramework("IOSurface")
            ]
        ),
        .testTarget(
            name: "ProtocolTests",
            dependencies: ["IrisShared"],
            path: "Tests/ProtocolTests"
        ),

        .testTarget(
            name: "FrameTransportTests",
            dependencies: ["FrameHost", "IrisShared"],
            path: "Tests/FrameTransportTests"
        ),
    ],
    cxxLanguageStandard: .cxx17
)
