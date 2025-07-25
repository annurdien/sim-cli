package cmd

import (
	"fmt"
	"os/exec"
	"runtime"
	"strings"

	"github.com/spf13/cobra"
)

var startCmd = &cobra.Command{
	Use:     "start [device-name-or-udid|lts]",
	Aliases: []string{"s"},
	Short:   "Start an iOS simulator or Android emulator",
	Long: `Start a specific iOS simulator or Android emulator by name or UDID.
Use 'lts' to start the last started device.`,
	Args: cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		deviceID := args[0]

		if deviceID == "lts" {
			lastDevice, err := getLastStartedDevice()
			if err != nil || lastDevice == nil {
				fmt.Println("No last started device found. Start a device first to use 'lts'.")
				return
			}

			fmt.Printf("Starting last device: %s (%s)\n", lastDevice.Name, lastDevice.Type)
			deviceID = lastDevice.Name
		}

		if runtime.GOOS == DarwinOS {
			if startIOSSimulator(deviceID) {
				return
			}
		}

		if startAndroidEmulator(deviceID) {
			return
		}

		fmt.Printf("Device '%s' not found or failed to start\n", deviceID)
	},
}

var stopCmd = &cobra.Command{
	Use:     "stop [device-name-or-udid]",
	Aliases: []string{"st"},
	Short:   "Stop a running iOS simulator or Android emulator",
	Long:    `Stop a specific running iOS simulator or Android emulator by name or UDID.`,
	Args:    cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		deviceID := args[0]

		if runtime.GOOS == DarwinOS {
			if stopIOSSimulator(deviceID) {
				return
			}
		}

		if stopAndroidEmulator(deviceID) {
			return
		}

		fmt.Printf("Device '%s' not found or failed to stop\n", deviceID)
	},
}

var shutdownCmd = &cobra.Command{
	Use:     "shutdown [device-name-or-udid]",
	Aliases: []string{"sd"},
	Short:   "Shutdown an iOS simulator or Android emulator",
	Long:    `Shutdown a specific iOS simulator or Android emulator by name or UDID.`,
	Args:    cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		deviceID := args[0]

		if runtime.GOOS == DarwinOS {
			if shutdownIOSSimulator(deviceID) {
				return
			}
		}

		if stopAndroidEmulator(deviceID) {
			return
		}

		fmt.Printf("Device '%s' not found or failed to shutdown\n", deviceID)
	},
}

var restartCmd = &cobra.Command{
	Use:     "restart [device-name-or-udid]",
	Aliases: []string{"r"},
	Short:   "Restart an iOS simulator or Android emulator",
	Long:    `Restart a specific iOS simulator or Android emulator by name or UDID.`,
	Args:    cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		deviceID := args[0]

		if runtime.GOOS == DarwinOS {
			if restartIOSSimulator(deviceID) {
				return
			}
		}

		if restartAndroidEmulator(deviceID) {
			return
		}

		fmt.Printf("Device '%s' not found or failed to restart\n", deviceID)
	},
}

var deleteCmd = &cobra.Command{
	Use:     "delete [device-name-or-udid]",
	Aliases: []string{"d", "del"},
	Short:   "Delete an iOS simulator or Android emulator",
	Long: `Delete a specific iOS simulator or Android emulator by name or UDID. ` +
		`This will permanently remove the device.`,
	Args: cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		deviceID := args[0]

		if runtime.GOOS == DarwinOS {
			if deleteIOSSimulator(deviceID) {
				return
			}
		}

		if deleteAndroidEmulator(deviceID) {
			return
		}

		fmt.Printf("Device '%s' not found or failed to delete\n", deviceID)
	},
}

var lastCmd = &cobra.Command{
	Use:   "last",
	Short: "Show the last started device",
	Long:  `Display information about the last started device.`,
	Run: func(cmd *cobra.Command, args []string) {
		lastDevice, err := getLastStartedDevice()
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
	Run: func(cmd *cobra.Command, args []string) {
		lastDevice, err := getLastStartedDevice()
		if err != nil || lastDevice == nil {
			fmt.Println("No last started device found. Start a device first to use 'lts'.")
			return
		}

		fmt.Printf("Starting last device: %s (%s)\n", lastDevice.Name, lastDevice.Type)
		deviceID := lastDevice.Name

		if runtime.GOOS == DarwinOS {
			if startIOSSimulator(deviceID) {
				return
			}
		}

		if startAndroidEmulator(deviceID) {
			return
		}

		fmt.Printf("Device '%s' not found or failed to start\n", deviceID)
	},
}

func startIOSSimulator(deviceID string) bool {
	device := findIOSSimulatorByID(deviceID)
	if device == nil {
		return false
	}

	fmt.Printf("Starting iOS simulator '%s'...\n", deviceID)
	cmd := exec.Command(CmdXCrun, CmdSimctl, "boot", device.UDID)
	if err := cmd.Run(); err != nil {
		fmt.Printf("Error starting iOS simulator: %v\n", err)
		return false
	}

	openCmd := exec.Command("open", "-a", "Simulator")
	if err := openCmd.Run(); err != nil {
		fmt.Printf("Warning: Could not open Simulator app: %v\n", err)
	}

	device.State = StateBooted
	if err := saveLastStartedDevice(device); err != nil {
		fmt.Printf("Warning: Could not save last started device: %v\n", err)
	}

	fmt.Printf("iOS simulator '%s' started successfully\n", deviceID)

	return true
}

func stopIOSSimulator(deviceID string) bool {
	udid, _ := findIOSSimulator(deviceID)
	if udid == "" {
		return false
	}

	fmt.Printf("Stopping iOS simulator '%s'...\n", deviceID)
	cmd := exec.Command(CmdXCrun, CmdSimctl, "shutdown", udid)
	if err := cmd.Run(); err != nil {
		fmt.Printf("Error stopping iOS simulator: %v\n", err)
		return false
	}

	fmt.Printf("iOS simulator '%s' stopped successfully\n", deviceID)

	return true
}

func shutdownIOSSimulator(deviceID string) bool {
	return stopIOSSimulator(deviceID) // Same as stop for iOS
}

func restartIOSSimulator(deviceID string) bool {
	device := findIOSSimulatorByID(deviceID)
	if device == nil {
		return false
	}

	fmt.Printf("Restarting iOS simulator '%s'...\n", deviceID)

	shutdownCmd := exec.Command(CmdXCrun, CmdSimctl, "shutdown", device.UDID)
	_ = shutdownCmd.Run() // Ignore error if already shutdown

	bootCmd := exec.Command(CmdXCrun, CmdSimctl, "boot", device.UDID)
	if err := bootCmd.Run(); err != nil {
		fmt.Printf("Error restarting iOS simulator: %v\n", err)
		return false
	}

	openCmd := exec.Command("open", "-a", "Simulator")
	if err := openCmd.Run(); err != nil {
		fmt.Printf("Warning: Could not open Simulator app: %v\n", err)
	}

	device.State = StateBooted
	if err := saveLastStartedDevice(device); err != nil {
		fmt.Printf("Warning: Could not save last started device: %v\n", err)
	}

	fmt.Printf("iOS simulator '%s' restarted successfully\n", deviceID)

	return true
}

func startAndroidEmulator(deviceID string) bool {
	if isAndroidEmulatorRunning(deviceID) {
		fmt.Printf("Android emulator '%s' is already running\n", deviceID)

		udid, name := findRunningAndroidEmulator(deviceID)
		device := &Device{
			Name:  name,
			UDID:  udid,
			Type:  TypeAndroidEmulator,
			State: StateBooted,
		}
		if err := saveLastStartedDevice(device); err != nil {
			fmt.Printf("Warning: Could not save last started device: %v\n", err)
		}

		return true
	}

	if !doesAndroidAVDExist(deviceID) {
		return false
	}

	fmt.Printf("Starting Android emulator '%s'...\n", deviceID)
	cmd := exec.Command(CmdEmulator, "-avd", deviceID)
	if err := cmd.Start(); err != nil {
		fmt.Printf("Error starting Android emulator: %v\n", err)
		return false
	}

	device := &Device{
		Name:  deviceID,
		UDID:  "starting",
		Type:  TypeAndroidEmulator,
		State: StateBooted,
	}
	if err := saveLastStartedDevice(device); err != nil {
		fmt.Printf("Warning: Could not save last started device: %v\n", err)
	}

	fmt.Printf("Android emulator '%s' started successfully\n", deviceID)

	return true
}

func stopAndroidEmulator(deviceID string) bool {
	udid, _ := findRunningAndroidEmulator(deviceID)
	if udid == "" {
		return false
	}

	fmt.Printf("Stopping Android emulator '%s'...\n", deviceID)
	cmd := exec.Command(CmdAdb, "-s", udid, "emu", "kill")
	if err := cmd.Run(); err != nil {
		fmt.Printf("Error stopping Android emulator: %v\n", err)
		return false
	}

	fmt.Printf("Android emulator '%s' stopped successfully\n", deviceID)

	return true
}

func restartAndroidEmulator(deviceID string) bool {
	fmt.Printf("Restarting Android emulator '%s'...\n", deviceID)

	stopAndroidEmulator(deviceID)

	if startAndroidEmulator(deviceID) {
		device := &Device{
			Name:  deviceID,
			UDID:  "restarting",
			Type:  TypeAndroidEmulator,
			State: StateBooted,
		}
		if err := saveLastStartedDevice(device); err != nil {
			fmt.Printf("Warning: Could not save last started device: %v\n", err)
		}

		return true
	}

	return false
}

func deleteIOSSimulator(deviceID string) bool {
	udid, _ := findIOSSimulator(deviceID)
	if udid == "" {
		return false
	}

	fmt.Printf("Deleting iOS simulator '%s'...\n", deviceID)

	shutdownCmd := exec.Command(CmdXCrun, CmdSimctl, "shutdown", udid)
	_ = shutdownCmd.Run()

	cmd := exec.Command(CmdXCrun, CmdSimctl, "delete", udid)
	if err := cmd.Run(); err != nil {
		fmt.Printf("Error deleting iOS simulator: %v\n", err)
		return false
	}

	fmt.Printf("iOS simulator '%s' deleted successfully\n", deviceID)

	return true
}

func deleteAndroidEmulator(deviceID string) bool {
	if !doesAndroidAVDExist(deviceID) {
		return false
	}

	fmt.Printf("Deleting Android emulator '%s'...\n", deviceID)

	stopAndroidEmulator(deviceID)

	cmd := exec.Command(CmdAvdManager, "delete", "avd", "-n", deviceID)
	if err := cmd.Run(); err != nil {
		fmt.Printf("Error deleting Android emulator: %v\n", err)
		return false
	}

	fmt.Printf("Android emulator '%s' deleted successfully\n", deviceID)

	return true
}

func findIOSSimulator(deviceID string) (string, string) {
	if len(deviceID) == 36 && strings.Count(deviceID, "-") == 4 {
		sims := getIOSSimulators()
		for _, sim := range sims {
			if sim.UDID == deviceID {
				return sim.UDID, sim.Name
			}
		}
	}

	sims := getIOSSimulators()
	for _, sim := range sims {
		if strings.EqualFold(sim.Name, deviceID) {
			return sim.UDID, sim.Name
		}
	}

	return "", ""
}

func findIOSSimulatorByID(deviceID string) *Device {
	simulators := getIOSSimulators()
	for _, sim := range simulators {
		if strings.EqualFold(sim.Name, deviceID) || sim.UDID == deviceID {
			return &sim
		}
	}

	return nil
}

func getRunningIOSSimulator() (*iOSSimulator, error) {
	sims := getIOSSimulators()
	for _, sim := range sims {
		if sim.State == StateBooted {
			return &iOSSimulator{udid: sim.UDID, name: sim.Name}, nil
		}
	}

	return nil, ErrNoRunningIOSSimulator
}

func doesAndroidAVDExist(avdName string) bool {
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

func isAndroidEmulatorRunning(avdName string) bool {
	udid, _ := findRunningAndroidEmulator(avdName)
	return udid != ""
}

func findRunningAndroidEmulator(avdName string) (string, string) {
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
				nameOutput, err := nameCmd.Output()
				if err == nil {
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

func getRunningAndroidEmulator() (*androidEmulator, error) {
	udid, name := findRunningAndroidEmulator("") // Find any running emulator
	if udid != "" {
		return &androidEmulator{udid: udid, name: name}, nil
	}

	return nil, ErrNoRunningAndroidEmulator
}
