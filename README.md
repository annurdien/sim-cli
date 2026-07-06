# SIM-CLI

[![Test and Build](https://github.com/annurdien/sim-cli/actions/workflows/ci.yml/badge.svg)](https://github.com/annurdien/sim-cli/actions/workflows/ci.yml)
[![Release Pipeline](https://github.com/annurdien/sim-cli/actions/workflows/release.yml/badge.svg)](https://github.com/annurdien/sim-cli/actions/workflows/release.yml)
[![GitHub Release](https://img.shields.io/github/v/release/annurdien/sim-cli)](https://github.com/annurdien/sim-cli/releases)
[![Go Version](https://img.shields.io/github/go-mod/go-version/annurdien/sim-cli)](https://golang.org)
[![License: MIT](https://img.shields.io/badge/License-MIT-yellow.svg)](https://opensource.org/licenses/MIT)

<div align="center">
  <img src="./assets/cli.png" alt="SIM-CLI Terminal Icon" width="600">
  <p>
    <strong>iOS Simulator & Android Emulator Manager</strong>
  </p>
  <p>
    A powerful, cross-platform command-line tool to streamline your mobile development workflow.
  </p>
</div>

## Overview

SIM-CLI provides a simple and unified interface to manage your iOS simulators and Android emulators. Stop switching between GUIs and manage all your devices directly from the terminal.

## Features

- **Device Management**: List, start, stop, shutdown, restart, and delete simulators/emulators.
- **Deep Linking**: Instantly open any URL or deeplink on a running device.
- **Media Capture**: Take screenshots and record screen activity with ease.
- **Clipboard Integration**: Copy screenshots and recordings directly to your clipboard.
- **GIF Conversion**: Automatically convert screen recordings to GIFs (requires `ffmpeg`).
- **Dependency Check**: Built-in `doctor` command to verify all system dependencies.
- **Cross-Platform**: Works on macOS (full iOS + Android support) and Linux/Windows (Android emulators only).
- **Shorthand Commands**: Quick aliases for all commands (e.g., `l` for list, `s` for start).
- **Smart Device Selection**: Start the last used device with a single `sim lts` command.

## Installation

### Install via Homebrew (macOS/Linux)

The recommended way to install SIM-CLI is via Homebrew. This will also automatically install `ffmpeg`:

```bash
brew tap annurdien/tap
brew install sim-cli
```

Or as a one-liner:

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

| Dependency | Required for | Install |
|---|---|---|
| Xcode / Command Line Tools | iOS simulators | App Store / `xcode-select --install` |
| Android SDK (`adb`, `emulator`) | Android emulators | Android Studio |
| `ffmpeg` | GIF recording | `brew install ffmpeg` |

After installing, run `sim doctor` to verify your setup.

## Usage

### Quick Start

```bash
# 1. Verify all dependencies are in place
sim doctor

# 2. List all available devices
sim list

# 3. Start a device by name
sim start "iPhone 15 Pro"

# 4. Take a screenshot and copy it to the clipboard
sim screenshot "iPhone 15 Pro" --copy

# 5. Open a deeplink or URL (auto-selects first booted device)
sim open "myapp://home"

# 6. Record a 10-second GIF
sim record "iPhone 15 Pro" --duration 10 --gif

# 7. Stop the device
sim stop "iPhone 15 Pro"
```

### Smart Device Shortcuts

```bash
# Start the last device you used
sim lts

# See which device was last used
sim last
```

## Commands Reference

| Command | Aliases | Description |
|---|---|---|
| `doctor` | - | Check all system dependencies (Xcode, Android SDK, ffmpeg). |
| `list` | `l`, `ls` | List all available simulators and emulators. |
| `start <device>` | `s` | Start a simulator or emulator by name or UDID. |
| `stop <device>` | `st` | Stop a running simulator or emulator. |
| `shutdown <device>` | `sd` | Shutdown a simulator or emulator. |
| `restart <device>` | `r` | Restart a simulator or emulator. |
| `delete <device>` | `d`, `del` | **Permanently** delete a simulator or emulator. |
| `open [device] <url>` | `o` | Open a deeplink or URL. Device is optional — defaults to the first booted device. |
| `screenshot <device> [file]` | `ss`, `shot` | Take a screenshot of a device. |
| `record <device> [file]` | `rec` | Record the screen of a device. |
| `last` | - | Show the last used device. |
| `lts` | - | Start the last used device (short for `sim start lts`). |
| `version` | `-v` | Show the current version. |
| `help` | - | Show help information. |

### screenshot Options

| Flag | Shorthand | Description |
|---|---|---|
| `--copy` | `-c` | Copy the screenshot to the clipboard after saving. |

### record Options

| Flag | Shorthand | Description |
|---|---|---|
| `--duration` | `-d` | Duration of the recording in seconds (e.g., `--duration 15`). |
| `--gif` | `-g` | Convert the recording to a GIF after it completes. |
| `--copy` | `-c` | Copy the output file path to the clipboard. |

### open Options

The `open` command accepts any valid URL or custom URI scheme. The device argument is optional:

```bash
# Auto-select the first booted device
sim open "myapp://settings"
sim o "https://example.com"

# Target a specific device
sim open "iPhone 15 Pro" "myapp://settings"
sim open "Pixel_7_API_34" "https://example.com"
```

## Configuration

SIM-CLI stores its runtime state (e.g., last used device) in a JSON file at:

```
~/.sim-cli/config.json
```

This file is managed automatically. You do not need to edit it manually.

The project-level `config.yaml` in the repository root defines the release version and metadata used by the build system and CI/CD pipeline:

```yaml
app:
  name: "sim-cli"
  version: "1.6.0"
  description: "CLI tool to manage iOS simulators and Android emulators"
```

To release a new version, bump `version` in `config.yaml` and push with `release:` in the commit message.

## Safety & Best Practices

### Delete Command

The `delete` command is destructive and **permanently removes** the simulator or emulator.

- Always double-check the device name or UDID with `sim list` before deleting.
- Running devices are automatically shut down before deletion.
- Deleted simulators must be recreated through Xcode (for iOS) or the AVD Manager (for Android).

## Contributing

Contributions are welcome! Please feel free to submit a pull request.

1. Fork the repository.
2. Create your feature branch (`git checkout -b feature/AmazingFeature`).
3. Commit your changes (`git commit -m 'Add some AmazingFeature'`).
4. Push to the branch (`git push origin feature/AmazingFeature`).
5. Open a pull request.

## License

This project is licensed under the MIT License - see the [LICENSE](LICENSE) file for details.
