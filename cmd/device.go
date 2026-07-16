package cmd

import (
	"errors"
	"fmt"
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
	ValidArgsFunction: validDeviceArgs,
	Args:              cobra.MaximumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		noWait, _ := cmd.Flags().GetBool("no-wait")
		var deviceID string
		if len(args) == 0 {
			selected, err := PromptDeviceSelector("all")
			if err != nil {
				return err
			}
			deviceID = selected
		} else {
			deviceID = args[0]
		}

		err := RunSpinner(fmt.Sprintf("Booting device %q...", deviceID), func() error {
			return startDevice(deviceID, noWait)
		})
		if err != nil {
			return err
		}
		PrintSuccess(fmt.Sprintf("Successfully booted %s", deviceID))

		return nil
	},
}

var stopCmd = &cobra.Command{
	Use:               "stop [device-name-or-udid]",
	Aliases:           []string{"st", "shutdown", "sd"},
	Short:             "Stop/Shutdown a running iOS simulator or Android emulator",
	Long:              `Stop or shutdown a specific running iOS simulator or Android emulator by name or UDID.`,
	ValidArgsFunction: validDeviceArgs,
	Args:              cobra.MaximumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		var deviceID string
		if len(args) == 0 {
			selected, err := PromptDeviceSelector("booted")
			if err != nil {
				return err
			}
			deviceID = selected
		} else {
			deviceID = args[0]
		}

		return executeDeviceAction("Stopping", "stopped", deviceID, func(m DeviceManager, id string) (bool, error) {
			return m.Stop(id)
		})
	},
}

var restartCmd = &cobra.Command{
	Use:               "restart [device-name-or-udid]",
	Aliases:           []string{"r"},
	Short:             "Restart an iOS simulator or Android emulator",
	Long:              `Restart a specific iOS simulator or Android emulator by name or UDID.`,
	ValidArgsFunction: validDeviceArgs,
	Args:              cobra.MaximumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		var deviceID string
		if len(args) == 0 {
			selected, err := PromptDeviceSelector("booted")
			if err != nil {
				return err
			}
			deviceID = selected
		} else {
			deviceID = args[0]
		}

		return executeDeviceAction("Restarting", "restarted", deviceID, func(m DeviceManager, id string) (bool, error) {
			return m.Restart(id)
		})
	},
}

var deleteCmd = &cobra.Command{
	Use:     "delete [device-name-or-udid]",
	Aliases: []string{"d", "del"},
	Short:   "Delete an iOS simulator or Android emulator",
	Long: `Delete a specific iOS simulator or Android emulator by name or UDID. ` +
		`This will permanently remove the device.`,
	ValidArgsFunction: validDeviceArgs,
	Args:              cobra.MaximumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		var deviceID string
		if len(args) == 0 {
			selected, err := PromptDeviceSelector("all")
			if err != nil {
				return err
			}
			deviceID = selected
		} else {
			deviceID = args[0]
		}
		force, _ := cmd.Flags().GetBool("force")

		if !force {
			PrintInfo(fmt.Sprintf("Are you sure you want to permanently delete %q? This cannot be undone. [y/N]: ", deviceID))

			var confirm string

			_, _ = fmt.Scanln(&confirm)

			if strings.ToLower(strings.TrimSpace(confirm)) != "y" {
				PrintInfo("Deletion cancelled.")

				return nil
			}
		}

		return executeDeviceAction("Deleting", "deleted", deviceID, func(m DeviceManager, id string) (bool, error) {
			return m.Delete(id)
		})
	},
}

var eraseCmd = &cobra.Command{
	Use:     "erase [device-name-or-udid]",
	Aliases: []string{"reset"},
	Short:   "Erase a device (factory reset)",
	Long: `Erase a specific iOS simulator or Android emulator by name or UDID.
This performs a factory reset. The device will be shut down if it is running.`,
	ValidArgsFunction: validDeviceArgs,
	Args:              cobra.MaximumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		var deviceID string
		if len(args) == 0 {
			selected, err := PromptDeviceSelector("all")
			if err != nil {
				return err
			}
			deviceID = selected
		} else {
			deviceID = args[0]
		}
		force, _ := cmd.Flags().GetBool("force")

		if !force {
			PrintInfo(fmt.Sprintf("Are you sure you want to permanently erase %q? This cannot be undone. [y/N]: ", deviceID))
			var confirm string
			_, _ = fmt.Scanln(&confirm)
			if strings.ToLower(strings.TrimSpace(confirm)) != "y" {
				PrintInfo("Erase cancelled.")
				return nil
			}
		}

		return executeDeviceAction("Erasing", "erased", deviceID, func(m DeviceManager, id string) (bool, error) {
			return m.Erase(id)
		})
	},
}

var cloneCmd = &cobra.Command{
	Use:               "clone [source-device] <new-name>",
	Short:             "Clone a device (iOS only)",
	Long:              `Clone a specific iOS simulator. Not supported for Android emulators.`,
	ValidArgsFunction: validDeviceAndFileArgs,
	Args:              cobra.ExactArgs(2),
	RunE: func(cmd *cobra.Command, args []string) error {
		if runtime.GOOS != DarwinOS {
			return ErrIOSMacOnly
		}

		sourceDevice := args[0]
		newName := args[1]

		return executeDeviceAction("Cloning", "cloned", sourceDevice, func(m DeviceManager, id string) (bool, error) {
			return m.Clone(id, newName)
		})
	},
}

var lastCmd = &cobra.Command{
	Use:   "last",
	Short: "Show the last started device",
	Long:  `Display information about the last started device.`,
	Run: func(cmd *cobra.Command, args []string) {
		lastDevice, err := GetLastStartedDevice()
		if err != nil {
			PrintInfo(fmt.Sprintf("Error getting last started device: %v", err))

			return
		}

		if lastDevice == nil {
			PrintInfo("No last started device found. Start a device first.")

			return
		}
		PrintInfo("Last started device:")
		PrintInfo(fmt.Sprintf("  Name: %s", lastDevice.Name))
		PrintInfo(fmt.Sprintf("  Type: %s", lastDevice.Type))
		PrintInfo(fmt.Sprintf("  UDID: %s", lastDevice.UDID))

		if lastDevice.Runtime != "" {
			PrintInfo(fmt.Sprintf("  Runtime: %s", lastDevice.Runtime))
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
		PrintInfo(fmt.Sprintf("Starting last device: %s (%s)", lastDevice.Name, lastDevice.Type))
		deviceID = lastDevice.Name
	}

	for _, m := range GetManagers() {
		found, err := m.Start(deviceID, noWait)
		if err != nil {
			return err
		}
		if found {
			return nil
		}
	}

	return fmt.Errorf("device %q: %w", deviceID, ErrDeviceNotFound)
}

// --- Old functions removed ---

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

// executeDeviceAction executes a device action with a spinner across all managers.
func executeDeviceAction(actionIng, actionEd, deviceID string, action func(m DeviceManager, id string) (bool, error)) error {
	err := RunSpinner(fmt.Sprintf("%s device %q...", actionIng, deviceID), func() error {
		for _, m := range GetManagers() {
			found, err := action(m, deviceID)
			if err != nil {
				return err
			}
			if found {
				return nil
			}
		}

		return ErrDeviceNotFound
	})

	if err == nil {
		PrintSuccess(fmt.Sprintf("Successfully %s %s", actionEd, deviceID))

		return nil
	}

	if !errors.Is(err, ErrDeviceNotFound) {
		return err
	}

	return fmt.Errorf("device %q: %w", deviceID, ErrDeviceNotFound)
}
