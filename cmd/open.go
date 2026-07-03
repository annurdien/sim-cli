package cmd

import (
	"fmt"
	"os/exec"
	"runtime"
	"strings"

	"github.com/spf13/cobra"
)

var openCmd = &cobra.Command{
	Use:     "open [device-name-or-udid] [url]",
	Aliases: []string{"o"},
	Short:   "Open a deeplink URL on a running iOS simulator or Android emulator",
	Long:    `Open a URL (deeplink) on a specific running iOS simulator or Android emulator.`,
	Args:    cobra.ExactArgs(2),
	RunE: func(cmd *cobra.Command, args []string) error {
		deviceID := args[0]
		url := args[1]

		if runtime.GOOS == DarwinOS {
			if found, err := openIOSUrl(deviceID, url); found {
				return err
			}
		}

		if found, err := openAndroidUrl(deviceID, url); found {
			return err
		}

		return fmt.Errorf("device %q: %w", deviceID, ErrDeviceNotRunning)
	},
}

// openIOSUrl opens a URL on a booted iOS simulator.
// Returns (true, nil) on success, (true, err) if found but failed to open, (false, nil) if not found.
func openIOSUrl(deviceID, url string) (bool, error) {
	device := FindIOSSimulatorByID(deviceID)
	if device == nil || !strings.Contains(strings.ToLower(device.State), "booted") {
		return false, nil
	}

	fmt.Printf("Opening URL on iOS simulator '%s'...\n", device.Name)
	cmd := exec.Command(CmdXCrun, CmdSimctl, "openurl", device.UDID, url)

	if output, err := cmd.CombinedOutput(); err != nil {
		return true, fmt.Errorf("failed to open URL on iOS simulator: %w\nOutput: %s", err, string(output))
	}

	fmt.Println("URL opened successfully.")

	return true, nil
}

// openAndroidUrl opens a URL on a running Android emulator.
// Returns (true, nil) on success, (true, err) if found but failed to open, (false, nil) if not found.
func openAndroidUrl(deviceID, url string) (bool, error) {
	udid, name := FindRunningAndroidEmulator(deviceID)
	if udid == "" {
		return false, nil
	}

	fmt.Printf("Opening URL on Android emulator '%s'...\n", name)
	cmdArgs := []string{"-s", udid, "shell", "am", "start", "-a", "android.intent.action.VIEW", "-d", url}
	cmd := exec.Command(CmdAdb, cmdArgs...)

	if output, err := cmd.CombinedOutput(); err != nil {
		return true, fmt.Errorf("failed to open URL on Android emulator: %w\nOutput: %s", err, string(output))
	}

	fmt.Println("URL opened successfully.")

	return true, nil
}
