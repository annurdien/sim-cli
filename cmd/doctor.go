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
		fmt.Println("Checking sim-cli dependencies...")
		fmt.Println("--------------------------------")
		allGood := true

		fmt.Print("iOS (xcrun simctl): ")
		if CommandExists(CmdXCrun) {
			fmt.Println("✅ Installed")
		} else {
			fmt.Println("❌ Not found (Xcode Command Line Tools required for iOS)")
			allGood = false
		}

		fmt.Print("Android (adb & emulator): ")
		if CommandExists(CmdAdb) && CommandExists(CmdEmulator) {
			fmt.Println("✅ Installed")
		} else {
			fmt.Println("❌ Not found (Android SDK required for Android)")
			allGood = false
		}

		fmt.Print("GIF Conversion (ffmpeg): ")
		if CommandExists(CmdFFmpeg) {
			fmt.Println("✅ Installed")
		} else {
			fmt.Println("❌ Not found (Required only for recording GIFs)")
			// Optional dependency, doesn't mark allGood as false.
		}

		fmt.Println("--------------------------------")
		if !allGood {
			fmt.Println("Warning: Some core dependencies are missing.")
			fmt.Println("You can still use sim-cli, but commands for the missing platforms will fail.")
		} else {
			fmt.Println("All core dependencies are met! 🎉")
		}
	},
}
