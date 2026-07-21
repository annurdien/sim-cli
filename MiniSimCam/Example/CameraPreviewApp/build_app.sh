#!/bin/bash
set -euo pipefail
cd "$(dirname "$0")"

APP_NAME="CameraPreviewApp"
BUNDLE_ID="com.example.camerapreview"
TARGET_DIR="build/${APP_NAME}.app"

mkdir -p "$TARGET_DIR"

echo "<?xml version=\"1.0\" encoding=\"UTF-8\"?>
<!DOCTYPE plist PUBLIC \"-//Apple//DTD PLIST 1.0//EN\" \"http://www.apple.com/DTDs/PropertyList-1.0.dtd\">
<plist version=\"1.0\">
<dict>
    <key>CFBundleExecutable</key>
    <string>${APP_NAME}</string>
    <key>CFBundleIdentifier</key>
    <string>${BUNDLE_ID}</string>
    <key>CFBundleName</key>
    <string>${APP_NAME}</string>
    <key>CFBundleVersion</key>
    <string>1</string>
    <key>CFBundleShortVersionString</key>
    <string>1.0</string>
    <key>UILaunchStoryboardName</key>
    <string>LaunchScreen</string>
    <key>NSCameraUsageDescription</key>
    <string>Camera test</string>
</dict>
</plist>" > "$TARGET_DIR/Info.plist"

xcrun swiftc \
  -target arm64-apple-ios17.0-simulator \
  -sdk $(xcrun --show-sdk-path --sdk iphonesimulator) \
  -parse-as-library \
  CameraPreviewApp.swift \
  -o "$TARGET_DIR/$APP_NAME"

xcrun simctl install booted "$TARGET_DIR"
echo "Installed $BUNDLE_ID"
