package cmd

import (
	"fmt"
	"runtime"
	"strings"
	"time"

	"github.com/spf13/cobra"
)

// --- Cobra Commands ---

var startCmd = &cobra.Command{
	Use:     "start [device-name-or-udid|lts]",
	Aliases: []string{"s"},
	Short:   "Start an iOS simulator or Android emulator",
	Long: `Start a specific iOS simulator or Android emulator by name or UDID.
Use 'lts' to start the last started device.`,
	ValidArgsFunction: validDeviceArgs,
	Args:              cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		noWait, _ := cmd.Flags().GetBool("no-wait")
		return startDevice(args[0], noWait)
	},
}

var stopCmd = &cobra.Command{
	Use:               "stop [device-name-or-udid]",
	Aliases:           []string{"st"},
	Short:             "Stop a running iOS simulator or Android emulator",
	Long:              `Stop a specific running iOS simulator or Android emulator by name or UDID.`,
	ValidArgsFunction: validDeviceArgs,
	Args:              cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		deviceID := args[0]

		if runtime.GOOS == DarwinOS {
			if found, err := stopIOSSimulator(deviceID); found {
				return err
			}
		}

		if found, err := stopAndroidEmulator(deviceID); found {
			return err
		}

		return fmt.Errorf("device %q: %w", deviceID, ErrDeviceNotFound)
	},
}

var shutdownCmd = &cobra.Command{
	Use:               "shutdown [device-name-or-udid]",
	Aliases:           []string{"sd"},
	Short:             "Shutdown an iOS simulator or Android emulator",
	Long:              `Shutdown a specific iOS simulator or Android emulator by name or UDID.`,
	ValidArgsFunction: validDeviceArgs,
	Args:              cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		deviceID := args[0]

		if runtime.GOOS == DarwinOS {
			if found, err := shutdownIOSSimulator(deviceID); found {
				return err
			}
		}

		if found, err := stopAndroidEmulator(deviceID); found {
			return err
		}

		return fmt.Errorf("device %q: %w", deviceID, ErrDeviceNotFound)
	},
}

var restartCmd = &cobra.Command{
	Use:               "restart [device-name-or-udid]",
	Aliases:           []string{"r"},
	Short:             "Restart an iOS simulator or Android emulator",
	Long:              `Restart a specific iOS simulator or Android emulator by name or UDID.`,
	ValidArgsFunction: validDeviceArgs,
	Args:              cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		deviceID := args[0]

		if runtime.GOOS == DarwinOS {
			if found, err := restartIOSSimulator(deviceID); found {
				return err
			}
		}

		if found, err := restartAndroidEmulator(deviceID); found {
			return err
		}

		return fmt.Errorf("device %q: %w", deviceID, ErrDeviceNotFound)
	},
}

var deleteCmd = &cobra.Command{
	Use:     "delete [device-name-or-udid]",
	Aliases: []string{"d", "del"},
	Short:   "Delete an iOS simulator or Android emulator",
	Long: `Delete a specific iOS simulator or Android emulator by name or UDID. ` +
		`This will permanently remove the device.`,
	ValidArgsFunction: validDeviceArgs,
	Args:              cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		deviceID := args[0]
		force, _ := cmd.Flags().GetBool("force")

		if !force {
			fmt.Printf("Are you sure you want to permanently delete %q? This cannot be undone. [y/N]: ", deviceID)

			var confirm string

			_, _ = fmt.Scanln(&confirm)

			if strings.ToLower(strings.TrimSpace(confirm)) != "y" {
				fmt.Println("Deletion cancelled.")

				return nil
			}
		}

		if runtime.GOOS == DarwinOS {
			if found, err := deleteIOSSimulator(deviceID); found {
				return err
			}
		}

		if found, err := deleteAndroidEmulator(deviceID); found {
			return err
		}

		return fmt.Errorf("device %q: %w", deviceID, ErrDeviceNotFound)
	},
}

var eraseCmd = &cobra.Command{
	Use:     "erase [device-name-or-udid]",
	Aliases: []string{"reset"},
	Short:   "Erase a device (factory reset)",
	Long: `Erase a specific iOS simulator or Android emulator by name or UDID.
This performs a factory reset. The device will be shut down if it is running.`,
	ValidArgsFunction: validDeviceArgs,
	Args:              cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		deviceID := args[0]
		force, _ := cmd.Flags().GetBool("force")

		if !force {
			fmt.Printf("Are you sure you want to permanently erase %q? This cannot be undone. [y/N]: ", deviceID)
			var confirm string
			_, _ = fmt.Scanln(&confirm)
			if strings.ToLower(strings.TrimSpace(confirm)) != "y" {
				fmt.Println("Erase cancelled.")
				return nil
			}
		}

		if runtime.GOOS == DarwinOS {
			if found, err := eraseIOSSimulator(deviceID); found {
				return err
			}
		}

		if found, err := eraseAndroidEmulator(deviceID); found {
			return err
		}

		return fmt.Errorf("device %q: %w", deviceID, ErrDeviceNotFound)
	},
}

var cloneCmd = &cobra.Command{
	Use:               "clone [source-device] <new-name>",
	Short:             "Clone a device (iOS only)",
	Long:              `Clone a specific iOS simulator. Not supported for Android emulators.`,
	ValidArgsFunction: validDeviceAndFileArgs,
	Args:              cobra.ExactArgs(2),
	RunE: func(cmd *cobra.Command, args []string) error {
		sourceDevice := args[0]
		newName := args[1]

		if runtime.GOOS == DarwinOS {
			if found, err := cloneIOSSimulator(sourceDevice, newName); found {
				return err
			}
		} else {
			return ErrIOSMacOnly
		}

		// Check if it's an Android device
		if DoesAndroidAVDExist(sourceDevice) {
			return ErrAndroidCloneNotSupported
		}

		return fmt.Errorf("device %q: %w", sourceDevice, ErrDeviceNotFound)
	},
}

var lastCmd = &cobra.Command{
	Use:   "last",
	Short: "Show the last started device",
	Long:  `Display information about the last started device.`,
	Run: func(cmd *cobra.Command, args []string) {
		lastDevice, err := GetLastStartedDevice()
		if err != nil {
			fmt.Printf("Error getting last started device: %v\n", err)

			return
		}

		if lastDevice == nil {
			fmt.Println("No last started device found. Start a device first.")

			return
		}

		fmt.Printf("Last started device:\n")
		fmt.Printf("  Name: %s\n", lastDevice.Name)
		fmt.Printf("  Type: %s\n", lastDevice.Type)
		fmt.Printf("  UDID: %s\n", lastDevice.UDID)

		if lastDevice.Runtime != "" {
			fmt.Printf("  Runtime: %s\n", lastDevice.Runtime)
		}
	},
}

var ltsCmd = &cobra.Command{
	Use:   "lts",
	Short: "Start the last started device",
	Long:  `Start the last started device quickly. This is a shortcut for 'sim start lts'.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		return startDevice("lts", false)
	},
}

// --- Shared Start Logic ---

// startDevice handles the 'start' action for both startCmd and ltsCmd.
// NoWait skips the Android boot-wait polling when true.
func startDevice(deviceID string, noWait bool) error {
	if deviceID == "lts" {
		lastDevice, err := GetLastStartedDevice()
		if err != nil || lastDevice == nil {
			return ErrNoLastDevice
		}

		fmt.Printf("Starting last device: %s (%s)\n", lastDevice.Name, lastDevice.Type)
		deviceID = lastDevice.Name
	}

	if runtime.GOOS == DarwinOS {
		if found, err := startIOSSimulator(deviceID); found {
			return err
		}
	}

	if found, err := startAndroidEmulator(deviceID, noWait); found {
		return err
	}

	return fmt.Errorf("device %q: %w", deviceID, ErrDeviceNotFound)
}

// --- iOS Simulator Operations ---

// startIOSSimulator boots an iOS simulator.
// Returns (true, nil) on success, (true, err) if found but boot failed, (false, nil) if not found.
func startIOSSimulator(deviceID string) (bool, error) {
	device := FindIOSSimulatorByID(deviceID)
	if device == nil {
		return false, nil
	}

	fmt.Printf("Starting iOS simulator '%s'...\n", deviceID)

	if err := packageExecutor.Run(CmdXCrun, CmdSimctl, "boot", device.UDID); err != nil {
		return true, fmt.Errorf("failed to boot iOS simulator '%s': %w", deviceID, err)
	}

	if err := packageExecutor.Run("open", "-a", "Simulator"); err != nil {
		fmt.Printf("Warning: could not open Simulator app: %v\n", err)
	}

	device.State = StateBooted
	if err := SaveLastStartedDevice(device); err != nil {
		fmt.Printf("Warning: could not save last started device: %v\n", err)
	}

	fmt.Printf("iOS simulator '%s' started successfully\n", deviceID)

	return true, nil
}

// stopIOSSimulator shuts down an iOS simulator.
// Returns (true, nil) on success, (true, err) if found but shutdown failed, (false, nil) if not found.
func stopIOSSimulator(deviceID string) (bool, error) {
	device := FindIOSSimulatorByID(deviceID)
	if device == nil {
		return false, nil
	}

	fmt.Printf("Stopping iOS simulator '%s'...\n", deviceID)

	if err := packageExecutor.Run(CmdXCrun, CmdSimctl, "shutdown", device.UDID); err != nil {
		return true, fmt.Errorf("failed to stop iOS simulator '%s': %w", deviceID, err)
	}

	fmt.Printf("iOS simulator '%s' stopped successfully\n", deviceID)

	return true, nil
}

// shutdownIOSSimulator is an alias for stopIOSSimulator.
func shutdownIOSSimulator(deviceID string) (bool, error) {
	return stopIOSSimulator(deviceID)
}

// restartIOSSimulator shuts down then boots an iOS simulator.
// Returns (true, nil) on success, (true, err) if found but restart failed, (false, nil) if not found.
func restartIOSSimulator(deviceID string) (bool, error) {
	device := FindIOSSimulatorByID(deviceID)
	if device == nil {
		return false, nil
	}

	fmt.Printf("Restarting iOS simulator '%s'...\n", deviceID)

	_ = packageExecutor.Run(CmdXCrun, CmdSimctl, "shutdown", device.UDID) // Ignore error if already shut down.

	if err := packageExecutor.Run(CmdXCrun, CmdSimctl, "boot", device.UDID); err != nil {
		return true, fmt.Errorf("failed to boot iOS simulator '%s' during restart: %w", deviceID, err)
	}

	if err := packageExecutor.Run("open", "-a", "Simulator"); err != nil {
		fmt.Printf("Warning: could not open Simulator app: %v\n", err)
	}

	device.State = StateBooted
	if err := SaveLastStartedDevice(device); err != nil {
		fmt.Printf("Warning: could not save last started device: %v\n", err)
	}

	fmt.Printf("iOS simulator '%s' restarted successfully\n", deviceID)

	return true, nil
}

// deleteIOSSimulator removes an iOS simulator permanently.
// Returns (true, nil) on success, (true, err) if found but delete failed, (false, nil) if not found.
func deleteIOSSimulator(deviceID string) (bool, error) {
	device := FindIOSSimulatorByID(deviceID)
	if device == nil {
		return false, nil
	}

	fmt.Printf("Deleting iOS simulator '%s'...\n", deviceID)

	_ = packageExecutor.Run(CmdXCrun, CmdSimctl, "shutdown", device.UDID)

	if err := packageExecutor.Run(CmdXCrun, CmdSimctl, "delete", device.UDID); err != nil {
		return true, fmt.Errorf("failed to delete iOS simulator '%s': %w", deviceID, err)
	}

	fmt.Printf("iOS simulator '%s' deleted successfully\n", deviceID)

	return true, nil
}

// eraseIOSSimulator factory resets an iOS simulator.
func eraseIOSSimulator(deviceID string) (bool, error) {
	device := FindIOSSimulatorByID(deviceID)
	if device == nil {
		return false, nil
	}

	fmt.Printf("Erasing iOS simulator '%s'...\n", deviceID)

	_ = packageExecutor.Run(CmdXCrun, CmdSimctl, "shutdown", device.UDID)

	if err := packageExecutor.Run(CmdXCrun, CmdSimctl, "erase", device.UDID); err != nil {
		return true, fmt.Errorf("failed to erase iOS simulator '%s': %w", deviceID, err)
	}

	fmt.Printf("iOS simulator '%s' erased successfully\n", deviceID)

	return true, nil
}

// cloneIOSSimulator clones an iOS simulator.
func cloneIOSSimulator(sourceDeviceID, newName string) (bool, error) {
	device := FindIOSSimulatorByID(sourceDeviceID)
	if device == nil {
		return false, nil
	}

	fmt.Printf("Cloning iOS simulator '%s' to '%s'...\n", sourceDeviceID, newName)

	if err := packageExecutor.Run(CmdXCrun, CmdSimctl, "clone", device.UDID, newName); err != nil {
		return true, fmt.Errorf("failed to clone iOS simulator '%s': %w", sourceDeviceID, err)
	}

	fmt.Printf("iOS simulator '%s' cloned to '%s' successfully\n", sourceDeviceID, newName)

	return true, nil
}

// --- Android Emulator Operations ---

// androidBootTimeout is the maximum time to wait for an Android emulator to finish booting.
const androidBootTimeout = 120 * time.Second

// androidBootPollInterval is how often to poll boot status.
const androidBootPollInterval = 3 * time.Second

// startAndroidEmulator starts an Android emulator.
// NoWait skips boot polling and returns immediately after launching the process.
// Returns (true, nil) on success, (true, err) if found but start failed, (false, nil) if not found.
func startAndroidEmulator(deviceID string, noWait bool) (bool, error) {
	if IsAndroidEmulatorRunning(deviceID) {
		fmt.Printf("Android emulator '%s' is already running\n", deviceID)

		udid, name := FindRunningAndroidEmulator(deviceID)
		device := &Device{
			Name:  name,
			UDID:  udid,
			Type:  TypeAndroidEmulator,
			State: StateBooted,
		}
		if err := SaveLastStartedDevice(device); err != nil {
			fmt.Printf("Warning: could not save last started device: %v\n", err)
		}

		return true, nil
	}

	if !DoesAndroidAVDExist(deviceID) {
		return false, nil
	}

	fmt.Printf("Starting Android emulator '%s'...\n", deviceID)

	_, err := packageExecutor.Start(CmdEmulator, "-avd", deviceID)
	if err != nil {
		return true, fmt.Errorf("failed to start Android emulator '%s': %w", deviceID, err)
	}

	if noWait {
		// Fire-and-forget: save a placeholder UDID and return immediately.
		device := &Device{
			Name:  deviceID,
			UDID:  "starting",
			Type:  TypeAndroidEmulator,
			State: StateBooted,
		}
		if err := SaveLastStartedDevice(device); err != nil {
			fmt.Printf("Warning: could not save last started device: %v\n", err)
		}

		fmt.Printf("Android emulator '%s' launched (--no-wait: skipping boot check)\n", deviceID)

		return true, nil
	}

	// Wait for the emulator to fully boot and resolve its real UDID.
	fmt.Printf("Waiting for Android emulator '%s' to boot...\n", deviceID)

	udid, bootErr := waitForAndroidBoot(deviceID)
	if bootErr != nil {
		// Non-fatal: emulator may still be booting. Save placeholder and warn.
		fmt.Printf("Warning: emulator may still be booting: %v\n", bootErr)
		fmt.Printf("Use 'sim last' to check the saved device once it finishes booting.\n")
		udid = "starting"
	}

	device := &Device{
		Name:  deviceID,
		UDID:  udid,
		Type:  TypeAndroidEmulator,
		State: StateBooted,
	}
	if err := SaveLastStartedDevice(device); err != nil {
		fmt.Printf("Warning: could not save last started device: %v\n", err)
	}

	if bootErr == nil {
		fmt.Printf("Android emulator '%s' started successfully (serial: %s)\n", deviceID, udid)
	}

	return true, nil
}

// waitForAndroidBoot polls adb until the emulator with the given AVD name has
// fully booted (sys.boot_completed == 1). Returns the emulator serial (UDID).
func waitForAndroidBoot(avdName string) (string, error) {
	deadline := time.Now().Add(androidBootTimeout)

	for time.Now().Before(deadline) {
		udid, _ := FindRunningAndroidEmulator(avdName)
		if udid != "" {
			// Check if the system has fully booted.
			out, err := packageExecutor.Output(CmdAdb, "-s", udid, "shell", "getprop", "sys.boot_completed")
			if err == nil && strings.TrimSpace(string(out)) == "1" {
				return udid, nil
			}
		}

		time.Sleep(androidBootPollInterval)
	}

	// Last attempt: maybe it booted right at the deadline.
	if udid, _ := FindRunningAndroidEmulator(avdName); udid != "" {
		return udid, nil
	}

	return "starting", fmt.Errorf("timed out waiting for Android emulator '%s' to boot after %s", avdName, androidBootTimeout) //nolint:err113
}

// stopAndroidEmulator kills a running Android emulator.
// Returns (true, nil) on success, (true, err) if found but kill failed, (false, nil) if not found.
func stopAndroidEmulator(deviceID string) (bool, error) {
	udid, _ := FindRunningAndroidEmulator(deviceID)
	if udid == "" {
		return false, nil
	}

	fmt.Printf("Stopping Android emulator '%s'...\n", deviceID)

	if err := packageExecutor.Run(CmdAdb, "-s", udid, "emu", "kill"); err != nil {
		return true, fmt.Errorf("failed to stop Android emulator '%s': %w", deviceID, err)
	}

	fmt.Printf("Android emulator '%s' stopped successfully\n", deviceID)

	return true, nil
}

// restartAndroidEmulator stops then starts an Android emulator.
// Returns (true, nil) on success, (true, err) if found but restart failed, (false, nil) if not found.
func restartAndroidEmulator(deviceID string) (bool, error) {
	fmt.Printf("Restarting Android emulator '%s'...\n", deviceID)

	_, _ = stopAndroidEmulator(deviceID)

	return startAndroidEmulator(deviceID, false)
}

// deleteAndroidEmulator removes an Android AVD permanently.
// Returns (true, nil) on success, (true, err) if found but delete failed, (false, nil) if not found.
func deleteAndroidEmulator(deviceID string) (bool, error) {
	if !DoesAndroidAVDExist(deviceID) {
		return false, nil
	}

	fmt.Printf("Deleting Android emulator '%s'...\n", deviceID)

	_, _ = stopAndroidEmulator(deviceID)

	if err := packageExecutor.Run(CmdAvdManager, "delete", "avd", "-n", deviceID); err != nil {
		return true, fmt.Errorf("failed to delete Android emulator '%s': %w", deviceID, err)
	}

	fmt.Printf("Android emulator '%s' deleted successfully\n", deviceID)

	return true, nil
}

// eraseAndroidEmulator factory resets an Android emulator.
// Returns (true, nil) on success, (true, err) if found but erase failed, (false, nil) if not found.
func eraseAndroidEmulator(deviceID string) (bool, error) {
	if !DoesAndroidAVDExist(deviceID) {
		return false, nil
	}

	fmt.Printf("Erasing Android emulator '%s'...\n", deviceID)

	_, _ = stopAndroidEmulator(deviceID)

	fmt.Printf("Wiping data for Android emulator '%s' (this will restart the emulator)...\n", deviceID)

	_, err := packageExecutor.Start(CmdEmulator, "-avd", deviceID, "-wipe-data")
	if err != nil {
		return true, fmt.Errorf("failed to erase Android emulator '%s': %w", deviceID, err)
	}

	fmt.Printf("Android emulator '%s' is restarting with data wiped.\n", deviceID)

	return true, nil
}

// --- Device Lookup Helpers ---

// FindIOSSimulatorByID finds an iOS simulator by name (case-insensitive) or exact UDID.
// Merges the former findIOSSimulator and findIOSSimulatorByID into a single efficient function.
func FindIOSSimulatorByID(deviceID string) *Device {
	simulators := GetIOSSimulators()
	for i := range simulators {
		if simulators[i].UDID == deviceID || strings.EqualFold(simulators[i].Name, deviceID) {
			return &simulators[i]
		}
	}

	return nil
}

// FindRunningAndroidEmulator finds a running emulator by AVD name.
// Pass an empty string to find any running emulator.
func FindRunningAndroidEmulator(avdName string) (string, string) {
	output, err := packageExecutor.Output(CmdAdb, "devices")
	if err != nil {
		return "", ""
	}

	lines := strings.SplitSeq(string(output), "\n")
	for line := range lines {
		if strings.Contains(line, "emulator-") && strings.Contains(line, "device") {
			parts := strings.Fields(line)
			if len(parts) > 0 {
				emulatorID := parts[0]
				nameOutput, nameErr := packageExecutor.Output(CmdAdb, "-s", emulatorID, "emu", "avd", "name")

				if nameErr == nil {
					nameLines := strings.SplitSeq(strings.TrimSpace(string(nameOutput)), "\n")
					actualName := ""

					for nameLine := range nameLines {
						trimmed := strings.TrimSpace(nameLine)
						if trimmed != "" && trimmed != "OK" {
							actualName = trimmed

							break
						}
					}

					// Match if looking for any, or if name matches, or if serial/UDID matches
					if actualName != "" && (avdName == "" || actualName == avdName || emulatorID == avdName) {
						return emulatorID, actualName
					}
				}
			}
		}
	}

	return "", ""
}

// FindRunningDevice finds a running device by ID, or the active device if ID is empty.
func FindRunningDevice(deviceID string) (udid, name string, isAndroid bool, err error) {
	if deviceID == "" {
		if runtime.GOOS == DarwinOS {
			if sim, errSim := getRunningIOSSimulator(); errSim == nil {
				return sim.udid, sim.name, false, nil
			}
		}

		if emu, errEmu := getRunningAndroidEmulator(); errEmu == nil {
			return emu.udid, emu.name, true, nil
		}

		return "", "", false, ErrNoActiveDevice
	}

	if runtime.GOOS == DarwinOS {
		device := FindIOSSimulatorByID(deviceID)
		if device != nil && device.State == StateBooted {
			return device.UDID, device.Name, false, nil
		}
	}

	u, n := FindRunningAndroidEmulator(deviceID)
	if u != "" {
		return u, n, true, nil
	}

	return "", "", false, fmt.Errorf("device %q: %w", deviceID, ErrDeviceNotRunning)
}

// DoesAndroidAVDExist checks whether an AVD with the given name is defined.
func DoesAndroidAVDExist(avdName string) bool {
	output, err := packageExecutor.Output(CmdEmulator, "-list-avds")
	if err != nil {
		return false
	}

	lines := strings.SplitSeq(strings.TrimSpace(string(output)), "\n")
	for line := range lines {
		if strings.TrimSpace(line) == avdName {
			return true
		}
	}

	return false
}

// IsAndroidEmulatorRunning reports whether an emulator with the given AVD name is currently running.
func IsAndroidEmulatorRunning(avdName string) bool {
	udid, _ := FindRunningAndroidEmulator(avdName)

	return udid != ""
}

// getRunningIOSSimulator returns the first booted iOS simulator, for auto-selection.
func getRunningIOSSimulator() (*iOSSimulator, error) {
	sims := GetIOSSimulators()
	for _, sim := range sims {
		if sim.State == StateBooted {
			return &iOSSimulator{udid: sim.UDID, name: sim.Name}, nil
		}
	}

	return nil, ErrNoRunningIOSSimulator
}

// getRunningAndroidEmulator returns any currently running Android emulator, for auto-selection.
func getRunningAndroidEmulator() (*androidEmulator, error) {
	udid, name := FindRunningAndroidEmulator("")
	if udid != "" {
		return &androidEmulator{udid: udid, name: name}, nil
	}

	return nil, ErrNoRunningAndroidEmulator
}
