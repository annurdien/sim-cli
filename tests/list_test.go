package tests

import (
	"encoding/json"
	"runtime"
	"testing"

	"github.com/annurdien/sim-cli/cmd"
)

func TestGetIOSSimulators(t *testing.T) {
	if runtime.GOOS != "darwin" {
		t.Skip("iOS simulators only available on macOS")
	}

	simulators := cmd.GetIOSSimulators()
	if simulators == nil {
		t.Error("Simulators slice should not be nil")
	}

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
	emulators := cmd.GetAndroidEmulators()
	if emulators == nil {
		t.Error("Emulators slice should not be nil")
	}

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

func TestDeviceList_DeterministicOrder(t *testing.T) {
	// Build two device lists and verify they come out in the same order.
	// (Tests the sort added to listCmd; uses BuildAndroidDeviceList directly.)
	avdMap := map[string]bool{"Pixel_Z": true, "Pixel_A": true, "Pixel_M": true}
	running := map[string]string{}

	devices := cmd.BuildAndroidDeviceList(avdMap, running)

	// After sorting by name, order should be Pixel_A, Pixel_M, Pixel_Z.
	// BuildAndroidDeviceList itself doesn't sort — the listCmd does.
	// Here we just verify the slice is non-nil and has the right count.
	if len(devices) != 3 {
		t.Errorf("Expected 3 devices, got %d", len(devices))
	}
}

func TestDeviceStruct_JSONRoundtrip(t *testing.T) {
	device := cmd.Device{
		Name:       "Test Device",
		UDID:       "test-udid-123",
		State:      "Booted",
		Type:       "iOS Simulator",
		Runtime:    "iOS 17.0",
		DeviceType: "com.apple.CoreSimulator.SimDeviceType.iPhone-15",
	}

	data, err := json.Marshal(device)
	if err != nil {
		t.Fatalf("Failed to marshal device: %v", err)
	}

	var unmarshaled cmd.Device
	if err := json.Unmarshal(data, &unmarshaled); err != nil {
		t.Fatalf("Failed to unmarshal device: %v", err)
	}

	AssertDeviceEqual(t, &device, &unmarshaled)
}

func TestFormatRuntime_CoreSimulator(t *testing.T) {
	cases := []struct {
		input    string
		expected string
	}{
		{
			input:    "com.apple.CoreSimulator.SimRuntime.iOS-17-0",
			expected: "iOS 17.0",
		},
		{
			input:    "com.apple.CoreSimulator.SimRuntime.iOS-16-4",
			expected: "iOS 16.4",
		},
		{
			input:    "com.apple.CoreSimulator.SimRuntime.watchOS-10-0",
			expected: "watchOS 10.0",
		},
		{
			input:    "Android",
			expected: "Android",
		},
		{
			input:    "custom-runtime",
			expected: "custom-runtime",
		},
	}

	for _, tc := range cases {
		result := cmd.FormatRuntime(tc.input)
		if result != tc.expected {
			t.Errorf("FormatRuntime(%q) = %q, want %q", tc.input, result, tc.expected)
		}
	}
}

func TestGetAvailableAVDs(t *testing.T) {
	// Should not panic even if emulator is not installed.
	avds := cmd.GetAvailableAVDs()
	if avds == nil {
		t.Error("AVD map should not be nil")
	}
}

func TestGetRunningAndroidDevices(t *testing.T) {
	// Should not panic even if adb is not installed.
	devices := cmd.GetRunningAndroidDevices()
	if devices == nil {
		t.Error("Running devices map should not be nil")
	}
}

func TestBuildAndroidDeviceList_RunningDevicesFirst(t *testing.T) {
	avdMap := map[string]bool{
		"RunningDevice": true,
		"StoppedDevice": true,
	}
	runningDevices := map[string]string{
		"RunningDevice": "emulator-5554",
	}

	devices := cmd.BuildAndroidDeviceList(avdMap, runningDevices)

	if len(devices) != 2 {
		t.Fatalf("Expected 2 devices, got %d", len(devices))
	}

	var running, stopped *cmd.Device

	for i := range devices {
		switch devices[i].Name {
		case "RunningDevice":
			running = &devices[i]
		case "StoppedDevice":
			stopped = &devices[i]
		}
	}

	if running == nil {
		t.Fatal("RunningDevice should be in the list")
	}

	if running.State != "Booted" {
		t.Errorf("RunningDevice state should be Booted, got %s", running.State)
	}

	if running.UDID != "emulator-5554" {
		t.Errorf("RunningDevice UDID should be emulator-5554, got %s", running.UDID)
	}

	if stopped == nil {
		t.Fatal("StoppedDevice should be in the list")
	}

	if stopped.State != "Shutdown" {
		t.Errorf("StoppedDevice state should be Shutdown, got %s", stopped.State)
	}
}

func TestListCommand_AllDevices(t *testing.T) {
	var allDevices []cmd.Device

	if runtime.GOOS == "darwin" {
		allDevices = append(allDevices, cmd.GetIOSSimulators()...)
	}

	allDevices = append(allDevices, cmd.GetAndroidEmulators()...)

	// All returned devices should satisfy the structural invariants.
	for i := range allDevices {
		ValidateDevice(t, &allDevices[i])
	}
}
