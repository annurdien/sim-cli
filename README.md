# SIM-CLI

CLI tool to manage iOS simulators and Android emulators with ease.

## Overview

SIM-CLI is a cross-platform command-line tool that provides a unified interface for managing iOS simulators and Android emulators. It simplifies common tasks like listing devices, starting/stopping simulators, taking screenshots, and recording screen activity.

## Features

- **Device Management**: List, start, stop, shutdown, and restart simulators/emulators
- **Media Capture**: Take screenshots and record screen activity
- **Cross-Platform**: Works on macOS (with full iOS simulator support) and Linux/Windows (Android emulators only)
- **User-Friendly**: Clean, intuitive CLI interface with helpful error messages

## Installation

### Prerequisites

- **For iOS simulators**: macOS with Xcode installed
- **For Android emulators**: Android SDK with `adb` and `emulator` tools in PATH

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

## Usage

### List Available Devices

```bash
sim list
```

Example output:
```
┌───────────────┬───────────────────────────────────────┬──────────┬──────────────────────────────────────┬──────────┐
│     TYPE      │                 NAME                  │  STATE   │                 UDID                 │ RUNTIME  │
├───────────────┼───────────────────────────────────────┼──────────┼──────────────────────────────────────┼──────────┤
│ iOS Simulator │ iPhone 15 Pro                         │ Shutdown │ AD3D3E8D-785C-4982-BADE-6D9A20D195A5 │ iOS 17.5 │
│ iOS Simulator │ iPhone 15 Pro Max                     │ Shutdown │ 9A3BAC90-B6A4-43B9-A6FF-40192D23F4D1 │ iOS 17.5 │
│ iOS Simulator │ iPhone 17.5                           │ Shutdown │ 28823504-3E61-427B-BB79-C8A845A501FC │ iOS 17.5 │
│ iOS Simulator │ iPhone 15 Plus                        │ Shutdown │ 323FE005-AF77-448B-AF0B-C4AC20DFBF6B │ iOS 17.5 │
└───────────────┴───────────────────────────────────────┴──────────┴──────────────────────────────────────┴──────────┘
```

### Start a Device

```bash
# Start by name
sim start "iPhone 15 Pro"

# Start by UDID
sim start A1B2C3D4-E5F6-G7H8-I9J0-K1L2M3N4O5P6
```

### Stop a Device

```bash
sim stop "iPhone 15 Pro"
```

### Restart a Device

```bash
sim restart "Pixel_7_API_34"
```

### Take a Screenshot

```bash
# Auto-generated filename
sim screenshot "iPhone 15 Pro"

# Custom filename
sim screenshot "iPhone 15 Pro" my_screenshot.png
```

### Record Screen

```bash
# Record until manually stopped (Ctrl+C)
sim record "iPhone 15 Pro"

# Record for specific duration (in seconds)
sim record "iPhone 15 Pro" --duration 30

# Custom filename
sim record "iPhone 15 Pro" my_recording.mp4 --duration 60
```

## Commands Reference

| Command | Description |
|---------|-------------|
| `list` | List all available simulators and emulators |
| `start <device>` | Start a simulator or emulator |
| `stop <device>` | Stop a running simulator or emulator |
| `shutdown <device>` | Shutdown a simulator or emulator |
| `restart <device>` | Restart a simulator or emulator |
| `screenshot <device> [file]` | Take a screenshot |
| `record <device> [file]` | Record screen activity |
| `help` | Show help information |
| `version` | Show version information |

## Device Identification

Devices can be identified by:
- **Name**: Exact device name (e.g., "iPhone 15 Pro")
- **UDID**: Unique device identifier

Use `sim list` to see available devices and their identifiers.

## Platform Support

- **macOS**: Full support for iOS simulators and Android emulators
- **Linux/Windows**: Android emulator support only

## Contributing

1. Fork the repository
2. Create a feature branch
3. Make your changes
4. Test thoroughly
5. Submit a pull request

## License

MIT License - see LICENSE file for details.
