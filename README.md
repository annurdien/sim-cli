# sim-cli

[![Test and Build](https://github.com/annurdien/sim-cli/actions/workflows/ci.yml/badge.svg)](https://github.com/annurdien/sim-cli/actions/workflows/ci.yml)
[![Release Pipeline](https://github.com/annurdien/sim-cli/actions/workflows/release.yml/badge.svg)](https://github.com/annurdien/sim-cli/actions/workflows/release.yml)
[![GitHub Release](https://img.shields.io/github/v/release/annurdien/sim-cli)](https://github.com/annurdien/sim-cli/releases)
[![Go Version](https://img.shields.io/github/go-mod/go-version/annurdien/sim-cli)](https://golang.org)
[![License: MIT](https://img.shields.io/badge/License-MIT-yellow.svg)](https://opensource.org/licenses/MIT)

<div align="center">
  <img src="./assets/cli.png" alt="sim-cli Terminal Icon" width="600">
  <p><strong>iOS Simulator & Android Emulator Manager</strong></p>
  <p>A unified command-line tool to control local mobile development environments.</p>
</div>

## Quick Start

Install via Homebrew:

```bash
brew install annurdien/tap/sim-cli
```

Manage devices directly from your terminal:

```bash
# View all available devices
sim list

# Boot a specific device
sim start "iPhone 15 Pro"

# Install a compiled app
sim install app-release.apk

# Shut down the device
sim stop "iPhone 15 Pro"
```

## Platform Requirements

`sim-cli` relies on the native developer tools provided by Apple and Google. Ensure you have the correct dependencies installed for your target platform.

### iOS (macOS only)
You must install Xcode and its associated command-line tools.
- Download Xcode from the Mac App Store.
- Run `xcode-select --install` in your terminal.

### Android (macOS / Linux / Windows)
You must install the Android SDK and the emulator tools.
- Download [Android Studio](https://developer.android.com/studio).
- Open the SDK Manager and check **Android SDK Command-line Tools**.

### Optional
- **FFmpeg**: Required to convert screen recordings to GIFs. Install via Homebrew: `brew install ffmpeg`.

Run `sim doctor` to verify your environment setup.

## Feature Support

Capabilities differ slightly depending on the underlying platform APIs.

| Feature / Command | iOS | Android | Details |
|---|:---:|:---:|---|
| **Lifecycle** (`start`, `stop`, `delete`, `erase`) | ✅ | ✅ | Boot, shutdown, delete, or factory-reset devices. |
| **App Deployment** (`install`, `uninstall`) | ✅ | ✅ | Supports `.apk`, `.app`, and `.ipa` files. |
| **Media Capture** (`screenshot`, `record`) | ✅ | ✅ | Output directly to clipboard or convert to GIF. |
| **Deep Linking** (`open`) | ✅ | ✅ | Open URLs or trigger custom URI schemes. |
| **Log Streaming** (`logs`) | ✅ | ✅ | Stream system and app-level logs in real-time. |
| **File Push** (`copy to`) | ✅ | ✅ | iOS: Adds to Photos. Android: Pushes to Downloads. |
| **File Pull** (`copy from`)| ❌ | ✅ | Pull files from the emulator to the host machine. |
| **Device Cloning** (`clone`) | ✅ | ❌ | Duplicate an existing simulator state. |
| **Push Notifications** (`push`) | ✅ | ❌ | Send custom APNs JSON payloads to an app. |
| **Watch Pairing** (`pair`) | ✅ | ❌ | Pair an Apple Watch simulator with an iPhone. |
| **Camera Injection** (`cam`) | ✅ | ❌ | Inject physical Mac webcams into the simulator. |

## Usage Guide

Most commands that require a device argument will launch an interactive Text User Interface (TUI) if run without one. 

### Media Capture
```bash
# Take a screenshot and copy to clipboard
sim screenshot "Pixel 7" --copy

# Record the screen for 15 seconds and output a GIF
sim record "iPhone 15 Pro" --duration 15 --gif
```

### Deep Linking
```bash
# Auto-select the first booted device
sim open "myapp://settings"

# Target a specific device
sim open "iPhone 15 Pro" "myapp://settings"
```

### Log Filtering
```bash
# Filter logs by severity and application package
sim logs "Pixel 7" --level error --app com.example.myapp
```

### File Transfer
```bash
# Push an image to the device
sim copy to "iPhone 15 Pro" ~/Pictures/test.png

# Pull a file from Android
sim copy from "Pixel 7" /sdcard/Download/test.png ./
```

## Camera Injection

`sim-cli` includes a custom injection engine that forces iOS Simulator apps to read from the Mac's physical cameras (webcams, Continuity Cameras) instead of rendering a black screen.

To manage camera injection, run:
```bash
sim cam
```

For technical details on how `sim-cli` achieves zero-copy frame delivery using `IOSurface` and `method_exchangeImplementations`, read the [SIM-CLI Camera Architecture](docs/SIM_CAM_ARCHITECTURE.md) document.

## Configuration

Settings and runtime state are stored in `~/.sim-cli/config.json`. 

```bash
# View configuration
sim config show

# Set the default directory for screenshots and recordings
sim config set outputDir ~/Desktop/captures

# Configure GIF recording quality
sim config set gifFps 15
sim config set gifScale 320
```

Enable shell autocompletion for device names:
```bash
sim completion zsh > ~/.zshrc_sim_completion
source ~/.zshrc_sim_completion
```

## Building from Source

```bash
git clone https://github.com/annurdien/sim-cli.git
cd sim-cli
make build
make install
```

To release a new version, bump the `version` in `config.yaml` and commit with a message starting with `release:`.

## License

This project is licensed under the MIT License. See the [LICENSE](LICENSE) file for details.
