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
			runtimeVal = formatRuntime(runtimeVal)

			_ = table.Append([]string{device.Type, device.Name, device.State, udid, runtimeVal})
		}

		_ = table.Render()
	},
}

func formatRuntime(runtimeVal string) string {
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

func getIOSSimulators() []Device {
	cmd := exec.Command(CmdXCrun, CmdSimctl, "list", "devices", "--json")
	output, err := cmd.Output()
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

func getAndroidEmulators() []Device {
	avdMap := getAvailableAVDs()
	runningDevices := getRunningAndroidDevices()
	return buildAndroidDeviceList(avdMap, runningDevices)
}

func getAvailableAVDs() map[string]bool {
	avdCmd := exec.Command(CmdEmulator, "-list-avds")
	avdOutput, err := avdCmd.Output()
	if err != nil {
		// Emulator command might not be in path, but adb might work.
		// We can proceed and just list running devices.
		fmt.Printf("Could not run 'emulator -list-avds': %v. Only running emulators will be listed.\n", err)
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

func getRunningAndroidDevices() map[string]string {
	adbCmd := exec.Command(CmdAdb, "devices")
	adbOutput, err := adbCmd.Output()
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
	name := getEmulatorName(udid)

	return name, udid
}

func getEmulatorName(udid string) string {
	nameCmd := exec.Command(CmdAdb, "-s", udid, "emu", "avd", "name")
	nameOutput, err := nameCmd.Output()
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

func buildAndroidDeviceList(avdMap map[string]bool, runningDevices map[string]string) []Device {
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
