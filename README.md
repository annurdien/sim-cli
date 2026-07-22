# sim-cli

[![Test and Build](https://github.com/annurdien/sim-cli/actions/workflows/ci.yml/badge.svg)](https://github.com/annurdien/sim-cli/actions/workflows/ci.yml)
[![Release Pipeline](https://github.com/annurdien/sim-cli/actions/workflows/release.yml/badge.svg)](https://github.com/annurdien/sim-cli/actions/workflows/release.yml)
[![GitHub Release](https://img.shields.io/github/v/release/annurdien/sim-cli)](https://github.com/annurdien/sim-cli/releases)
[![Go Version](https://img.shields.io/github/go-mod/go-version/annurdien/sim-cli)](https://golang.org)
[![License: MIT](https://img.shields.io/badge/License-MIT-yellow.svg)](https://opensource.org/licenses/MIT)

<div align="center">
  <img src="./assets/cli.png" alt="sim-cli Terminal Icon" width="600">
  <p>
    <strong>iOS Simulator & Android Emulator Manager</strong>
  </p>
  <p>
    A cross-platform command-line tool for managing mobile development environments.
  </p>
</div>

## Overview

`sim-cli` provides a unified interface for iOS simulators and Android emulators. It allows you to manage devices, install apps, capture media, and view logs directly from the terminal.

## Features

- **Device Management**: List, start, stop, restart, delete, and erase simulators and emulators.
- **App Management**: Install and uninstall `.apk`, `.app`, and `.ipa` files.
- **Media Capture**: Take screenshots, record screen activity, and copy outputs to the clipboard.
- **GIF Conversion**: Convert screen recordings to GIFs using `ffmpeg`.
- **Deep Linking**: Open URLs or deeplinks on running devices.
- **Camera Injection**: Inject physical webcams or static images into iOS simulators.
- **Shell Autocompletion**: Tab completion for bash, zsh, fish, and PowerShell.

## Installation

### Homebrew (macOS/Linux)

Install `sim-cli` and `ffmpeg` via Homebrew:

```bash
brew install annurdien/tap/sim-cli
```

### Build from Source

```bash
git clone https://github.com/annurdien/sim-cli.git
cd sim-cli
make build
make install
```

### Prerequisites

`sim-cli` interacts with native tools provided by Apple and Google. You need the following dependencies installed based on your target platform:

**iOS Simulators (macOS only):**
- **Xcode / Command Line Tools**: Install via the Mac App Store or run `xcode-select --install`.

**Android Emulators:**
- **Android SDK / Command-line Tools**: Requires `adb` and the `emulator` tool. Install via [Android Studio](https://developer.android.com/studio). Check **Android SDK Command-line Tools** in the SDK Manager.

**Optional:**
- **ffmpeg**: Required for GIF recording (`brew install ffmpeg`).

Run `sim doctor` after installation to verify dependencies.

## Usage

### Quick Start

```bash
# List available devices
sim list

# Start a device
sim start "iPhone 15 Pro"

# Install an app
sim install app-release.apk

# Stop the device
sim stop "iPhone 15 Pro"
```

### Shell Autocompletion

Enable tab completion for device names and commands:

```bash
# Zsh
sim completion zsh > ~/.zshrc_sim_completion
source ~/.zshrc_sim_completion

# Bash
sim completion bash > /etc/bash_completion.d/sim

# Fish
sim completion fish > ~/.config/fish/completions/sim.fish
```

## Feature Support

| Feature / Command | iOS | Android | Notes |
|---|---|---|---|
| **Core Lifecycle** (`start`, `stop`, `delete`, `erase`) | ✅ | ✅ | Full support on both platforms. |
| **App Management** (`install`, `uninstall`) | ✅ | ✅ | Handles `.apk`, `.app`, and `.ipa`. |
| **Media** (`screenshot`, `record`) | ✅ | ✅ | Includes clipboard copy and GIF conversion. |
| **Deep Linking** (`open`) | ✅ | ✅ | Opens URLs or custom URI schemes. |
| **Real-time Logs** (`logs`) | ✅ | ✅ | Streams and filters system and app logs. |
| **Copy File To Device** (`copy to`) | ✅ | ✅ | iOS: Adds to Photos. Android: Pushes to Download. |
| **Copy File From Device** (`copy from`)| ❌ | ✅ | Pulls files from Android to the local machine. |
| **Clone Device** (`clone`) | ✅ | ❌ | Duplicates an existing simulator. |
| **Push Notifications** (`push`) | ✅ | ❌ | Sends custom push payloads. |
| **Watch Pairing** (`pair`) | ✅ | ❌ | Pairs an Apple Watch with an iPhone simulator. |
| **Camera Injection** (`cam`) | ✅ | ❌ | Injects frames into the iOS Simulator camera. |

## Commands Reference

| Command | Aliases | Description |
|---|---|---|
| `doctor` | - | Check system dependencies (Xcode, Android SDK, ffmpeg). |
| `list` | `l`, `ls` | List available simulators and emulators. |
| `start <device>` | `s` | Start a device by name or UDID. |
| `stop <device>` | `st`, `sd`, `shutdown` | Stop or shut down a running device. |
| `restart <device>` | `r` | Restart a device. |
| `delete <device>` | `d`, `del` | Permanently delete a device. |
| `erase <device>` | `reset` | Factory reset a device. |
| `clone <source> <new>`| - | Clone an iOS simulator. |
| `install [dev] <app>`| `i` | Install an app (`.apk`, `.app`, `.ipa`). |
| `uninstall [dev] <id>`| `u`, `remove`| Uninstall an app by ID or package name. |
| `open [device] <url>` | `o` | Open a deeplink or URL. |
| `screenshot <device> [file]` | `ss`, `shot` | Take a screenshot. |
| `record <device> [file]` | `rec` | Record the screen. |
| `logs [device]` | `log` | Stream real-time logs. |
| `push [dev] <id> <file>`| - | Send a push notification (iOS only). |
| `create` | - | Create a new iOS simulator or Android emulator. |
| `status` | - | Show a dashboard of running devices. |
| `copy to/from` | - | Transfer files to or from a device. |
| `pair [watch] [phone]` | - | Pair an Apple Watch simulator with an iPhone simulator. |
| `config` | - | Manage sim-cli configuration values. |
| `last` | - | Show the last used device. |
| `lts` | - | Start the last used device. |
| `completion <shell>` | - | Generate a shell autocompletion script. |
| `cam` | - | iOS Simulator camera injection and management. |
| `version` | `-v` | Show the current version. |
| `help` | - | Show help information. |

## Interactive Mode (TUI)

Commands that require a device argument (`start`, `stop`, `shutdown`, `restart`, `delete`, `erase`, `logs`, `pair`) will launch an interactive Text User Interface (TUI) if run without an argument, allowing you to select a target device.

### screenshot Options

| Flag | Shorthand | Description |
|---|---|---|
| `--copy` | `-c` | Copy the screenshot to the clipboard. |

### record Options

| Flag | Shorthand | Description |
|---|---|---|
| `--duration` | `-d` | Recording duration in seconds (e.g., `--duration 15`). |
| `--gif` | `-g` | Convert the recording to a GIF. |
| `--copy` | `-c` | Copy the output file path to the clipboard. |

### open Options

The `open` command accepts URLs or custom URI schemes. If no device is specified, it targets the first booted device:

```bash
# Auto-select the first booted device
sim open "myapp://settings"

# Target a specific device
sim open "iPhone 15 Pro" "myapp://settings"
```

### logs Options

| Flag | Shorthand | Description |
|---|---|---|
| `--level` | `-l` | Filter logs by level (`debug`, `info`, `warn`, `error`). |
| `--filter` | `-f` | Filter logs with a grep pattern. |
| `--app` | `-a` | Filter logs by app bundle ID (iOS) or package name (Android). |

### push Options

| Flag | Shorthand | Description |
|---|---|---|
| `--template` | `-t` | Generate a sample payload template (`push.json`). |

### create Options

| Flag | Shorthand | Description |
|---|---|---|
| `--ios` / `--android` | - | Specify the target platform (required). |
| `--name` | `-n` | The name for the new device. |
| `--type` | `-t` | The hardware device type (e.g., `iPhone-15`). |
| `--runtime` | `-r` | The OS runtime/system image (e.g., `iOS-17-0`). |
| `--list-types` | - | Display available hardware types and OS runtimes. |

### copy Usage

```bash
# Copy a local file to a device (iOS: Photos, Android: /sdcard/Download)
sim copy to "iPhone 15 Pro" ~/Pictures/test.png

# Copy a remote file from an Android device to your machine
sim copy from "Pixel_7" /sdcard/Download/test.png ./
```

## Camera Injection

`sim-cli` allows you to inject physical webcams, Continuity Cameras, or static images into iOS Simulator applications.

Open the camera management dashboard:
```bash
sim cam
```

Read the [SIM-CLI Camera Architecture](docs/SIM_CAM_ARCHITECTURE.md) for technical details on shared memory and dylib injection.

## Configuration

Configuration and runtime state (like the last used device) are stored in `~/.sim-cli/config.json`. Manage settings using the `config` command:

```bash
# View configuration
sim config show

# Set the default directory for captures
sim config set outputDir ~/Desktop/captures

# Set GIF recording framerate and scale
sim config set gifFps 15
sim config set gifScale 320
```

Supported keys: `defaultDevice`, `outputDir`, `gifFps`, `gifScale`.

The repository's `config.yaml` file stores version metadata for the build system:

```yaml
app:
  name: "sim-cli"
  version: "1.7.0"
  description: "CLI tool to manage iOS simulators and Android emulators"
```

To release a new version, bump the `version` in `config.yaml` and commit with a message starting with `release:`.

## Delete and Erase Commands

- `delete`: Permanently removes the simulator or emulator. Recreate the device via Xcode or AVD Manager.
- `erase`: Wipes all user data on the device (factory reset).

Use the `--force` or `-f` flag to bypass the interactive confirmation prompt.

## Contributing

1. Fork the repository.
2. Create a feature branch (`git checkout -b feature/name`).
3. Commit your changes (`git commit -m 'Add feature'`).
4. Push to the branch (`git push origin feature/name`).
5. Open a pull request.

## License

MIT License. See [LICENSE](LICENSE).
