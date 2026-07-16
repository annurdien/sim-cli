package tests

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/annurdien/sim-cli/cmd"
)

func TestGetConfigDir(t *testing.T) {
	_ = NewTestHelpers(t) // sets up temp HOME via t.Setenv

	configDir, err := cmd.GetConfigDir()
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}
	if configDir == "" {
		t.Error("Config directory should not be empty")
	}
}

func TestGetConfigPath(t *testing.T) {
	_ = NewTestHelpers(t)

	configPath, err := cmd.GetConfigPath()
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}
	if configPath == "" {
		t.Error("Config path should not be empty")
	}

	if !filepath.IsAbs(configPath) {
		t.Error("Config path should be absolute")
	}
}

func TestLoadConfig_NonExistent(t *testing.T) {
	_ = NewTestHelpers(t)

	config, err := cmd.LoadConfig()
	if err != nil {
		t.Errorf("Expected no error for non-existent config, got: %v", err)
	}

	if config == nil {
		t.Error("Config should not be nil")
	}
}

func TestSaveAndLoadConfig(t *testing.T) {
	_ = NewTestHelpers(t)

	testDevice := &cmd.Device{
		Name:    "Test Device",
		UDID:    "test-udid-123",
		Type:    "iOS Simulator",
		State:   "Booted",
		Runtime: "iOS 17.0",
	}

	if err := cmd.SaveLastStartedDevice(testDevice); err != nil {
		t.Fatalf("Failed to save last started device: %v", err)
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

func TestSaveConfig_EmptyConfig(t *testing.T) {
	_ = NewTestHelpers(t)

	config := &cmd.Config{}

	if err := cmd.SaveConfig(config); err != nil {
		t.Errorf("Should be able to save empty config: %v", err)
	}
}

func TestLoadConfig_CorruptedFile(t *testing.T) {
	h := NewTestHelpers(t)

	// Write invalid JSON directly into the config location.
	configDir := filepath.Join(h.TempDir, ".sim-cli")
	if err := os.MkdirAll(configDir, 0o755); err != nil {
		t.Fatalf("Failed to create config dir: %v", err)
	}

	configPath := filepath.Join(configDir, "config.json")
	if err := os.WriteFile(configPath, []byte("invalid json {"), 0o644); err != nil {
		t.Fatalf("Failed to write corrupted config: %v", err)
	}

	config, err := cmd.LoadConfig()
	if err == nil {
		t.Error("Expected error for corrupted config file")
	}

	if config == nil {
		t.Error("Config should not be nil even on error")
	}
}

func TestGetLastStartedDevice_Empty(t *testing.T) {
	_ = NewTestHelpers(t)

	device, err := cmd.GetLastStartedDevice()
	if err != nil {
		t.Errorf("Expected no error for empty config, got: %v", err)
	}

	if device != nil {
		t.Error("Expected nil device when none has been saved")
	}
}

func TestSaveAndGetLastStartedDevice(t *testing.T) {
	_ = NewTestHelpers(t)

	testDevice := CreateTestDevice("roundtrip-device")

	if err := cmd.SaveLastStartedDevice(testDevice); err != nil {
		t.Fatalf("Failed to save device: %v", err)
	}

	retrieved, err := cmd.GetLastStartedDevice()
	if err != nil {
		t.Fatalf("Failed to get last device: %v", err)
	}

	AssertDeviceEqual(t, testDevice, retrieved)
}

func TestConfigFilePermissions(t *testing.T) {
	_ = NewTestHelpers(t)

	if err := cmd.SaveLastStartedDevice(CreateTestDevice("perm-test")); err != nil {
		t.Fatalf("Failed to save device: %v", err)
	}

	path, err := cmd.GetConfigPath()
	if err != nil {
		t.Fatalf("Failed to get config path: %v", err)
	}
	info, err := os.Stat(path)
	if err != nil {
		t.Fatalf("Failed to stat config file: %v", err)
	}

	// Config file must be owner read/write only (0600).
	if info.Mode().Perm() != 0o600 {
		t.Errorf("Expected config file permissions 0600, got %o", info.Mode().Perm())
	}
}
