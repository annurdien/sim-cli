#!/usr/bin/env bash
# build.sh -- Builds FrameHost (macOS) and IrisInject.dylib (iOS Simulator).
# Usage: ./Scripts/build.sh [project-dir]
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_DIR="${1:-"$SCRIPT_DIR/.."}"  # Iris root

# Resolve repo root robustly: prefer git, fall back to PROJECT_DIR/..
if REPO_ROOT="$(git -C "${PROJECT_DIR}" rev-parse --show-toplevel 2>/dev/null)"; then
  : # already set
else
  REPO_ROOT="$(cd "${PROJECT_DIR}/.." && pwd)"
fi

cd "$PROJECT_DIR"

# ── Colors & symbols ─────────────────────────────────────────────────────────
if [[ -t 1 ]]; then
  BOLD='\033[1m'
  DIM='\033[2m'
  RESET='\033[0m'
  GREEN='\033[32m'
  CYAN='\033[36m'
  YELLOW='\033[33m'
  CHECK='✓'
  ARROW='›'
else
  BOLD='' DIM='' RESET='' GREEN='' CYAN='' YELLOW=''
  CHECK='OK' ARROW='>'
fi

# ── Helpers ───────────────────────────────────────────────────────────────────
step() {
  printf "  ${DIM}${ARROW}${RESET} %s" "$1"
}

done_step() {
  local elapsed="$1"
  printf "\r  ${GREEN}${CHECK}${RESET} %s ${DIM}(%s)${RESET}\n" "$2" "$elapsed"
}

elapsed_since() {
  local start="$1"
  local end
  end=$(date +%s)
  local secs=$(( end - start ))
  if (( secs == 0 )); then
    printf "<1s"
  elif (( secs < 60 )); then
    printf "%ds" "$secs"
  else
    printf "%dm %ds" $(( secs / 60 )) $(( secs % 60 ))
  fi
}

BUILD_START=$(date +%s)

printf "\n"
printf "  ${BOLD}Iris${RESET} ${DIM}build${RESET}\n"
printf "  ${DIM}──────────────────────────────────${RESET}\n"

# -- 1. Build FrameHost (macOS release) ----------------------------------------
step "FrameHost (macOS)..."
STEP_START=$(date +%s)

swift build --disable-sandbox -c release --product FrameHost > /dev/null 2>&1

FRAME_HOST_BIN="$(swift build --disable-sandbox -c release --product FrameHost --show-bin-path 2>/dev/null)/FrameHost"
done_step "$(elapsed_since $STEP_START)" "FrameHost (macOS)"

# -- 2. Build IrisInject.dylib (iOS Simulator) --------------------------------
BUILD_DIR="${PROJECT_DIR}/.build/injector"
mkdir -p "${BUILD_DIR}"

SIM_SDK="$(xcrun --sdk iphonesimulator --show-sdk-path)"
INCLUDE_FLAGS="-I ${PROJECT_DIR}/Shared/include -I ${PROJECT_DIR}/Sources/IrisInject"

compile_arch() {
    local ARCH="$1"
    local TARGET="${ARCH}-apple-ios16.0-simulator"

    # SharedFrameReader.cpp -- pure C++17 (no ObjC)
    clang++ \
        -target "${TARGET}" \
        -isysroot "${SIM_SDK}" \
        ${INCLUDE_FLAGS} \
        -std=c++17 \
        -fPIC -g \
        -c "${PROJECT_DIR}/Sources/IrisInject/SharedFrameReader.cpp" \
        -o "${BUILD_DIR}/SharedFrameReader_${ARCH}.o"

    # SampleBufferFactory.mm -- ObjC++ with ARC
    clang++ \
        -target "${TARGET}" \
        -isysroot "${SIM_SDK}" \
        ${INCLUDE_FLAGS} \
        -std=c++17 \
        -fPIC -g \
        -fobjc-arc \
        -c "${PROJECT_DIR}/Sources/IrisInject/SampleBufferFactory.mm" \
        -o "${BUILD_DIR}/SampleBufferFactory_${ARCH}.o"

    # CaptureHooks.mm -- ObjC++ with ARC
    clang++ \
        -target "${TARGET}" \
        -isysroot "${SIM_SDK}" \
        ${INCLUDE_FLAGS} \
        -std=c++17 \
        -fPIC -g \
        -fobjc-arc \
        -c "${PROJECT_DIR}/Sources/IrisInject/CaptureHooks.mm" \
        -o "${BUILD_DIR}/CaptureHooks_${ARCH}.o"

    # EntryPoint.mm -- ObjC++ with ARC
    clang++ \
        -target "${TARGET}" \
        -isysroot "${SIM_SDK}" \
        ${INCLUDE_FLAGS} \
        -std=c++17 \
        -fPIC -g \
        -fobjc-arc \
        -c "${PROJECT_DIR}/Sources/IrisInject/EntryPoint.mm" \
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
        -o "${BUILD_DIR}/IrisInject_${ARCH}.dylib"
}

step "IrisInject (arm64)..."
STEP_START=$(date +%s)
compile_arch arm64
done_step "$(elapsed_since $STEP_START)" "IrisInject (arm64)"

step "IrisInject (x86_64)..."
STEP_START=$(date +%s)
compile_arch x86_64
done_step "$(elapsed_since $STEP_START)" "IrisInject (x86_64)"

# Create universal binary.
step "Universal binary (lipo)..."
STEP_START=$(date +%s)
lipo -create \
    "${BUILD_DIR}/IrisInject_arm64.dylib" \
    "${BUILD_DIR}/IrisInject_x86_64.dylib" \
    -output "${BUILD_DIR}/IrisInject.dylib"
done_step "$(elapsed_since $STEP_START)" "Universal binary (lipo)"

# Copy binaries to cmd/assets for go:embed
ASSETS_DIR="${REPO_ROOT}/cmd/assets"
mkdir -p "${ASSETS_DIR}"
cp "${FRAME_HOST_BIN}" "${ASSETS_DIR}/FrameHost"
cp "${BUILD_DIR}/IrisInject.dylib" "${ASSETS_DIR}/IrisInject.dylib"

printf "  ${GREEN}${CHECK}${RESET} Assets copied\n"

TOTAL_ELAPSED="$(elapsed_since $BUILD_START)"
printf "  ${DIM}──────────────────────────────────${RESET}\n"
printf "  ${GREEN}${BOLD}Done${RESET} ${DIM}in ${TOTAL_ELAPSED}${RESET}\n\n"
