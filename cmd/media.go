package cmd

import (
	"fmt"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"github.com/spf13/cobra"
)

var screenshotCmd = &cobra.Command{
	Use:     "screenshot [device-name-or-udid] [output-file]",
	Aliases: []string{"ss", "shot"},
	Short:   "Take a screenshot of an iOS simulator or Android emulator",
	Long:    `Take a screenshot of a running iOS simulator or Android emulator and save it to a file.`,
	Args:    cobra.RangeArgs(1, 2),
	Run: func(cmd *cobra.Command, args []string) {
		deviceID := args[0]

		// Generate output filename if not provided
		outputFile := ""
		if len(args) > 1 {
			outputFile = args[1]
		} else {
			timestamp := time.Now().Format("20060102_150405")
			outputFile = fmt.Sprintf("screenshot_%s_%s.png", deviceID, timestamp)
		}

		if runtime.GOOS == "darwin" {
			if takeIOSScreenshot(deviceID, outputFile) {
				return
			}
		}

		if takeAndroidScreenshot(deviceID, outputFile) {
			return
		}

		fmt.Printf("Device '%s' not found or failed to take screenshot\n", deviceID)
	},
}

var recordCmd = &cobra.Command{
	Use:     "record [device-name-or-udid] [output-file]",
	Aliases: []string{"rec"},
	Short:   "Record screen of an iOS simulator or Android emulator",
	Long:    `Start screen recording of a running iOS simulator or Android emulator.`,
	Args:    cobra.RangeArgs(1, 2),
	Run: func(cmd *cobra.Command, args []string) {
		deviceID := args[0]

		// Generate output filename if not provided
		outputFile := ""
		if len(args) > 1 {
			outputFile = args[1]
		} else {
			timestamp := time.Now().Format("20060102_150405")
			outputFile = fmt.Sprintf("recording_%s_%s.mp4", deviceID, timestamp)
		}

		duration, _ := cmd.Flags().GetInt("duration")

		if runtime.GOOS == "darwin" {
			if recordIOSScreen(deviceID, outputFile, duration) {
				return
			}
		}

		if recordAndroidScreen(deviceID, outputFile, duration) {
			return
		}

		fmt.Printf("Device '%s' not found or failed to start recording\n", deviceID)
	},
}

func takeIOSScreenshot(deviceID, outputFile string) bool {
	udid := findIOSSimulatorUDID(deviceID)
	if udid == "" {
		return false
	}

	fmt.Printf("Taking screenshot of iOS simulator '%s'...\n", deviceID)

	if !strings.HasSuffix(strings.ToLower(outputFile), ".png") {
		outputFile = strings.TrimSuffix(outputFile, filepath.Ext(outputFile)) + ".png"
	}

	cmd := exec.Command("xcrun", "simctl", "io", udid, "screenshot", outputFile)
	if err := cmd.Run(); err != nil {
		fmt.Printf("Error taking iOS screenshot: %v\n", err)
		return false
	}

	fmt.Printf("Screenshot saved to: %s\n", outputFile)
	return true
}

func takeAndroidScreenshot(deviceID, outputFile string) bool {
	runningUDID := findRunningAndroidEmulator(deviceID)
	if runningUDID == "" {
		return false
	}

	fmt.Printf("Taking screenshot of Android emulator '%s'...\n", deviceID)

	if !strings.HasSuffix(strings.ToLower(outputFile), ".png") {
		outputFile = strings.TrimSuffix(outputFile, filepath.Ext(outputFile)) + ".png"
	}

	devicePath := "/sdcard/screenshot.png"
	screenshotCmd := exec.Command("adb", "-s", runningUDID, "shell", "screencap", "-p", devicePath)
	if err := screenshotCmd.Run(); err != nil {
		fmt.Printf("Error taking Android screenshot: %v\n", err)
		return false
	}

	pullCmd := exec.Command("adb", "-s", runningUDID, "pull", devicePath, outputFile)
	if err := pullCmd.Run(); err != nil {
		fmt.Printf("Error pulling Android screenshot: %v\n", err)
		return false
	}

	cleanupCmd := exec.Command("adb", "-s", runningUDID, "shell", "rm", devicePath)
	cleanupCmd.Run() // Ignore errors

	fmt.Printf("Screenshot saved to: %s\n", outputFile)
	return true
}

func recordIOSScreen(deviceID, outputFile string, duration int) bool {
	udid := findIOSSimulatorUDID(deviceID)
	if udid == "" {
		return false
	}

	fmt.Printf("Recording iOS simulator '%s' screen...\n", deviceID)

	if !strings.HasSuffix(strings.ToLower(outputFile), ".mp4") {
		outputFile = strings.TrimSuffix(outputFile, filepath.Ext(outputFile)) + ".mp4"
	}

	args := []string{"simctl", "io", udid, "recordVideo"}
	if duration > 0 {
		fmt.Printf("Recording for %d seconds...\n", duration)
		// Note: simctl doesn't have a built-in duration option, so we'll need to handle this differently
		// For now, we'll start recording and let the user stop it manually
	}
	args = append(args, outputFile)

	cmd := exec.Command("xcrun", args...)

	if duration > 0 {
		if err := cmd.Start(); err != nil {
			fmt.Printf("Error starting iOS screen recording: %v\n", err)
			return false
		}

		time.Sleep(time.Duration(duration) * time.Second)

		if err := cmd.Process.Kill(); err != nil {
			fmt.Printf("Error stopping recording: %v\n", err)
		}

		cmd.Wait()
	} else {
		fmt.Println("Press Ctrl+C to stop recording...")
		if err := cmd.Run(); err != nil {
			fmt.Printf("Error recording iOS screen: %v\n", err)
			return false
		}
	}

	fmt.Printf("Recording saved to: %s\n", outputFile)
	return true
}

func recordAndroidScreen(deviceID, outputFile string, duration int) bool {
	runningUDID := findRunningAndroidEmulator(deviceID)
	if runningUDID == "" {
		return false
	}

	fmt.Printf("Recording Android emulator '%s' screen...\n", deviceID)

	if !strings.HasSuffix(strings.ToLower(outputFile), ".mp4") {
		outputFile = strings.TrimSuffix(outputFile, filepath.Ext(outputFile)) + ".mp4"
	}

	devicePath := "/sdcard/recording.mp4"

	args := []string{"-s", runningUDID, "shell", "screenrecord"}
	if duration > 0 {
		args = append(args, "--time-limit", fmt.Sprintf("%d", duration))
		fmt.Printf("Recording for %d seconds...\n", duration)
	} else {
		fmt.Println("Press Ctrl+C to stop recording...")
	}
	args = append(args, devicePath)

	cmd := exec.Command("adb", args...)
	if err := cmd.Run(); err != nil {
		fmt.Printf("Error recording Android screen: %v\n", err)
		return false
	}

	pullCmd := exec.Command("adb", "-s", runningUDID, "pull", devicePath, outputFile)
	if err := pullCmd.Run(); err != nil {
		fmt.Printf("Error pulling Android recording: %v\n", err)
		return false
	}

	cleanupCmd := exec.Command("adb", "-s", runningUDID, "shell", "rm", devicePath)
	cleanupCmd.Run() // Ignore errors

	fmt.Printf("Recording saved to: %s\n", outputFile)
	return true
}

func init() {
	rootCmd.AddCommand(screenshotCmd)
	rootCmd.AddCommand(recordCmd)

	recordCmd.Flags().IntP("duration", "d", 0, "Recording duration in seconds (0 for manual stop)")
}
