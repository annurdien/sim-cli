package cmd

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"runtime"
	"strings"
	"syscall"
	"time"

	"github.com/spf13/cobra"
)

type capturer interface {
	Screenshot(outputFile string) error
	Record(ctx context.Context, outputFile string) error
	GetName() string
}

// --- iOS Simulator ---.
type iOSSimulator struct {
	udid string
	name string
}

func newIOSSimulator(deviceNameOrUDID string) (*iOSSimulator, error) {
	udid, name := findIOSSimulator(deviceNameOrUDID)
	if udid == "" {
		return nil, ErrIOSSimulatorNotRunning
	}

	return &iOSSimulator{udid: udid, name: name}, nil
}

func (s *iOSSimulator) Screenshot(outputFile string) error {
	fmt.Printf("Taking screenshot of iOS simulator '%s'...\n", s.name)
	fullPath := ensureExtension(outputFile, ExtPNG)

	cmd := exec.Command(CmdXCrun, CmdSimctl, "io", s.udid, "screenshot", fullPath)
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed to take iOS screenshot: %w", err)
	}
	fmt.Printf("Screenshot saved to: %s\n", fullPath)

	return nil
}

func (s *iOSSimulator) Record(ctx context.Context, outputFile string) error {
	fmt.Printf("Recording iOS simulator '%s' screen...\n", s.name)
	fullPath := ensureExtension(outputFile, ExtMP4)

	cmd := exec.Command(CmdXCrun, CmdSimctl, "io", s.udid, "recordVideo", "--codec=h264", "--force", fullPath)

	if err := cmd.Start(); err != nil {
		return fmt.Errorf("failed to start iOS screen recording: %w", err)
	}

	fmt.Println("Recording started. Press Ctrl+C to stop.")

	<-ctx.Done()

	if err := cmd.Process.Signal(syscall.SIGINT); err != nil {
		_ = cmd.Process.Kill()
		return fmt.Errorf("failed to send interrupt signal to recording process: %w", err)
	}

	err := cmd.Wait()
	var exitErr *exec.ExitError
	if err != nil && !errors.As(err, &exitErr) {
		return fmt.Errorf("error during iOS screen recording: %w", err)
	}

	fmt.Printf("\nRecording saved to: %s\n", fullPath)

	return nil
}

func (s *iOSSimulator) GetName() string {
	return s.name
}

// --- Android Emulator ---

type androidEmulator struct {
	udid string
	name string
}

func newAndroidEmulator(deviceNameOrUDID string) (*androidEmulator, error) {
	udid, name := findRunningAndroidEmulator(deviceNameOrUDID)
	if udid == "" {
		return nil, ErrAndroidEmulatorNotRunning
	}

	return &androidEmulator{udid: udid, name: name}, nil
}

func (e *androidEmulator) runADB(args ...string) error {
	baseArgs := []string{"-s", e.udid}
	cmd := exec.Command(CmdAdb, append(baseArgs, args...)...)
	if output, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("adb command failed: %w\nOutput: %s", err, string(output))
	}

	return nil
}

func (e *androidEmulator) Screenshot(outputFile string) error {
	fmt.Printf("Taking screenshot of Android emulator '%s'...\n", e.name)
	fullPath := ensureExtension(outputFile, ExtPNG)
	devicePath := "/sdcard/screenshot.png"

	defer func() {
		_ = e.runADB("shell", "rm", devicePath)
	}()

	if err := e.runADB("shell", "screencap", "-p", devicePath); err != nil {
		return fmt.Errorf("failed to take Android screenshot: %w", err)
	}

	if err := e.runADB("pull", devicePath, fullPath); err != nil {
		return fmt.Errorf("failed to pull Android screenshot: %w", err)
	}

	fmt.Printf("Screenshot saved to: %s\n", fullPath)

	return nil
}

func (e *androidEmulator) Record(ctx context.Context, outputFile string) error {
	fmt.Printf("Recording Android emulator '%s' screen...\n", e.name)
	fullPath := ensureExtension(outputFile, ExtMP4)
	devicePath := "/sdcard/recording.mp4"

	defer func() {
		_ = e.runADB("shell", "rm", devicePath)
	}()

	args := []string{"-s", e.udid, "shell", "screenrecord", devicePath}
	cmd := exec.Command(CmdAdb, args...)

	if err := cmd.Start(); err != nil {
		return fmt.Errorf("failed to start Android screen recording: %w", err)
	}

	fmt.Println("Recording started. Press Ctrl+C to stop.")

	<-ctx.Done()

	if err := cmd.Process.Signal(syscall.SIGINT); err != nil {
		_ = cmd.Process.Kill()
		return fmt.Errorf("failed to send interrupt signal to adb process: %w", err)
	}

	err := cmd.Wait()
	var exitErr *exec.ExitError
	if err != nil && !errors.As(err, &exitErr) {
		return fmt.Errorf("error during Android screen recording: %w", err)
	}

	if err := e.runADB("pull", devicePath, fullPath); err != nil {
		return fmt.Errorf("failed to pull Android recording: %w", err)
	}

	fmt.Printf("\nRecording saved to: %s\n", fullPath)

	return nil
}

func (e *androidEmulator) GetName() string {
	return e.name
}

func getCapturer(deviceID string) (capturer, error) {
	if deviceID == "" {
		return getActiveDevice()
	}

	if runtime.GOOS == DarwinOS {
		if sim, err := newIOSSimulator(deviceID); err == nil {
			return sim, nil
		}
	}
	if emu, err := newAndroidEmulator(deviceID); err == nil {
		return emu, nil
	}

	return nil, ErrDeviceNotRunning
}

func getActiveDevice() (capturer, error) {
	if runtime.GOOS == DarwinOS {
		if sim, err := getRunningIOSSimulator(); err == nil {
			fmt.Printf("Active device found: iOS Simulator '%s'\n", sim.name)
			return sim, nil
		}
	}

	if emu, err := getRunningAndroidEmulator(); err == nil {
		fmt.Printf("Active device found: Android Emulator '%s'\n", emu.name)
		return emu, nil
	}

	return nil, ErrNoActiveDevice
}

func handleRecording(c capturer, outputFile string, duration int, convertToGif, shouldCopy bool) error {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	if duration > 0 {
		fmt.Printf("Recording for %d seconds...\n", duration)
		time.AfterFunc(time.Duration(duration)*time.Second, cancel)
	}

	// Handle Ctrl+C signal for graceful shutdown
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		<-sigChan
		fmt.Println("\nStopping recording...")
		cancel()
	}()

	err := c.Record(ctx, outputFile)
	if err != nil {
		return err
	}

	finalPath := outputFile
	if convertToGif {
		gifPath := strings.TrimSuffix(outputFile, ExtMP4) + ExtGIF
		if err := convertToGIF(outputFile, gifPath); err != nil {
			return err
		}
		finalPath = gifPath

		if err := os.Remove(outputFile); err != nil {
			fmt.Printf("Warning: could not remove original MP4 file: %v\n", err)
		}
	}

	if shouldCopy {
		if err := copyFileToClipboard(finalPath); err != nil {
			fmt.Printf("Warning: could not copy to clipboard: %v\n", err)
		} else {
			fileType := strings.ToUpper(strings.TrimPrefix(filepath.Ext(finalPath), "."))
			fmt.Printf("%s file copied to clipboard.\n", fileType)
		}
	}

	return nil
}

// --- Cobra ---

var screenshotCmd = &cobra.Command{
	Use:     "screenshot [device-name-or-udid] [output-file]",
	Aliases: []string{"ss", "shot"},
	Short:   "Take a screenshot of a device",
	Long:    `Take a screenshot of a running iOS simulator or Android emulator and save it to a file. If no device is specified, it will try to find the active one.`,
	Args:    cobra.RangeArgs(0, 2),
	RunE: func(cmd *cobra.Command, args []string) error {
		var deviceID, outputFile string
		if len(args) > 0 {
			_, err := newIOSSimulator(args[0])
			isDevice := err == nil
			if !isDevice {
				_, err = newAndroidEmulator(args[0])
				isDevice = err == nil
			}

			if isDevice {
				deviceID = args[0]
				if len(args) > 1 {
					outputFile = args[1]
				}
			} else {
				outputFile = args[0]
			}
		}

		c, err := getCapturer(deviceID)
		if err != nil {
			return err
		}

		if outputFile == "" {
			outputFile = generateFilename(PrefixScreenshot, c.GetName(), ExtPNG)
		}

		if err := c.Screenshot(outputFile); err != nil {
			return err
		}

		if shouldCopy, _ := cmd.Flags().GetBool("copy"); shouldCopy {
			if err := copyFileToClipboard(outputFile); err != nil {
				fmt.Printf("Warning: could not copy to clipboard: %v\n", err)
			} else {
				fmt.Println("Screenshot copied to clipboard.")
			}
		}

		return nil
	},
}

var recordCmd = &cobra.Command{
	Use:     "record [device-name-or-udid] [output-file]",
	Aliases: []string{"rec"},
	Short:   "Record screen of a device",
	Long: `Start screen recording of a running iOS simulator or Android emulator.
If no device is specified, it will try to find the active one.
The recording can be stopped by pressing Ctrl+C or by specifying a duration.`,
	Args: cobra.RangeArgs(0, 2),
	RunE: func(cmd *cobra.Command, args []string) error {
		var deviceID, outputFile string
		if len(args) > 0 {
			_, err := newIOSSimulator(args[0])
			isDevice := err == nil
			if !isDevice {
				_, err = newAndroidEmulator(args[0])
				isDevice = err == nil
			}

			if isDevice {
				deviceID = args[0]
				if len(args) > 1 {
					outputFile = args[1]
				}
			} else {
				outputFile = args[0]
			}
		}

		c, err := getCapturer(deviceID)
		if err != nil {
			return err
		}

		if outputFile == "" {
			outputFile = generateFilename(PrefixRecording, c.GetName(), ExtMP4)
		}

		duration, _ := cmd.Flags().GetInt("duration")
		convertToGif, _ := cmd.Flags().GetBool("gif")
		shouldCopy, _ := cmd.Flags().GetBool("copy")

		return handleRecording(c, outputFile, duration, convertToGif, shouldCopy)
	},
}
