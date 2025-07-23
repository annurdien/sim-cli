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

		if runtime.GOOS == DarwinOS {
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

			runtimeVal := device.Runtime
			if strings.Contains(runtimeVal, "com.apple.CoreSimulator.SimRuntime.iOS-") {
				// Extract iOS version from runtime string
				parts := strings.Split(runtimeVal, "-")
				if len(parts) >= 2 {
					version := strings.Join(parts[len(parts)-2:], ".")
					runtimeVal = "iOS " + version
				}
			}

			table.Append([]string{device.Type, device.Name, device.State, udid, runtimeVal})
		}

		table.Render()
	},
}

func getIOSSimulators() []Device {
	cmd := exec.Command(CmdXCrun, CmdSimctl, "list", "devices", "--json")
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
	for runtimeVal, deviceList := range result.Devices {
		for _, device := range deviceList {
			devices = append(devices, Device{
				Name:       device.Name,
				UDID:       device.UDID,
				State:      device.State,
				Type:       TypeIOSSimulator,
				Runtime:    runtimeVal,
				DeviceType: device.DeviceTypeIdentifier,
			})
		}
	}

	return devices
}

func getAndroidEmulators() []Device {
	avdCmd := exec.Command(CmdEmulator, "-list-avds")
	avdOutput, err := avdCmd.Output()
	if err != nil {
		// Emulator command might not be in path, but adb might work.
		// We can proceed and just list running devices.
	}
	avdLines := strings.Split(strings.TrimSpace(string(avdOutput)), "\n")
	avdMap := make(map[string]bool)
	for _, line := range avdLines {
		trimmedLine := strings.TrimSpace(line)
		if trimmedLine != "" {
			avdMap[trimmedLine] = true
		}
	}

	adbCmd := exec.Command(CmdAdb, "devices")
	adbOutput, err := adbCmd.Output()
	if err != nil {
		var devices []Device
		for avd := range avdMap {
			devices = append(devices, Device{
				Name:    avd,
				UDID:    "offline",
				State:   StateShutdown,
				Type:    TypeAndroidEmulator,
				Runtime: "Android",
			})
		}
		return devices
	}

	runningDevices := make(map[string]string) // map[name]udid
	lines := strings.Split(string(adbOutput), "\n")
	for _, line := range lines {
		if strings.Contains(line, "emulator-") && strings.Contains(line, "device") {
			parts := strings.Fields(line)
			if len(parts) > 0 {
				udid := parts[0]
				nameCmd := exec.Command(CmdAdb, "-s", udid, "emu", "avd", "name")
				nameOutput, err := nameCmd.Output()
				if err == nil {
					name := strings.TrimSpace(string(nameOutput))
					runningDevices[name] = udid
				}
			}
		}
	}

	var devices []Device
	for name, udid := range runningDevices {
		devices = append(devices, Device{
			Name:    name,
			UDID:    udid,
			State:   StateBooted,
			Type:    TypeAndroidEmulator,
			Runtime: "Android",
		})
		// Remove from avdMap so we don't list it twice
		delete(avdMap, name)
	}

	for avd := range avdMap {
		devices = append(devices, Device{
			Name:    avd,
			UDID:    "offline",
			State:   StateShutdown,
			Type:    TypeAndroidEmulator,
			Runtime: "Android",
		})
	}

	return devices
}

func init() {
	rootCmd.AddCommand(listCmd)
}
