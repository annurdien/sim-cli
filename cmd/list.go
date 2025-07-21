package cmd

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"runtime"
	"strings"

	"github.com/olekukonko/tablewriter"
	"github.com/spf13/cobra"
)

type Device struct {
	Name       string `json:"name"`
	UDID       string `json:"udid"`
	State      string `json:"state"`
	Type       string `json:"type"` // "simulator" or "emulator"
	Runtime    string `json:"runtime,omitempty"`
	DeviceType string `json:"deviceTypeIdentifier,omitempty"`
}

var listCmd = &cobra.Command{
	Use:     "list",
	Aliases: []string{"l", "ls"},
	Short:   "List available iOS simulators and Android emulators",
	Long:    `Display a list of all available iOS simulators and Android emulators with their current status.`,
	Run: func(cmd *cobra.Command, args []string) {
		devices := []Device{}

		if runtime.GOOS == "darwin" {
			simulators := getIOSSimulators()
			devices = append(devices, simulators...)
		} else {
			fmt.Println("Note: iOS simulators are only available on macOS")
		}

		emulators := getAndroidEmulators()
		devices = append(devices, emulators...)

		if len(devices) == 0 {
			fmt.Println("No simulators or emulators found")
			return
		}

		table := tablewriter.NewWriter(os.Stdout)
		table.Header("Type", "Name", "State", "UDID", "Runtime")

		for _, device := range devices {
			udid := device.UDID

			runtime := device.Runtime
			if strings.Contains(runtime, "com.apple.CoreSimulator.SimRuntime.iOS-") {
				// Extract iOS version from runtime string
				parts := strings.Split(runtime, "-")
				if len(parts) >= 2 {
					version := strings.Join(parts[len(parts)-2:], ".")
					runtime = "iOS " + version
				}
			}

			table.Append(device.Type, device.Name, device.State, udid, runtime)
		}

		table.Render()
	},
}

func getIOSSimulators() []Device {
	cmd := exec.Command("xcrun", "simctl", "list", "devices", "--json")
	output, err := cmd.Output()
	if err != nil {
		fmt.Printf("Error getting iOS simulators: %v\n", err)
		return []Device{}
	}

	var result struct {
		Devices map[string][]struct {
			Name                 string `json:"name"`
			UDID                 string `json:"udid"`
			State                string `json:"state"`
			DeviceTypeIdentifier string `json:"deviceTypeIdentifier"`
		} `json:"devices"`
	}

	if err := json.Unmarshal(output, &result); err != nil {
		fmt.Printf("Error parsing iOS simulator data: %v\n", err)
		return []Device{}
	}

	var devices []Device
	for runtime, deviceList := range result.Devices {
		for _, device := range deviceList {
			devices = append(devices, Device{
				Name:       device.Name,
				UDID:       device.UDID,
				State:      device.State,
				Type:       "iOS Simulator",
				Runtime:    runtime,
				DeviceType: device.DeviceTypeIdentifier,
			})
		}
	}

	return devices
}

func getAndroidEmulators() []Device {
	runningCmd := exec.Command("adb", "devices")
	runningOutput, err := runningCmd.Output()
	runningDevices := make(map[string]bool)

	if err == nil {
		lines := strings.Split(string(runningOutput), "\n")
		for _, line := range lines {
			if strings.Contains(line, "emulator-") && strings.Contains(line, "device") {
				parts := strings.Fields(line)
				if len(parts) >= 2 {
					runningDevices[parts[0]] = true
				}
			}
		}
	}

	cmd := exec.Command("emulator", "-list-avds")
	output, err := cmd.Output()
	if err != nil {
		return []Device{}
	}

	var devices []Device
	lines := strings.Split(strings.TrimSpace(string(output)), "\n")

	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line != "" {
			state := "Shutdown"
			udid := ""

			// Check if this emulator is running
			for runningUDID := range runningDevices {
				if strings.Contains(runningUDID, "emulator-") {
					state = "Booted"
					udid = runningUDID
					break
				}
			}

			if udid == "" {
				udid = "offline"
			}

			devices = append(devices, Device{
				Name:    line,
				UDID:    udid,
				State:   state,
				Type:    "Android Emulator",
				Runtime: "Android",
			})
		}
	}

	return devices
}

func init() {
	rootCmd.AddCommand(listCmd)
}
