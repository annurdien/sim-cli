package cmd

import (
	"fmt"
	"os/exec"
	"runtime"
	"strings"

	"github.com/spf13/cobra"
)

// --- Cobra Commands ---

var startCmd = &cobra.Command{
	Use:     "start [device-name-or-udid|lts]",
	Aliases: []string{"s"},
	Short:   "Start an iOS simulator or Android emulator",
	Long: `Start a specific iOS simulator or Android emulator by name or UDID.
Use 'lts' to start the last started device.`,
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		return startDevice(args[0])
	},
}

var stopCmd = &cobra.Command{
	Use:     "stop [device-name-or-udid]",
	Aliases: []string{"st"},
	Short:   "Stop a running iOS simulator or Android emulator",
	Long:    `Stop a specific running iOS simulator or Android emulator by name or UDID.`,
	Args:    cobra.ExactArgs(1),
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
	Use:     "shutdown [device-name-or-udid]",
	Aliases: []string{"sd"},
	Short:   "Shutdown an iOS simulator or Android emulator",
	Long:    `Shutdown a specific iOS simulator or Android emulator by name or UDID.`,
	Args:    cobra.ExactArgs(1),
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
	Use:     "restart [device-name-or-udid]",
	Aliases: []string{"r"},
	Short:   "Restart an iOS simulator or Android emulator",
	Long:    `Restart a specific iOS simulator or Android emulator by name or UDID.`,
	Args:    cobra.ExactArgs(1),
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
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		deviceID := args[0]
		force, _ := cmd.Flags().GetBool("force")

		if !force {
			fmt.Printf("Are you sure you want to permanently delete %q? This cannot be undone. [y/N]: ", deviceID)

			var confirm string

			fmt.Scanln(&confirm)

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
		return startDevice("lts")
	},
}

// --- Shared Start Logic ---

// startDevice handles the 'start' action for both startCmd and ltsCmd.
func startDevice(deviceID string) error {
	if deviceID == "lts" {
		lastDevice, err := GetLastStartedDevice()
		if err != nil || lastDevice == nil {
			return fmt.Errorf("no last started device found; start a device first to use 'lts'")
		}

		fmt.Printf("Starting last device: %s (%s)\n", lastDevice.Name, lastDevice.Type)
		deviceID = lastDevice.Name
	}

	if runtime.GOOS == DarwinOS {
		if found, err := startIOSSimulator(deviceID); found {
			return err
		}
	}

	if found, err := startAndroidEmulator(deviceID); found {
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
	bootCmd := exec.Command(CmdXCrun, CmdSimctl, "boot", device.UDID)

	if err := bootCmd.Run(); err != nil {
		return true, fmt.Errorf("failed to boot iOS simulator '%s': %w", deviceID, err)
	}

	openCmd := exec.Command("open", "-a", "Simulator")
	if err := openCmd.Run(); err != nil {
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
	cmd := exec.Command(CmdXCrun, CmdSimctl, "shutdown", device.UDID)

	if err := cmd.Run(); err != nil {
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

	shutdownCmd := exec.Command(CmdXCrun, CmdSimctl, "shutdown", device.UDID)
	_ = shutdownCmd.Run() // Ignore error if already shut down.

	bootCmd := exec.Command(CmdXCrun, CmdSimctl, "boot", device.UDID)
	if err := bootCmd.Run(); err != nil {
		return true, fmt.Errorf("failed to boot iOS simulator '%s' during restart: %w", deviceID, err)
	}

	openCmd := exec.Command("open", "-a", "Simulator")
	if err := openCmd.Run(); err != nil {
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

	shutdownCmd := exec.Command(CmdXCrun, CmdSimctl, "shutdown", device.UDID)
	_ = shutdownCmd.Run()

	deleteCmd := exec.Command(CmdXCrun, CmdSimctl, "delete", device.UDID)
	if err := deleteCmd.Run(); err != nil {
		return true, fmt.Errorf("failed to delete iOS simulator '%s': %w", deviceID, err)
	}

	fmt.Printf("iOS simulator '%s' deleted successfully\n", deviceID)

	return true, nil
}

// --- Android Emulator Operations ---

// startAndroidEmulator starts an Android emulator.
// Returns (true, nil) on success, (true, err) if found but start failed, (false, nil) if not found.
func startAndroidEmulator(deviceID string) (bool, error) {
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
	cmd := exec.Command(CmdEmulator, "-avd", deviceID)

	if err := cmd.Start(); err != nil {
		return true, fmt.Errorf("failed to start Android emulator '%s': %w", deviceID, err)
	}

	// The UDID is assigned by adb once the emulator boots; save a placeholder.
	device := &Device{
		Name:  deviceID,
		UDID:  "starting",
		Type:  TypeAndroidEmulator,
		State: StateBooted,
	}
	if err := SaveLastStartedDevice(device); err != nil {
		fmt.Printf("Warning: could not save last started device: %v\n", err)
	}

	fmt.Printf("Android emulator '%s' started successfully\n", deviceID)

	return true, nil
}

// stopAndroidEmulator kills a running Android emulator.
// Returns (true, nil) on success, (true, err) if found but kill failed, (false, nil) if not found.
func stopAndroidEmulator(deviceID string) (bool, error) {
	udid, _ := FindRunningAndroidEmulator(deviceID)
	if udid == "" {
		return false, nil
	}

	fmt.Printf("Stopping Android emulator '%s'...\n", deviceID)
	cmd := exec.Command(CmdAdb, "-s", udid, "emu", "kill")

	if err := cmd.Run(); err != nil {
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

	return startAndroidEmulator(deviceID)
}

// deleteAndroidEmulator removes an Android AVD permanently.
// Returns (true, nil) on success, (true, err) if found but delete failed, (false, nil) if not found.
func deleteAndroidEmulator(deviceID string) (bool, error) {
	if !DoesAndroidAVDExist(deviceID) {
		return false, nil
	}

	fmt.Printf("Deleting Android emulator '%s'...\n", deviceID)

	_, _ = stopAndroidEmulator(deviceID)

	cmd := exec.Command(CmdAvdManager, "delete", "avd", "-n", deviceID)
	if err := cmd.Run(); err != nil {
		return true, fmt.Errorf("failed to delete Android emulator '%s': %w", deviceID, err)
	}

	fmt.Printf("Android emulator '%s' deleted successfully\n", deviceID)

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
	cmd := exec.Command(CmdAdb, "devices")
	output, err := cmd.Output()
	if err != nil {
		return "", ""
	}

	lines := strings.SplitSeq(string(output), "\n")
	for line := range lines {
		if strings.Contains(line, "emulator-") && strings.Contains(line, "device") {
			parts := strings.Fields(line)
			if len(parts) > 0 {
				emulatorID := parts[0]
				nameCmd := exec.Command(CmdAdb, "-s", emulatorID, "emu", "avd", "name")
				nameOutput, nameErr := nameCmd.Output()

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

					if actualName != "" && (avdName == "" || actualName == avdName) {
						return emulatorID, actualName
					}
				}
			}
		}
	}

	return "", ""
}

// DoesAndroidAVDExist checks whether an AVD with the given name is defined.
func DoesAndroidAVDExist(avdName string) bool {
	cmd := exec.Command(CmdEmulator, "-list-avds")
	output, err := cmd.Output()
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
