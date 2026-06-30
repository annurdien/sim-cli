package tests

import (
	"testing"

	"github.com/annurdien/sim-cli/cmd"
)

// NewTestHelpers creates a new TestHelpers instance with a temp directory.
type TestHelpers struct {
	TempDir string
}

// NewTestHelpers creates a new test helpers instance with an isolated temp home directory.
func NewTestHelpers(t *testing.T) *TestHelpers {
	t.Helper()

	tempDir := t.TempDir()
	t.Setenv("HOME", tempDir)

	return &TestHelpers{TempDir: tempDir}
}

// CreateTestDevice creates a cmd.Device with sensible test defaults.
func CreateTestDevice(name string) *cmd.Device {
	return &cmd.Device{
		Name:    name,
		UDID:    "test-udid-" + name,
		Type:    "iOS Simulator",
		State:   "Shutdown",
		Runtime: "iOS 17.0",
	}
}

// CreateTestConfig creates a cmd.Config containing a single test device.
func CreateTestConfig() *cmd.Config {
	return &cmd.Config{
		LastStartedDevice: CreateTestDevice("test-device"),
	}
}

// GetTestDeviceData returns a set of canonical test device fixtures.
func GetTestDeviceData() struct {
	IOSSimulator     cmd.Device
	AndroidEmulator  cmd.Device
	BootedSimulator  cmd.Device
	ShutdownEmulator cmd.Device
} {
	return struct {
		IOSSimulator     cmd.Device
		AndroidEmulator  cmd.Device
		BootedSimulator  cmd.Device
		ShutdownEmulator cmd.Device
	}{
		IOSSimulator: cmd.Device{
			Name:       "iPhone 15",
			UDID:       "12345678-1234-5678-9012-123456789012",
			State:      "Shutdown",
			Type:       "iOS Simulator",
			Runtime:    "iOS 17.0",
			DeviceType: "com.apple.CoreSimulator.SimDeviceType.iPhone-15",
		},
		AndroidEmulator: cmd.Device{
			Name:    "Pixel_7_API_34",
			UDID:    "emulator-5554",
			State:   "Shutdown",
			Type:    "Android Emulator",
			Runtime: "Android",
		},
		BootedSimulator: cmd.Device{
			Name:       "iPhone 15 Pro",
			UDID:       "87654321-4321-8765-2109-876543210987",
			State:      "Booted",
			Type:       "iOS Simulator",
			Runtime:    "iOS 17.0",
			DeviceType: "com.apple.CoreSimulator.SimDeviceType.iPhone-15-Pro",
		},
		ShutdownEmulator: cmd.Device{
			Name:    "Pixel_8_API_34",
			UDID:    "offline",
			State:   "Shutdown",
			Type:    "Android Emulator",
			Runtime: "Android",
		},
	}
}

// ValidateDevice asserts that a device has all required fields and valid values.
func ValidateDevice(t *testing.T, device *cmd.Device) {
	t.Helper()

	if device == nil {
		t.Fatal("Device should not be nil")
	}

	if device.Name == "" {
		t.Error("Device name should not be empty")
	}

	if device.UDID == "" {
		t.Error("Device UDID should not be empty")
	}

	if device.Type == "" {
		t.Error("Device type should not be empty")
	}

	validTypes := []string{"iOS Simulator", "Android Emulator"}
	validType := false

	for _, vt := range validTypes {
		if device.Type == vt {
			validType = true

			break
		}
	}

	if !validType {
		t.Errorf("Device type should be one of %v, got %s", validTypes, device.Type)
	}

	validStates := []string{"Booted", "Shutdown", "Shutting Down", "Booting"}
	if device.State != "" {
		validState := false

		for _, vs := range validStates {
			if device.State == vs {
				validState = true

				break
			}
		}

		if !validState {
			t.Errorf("Device state should be one of %v, got %s", validStates, device.State)
		}
	}
}

// ValidateConfig asserts that a config is properly structured.
func ValidateConfig(t *testing.T, config *cmd.Config) {
	t.Helper()

	if config == nil {
		t.Fatal("Config should not be nil")
	}

	if config.LastStartedDevice != nil {
		ValidateDevice(t, config.LastStartedDevice)
	}
}

// AssertDeviceEqual asserts that two devices have equal field values.
func AssertDeviceEqual(t *testing.T, expected, actual *cmd.Device) {
	t.Helper()

	if expected == nil && actual == nil {
		return
	}

	if expected == nil || actual == nil {
		t.Fatalf("One device is nil: expected=%v, actual=%v", expected, actual)
	}

	if expected.Name != actual.Name {
		t.Errorf("Device name mismatch: expected=%s, actual=%s", expected.Name, actual.Name)
	}

	if expected.UDID != actual.UDID {
		t.Errorf("Device UDID mismatch: expected=%s, actual=%s", expected.UDID, actual.UDID)
	}

	if expected.Type != actual.Type {
		t.Errorf("Device type mismatch: expected=%s, actual=%s", expected.Type, actual.Type)
	}

	if expected.State != actual.State {
		t.Errorf("Device state mismatch: expected=%s, actual=%s", expected.State, actual.State)
	}

	if expected.Runtime != actual.Runtime {
		t.Errorf("Device runtime mismatch: expected=%s, actual=%s", expected.Runtime, actual.Runtime)
	}
}
