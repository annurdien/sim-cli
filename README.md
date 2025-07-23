# SIM-CLI

[![Test and Build](https://github.com/annurdien/sim-cli/actions/workflows/ci.yml/badge.svg)](https://github.com/annurdien/sim-cli/actions/workflows/ci.yml)
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

SIM-CLI provides a simple and unified interface to manage your iOS simulators and Android emulators. Say goodbye to tedious GUI interactions and manage your devices directly from the terminal.

## ✨ Features

- **📱 Device Management**: List, start, stop, shutdown, restart, and delete simulators/emulators.
- **📸 Media Capture**: Take screenshots and record screen activity with ease.
- **📋 Clipboard Integration**: Copy screenshots and recordings directly to your clipboard.
- **GIF Conversion**: Automatically convert screen recordings to GIFs.
- **🚀 Cross-Platform**: Works on macOS (with full iOS simulator support) and Linux/Windows (Android emulators only).
- **⌨️ User-Friendly**: Clean, intuitive CLI interface with helpful error messages.
- **⚡️ Shorthand Commands**: Quick aliases for all commands (e.g., `l` for list, `s` for start).
- **🧠 Smart Device Selection**: Easily start the last used device.

## 🛠️ Installation

### Prerequisites

- **For iOS simulators**: macOS with Xcode installed.
- **For Android emulators**: Android SDK with `adb` and `emulator` tools in your PATH.

### Build from Source

```bash
git clone https://github.com/annurdien/sim-cli.git
cd sim-cli
make build
```

### Install

```bash
make install
```

## 🚀 Usage

### Quick Start

```bash
# List all available devices
sim list

# Start a device by name
sim start "iPhone 15 Pro"

# Take a screenshot and copy it to the clipboard
sim screenshot "iPhone 15 Pro" --copy

# Record a 10-second GIF
sim record "iPhone 15 Pro" --duration 10 --gif

# Stop the device
sim stop "iPhone 15 Pro"
```

## 📚 Commands Reference

Here is a complete list of available commands and their options.

| Command | Aliases | Description |
|---|---|---|
| `list` | `l`, `ls` | List all available simulators and emulators. |
| `start <device>` | `s` | Start a simulator or emulator. Use `lts` to start the last used device. |
| `stop <device>` | `st` | Stop a running simulator or emulator. |
| `shutdown <device>` | `sd` | Shutdown a simulator or emulator. |
| `restart <device>` | `r` | Restart a simulator or emulator. |
| `delete <device>` | `d`, `del` | **Permanently** delete a simulator or emulator. |
| `screenshot <device> [file]` | `ss`, `shot` | Take a screenshot of a device. |
| `record <device> [file]` | `rec` | Record screen activity of a device. |
| `last` | - | Show the last used device. |
| `lts` | - | A shorthand to start the last used device (`sim start lts`). |
| `help` | - | Show help information. |
| `version` | `-v` | Show version information. |

### `screenshot` Options

| Flag | Shorthand | Description |
|---|---|---|
| `--copy` | `-c` | Copy the screenshot to the clipboard. |

### `record` Options

| Flag | Shorthand | Description |
|---|---|---|
| `--duration` | `-d` | Duration of the recording in seconds (e.g., `--duration 15`). |
| `--gif` | `-g` | Convert the recording to a GIF. |
| `--copy` | `-c` | Copy the recording file path to the clipboard. |


## ⚠️ Safety & Best Practices

### Delete Command

The `delete` command is destructive and **permanently removes** the simulator or emulator.

- Always double-check the device name or UDID with `sim list` before deleting.
- Running devices are automatically stopped before deletion.
- Deleted devices must be recreated through Xcode (for iOS) or the AVD Manager (for Android).

## 🤝 Contributing

Contributions are welcome! Please feel free to submit a pull request.

1.  Fork the repository.
2.  Create your feature branch (`git checkout -b feature/AmazingFeature`).
3.  Commit your changes (`git commit -m 'Add some AmazingFeature'`).
4.  Push to the branch (`git push origin feature/AmazingFeature`).
5.  Open a pull request.

## 📄 License

This project is licensed under the MIT License - see the [LICENSE](LICENSE) file for details.
