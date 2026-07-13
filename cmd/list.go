package cmd

import (
	"encoding/json"
	"fmt"
	"os"
	"runtime"
	"sort"
	"strings"

	"github.com/olekukonko/tablewriter"
	"github.com/spf13/cobra"
)

// Device represents an iOS simulator or Android emulator.
type Device struct {
	Name       string `json:"name"`
	UDID       string `json:"udid"`
	State      string `json:"state"`
	Type       string `json:"type"` // "iOS Simulator" or "Android Emulator"
	Runtime    string `json:"runtime,omitempty"`
	DeviceType string `json:"deviceTypeIdentifier,omitempty"`
}

var listCmd = &cobra.Command{
	Use:     "list",
	Aliases: []string{"l", "ls"},
	Short:   "List available iOS simulators and Android emulators",
	Long:    `Display a list of all available iOS simulators and Android emulators with their current status.`,
	Run: func(cmd *cobra.Command, args []string) {
		var devices []Device

		if runtime.GOOS == DarwinOS {
			simulators := GetIOSSimulators()
			devices = append(devices, simulators...)
		}

		emulators := GetAndroidEmulators()
		devices = append(devices, emulators...)

		if len(devices) == 0 {
			fmt.Println("No simulators or emulators found")

			return
		}

		// Sort deterministically: by Type first, then by Name.
		sort.Slice(devices, func(i, j int) bool {
			if devices[i].Type != devices[j].Type {
				return devices[i].Type < devices[j].Type
			}

			return devices[i].Name < devices[j].Name
		})

		table := tablewriter.NewWriter(os.Stdout)
		table.Header("Type", "Name", "State", "UDID", "Runtime")

		for _, device := range devices {
			runtimeVal := FormatRuntime(device.Runtime)
			_ = table.Append([]string{device.Type, device.Name, device.State, device.UDID, runtimeVal})
		}

		_ = table.Render()
	},
}

// FormatRuntime converts a raw CoreSimulator runtime identifier to a human-readable string.
func FormatRuntime(runtimeVal string) string {
	if strings.Contains(runtimeVal, "com.apple.CoreSimulator.SimRuntime.") {
		parts := strings.Split(runtimeVal, ".")
		if len(parts) >= 4 {
			platformVersion := parts[len(parts)-1]
			platformParts := strings.Split(platformVersion, "-")

			if len(platformParts) >= 2 {
				platform := platformParts[0]
				version := strings.Join(platformParts[1:], ".")

				return platform + " " + version
			}
		}
	}

	if strings.Contains(runtimeVal, "Android") {
		return "Android"
	}

	return runtimeVal
}

// GetIOSSimulators returns all iOS simulators reported by xcrun simctl.
func GetIOSSimulators() []Device {
	output, err := packageExecutor.Output(CmdXCrun, CmdSimctl, "list", "devices", "--json")
	if err != nil {
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

// GetAndroidEmulators returns all Android emulators (both running and defined AVDs).
func GetAndroidEmulators() []Device {
	avdMap := GetAvailableAVDs()
	runningDevices := GetRunningAndroidDevices()

	return BuildAndroidDeviceList(avdMap, runningDevices)
}

// GetAvailableAVDs returns a set of all AVD names defined on this machine.
func GetAvailableAVDs() map[string]bool {
	avdOutput, err := packageExecutor.Output(CmdEmulator, "-list-avds")
	if err != nil {
		// Emulator may not be in PATH; only running devices will be listed.
		fmt.Fprintf(os.Stderr, "Warning: could not run 'emulator -list-avds': %v. Only running emulators will be listed.\n", err)

		return make(map[string]bool)
	}

	avdLines := strings.Split(strings.TrimSpace(string(avdOutput)), "\n")
	avdMap := make(map[string]bool)

	for _, line := range avdLines {
		trimmedLine := strings.TrimSpace(line)
		if trimmedLine != "" {
			avdMap[trimmedLine] = true
		}
	}

	return avdMap
}

// GetRunningAndroidDevices returns a map of running emulator name → UDID from adb devices.
func GetRunningAndroidDevices() map[string]string {
	adbOutput, err := packageExecutor.Output(CmdAdb, "devices")
	if err != nil {
		return make(map[string]string)
	}

	runningDevices := make(map[string]string) // map[name]udid
	lines := strings.SplitSeq(strings.TrimSpace(string(adbOutput)), "\n")
	for line := range lines {
		line = strings.TrimSpace(line)
		if isValidEmulatorLine(line) {
			name, udid := parseEmulatorLine(line)
			if name != "" && udid != "" {
				runningDevices[name] = udid
			}
		}
	}

	return runningDevices
}

func isValidEmulatorLine(line string) bool {
	return strings.Contains(line, "emulator-") &&
		strings.Contains(line, "device") &&
		!strings.Contains(line, "List of devices attached")
}

func parseEmulatorLine(line string) (string, string) {
	parts := strings.Fields(line)
	if len(parts) < 2 || parts[1] != "device" {
		return "", ""
	}

	udid := parts[0]
	name := GetEmulatorName(udid)

	return name, udid
}

// GetEmulatorName retrieves the AVD name of a running emulator by its serial (e.g. "emulator-5554").
func GetEmulatorName(udid string) string {
	nameOutput, err := packageExecutor.Output(CmdAdb, "-s", udid, "emu", "avd", "name")
	if err != nil {
		return ""
	}

	output := strings.TrimSpace(string(nameOutput))
	lines := strings.Split(output, "\n")
	if len(lines) > 0 {
		return strings.TrimSpace(lines[0])
	}

	return ""
}

// BuildAndroidDeviceList merges running and available AVDs into a unified Device slice.
func BuildAndroidDeviceList(avdMap map[string]bool, runningDevices map[string]string) []Device {
	devices := make([]Device, 0, len(avdMap)+len(runningDevices))

	for name, udid := range runningDevices {
		devices = append(devices, Device{
			Name:    name,
			UDID:    udid,
			State:   StateBooted,
			Type:    TypeAndroidEmulator,
			Runtime: "Android",
		})
		delete(avdMap, name)
	}

	for avd := range avdMap {
		devices = append(devices, Device{
			Name:    avd,
			UDID:    "N/A",
			State:   StateShutdown,
			Type:    TypeAndroidEmulator,
			Runtime: "Android",
		})
	}

	return devices
}
