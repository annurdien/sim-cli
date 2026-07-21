package cmd

import "errors"

var (
	// ErrNoRunningIOSSimulator is returned when no booted iOS simulator is found.
	ErrNoRunningIOSSimulator = errors.New("no running iOS simulator found")
	// ErrNoRunningAndroidEmulator is returned when no running Android emulator is found.
	ErrNoRunningAndroidEmulator = errors.New("no running Android emulator found")
	// ErrIOSSimulatorNotRunning is returned when a specific iOS simulator is not found or not running.
	ErrIOSSimulatorNotRunning = errors.New("iOS simulator not found or not running")
	// ErrAndroidEmulatorNotRunning is returned when a specific Android emulator is not found or not running.
	ErrAndroidEmulatorNotRunning = errors.New("android emulator not found or not running")
	// ErrDeviceNotRunning is returned when no matching running device is found.
	ErrDeviceNotRunning = errors.New("device not found or not a running iOS simulator or Android emulator")
	// ErrNoActiveDevice is returned when no active device is found for auto-selection.
	ErrNoActiveDevice = errors.New("no active iOS simulator or Android emulator found")
	// ErrFFmpegNotInstalled is returned when ffmpeg is required but not found in PATH.
	ErrFFmpegNotInstalled = errors.New("ffmpeg is not installed. Please install ffmpeg to use the GIF conversion feature")
	// ErrDeviceNotFound is returned when a device cannot be located by name or UDID.
	ErrDeviceNotFound = errors.New("device not found")
	// ErrInvalidDuration is returned when a recording duration value is invalid.
	ErrInvalidDuration = errors.New("invalid recording duration")
	// ErrNoLastDevice is returned when no last started device configuration is found.
	ErrNoLastDevice = errors.New("no last started device found; start a device first to use 'lts'")
	// ErrUnsupportedAppFormat is returned when the app file extension is not supported for the target device.
	ErrUnsupportedAppFormat = errors.New("unsupported app format for target device")
	// ErrInstallFailed is returned when an app fails to install.
	ErrInstallFailed = errors.New("failed to install app")
	// ErrUninstallFailed is returned when an app fails to uninstall.
	ErrUninstallFailed = errors.New("failed to uninstall app")
	// ErrIOSMacOnly is returned when an iOS-only operation is attempted on non-macOS.
	ErrIOSMacOnly = errors.New("iOS operations are only supported on macOS")
	// ErrAndroidCloneNotSupported is returned when attempting to clone an Android emulator.
	ErrAndroidCloneNotSupported = errors.New("cloning not supported for Android emulators")
	// ErrNoAppsInstalled is returned when no (non-system) apps are found on the simulator.
	ErrNoAppsInstalled = errors.New("no apps installed on this simulator")
	// ErrDefaultsNotFound is returned when no UserDefaults plist file exists for an app.
	ErrDefaultsNotFound = errors.New("no UserDefaults plist found for this app")
)
