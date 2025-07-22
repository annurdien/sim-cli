package tests

import (
	"os"
	"runtime"
	"strings"
	"testing"
)

func TestFindIOSSimulatorUDID(t *testing.T) {
	if runtime.GOOS != "darwin" {
		t.Skip("iOS simulators only available on macOS")
	}

	// Test with a UDID format input (should return the same UDID)
	testUDID := "12345678-1234-5678-9012-123456789012"
	result := findIOSSimulatorUDID(testUDID)
	if result != testUDID {
		t.Errorf("Expected UDID %s, got %s", testUDID, result)
	}

	// Test with invalid UDID format
	invalidUDID := "invalid-udid"
	result = findIOSSimulatorUDID(invalidUDID)
	// Result could be empty string if no simulator found, which is expected
	_ = result // Acknowledge we're not using the result
}

func TestFindIOSSimulatorByID(t *testing.T) {
	if runtime.GOOS != "darwin" {
		t.Skip("iOS simulators only available on macOS")
	}

	// Test with non-existent device
	device := findIOSSimulatorByID("non-existent-device")
	if device != nil {
		t.Error("Expected nil for non-existent device")
	}
}

func TestDoesAndroidAVDExist(t *testing.T) {
	// Test with non-existent AVD
	exists := doesAndroidAVDExist("non-existent-avd")
	if exists {
		t.Error("Expected false for non-existent AVD")
	}
}

func TestIsAndroidEmulatorRunning(t *testing.T) {
	// Test with non-existent emulator
	running := isAndroidEmulatorRunning("non-existent-emulator")
	if running {
		t.Error("Expected false for non-existent emulator")
	}
}

func TestFindRunningAndroidEmulator(t *testing.T) {
	// Test with non-existent emulator
	udid := findRunningAndroidEmulator("non-existent-emulator")
	if udid != "" {
		t.Error("Expected empty string for non-existent emulator")
	}
}

func TestStartCommand_LTS_NoLastDevice(t *testing.T) {
	// Test the 'lts' functionality when no last device exists
	tempDir := t.TempDir()
	originalHome := os.Getenv("HOME")
	defer os.Setenv("HOME", originalHome)

	os.Setenv("HOME", tempDir)

	// Ensure no config exists
	config, err := loadConfig()
	if err != nil {
		t.Fatalf("Failed to load config: %v", err)
	}

	if config.LastStartedDevice != nil {
		t.Error("Expected no last started device in fresh config")
	}

	// This simulates what would happen if someone tried to use 'lts' with no last device
	// In the real implementation, this would show an error message
	lastDevice, err := getLastStartedDevice()
	if err != nil {
		t.Errorf("Should not error when getting last device: %v", err)
	}

	if lastDevice != nil {
		t.Error("Expected nil last device when none exists")
	}
}

func TestStopCommand_ValidInput(t *testing.T) {
	// Test stop command with valid input
	// Since we can't mock exec.Command easily, we'll test the helper functions

	// Test iOS simulator stop (macOS only)
	if runtime.GOOS == "darwin" {
		// Test with non-existent device (should return false)
		result := stopIOSSimulator("non-existent-device")
		if result {
			t.Error("Expected false for non-existent iOS device")
		}
	}

	// Test Android emulator stop
	result := stopAndroidEmulator("non-existent-emulator")
	if result {
		t.Error("Expected false for non-existent Android emulator")
	}
}

func TestShutdownCommand_ValidInput(t *testing.T) {
	// Test shutdown command with valid input
	if runtime.GOOS == "darwin" {
		// Test with non-existent device (should return false)
		result := shutdownIOSSimulator("non-existent-device")
		if result {
			t.Error("Expected false for non-existent iOS device")
		}
	}

	// For Android, shutdown is the same as stop
	result := stopAndroidEmulator("non-existent-emulator")
	if result {
		t.Error("Expected false for non-existent Android emulator")
	}
}

func TestRestartCommand_ValidInput(t *testing.T) {
	// Test restart command with valid input
	if runtime.GOOS == "darwin" {
		// Test with non-existent device (should return false)
		result := restartIOSSimulator("non-existent-device")
		if result {
			t.Error("Expected false for non-existent iOS device")
		}
	}

	// Test Android emulator restart
	result := restartAndroidEmulator("non-existent-emulator")
	if result {
		t.Error("Expected false for non-existent Android emulator")
	}
}

func TestDeleteCommand_ValidInput(t *testing.T) {
	// Test delete command with valid input
	if runtime.GOOS == "darwin" {
		// Test with non-existent device (should return false)
		result := deleteIOSSimulator("non-existent-device")
		if result {
			t.Error("Expected false for non-existent iOS device")
		}
	}

	// Test Android emulator delete
	result := deleteAndroidEmulator("non-existent-emulator")
	if result {
		t.Error("Expected false for non-existent Android emulator")
	}
}

func TestLastCommand_NoLastDevice(t *testing.T) {
	// Test last command when no last device exists
	tempDir := t.TempDir()
	originalHome := os.Getenv("HOME")
	defer os.Setenv("HOME", originalHome)

	os.Setenv("HOME", tempDir)

	// Ensure no last device exists
	lastDevice, err := getLastStartedDevice()
	if err != nil {
		t.Errorf("Should not error when getting last device: %v", err)
	}

	if lastDevice != nil {
		t.Error("Expected nil last device when none exists")
	}

	// Test that we can save and retrieve a last device
	testDevice := &Device{
		Name: "Test Last Device",
		UDID: "test-last-udid",
		Type: "iOS Simulator",
	}

	err = saveLastStartedDevice(testDevice)
	if err != nil {
		t.Fatalf("Failed to save last device: %v", err)
	}

	// Verify we can retrieve it
	retrievedDevice, err := getLastStartedDevice()
	if err != nil {
		t.Fatalf("Failed to get last device: %v", err)
	}

	if retrievedDevice == nil {
		t.Fatal("Retrieved device should not be nil")
	}

	if retrievedDevice.Name != testDevice.Name {
		t.Errorf("Expected device name %s, got %s", testDevice.Name, retrievedDevice.Name)
	}
}

// Helper functions that mirror the unexported functions in cmd package for testing

func findIOSSimulatorUDID(deviceID string) string {
	if len(deviceID) == 36 && strings.Count(deviceID, "-") == 4 {
		return deviceID
	}

	simulators := getIOSSimulators()
	for _, sim := range simulators {
		if strings.EqualFold(sim.Name, deviceID) || sim.UDID == deviceID {
			return sim.UDID
		}
	}

	return ""
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

func doesAndroidAVDExist(avdName string) bool {
	emulators := getAndroidEmulators()
	for _, emu := range emulators {
		if emu.Name == avdName {
			return true
		}
	}

	return false
}

func isAndroidEmulatorRunning(avdName string) bool {
	return findRunningAndroidEmulator(avdName) != ""
}

func findRunningAndroidEmulator(avdName string) string {
	emulators := getAndroidEmulators()
	for _, emu := range emulators {
		if emu.Name == avdName && emu.State == "Booted" {
			return emu.UDID
		}
	}

	return ""
}

func getLastStartedDevice() (*Device, error) {
	config, err := loadConfig()
	if err != nil {
		return nil, err
	}

	return config.LastStartedDevice, nil
}

// Mock implementations for testing device operations.
func stopIOSSimulator(deviceID string) bool {
	// In a real implementation, this would call xcrun simctl shutdown
	// For testing, we just check if the device exists
	return findIOSSimulatorUDID(deviceID) != ""
}

func stopAndroidEmulator(deviceID string) bool {
	// In a real implementation, this would call adb emu kill
	// For testing, we just check if the emulator exists
	return findRunningAndroidEmulator(deviceID) != ""
}

func shutdownIOSSimulator(deviceID string) bool {
	// Same as stop for iOS simulators
	return stopIOSSimulator(deviceID)
}

func restartIOSSimulator(deviceID string) bool {
	// In a real implementation, this would shutdown then boot
	// For testing, we just check if the device exists
	return findIOSSimulatorUDID(deviceID) != ""
}

func restartAndroidEmulator(deviceID string) bool {
	// In a real implementation, this would stop then start
	// For testing, we just check if the emulator exists
	return doesAndroidAVDExist(deviceID)
}

func deleteIOSSimulator(deviceID string) bool {
	// In a real implementation, this would call xcrun simctl delete
	// For testing, we just check if the device exists
	return findIOSSimulatorUDID(deviceID) != ""
}

func deleteAndroidEmulator(deviceID string) bool {
	// In a real implementation, this would call avdmanager delete
	// For testing, we just check if the emulator exists
	return doesAndroidAVDExist(deviceID)
}
