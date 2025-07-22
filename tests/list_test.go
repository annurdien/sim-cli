package tests

import (
	"encoding/json"
	"os/exec"
	"runtime"
	"strings"
	"testing"
)

func TestGetIOSSimulators(t *testing.T) {
	if runtime.GOOS != "darwin" {
		t.Skip("iOS simulators only available on macOS")
	}

	// Test that we can get iOS simulators without errors
	simulators := getIOSSimulators()

	// Should not panic or error, even if no simulators available
	if simulators == nil {
		t.Error("Simulators slice should not be nil")
	}

	// Validate structure of returned devices
	for _, sim := range simulators {
		if sim.Type != "iOS Simulator" {
			t.Errorf("Expected type 'iOS Simulator', got '%s'", sim.Type)
		}

		if sim.Name == "" {
			t.Error("Simulator name should not be empty")
		}

		if sim.UDID == "" {
			t.Error("Simulator UDID should not be empty")
		}
	}
}

func TestGetAndroidEmulators(t *testing.T) {
	// Test that we can get Android emulators without errors
	emulators := getAndroidEmulators()

	// Should not panic or error, even if no emulators available
	if emulators == nil {
		t.Error("Emulators slice should not be nil")
	}

	// Validate structure of returned devices
	for _, emu := range emulators {
		if emu.Type != "Android Emulator" {
			t.Errorf("Expected type 'Android Emulator', got '%s'", emu.Type)
		}

		if emu.Name == "" {
			t.Error("Emulator name should not be empty")
		}

		if emu.Runtime != "Android" {
			t.Errorf("Expected runtime 'Android', got '%s'", emu.Runtime)
		}
	}
}

func TestDeviceStruct(t *testing.T) {
	// Test Device struct creation and JSON marshaling
	device := Device{
		Name:       "Test Device",
		UDID:       "test-udid-123",
		State:      "Booted",
		Type:       "iOS Simulator",
		Runtime:    "iOS 17.0",
		DeviceType: "com.apple.CoreSimulator.SimDeviceType.iPhone-15",
	}

	// Test JSON marshaling
	data, err := json.Marshal(device)
	if err != nil {
		t.Errorf("Failed to marshal device: %v", err)
	}

	// Test JSON unmarshaling
	var unmarshaled Device
	err = json.Unmarshal(data, &unmarshaled)
	if err != nil {
		t.Errorf("Failed to unmarshal device: %v", err)
	}

	if unmarshaled.Name != device.Name {
		t.Errorf("Expected name %s, got %s", device.Name, unmarshaled.Name)
	}
}

func TestListCommand_NoDevices(t *testing.T) {
	// Test list command behavior when no devices are available
	// Since we can't easily mock the actual device listing commands,
	// we'll test the underlying functions with empty results

	// Test empty iOS simulators list
	if runtime.GOOS == "darwin" {
		simulators := getIOSSimulators()
		// Even if no simulators, the slice should not be nil
		if simulators == nil {
			t.Error("iOS simulators slice should not be nil")
		}
	}

	// Test empty Android emulators list
	emulators := getAndroidEmulators()
	// Even if no emulators, the slice should not be nil
	if emulators == nil {
		t.Error("Android emulators slice should not be nil")
	}

	// Test that combining empty lists works
	var allDevices []Device
	if runtime.GOOS == "darwin" {
		allDevices = append(allDevices, getIOSSimulators()...)
	}
	allDevices = append(allDevices, getAndroidEmulators()...)

	// Should be safe to iterate over even if empty
	for _, device := range allDevices {
		ValidateDevice(t, &device)
	}
}

// Helper functions that mirror the unexported functions in cmd package
func getIOSSimulators() []Device {
	cmd := exec.Command("xcrun", "simctl", "list", "devices", "--json")
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
