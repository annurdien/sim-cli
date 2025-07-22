package tests

import (
	"fmt"
	"os"
	"runtime"
	"strings"
	"testing"
)

func TestIntegration_ConfigPersistence(t *testing.T) {
	// Test that config persists across operations
	tempDir := t.TempDir()
	originalHome := os.Getenv("HOME")
	defer os.Setenv("HOME", originalHome)

	os.Setenv("HOME", tempDir)

	// Create and save a device
	testDevice := &Device{
		Name:    "Integration Test Device",
		UDID:    "integration-test-udid",
		Type:    "iOS Simulator",
		State:   "Booted",
		Runtime: "iOS 17.0",
	}

	err := saveLastStartedDevice(testDevice)
	if err != nil {
		t.Fatalf("Failed to save device: %v", err)
	}

	// Load config in a new "session"
	config, err := loadConfig()
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
	// Test complete device lifecycle (start, stop, restart, delete)
	tempDir := t.TempDir()
	originalHome := os.Getenv("HOME")
	defer os.Setenv("HOME", originalHome)

	os.Setenv("HOME", tempDir)

	// Test device creation and tracking
	testDevice := &Device{
		Name:    "Lifecycle Test Device",
		UDID:    "lifecycle-test-udid",
		Type:    "iOS Simulator",
		State:   "Shutdown",
		Runtime: "iOS 17.0",
	}

	// Test saving device as last started
	err := saveLastStartedDevice(testDevice)
	if err != nil {
		t.Fatalf("Failed to save device: %v", err)
	}

	// Test device state changes
	testDevice.State = "Booted"
	err = saveLastStartedDevice(testDevice)
	if err != nil {
		t.Fatalf("Failed to update device state: %v", err)
	}

	// Verify state persistence
	config, err := loadConfig()
	if err != nil {
		t.Fatalf("Failed to load config: %v", err)
	}

	if config.LastStartedDevice.State != "Booted" {
		t.Errorf("Expected device state 'Booted', got '%s'", config.LastStartedDevice.State)
	}

	// Test device removal (simulate deletion)
	testDevice.State = "Deleted"
	err = saveLastStartedDevice(testDevice)
	if err != nil {
		t.Fatalf("Failed to mark device as deleted: %v", err)
	}
}

func TestIntegration_MediaCapture(t *testing.T) {
	// Test screenshot and recording functionality together

	deviceID := "media-test-device"

	// Test screenshot filename generation
	screenshotFile := generateScreenshotFilename(deviceID)
	if !strings.Contains(screenshotFile, deviceID) {
		t.Errorf("Screenshot filename should contain device ID %s", deviceID)
	}
	if !strings.HasSuffix(screenshotFile, ".png") {
		t.Error("Screenshot filename should end with .png")
	}

	// Test recording filename generation
	recordingFile := generateRecordingFilename(deviceID)
	if !strings.Contains(recordingFile, deviceID) {
		t.Errorf("Recording filename should contain device ID %s", deviceID)
	}
	if !strings.HasSuffix(recordingFile, ".mp4") {
		t.Error("Recording filename should end with .mp4")
	}

	// Test extension correction
	wrongExtFile := "test.jpg"
	correctedFile := ensurePNGExtension(wrongExtFile)
	if !strings.HasSuffix(correctedFile, ".png") {
		t.Errorf("Expected .png extension, got %s", correctedFile)
	}

	wrongVideoFile := "test.avi"
	correctedVideoFile := ensureMP4Extension(wrongVideoFile)
	if !strings.HasSuffix(correctedVideoFile, ".mp4") {
		t.Errorf("Expected .mp4 extension, got %s", correctedVideoFile)
	}
}

func TestIntegration_CommandChaining(t *testing.T) {
	// Test running multiple commands in sequence
	tempDir := t.TempDir()
	originalHome := os.Getenv("HOME")
	defer os.Setenv("HOME", originalHome)

	os.Setenv("HOME", tempDir)

	// Simulate command chain: start -> screenshot -> stop
	testDevice := &Device{
		Name:    "Chain Test Device",
		UDID:    "chain-test-udid",
		Type:    "iOS Simulator",
		State:   "Shutdown",
		Runtime: "iOS 17.0",
	}

	// Step 1: "Start" device (save as last started)
	testDevice.State = "Booted"
	err := saveLastStartedDevice(testDevice)
	if err != nil {
		t.Fatalf("Failed to start device: %v", err)
	}

	// Step 2: Verify device is tracked
	lastDevice, err := getLastStartedDevice()
	if err != nil {
		t.Fatalf("Failed to get last device: %v", err)
	}
	if lastDevice.State != "Booted" {
		t.Errorf("Expected device state 'Booted', got '%s'", lastDevice.State)
	}

	// Step 3: "Take screenshot" (test filename generation)
	screenshotFile := generateScreenshotFilename(testDevice.Name)
	if !strings.Contains(screenshotFile, testDevice.Name) {
		t.Error("Screenshot filename should contain device name")
	}

	// Step 4: "Stop" device
	testDevice.State = "Shutdown"
	err = saveLastStartedDevice(testDevice)
	if err != nil {
		t.Fatalf("Failed to stop device: %v", err)
	}

	// Verify final state
	finalDevice, err := getLastStartedDevice()
	if err != nil {
		t.Fatalf("Failed to get final device state: %v", err)
	}
	if finalDevice.State != "Shutdown" {
		t.Errorf("Expected final state 'Shutdown', got '%s'", finalDevice.State)
	}
}

func TestIntegration_ErrorHandling(t *testing.T) {
	// Test error handling across different scenarios

	// Test 1: Invalid config directory
	tempDir := t.TempDir()
	originalHome := os.Getenv("HOME")
	defer os.Setenv("HOME", originalHome)

	// Set HOME to a file instead of directory (should handle gracefully)
	invalidFile := tempDir + "/not-a-directory"
	f, err := os.Create(invalidFile)
	if err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}
	f.Close()

	os.Setenv("HOME", invalidFile)

	// Should fallback to temp directory
	configDir := getConfigDir()
	if configDir == "" {
		t.Error("Config directory should not be empty even with invalid HOME")
	}

	// Test 2: Device with missing required fields
	os.Setenv("HOME", tempDir)

	incompleteDevice := &Device{
		Name: "", // Empty name
		UDID: "test-udid",
		Type: "iOS Simulator",
	}

	// Should still be able to save (validation is at usage time)
	err = saveLastStartedDevice(incompleteDevice)
	if err != nil {
		t.Errorf("Should be able to save device with empty name: %v", err)
	}

	// Test 3: Corrupted config recovery
	// Write invalid JSON to config file
	configPath := getConfigPath()
	err = os.WriteFile(configPath, []byte("invalid json"), 0644)
	if err != nil {
		t.Fatalf("Failed to write invalid config: %v", err)
	}

	// Should return error but not crash
	_, err = loadConfig()
	if err == nil {
		t.Error("Expected error when loading corrupted config")
	}
}

func TestIntegration_CrossPlatform(t *testing.T) {
	// Test platform-specific behavior

	// Test 1: iOS-specific functionality (macOS only)
	if runtime.GOOS == "darwin" {
		// Test iOS simulator functions exist and handle non-existent devices
		udid := findIOSSimulatorUDID("non-existent-device")
		if udid != "" {
			t.Error("Should return empty string for non-existent iOS device")
		}

		device := findIOSSimulatorByID("non-existent-device")
		if device != nil {
			t.Error("Should return nil for non-existent iOS device")
		}

		// Test iOS screenshot function
		result := takeIOSScreenshot("non-existent-device", "test.png")
		if result {
			t.Error("Should return false for non-existent iOS device")
		}
	} else {
		// On non-macOS, iOS functions should either not exist or handle gracefully
		t.Log("Skipping iOS-specific tests on non-macOS platform")
	}

	// Test 2: Android functionality (cross-platform)
	// Test Android emulator functions
	exists := doesAndroidAVDExist("non-existent-avd")
	if exists {
		t.Error("Should return false for non-existent Android AVD")
	}

	running := isAndroidEmulatorRunning("non-existent-emulator")
	if running {
		t.Error("Should return false for non-existent Android emulator")
	}

	// Test Android screenshot function
	result := takeAndroidScreenshot("non-existent-device", "test.png")
	if result {
		t.Error("Should return false for non-existent Android device")
	}

	// Test 3: Platform-agnostic functionality
	// Config operations should work on all platforms
	tempDir := t.TempDir()
	originalHome := os.Getenv("HOME")
	defer os.Setenv("HOME", originalHome)

	os.Setenv("HOME", tempDir)

	testDevice := &Device{
		Name: "Cross-platform Test Device",
		UDID: "cross-platform-test-udid",
		Type: "iOS Simulator",
	}

	err := saveLastStartedDevice(testDevice)
	if err != nil {
		t.Errorf("Config operations should work cross-platform: %v", err)
	}
}

func TestPerformance_ListCommand(t *testing.T) {
	// Test performance of list command with many devices

	// Test that device list operations are efficient
	startTime := testing.Benchmark(func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			// Test iOS simulators (macOS only)
			if runtime.GOOS == "darwin" {
				simulators := getIOSSimulators()
				_ = simulators // Use the result
			}

			// Test Android emulators
			emulators := getAndroidEmulators()
			_ = emulators // Use the result
		}
	})

	t.Logf("List command benchmark: %s", startTime)

	// Test with simulated large device list
	var allDevices []Device

	// Add simulated iOS devices
	for i := 0; i < 10; i++ {
		device := Device{
			Name:    fmt.Sprintf("iPhone %d", i),
			UDID:    fmt.Sprintf("ios-udid-%d", i),
			Type:    "iOS Simulator",
			State:   "Shutdown",
			Runtime: "iOS 17.0",
		}
		allDevices = append(allDevices, device)
	}

	// Add simulated Android devices
	for i := 0; i < 10; i++ {
		device := Device{
			Name:    fmt.Sprintf("Pixel_%d", i),
			UDID:    fmt.Sprintf("android-udid-%d", i),
			Type:    "Android Emulator",
			State:   "Shutdown",
			Runtime: "Android",
		}
		allDevices = append(allDevices, device)
	}

	// Test that we can efficiently process the list
	if len(allDevices) != 20 {
		t.Errorf("Expected 20 devices, got %d", len(allDevices))
	}

	// Test validation performance
	for _, device := range allDevices {
		ValidateDevice(t, &device)
	}
}

func TestPerformance_ConfigOperations(t *testing.T) {
	// Test performance of config save/load operations
	tempDir := t.TempDir()
	originalHome := os.Getenv("HOME")
	defer os.Setenv("HOME", originalHome)

	os.Setenv("HOME", tempDir)

	// Test multiple save/load cycles
	testDevice := &Device{
		Name:    "Performance Test Device",
		UDID:    "performance-test-udid",
		Type:    "iOS Simulator",
		State:   "Booted",
		Runtime: "iOS 17.0",
	}

	// Benchmark config save operations
	saveTime := testing.Benchmark(func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			testDevice.Name = fmt.Sprintf("Device %d", i)
			err := saveLastStartedDevice(testDevice)
			if err != nil {
				b.Fatalf("Failed to save device: %v", err)
			}
		}
	})

	t.Logf("Config save benchmark: %s", saveTime)

	// Benchmark config load operations
	loadTime := testing.Benchmark(func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			_, err := loadConfig()
			if err != nil {
				b.Fatalf("Failed to load config: %v", err)
			}
		}
	})

	t.Logf("Config load benchmark: %s", loadTime)

	// Test with large device name (stress test)
	largeNameDevice := &Device{
		Name:    strings.Repeat("LongDeviceName", 100), // 1400 characters
		UDID:    "large-name-test-udid",
		Type:    "iOS Simulator",
		State:   "Booted",
		Runtime: "iOS 17.0",
	}

	err := saveLastStartedDevice(largeNameDevice)
	if err != nil {
		t.Errorf("Should handle large device names: %v", err)
	}

	// Verify it can be loaded back
	config, err := loadConfig()
	if err != nil {
		t.Errorf("Should be able to load config with large device name: %v", err)
	}

	if len(config.LastStartedDevice.Name) != len(largeNameDevice.Name) {
		t.Error("Large device name should be preserved completely")
	}
}

func TestSecurity_ConfigFilePermissions(t *testing.T) {
	// Test that config files have appropriate permissions
	tempDir := t.TempDir()
	originalHome := os.Getenv("HOME")
	defer os.Setenv("HOME", originalHome)

	os.Setenv("HOME", tempDir)

	// Save a config
	testDevice := &Device{
		Name: "Security Test Device",
		UDID: "security-test-udid",
		Type: "iOS Simulator",
	}

	err := saveLastStartedDevice(testDevice)
	if err != nil {
		t.Fatalf("Failed to save device: %v", err)
	}

	// Check file permissions
	configPath := getConfigPath()
	info, err := os.Stat(configPath)
	if err != nil {
		t.Fatalf("Failed to stat config file: %v", err)
	}

	mode := info.Mode()
	if mode.Perm() != 0644 {
		t.Errorf("Expected config file permissions 0644, got %o", mode.Perm())
	}
}

func TestSecurity_ConfigDirectory(t *testing.T) {
	// Test that config directory has appropriate permissions
	tempDir := t.TempDir()
	originalHome := os.Getenv("HOME")
	defer os.Setenv("HOME", originalHome)

	os.Setenv("HOME", tempDir)

	// Save a config to trigger directory creation
	testDevice := &Device{
		Name: "Directory Test Device",
		UDID: "directory-test-udid",
		Type: "iOS Simulator",
	}

	err := saveLastStartedDevice(testDevice)
	if err != nil {
		t.Fatalf("Failed to save device: %v", err)
	}

	// Check directory permissions
	configDir := getConfigDir()
	info, err := os.Stat(configDir)
	if err != nil {
		t.Fatalf("Failed to stat config directory: %v", err)
	}

	mode := info.Mode()
	if mode.Perm() != 0755 {
		t.Errorf("Expected config directory permissions 0755, got %o", mode.Perm())
	}
}

func TestEdgeCases_EmptyDeviceName(t *testing.T) {
	// Test handling of empty device names
	device := &Device{
		Name: "",
		UDID: "empty-name-test",
		Type: "iOS Simulator",
	}

	// Should be able to save device with empty name
	tempDir := t.TempDir()
	originalHome := os.Getenv("HOME")
	defer os.Setenv("HOME", originalHome)

	os.Setenv("HOME", tempDir)

	err := saveLastStartedDevice(device)
	if err != nil {
		t.Errorf("Should be able to save device with empty name: %v", err)
	}
}

func TestEdgeCases_LongDeviceName(t *testing.T) {
	// Test handling of very long device names
	longName := string(make([]byte, 1000))
	for i := range longName {
		longName = longName[:i] + "a" + longName[i+1:]
	}

	device := &Device{
		Name: longName,
		UDID: "long-name-test",
		Type: "iOS Simulator",
	}

	tempDir := t.TempDir()
	originalHome := os.Getenv("HOME")
	defer os.Setenv("HOME", originalHome)

	os.Setenv("HOME", tempDir)

	err := saveLastStartedDevice(device)
	if err != nil {
		t.Errorf("Should be able to save device with long name: %v", err)
	}

	// Verify we can load it back
	config, err := loadConfig()
	if err != nil {
		t.Errorf("Should be able to load config with long device name: %v", err)
	}

	if config.LastStartedDevice.Name != longName {
		t.Error("Long device name should be preserved")
	}
}

func TestEdgeCases_SpecialCharacters(t *testing.T) {
	// Test handling of special characters in device names
	specialChars := "Device with 特殊字符 and émojis 🚀 and symbols @#$%"

	device := &Device{
		Name: specialChars,
		UDID: "special-chars-test",
		Type: "iOS Simulator",
	}

	tempDir := t.TempDir()
	originalHome := os.Getenv("HOME")
	defer os.Setenv("HOME", originalHome)

	os.Setenv("HOME", tempDir)

	err := saveLastStartedDevice(device)
	if err != nil {
		t.Errorf("Should be able to save device with special characters: %v", err)
	}

	// Verify we can load it back
	config, err := loadConfig()
	if err != nil {
		t.Errorf("Should be able to load config with special characters: %v", err)
	}

	if config.LastStartedDevice.Name != specialChars {
		t.Error("Special characters in device name should be preserved")
	}
}
