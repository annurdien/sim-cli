package cmd

import (
	"fmt"

	"github.com/spf13/cobra"
)

var doctorCmd = &cobra.Command{
	Use:   "doctor",
	Short: "Check system dependencies for sim-cli",
	Long:  `Check if all required and optional system dependencies (Xcode, Android SDK, ffmpeg) are installed.`,
	Run: func(cmd *cobra.Command, args []string) {
		PrintInfo("Checking sim-cli dependencies...")
		PrintInfo("--------------------------------")
		allGood := true

		fmt.Print("iOS (xcrun simctl): ")
		if CommandExists(CmdXCrun) {
			PrintInfo("✅ Installed")
		} else {
			PrintInfo("❌ Not found (Xcode Command Line Tools required for iOS)")
			allGood = false
		}

		fmt.Print("Android (adb & emulator): ")
		if CommandExists(CmdAdb) && CommandExists(CmdEmulator) {
			PrintInfo("✅ Installed")
		} else {
			PrintInfo("❌ Not found (Android SDK required for Android)")
			allGood = false
		}

		fmt.Print("GIF Conversion (ffmpeg): ")
		if CommandExists(CmdFFmpeg) {
			PrintInfo("✅ Installed")
		} else {
			PrintInfo("❌ Not found (Required only for recording GIFs)")
			// Optional dependency, doesn't mark allGood as false.
		}

		PrintInfo("--------------------------------")
		if !allGood {
			PrintInfo("Warning: Some core dependencies are missing.")
			PrintInfo("You can still use sim-cli, but commands for the missing platforms will fail.")
		} else {
			PrintInfo("All core dependencies are met! 🎉")
		}
	},
}
