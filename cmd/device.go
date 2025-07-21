package cmd

import (
	"fmt"
	"os/exec"
	"runtime"
	"strings"

	"github.com/spf13/cobra"
)

var startCmd = &cobra.Command{
	Use:   "start [device-name-or-udid]",
	Short: "Start an iOS simulator or Android emulator",
	Long:  `Start a specific iOS simulator or Android emulator by name or UDID.`,
	Args:  cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		deviceID := args[0]
		
		// Try iOS simulator first (only on macOS)
		if runtime.GOOS == "darwin" {
			if startIOSSimulator(deviceID) {
				return
			}
		}
		
		// Try Android emulator
		if startAndroidEmulator(deviceID) {
			return
		}
		
		fmt.Printf("Device '%s' not found or failed to start\n", deviceID)
	},
}

var stopCmd = &cobra.Command{
	Use:   "stop [device-name-or-udid]",
	Short: "Stop a running iOS simulator or Android emulator",
	Long:  `Stop a specific running iOS simulator or Android emulator by name or UDID.`,
	Args:  cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		deviceID := args[0]
		
		// Try iOS simulator first (only on macOS)
		if runtime.GOOS == "darwin" {
			if stopIOSSimulator(deviceID) {
				return
			}
		}
		
		// Try Android emulator
		if stopAndroidEmulator(deviceID) {
			return
		}
		
		fmt.Printf("Device '%s' not found or failed to stop\n", deviceID)
	},
}

var shutdownCmd = &cobra.Command{
	Use:   "shutdown [device-name-or-udid]",
	Short: "Shutdown an iOS simulator or Android emulator",
	Long:  `Shutdown a specific iOS simulator or Android emulator by name or UDID.`,
	Args:  cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		deviceID := args[0]
		
		// Try iOS simulator first (only on macOS)
		if runtime.GOOS == "darwin" {
			if shutdownIOSSimulator(deviceID) {
				return
			}
		}
		
		// Try Android emulator
		if stopAndroidEmulator(deviceID) { // Android doesn't distinguish between stop and shutdown
			return
		}
		
		fmt.Printf("Device '%s' not found or failed to shutdown\n", deviceID)
	},
}

var restartCmd = &cobra.Command{
	Use:   "restart [device-name-or-udid]",
	Short: "Restart an iOS simulator or Android emulator",
	Long:  `Restart a specific iOS simulator or Android emulator by name or UDID.`,
	Args:  cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		deviceID := args[0]
		
		// Try iOS simulator first (only on macOS)
		if runtime.GOOS == "darwin" {
			if restartIOSSimulator(deviceID) {
				return
			}
		}
		
		// Try Android emulator
		if restartAndroidEmulator(deviceID) {
			return
		}
		
		fmt.Printf("Device '%s' not found or failed to restart\n", deviceID)
	},
}

func startIOSSimulator(deviceID string) bool {
	// First try to find the device by name or UDID
	udid := findIOSSimulatorUDID(deviceID)
	if udid == "" {
		return false
	}
	
	fmt.Printf("Starting iOS simulator '%s'...\n", deviceID)
	cmd := exec.Command("xcrun", "simctl", "boot", udid)
	if err := cmd.Run(); err != nil {
		fmt.Printf("Error starting iOS simulator: %v\n", err)
		return false
	}
	
	// Open Simulator.app
	openCmd := exec.Command("open", "-a", "Simulator")
	if err := openCmd.Run(); err != nil {
		fmt.Printf("Warning: Could not open Simulator app: %v\n", err)
	}
	
	fmt.Printf("iOS simulator '%s' started successfully\n", deviceID)
	return true
}

func stopIOSSimulator(deviceID string) bool {
	udid := findIOSSimulatorUDID(deviceID)
	if udid == "" {
		return false
	}
	
	fmt.Printf("Stopping iOS simulator '%s'...\n", deviceID)
	cmd := exec.Command("xcrun", "simctl", "shutdown", udid)
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
	udid := findIOSSimulatorUDID(deviceID)
	if udid == "" {
		return false
	}
	
	fmt.Printf("Restarting iOS simulator '%s'...\n", deviceID)
	
	// First shutdown
	shutdownCmd := exec.Command("xcrun", "simctl", "shutdown", udid)
	shutdownCmd.Run() // Ignore error if already shutdown
	
	// Then boot
	bootCmd := exec.Command("xcrun", "simctl", "boot", udid)
	if err := bootCmd.Run(); err != nil {
		fmt.Printf("Error restarting iOS simulator: %v\n", err)
		return false
	}
	
	// Open Simulator.app
	openCmd := exec.Command("open", "-a", "Simulator")
	if err := openCmd.Run(); err != nil {
		fmt.Printf("Warning: Could not open Simulator app: %v\n", err)
	}
	
	fmt.Printf("iOS simulator '%s' restarted successfully\n", deviceID)
	return true
}

func startAndroidEmulator(deviceID string) bool {
	// Check if it's already running
	if isAndroidEmulatorRunning(deviceID) {
		fmt.Printf("Android emulator '%s' is already running\n", deviceID)
		return true
	}
	
	// Check if the AVD exists
	if !doesAndroidAVDExist(deviceID) {
		return false
	}
	
	fmt.Printf("Starting Android emulator '%s'...\n", deviceID)
	cmd := exec.Command("emulator", "-avd", deviceID)
	if err := cmd.Start(); err != nil {
		fmt.Printf("Error starting Android emulator: %v\n", err)
		return false
	}
	
	fmt.Printf("Android emulator '%s' started successfully\n", deviceID)
	return true
}

func stopAndroidEmulator(deviceID string) bool {
	// Find running emulator with this name
	runningUDID := findRunningAndroidEmulator(deviceID)
	if runningUDID == "" {
		return false
	}
	
	fmt.Printf("Stopping Android emulator '%s'...\n", deviceID)
	cmd := exec.Command("adb", "-s", runningUDID, "emu", "kill")
	if err := cmd.Run(); err != nil {
		fmt.Printf("Error stopping Android emulator: %v\n", err)
		return false
	}
	
	fmt.Printf("Android emulator '%s' stopped successfully\n", deviceID)
	return true
}

func restartAndroidEmulator(deviceID string) bool {
	fmt.Printf("Restarting Android emulator '%s'...\n", deviceID)
	
	// Stop if running
	stopAndroidEmulator(deviceID)
	
	// Start
	return startAndroidEmulator(deviceID)
}

func findIOSSimulatorUDID(deviceID string) string {
	// If it's already a UDID (looks like UUID), return it
	if len(deviceID) == 36 && strings.Count(deviceID, "-") == 4 {
		return deviceID
	}
	
	// Find by name
	simulators := getIOSSimulators()
	for _, sim := range simulators {
		if strings.EqualFold(sim.Name, deviceID) || sim.UDID == deviceID {
			return sim.UDID
		}
	}
	
	return ""
}

func doesAndroidAVDExist(avdName string) bool {
	cmd := exec.Command("emulator", "-list-avds")
	output, err := cmd.Output()
	if err != nil {
		return false
	}
	
	lines := strings.Split(strings.TrimSpace(string(output)), "\n")
	for _, line := range lines {
		if strings.TrimSpace(line) == avdName {
			return true
		}
	}
	
	return false
}

func isAndroidEmulatorRunning(avdName string) bool {
	return findRunningAndroidEmulator(avdName) != ""
}

func findRunningAndroidEmulator(avdName string) string {
	cmd := exec.Command("adb", "devices")
	output, err := cmd.Output()
	if err != nil {
		return ""
	}
	
	lines := strings.Split(string(output), "\n")
	for _, line := range lines {
		if strings.Contains(line, "emulator-") && strings.Contains(line, "device") {
			parts := strings.Fields(line)
			if len(parts) >= 2 {
				// Get the AVD name for this emulator
				emulatorID := parts[0]
				nameCmd := exec.Command("adb", "-s", emulatorID, "emu", "avd", "name")
				nameOutput, err := nameCmd.Output()
				if err == nil {
					actualName := strings.TrimSpace(string(nameOutput))
					if actualName == avdName {
						return emulatorID
					}
				}
			}
		}
	}
	
	return ""
}

func init() {
	rootCmd.AddCommand(startCmd)
	rootCmd.AddCommand(stopCmd)
	rootCmd.AddCommand(shutdownCmd)
	rootCmd.AddCommand(restartCmd)
}