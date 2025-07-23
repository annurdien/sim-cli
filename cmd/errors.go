package cmd

import "errors"

var (
	ErrNoRunningIOSSimulator     = errors.New("no running iOS simulator found")
	ErrNoRunningAndroidEmulator  = errors.New("no running Android emulator found")
	ErrIOSSimulatorNotRunning    = errors.New("iOS simulator not found or not running")
	ErrAndroidEmulatorNotRunning = errors.New("android emulator not found or not running")
	ErrDeviceNotRunning          = errors.New("device not found or not a running iOS simulator or Android emulator")
	ErrNoActiveDevice            = errors.New("no active iOS simulator or Android emulator found")
	ErrFFmpegNotInstalled        = errors.New("ffmpeg is not installed. Please install ffmpeg to use the GIF conversion feature")
)
