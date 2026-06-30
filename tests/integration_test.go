package tests

import (
	"fmt"
	"os"
	"runtime"
	"strings"
	"testing"

	"github.com/annurdien/sim-cli/cmd"
)

func TestIntegration_ConfigPersistence(t *testing.T) {
	_ = NewTestHelpers(t)

	testDevice := &cmd.Device{
		Name:    "Integration Test Device",
		UDID:    "integration-test-udid",
		Type:    "iOS Simulator",
		State:   "Booted",
		Runtime: "iOS 17.0",
	}

	if err := cmd.SaveLastStartedDevice(testDevice); err != nil {
		t.Fatalf("Failed to save device: %v", err)
	}

	config, err := cmd.LoadConfig()
	if err != nil {
		t.Fatalf("Failed to load config: %v", err)
	}

	if config.LastStartedDevice == nil {
		t.Fatal("Last started device should not be nil")
	}

	if config.LastStartedDevice.Name != testDevice.Name {
		t.Errorf("Expected device name %s, got %s", testDevice.Name, config.LastStartedDevice.Name)
	}
}

func TestIntegration_DeviceLifecycle(t *testing.T) {
	_ = NewTestHelpers(t)

	testDevice := &cmd.Device{
		Name:    "Lifecycle Test Device",
		UDID:    "lifecycle-test-udid",
		Type:    "iOS Simulator",
		State:   "Shutdown",
		Runtime: "iOS 17.0",
	}

	if err := cmd.SaveLastStartedDevice(testDevice); err != nil {
		t.Fatalf("Failed to save device: %v", err)
	}

	testDevice.State = "Booted"
	if err := cmd.SaveLastStartedDevice(testDevice); err != nil {
		t.Fatalf("Failed to update device state: %v", err)
	}

	config, err := cmd.LoadConfig()
	if err != nil {
		t.Fatalf("Failed to load config: %v", err)
	}

	if config.LastStartedDevice.State != "Booted" {
		t.Errorf("Expected device state 'Booted', got '%s'", config.LastStartedDevice.State)
	}
}

func TestIntegration_MediaCapture(t *testing.T) {
	deviceID := "media-test-device"

	screenshotFile := cmd.GenerateFilename("screenshot", deviceID, ".png")
	if !containsSubstring(screenshotFile, deviceID) {
		t.Errorf("Screenshot filename should contain device ID %s", deviceID)
	}

	if !hasSuffix(screenshotFile, ".png") {
		t.Error("Screenshot filename should end with .png")
	}

	recordingFile := cmd.GenerateFilename("recording", deviceID, ".mp4")
	if !containsSubstring(recordingFile, deviceID) {
		t.Errorf("Recording filename should contain device ID %s", deviceID)
	}

	if !hasSuffix(recordingFile, ".mp4") {
		t.Error("Recording filename should end with .mp4")
	}

	correctedFile := cmd.EnsureExtension("test.jpg", ".png")
	if !hasSuffix(correctedFile, ".png") {
		t.Errorf("Expected .png extension, got %s", correctedFile)
	}

	correctedVideoFile := cmd.EnsureExtension("test.avi", ".mp4")
	if !hasSuffix(correctedVideoFile, ".mp4") {
		t.Errorf("Expected .mp4 extension, got %s", correctedVideoFile)
	}
}

func TestIntegration_CommandChaining(t *testing.T) {
	_ = NewTestHelpers(t)

	testDevice := &cmd.Device{
		Name:    "Chain Test Device",
		UDID:    "chain-test-udid",
		Type:    "iOS Simulator",
		State:   "Shutdown",
		Runtime: "iOS 17.0",
	}

	testDevice.State = "Booted"
	if err := cmd.SaveLastStartedDevice(testDevice); err != nil {
		t.Fatalf("Failed to start device: %v", err)
	}

	lastDevice, err := cmd.GetLastStartedDevice()
	if err != nil {
		t.Fatalf("Failed to get last device: %v", err)
	}

	if lastDevice.State != "Booted" {
		t.Errorf("Expected device state 'Booted', got '%s'", lastDevice.State)
	}

	screenshotFile := cmd.GenerateFilename("screenshot", testDevice.Name, ".png")
	if !containsSubstring(screenshotFile, "Chain_Test_Device") {
		t.Errorf("Screenshot filename should contain sanitized device name, got %s", screenshotFile)
	}

	testDevice.State = "Shutdown"
	if err := cmd.SaveLastStartedDevice(testDevice); err != nil {
		t.Fatalf("Failed to stop device: %v", err)
	}

	finalDevice, err := cmd.GetLastStartedDevice()
	if err != nil {
		t.Fatalf("Failed to get final device state: %v", err)
	}

	if finalDevice.State != "Shutdown" {
		t.Errorf("Expected final state 'Shutdown', got '%s'", finalDevice.State)
	}
}

func TestIntegration_ErrorHandling(t *testing.T) {
	h := NewTestHelpers(t)

	// Verify config dir is accessible under temp HOME.
	configDir := cmd.GetConfigDir()
	if configDir == "" {
		t.Error("Config directory should not be empty")
	}

	// Device with empty name should still be saveable (validation is at usage time).
	incompleteDevice := &cmd.Device{
		Name: "",
		UDID: "test-udid",
		Type: "iOS Simulator",
	}

	if err := cmd.SaveLastStartedDevice(incompleteDevice); err != nil {
		t.Errorf("Should be able to save device with empty name: %v", err)
	}

	// Corrupted config should return an error.
	configPath := cmd.GetConfigPath()
	if err := os.WriteFile(configPath, []byte("invalid json"), 0o644); err != nil {
		t.Fatalf("Failed to write invalid config: %v", err)
	}

	_, err := cmd.LoadConfig()
	if err == nil {
		t.Error("Expected error when loading corrupted config")
	}

	_ = h // suppress unused warning
}

func TestIntegration_CrossPlatform(t *testing.T) {
	if runtime.GOOS == "darwin" {
		device := cmd.FindIOSSimulatorByID("non-existent-device")
		if device != nil {
			t.Error("Should return nil for non-existent iOS device")
		}
	} else {
		t.Log("Skipping iOS-specific tests on non-macOS platform")
	}

	if cmd.DoesAndroidAVDExist("non-existent-avd-xyz") {
		t.Error("Should return false for non-existent Android AVD")
	}

	if cmd.IsAndroidEmulatorRunning("non-existent-emulator-xyz") {
		t.Error("Should return false for non-existent Android emulator")
	}

	_ = NewTestHelpers(t)

	testDevice := &cmd.Device{
		Name: "Cross-platform Test Device",
		UDID: "cross-platform-test-udid",
		Type: "iOS Simulator",
	}

	if err := cmd.SaveLastStartedDevice(testDevice); err != nil {
		t.Errorf("Config operations should work cross-platform: %v", err)
	}
}

func TestPerformance_ListCommand(t *testing.T) {
	var allDevices []cmd.Device

	if runtime.GOOS == "darwin" {
		allDevices = append(allDevices, cmd.GetIOSSimulators()...)
	}

	allDevices = append(allDevices, cmd.GetAndroidEmulators()...)

	// Add simulated devices.
	for i := 0; i < 10; i++ {
		allDevices = append(allDevices, cmd.Device{
			Name:    fmt.Sprintf("iPhone %d", i),
			UDID:    fmt.Sprintf("ios-udid-%d", i),
			Type:    "iOS Simulator",
			State:   "Shutdown",
			Runtime: "iOS 17.0",
		})
	}

	for i := 0; i < 10; i++ {
		allDevices = append(allDevices, cmd.Device{
			Name:    fmt.Sprintf("Pixel_%d", i),
			UDID:    fmt.Sprintf("android-udid-%d", i),
			Type:    "Android Emulator",
			State:   "Shutdown",
			Runtime: "Android",
		})
	}

	// Validate all devices satisfy structural invariants.
	for i := range allDevices {
		ValidateDevice(t, &allDevices[i])
	}
}

func TestPerformance_ConfigOperations(t *testing.T) {
	_ = NewTestHelpers(t)

	testDevice := &cmd.Device{
		Name:    "Performance Test Device",
		UDID:    "performance-test-udid",
		Type:    "iOS Simulator",
		State:   "Booted",
		Runtime: "iOS 17.0",
	}

	// Run multiple save/load cycles and verify correctness.
	for i := 0; i < 5; i++ {
		testDevice.Name = fmt.Sprintf("Device %d", i)
		if err := cmd.SaveLastStartedDevice(testDevice); err != nil {
			t.Fatalf("Iteration %d: failed to save device: %v", i, err)
		}

		loaded, err := cmd.LoadConfig()
		if err != nil {
			t.Fatalf("Iteration %d: failed to load config: %v", i, err)
		}

		if loaded.LastStartedDevice.Name != testDevice.Name {
			t.Errorf("Iteration %d: name mismatch: want %s, got %s", i, testDevice.Name, loaded.LastStartedDevice.Name)
		}
	}

	// Stress test with a large device name.
	largeNameDevice := &cmd.Device{
		Name:    strings.Repeat("LongDeviceName", 100), // 1400 characters
		UDID:    "large-name-test-udid",
		Type:    "iOS Simulator",
		State:   "Booted",
		Runtime: "iOS 17.0",
	}

	if err := cmd.SaveLastStartedDevice(largeNameDevice); err != nil {
		t.Errorf("Should handle large device names: %v", err)
	}

	config, err := cmd.LoadConfig()
	if err != nil {
		t.Errorf("Should load config with large device name: %v", err)
	}

	if len(config.LastStartedDevice.Name) != len(largeNameDevice.Name) {
		t.Error("Large device name should be preserved completely")
	}
}

func TestSecurity_ConfigFilePermissions(t *testing.T) {
	_ = NewTestHelpers(t)

	if err := cmd.SaveLastStartedDevice(CreateTestDevice("security-test")); err != nil {
		t.Fatalf("Failed to save device: %v", err)
	}

	info, err := os.Stat(cmd.GetConfigPath())
	if err != nil {
		t.Fatalf("Failed to stat config file: %v", err)
	}

	// Must be 0600 (owner read/write only) — matches the source code's WriteFile call.
	if info.Mode().Perm() != 0o600 {
		t.Errorf("Expected config file permissions 0600, got %o", info.Mode().Perm())
	}
}

func TestSecurity_ConfigDirectory(t *testing.T) {
	_ = NewTestHelpers(t)

	if err := cmd.SaveLastStartedDevice(CreateTestDevice("dir-test")); err != nil {
		t.Fatalf("Failed to save device: %v", err)
	}

	info, err := os.Stat(cmd.GetConfigDir())
	if err != nil {
		t.Fatalf("Failed to stat config directory: %v", err)
	}

	if info.Mode().Perm() != 0o755 {
		t.Errorf("Expected config directory permissions 0755, got %o", info.Mode().Perm())
	}
}

func TestEdgeCases_EmptyDeviceName(t *testing.T) {
	_ = NewTestHelpers(t)

	device := &cmd.Device{
		Name: "",
		UDID: "empty-name-test",
		Type: "iOS Simulator",
	}

	if err := cmd.SaveLastStartedDevice(device); err != nil {
		t.Errorf("Should save device with empty name: %v", err)
	}
}

func TestEdgeCases_LongDeviceName(t *testing.T) {
	_ = NewTestHelpers(t)

	longName := strings.Repeat("a", 1000) // Fixed: was O(n²) loop

	device := &cmd.Device{
		Name: longName,
		UDID: "long-name-test",
		Type: "iOS Simulator",
	}

	if err := cmd.SaveLastStartedDevice(device); err != nil {
		t.Errorf("Should save device with long name: %v", err)
	}

	config, err := cmd.LoadConfig()
	if err != nil {
		t.Fatalf("Should load config with long device name: %v", err)
	}

	if config.LastStartedDevice.Name != longName {
		t.Error("Long device name should be preserved")
	}
}

func TestEdgeCases_SpecialCharacters(t *testing.T) {
	_ = NewTestHelpers(t)

	specialChars := "Device with 特殊字符 and émojis 🚀 and symbols @#$%"

	device := &cmd.Device{
		Name: specialChars,
		UDID: "special-chars-test",
		Type: "iOS Simulator",
	}

	if err := cmd.SaveLastStartedDevice(device); err != nil {
		t.Errorf("Should save device with special characters: %v", err)
	}

	config, err := cmd.LoadConfig()
	if err != nil {
		t.Fatalf("Should load config with special characters: %v", err)
	}

	if config.LastStartedDevice.Name != specialChars {
		t.Error("Special characters in device name should be preserved")
	}
}
