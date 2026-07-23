#!/usr/bin/env bash
# launch.sh — Launches an app on the booted simulator with IrisInject loaded.
# Usage: ./launch.sh <bundle-id> [udid]
set -euo pipefail

BUNDLE_ID="${1:-}"
UDID="${2:-booted}"

if [[ -z "$BUNDLE_ID" ]]; then
    echo "Usage: launch.sh <bundle-id> [udid]" >&2
    exit 1
fi

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_DIR="$SCRIPT_DIR/.."
DYLIB="$PROJECT_DIR/.build/injector/IrisInject.dylib"
SHM_PATH="/tmp/iris.${UDID}.frames"

if [[ ! -f "$DYLIB" ]]; then
    echo "⚠️  IrisInject.dylib not found. Run 'sim cam build' first." >&2
    exit 1
fi

echo "Launching $BUNDLE_ID on simulator $UDID"
echo "  dylib: $DYLIB"
echo "  shm:   $SHM_PATH"

SIMCTL_CHILD_DYLD_INSERT_LIBRARIES="$DYLIB" \
SIMCTL_CHILD_IRIS_PATH="$SHM_PATH" \
xcrun simctl launch "$UDID" "$BUNDLE_ID"
