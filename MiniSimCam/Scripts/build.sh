#!/usr/bin/env bash
# build.sh -- Builds FrameHost (macOS) and MiniCamInject.dylib (iOS Simulator).
# Usage: ./Scripts/build.sh [project-dir]
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_DIR="${1:-"$SCRIPT_DIR/.."}"  # MiniSimCam root

# Resolve repo root robustly: prefer git, fall back to PROJECT_DIR/..
if REPO_ROOT="$(git -C "${PROJECT_DIR}" rev-parse --show-toplevel 2>/dev/null)"; then
  : # already set
else
  REPO_ROOT="$(cd "${PROJECT_DIR}/.." && pwd)"
fi

cd "$PROJECT_DIR"

echo "=================================================="
echo "  MiniSimCam Build"
echo "=================================================="

# -- 1. Build FrameHost (macOS release) ----------------------------------------
echo ""
echo ">> Building FrameHost (macOS)..."
swift build --disable-sandbox -c release --product FrameHost 2>&1

FRAME_HOST_BIN="$(swift build --disable-sandbox -c release --product FrameHost --show-bin-path 2>/dev/null)/FrameHost"
echo "   OK FrameHost -> ${FRAME_HOST_BIN}"

# -- 2. Build MiniCamInject.dylib (iOS Simulator) ------------------------------
echo ""
echo ">> Building MiniCamInject (iOS Simulator)..."

BUILD_DIR="${PROJECT_DIR}/.build/injector"
mkdir -p "${BUILD_DIR}"

SIM_SDK="$(xcrun --sdk iphonesimulator --show-sdk-path)"
INCLUDE_FLAGS="-I ${PROJECT_DIR}/Shared/include -I ${PROJECT_DIR}/Sources/MiniCamInject"

compile_arch() {
    local ARCH="$1"
    local TARGET="${ARCH}-apple-ios16.0-simulator"
    echo "   Compiling ${ARCH}..."

    # SharedFrameReader.cpp -- pure C++17 (no ObjC)
    clang++ \
        -target "${TARGET}" \
        -isysroot "${SIM_SDK}" \
        ${INCLUDE_FLAGS} \
        -std=c++17 \
        -fPIC -g \
        -c "${PROJECT_DIR}/Sources/MiniCamInject/SharedFrameReader.cpp" \
        -o "${BUILD_DIR}/SharedFrameReader_${ARCH}.o"

    # SampleBufferFactory.mm -- ObjC++ with ARC
    clang++ \
        -target "${TARGET}" \
        -isysroot "${SIM_SDK}" \
        ${INCLUDE_FLAGS} \
        -std=c++17 \
        -fPIC -g \
        -fobjc-arc \
        -c "${PROJECT_DIR}/Sources/MiniCamInject/SampleBufferFactory.mm" \
        -o "${BUILD_DIR}/SampleBufferFactory_${ARCH}.o"

    # CaptureHooks.mm -- ObjC++ with ARC
    clang++ \
        -target "${TARGET}" \
        -isysroot "${SIM_SDK}" \
        ${INCLUDE_FLAGS} \
        -std=c++17 \
        -fPIC -g \
        -fobjc-arc \
        -c "${PROJECT_DIR}/Sources/MiniCamInject/CaptureHooks.mm" \
        -o "${BUILD_DIR}/CaptureHooks_${ARCH}.o"

    # EntryPoint.mm -- ObjC++ with ARC
    clang++ \
        -target "${TARGET}" \
        -isysroot "${SIM_SDK}" \
        ${INCLUDE_FLAGS} \
        -std=c++17 \
        -fPIC -g \
        -fobjc-arc \
        -c "${PROJECT_DIR}/Sources/MiniCamInject/EntryPoint.mm" \
        -o "${BUILD_DIR}/EntryPoint_${ARCH}.o"

    # Link dylib
    clang++ \
        -target "${TARGET}" \
        -dynamiclib \
        -isysroot "${SIM_SDK}" \
        -framework AVFoundation \
        -framework CoreMedia \
        -framework CoreVideo \
        -framework Foundation \
        -framework QuartzCore \
        -framework VideoToolbox \
        -framework CoreGraphics \
        -framework IOSurface \
        "${BUILD_DIR}/SharedFrameReader_${ARCH}.o" \
        "${BUILD_DIR}/SampleBufferFactory_${ARCH}.o" \
        "${BUILD_DIR}/CaptureHooks_${ARCH}.o" \
        "${BUILD_DIR}/EntryPoint_${ARCH}.o" \
        -o "${BUILD_DIR}/MiniCamInject_${ARCH}.dylib"
}

compile_arch arm64
compile_arch x86_64

# Create universal binary.
lipo -create \
    "${BUILD_DIR}/MiniCamInject_arm64.dylib" \
    "${BUILD_DIR}/MiniCamInject_x86_64.dylib" \
    -output "${BUILD_DIR}/MiniCamInject.dylib"

# Copy binaries to cmd/assets for go:embed
ASSETS_DIR="${REPO_ROOT}/cmd/assets"
mkdir -p "${ASSETS_DIR}"
cp "${FRAME_HOST_BIN}" "${ASSETS_DIR}/FrameHost"
cp "${BUILD_DIR}/MiniCamInject.dylib" "${ASSETS_DIR}/MiniCamInject.dylib"
echo "   OK Copied assets to ${ASSETS_DIR}"

echo ""
echo "=================================================="
echo "  Build complete."
echo ""
echo "  FrameHost:     ${FRAME_HOST_BIN}"
echo "  MiniCamInject: ${BUILD_DIR}/MiniCamInject.dylib"
echo ""
echo "  Next steps:"
echo "    sim cam start --image <file.png>"
echo "    sim cam launch --bundle-id com.example.CameraPreviewApp"
echo "=================================================="
