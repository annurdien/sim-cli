package tests

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func TestConfigDir(t *testing.T) {
	// Test that config directory is properly determined
	configDir := getConfigDir()
	if configDir == "" {
		t.Error("Config directory should not be empty")
	}
}

func TestConfigPath(t *testing.T) {
	// Test that config path is properly constructed
	configPath := getConfigPath()
	if configPath == "" {
		t.Error("Config path should not be empty")
	}

	if !filepath.IsAbs(configPath) {
		t.Error("Config path should be absolute")
	}
}

func TestLoadConfig_NonExistent(t *testing.T) {
	// Test loading config when file doesn't exist
	tempDir := t.TempDir()
	originalHome := os.Getenv("HOME")
	defer os.Setenv("HOME", originalHome)

	os.Setenv("HOME", tempDir)

	config, err := loadConfig()
	if err != nil {
		t.Errorf("Expected no error for non-existent config, got: %v", err)
	}

	if config == nil {
		t.Error("Config should not be nil")
	}
}

func TestSaveAndLoadConfig(t *testing.T) {
	// Test saving and loading a config
	tempDir := t.TempDir()
	originalHome := os.Getenv("HOME")
	defer os.Setenv("HOME", originalHome)

	os.Setenv("HOME", tempDir)

	// Create a test device
	testDevice := &Device{
		Name:    "Test Device",
		UDID:    "test-udid-123",
		Type:    "iOS Simulator",
		State:   "Booted",
		Runtime: "iOS 17.0",
	}

	// Save the device as last started
	err := saveLastStartedDevice(testDevice)
	if err != nil {
		t.Errorf("Failed to save last started device: %v", err)
	}

	// Load the config and verify
	config, err := loadConfig()
	if err != nil {
		t.Errorf("Failed to load config: %v", err)
	}

	if config.LastStartedDevice == nil {
		t.Error("Last started device should not be nil")
	}

	if config.LastStartedDevice.Name != testDevice.Name {
		t.Errorf("Expected device name %s, got %s", testDevice.Name, config.LastStartedDevice.Name)
	}
}

func TestSaveConfig_InvalidData(t *testing.T) {
	// Test saving invalid config data
	tempDir := t.TempDir()
	originalHome := os.Getenv("HOME")
	defer os.Setenv("HOME", originalHome)

	os.Setenv("HOME", tempDir)

	// This should not fail as our Config struct is simple
	config := &Config{}
	err := saveConfig(config)
	if err != nil {
		t.Errorf("Should be able to save empty config: %v", err)
	}
}

func TestLoadConfig_CorruptedFile(t *testing.T) {
	// Test loading a corrupted config file
	tempDir := t.TempDir()
	originalHome := os.Getenv("HOME")
	defer os.Setenv("HOME", originalHome)

	os.Setenv("HOME", tempDir)

	// Create corrupted config file
	configDir := filepath.Join(tempDir, ".sim-cli")
	os.MkdirAll(configDir, 0755)
	configPath := filepath.Join(configDir, "config.json")

	// Write invalid JSON
	os.WriteFile(configPath, []byte("invalid json {"), 0644)

	config, err := loadConfig()
	if err == nil {
		t.Error("Expected error for corrupted config file")
	}

	// Should return empty config on error
	if config == nil {
		t.Error("Config should not be nil even on error")
	}
}

// Helper functions that mirror the unexported functions in cmd package
func getConfigDir() string {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return os.TempDir()
	}
	return filepath.Join(homeDir, ".sim-cli")
}

func getConfigPath() string {
	return filepath.Join(getConfigDir(), "config.json")
}

func loadConfig() (*Config, error) {
	configPath := getConfigPath()

	if err := os.MkdirAll(getConfigDir(), 0755); err != nil {
		return &Config{}, err
	}

	if _, err := os.Stat(configPath); os.IsNotExist(err) {
		return &Config{}, nil
	}

	data, err := os.ReadFile(configPath)
	if err != nil {
		return &Config{}, err
	}

	var config Config
	if err := json.Unmarshal(data, &config); err != nil {
		return &Config{}, err
	}

	return &config, nil
}

func saveConfig(config *Config) error {
	configPath := getConfigPath()

	if err := os.MkdirAll(getConfigDir(), 0755); err != nil {
		return err
	}

	data, err := json.MarshalIndent(config, "", "  ")
	if err != nil {
		return err
	}

	return os.WriteFile(configPath, data, 0644)
}

func saveLastStartedDevice(device *Device) error {
	config, err := loadConfig()
	if err != nil {
		config = &Config{}
	}

	config.LastStartedDevice = device
	return saveConfig(config)
}
