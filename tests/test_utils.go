package tests

import (
	"os"
	"testing"
)

// Config struct mirrors the one in cmd package for testing
type Config struct {
	LastStartedDevice *Device `json:"lastStartedDevice,omitempty"`
}

// Device struct mirrors the one in cmd package for testing
type Device struct {
	Name       string `json:"name"`
	UDID       string `json:"udid"`
	State      string `json:"state"`
	Type       string `json:"type"` // "simulator" or "emulator"
	Runtime    string `json:"runtime,omitempty"`
	DeviceType string `json:"deviceTypeIdentifier,omitempty"`
}

// TestHelpers provides utility functions for testing
type TestHelpers struct {
	tempDir      string
	originalHome string
}

// NewTestHelpers creates a new test helpers instance
func NewTestHelpers(t *testing.T) *TestHelpers {
	tempDir := t.TempDir()
	originalHome := os.Getenv("HOME")

	return &TestHelpers{
		tempDir:      tempDir,
		originalHome: originalHome,
	}
}

// SetupTempHome sets up a temporary home directory for testing
func (h *TestHelpers) SetupTempHome() {
	os.Setenv("HOME", h.tempDir)
}

// RestoreHome restores the original home directory
func (h *TestHelpers) RestoreHome() {
	os.Setenv("HOME", h.originalHome)
}

// CreateTestDevice creates a test device with default values
func CreateTestDevice(name string) *Device {
	return &Device{
		Name:    name,
		UDID:    "test-udid-" + name,
		Type:    "iOS Simulator",
		State:   "Shutdown",
		Runtime: "iOS 17.0",
	}
}

// CreateTestConfig creates a test config with a test device
func CreateTestConfig() *Config {
	return &Config{
		LastStartedDevice: CreateTestDevice("test-device"),
	}
}

// MockExecCommand can be used to mock exec.Command calls in tests
// This would be expanded with actual mocking functionality
type MockExecCommand struct {
	Commands []string
	Outputs  map[string]string
	Errors   map[string]error
}

// NewMockExecCommand creates a new mock exec command
func NewMockExecCommand() *MockExecCommand {
	return &MockExecCommand{
		Commands: []string{},
		Outputs:  make(map[string]string),
		Errors:   make(map[string]error),
	}
}

// SetOutput sets the expected output for a command
func (m *MockExecCommand) SetOutput(command, output string) {
	m.Outputs[command] = output
}

// SetError sets the expected error for a command
func (m *MockExecCommand) SetError(command string, err error) {
	m.Errors[command] = err
}

// AssertCommandCalled checks if a command was called
func (m *MockExecCommand) AssertCommandCalled(t *testing.T, command string) {
	for _, cmd := range m.Commands {
		if cmd == command {
			return
		}
	}
	t.Errorf("Expected command %s to be called", command)
}

// TestDeviceData provides common test device data
var TestDeviceData = struct {
	IOSSimulator     Device
	AndroidEmulator  Device
	BootedSimulator  Device
	ShutdownEmulator Device
}{
	IOSSimulator: Device{
		Name:       "iPhone 15",
		UDID:       "12345678-1234-5678-9012-123456789012",
		State:      "Shutdown",
		Type:       "iOS Simulator",
		Runtime:    "iOS 17.0",
		DeviceType: "com.apple.CoreSimulator.SimDeviceType.iPhone-15",
	},
	AndroidEmulator: Device{
		Name:    "Pixel_7_API_34",
		UDID:    "emulator-5554",
		State:   "Shutdown",
		Type:    "Android Emulator",
		Runtime: "Android",
	},
	BootedSimulator: Device{
		Name:       "iPhone 15 Pro",
		UDID:       "87654321-4321-8765-2109-876543210987",
		State:      "Booted",
		Type:       "iOS Simulator",
		Runtime:    "iOS 17.0",
		DeviceType: "com.apple.CoreSimulator.SimDeviceType.iPhone-15-Pro",
	},
	ShutdownEmulator: Device{
		Name:    "Pixel_8_API_34",
		UDID:    "offline",
		State:   "Shutdown",
		Type:    "Android Emulator",
		Runtime: "Android",
	},
}

// ValidateDevice validates that a device has required fields
func ValidateDevice(t *testing.T, device *Device) {
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
	for _, validT := range validTypes {
		if device.Type == validT {
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
		for _, validS := range validStates {
			if device.State == validS {
				validState = true
				break
			}
		}
		if !validState {
			t.Errorf("Device state should be one of %v, got %s", validStates, device.State)
		}
	}
}

// ValidateConfig validates that a config is properly structured
func ValidateConfig(t *testing.T, config *Config) {
	if config == nil {
		t.Fatal("Config should not be nil")
	}

	if config.LastStartedDevice != nil {
		ValidateDevice(t, config.LastStartedDevice)
	}
}

// AssertDeviceEqual asserts that two devices are equal
func AssertDeviceEqual(t *testing.T, expected, actual *Device) {
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
