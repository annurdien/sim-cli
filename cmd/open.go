package cmd

import (
	"fmt"
	"runtime"

	"github.com/spf13/cobra"
)

var openCmd = &cobra.Command{
	Use:     "open [device-name-or-udid] <url>",
	Aliases: []string{"o"},
	Short:   "Open a deeplink URL on a running iOS simulator or Android emulator",
	Long: `Open a URL (deeplink) on a running iOS simulator or Android emulator.

If no device is specified, the first booted device is used automatically.

Examples:
  sim open "myapp://home"
  sim open "iPhone 15 Pro" "myapp://home"
  sim open "Pixel_7_API_34" "https://example.com"`,
	Args: cobra.RangeArgs(1, 2),
	RunE: func(cmd *cobra.Command, args []string) error {
		var deviceID, url string

		if len(args) == 1 {
			// Only URL provided — auto-select the first booted device.
			url = args[0]
			deviceID = ""
		} else {
			deviceID = args[0]
			url = args[1]
		}

		return openURL(deviceID, url)
	},
}

// openURL dispatches to the appropriate platform handler.
// Pass an empty deviceID to auto-select the first booted device.
func openURL(deviceID, url string) error {
	if runtime.GOOS == DarwinOS {
		if found, err := openIOSUrl(deviceID, url); found {
			return err
		}
	}

	if found, err := openAndroidUrl(deviceID, url); found {
		return err
	}

	if deviceID == "" {
		return fmt.Errorf("no booted device found: %w", ErrDeviceNotRunning)
	}

	return fmt.Errorf("device %q: %w", deviceID, ErrDeviceNotRunning)
}

// openIOSUrl opens a URL on a booted iOS simulator.
// If deviceID is empty, uses the first booted simulator.
// Returns (true, nil) on success, (true, err) if found but failed, (false, nil) if not found.
func openIOSUrl(deviceID, url string) (bool, error) {
	var udid, name string

	if deviceID == "" {
		sims := GetIOSSimulators()
		found := false
		for i := range sims {
			if sims[i].State == StateBooted {
				udid = sims[i].UDID
				name = sims[i].Name
				found = true

				break
			}
		}
		if !found {
			return false, nil
		}
	} else {
		device := FindIOSSimulatorByID(deviceID)
		if device == nil || device.State != StateBooted {
			return false, nil
		}
		udid = device.UDID
		name = device.Name
	}

	fmt.Printf("Opening URL on iOS simulator '%s'...\n", name)

	if output, err := packageExecutor.Output(CmdXCrun, CmdSimctl, "openurl", udid, url); err != nil {
		return true, fmt.Errorf("failed to open URL on iOS simulator: %w\nOutput: %s", err, string(output))
	}

	fmt.Println("URL opened successfully.")

	return true, nil
}

// openAndroidUrl opens a URL on a running Android emulator.
// If deviceID is empty, uses any running emulator.
// Returns (true, nil) on success, (true, err) if found but failed, (false, nil) if not found.
func openAndroidUrl(deviceID, url string) (bool, error) {
	udid, name := FindRunningAndroidEmulator(deviceID)
	if udid == "" {
		return false, nil
	}

	fmt.Printf("Opening URL on Android emulator '%s'...\n", name)
	cmdArgs := []string{"-s", udid, "shell", "am", "start", "-a", "android.intent.action.VIEW", "-d", url}

	if output, err := packageExecutor.Output(CmdAdb, cmdArgs...); err != nil {
		return true, fmt.Errorf("failed to open URL on Android emulator: %w\nOutput: %s", err, string(output))
	}

	fmt.Println("URL opened successfully.")

	return true, nil
}
