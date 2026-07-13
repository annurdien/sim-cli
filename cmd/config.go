package cmd

import (
	"encoding/json"
	"os"
	"path/filepath"
)

// Config holds the application's persistent configuration.
type Config struct {
	LastStartedDevice *Device `json:"lastStartedDevice,omitempty"`
	DefaultDevice     string  `json:"defaultDevice,omitempty"`
	OutputDir         string  `json:"outputDir,omitempty"`
	GifFps            int     `json:"gifFps,omitempty"`
	GifScale          int     `json:"gifScale,omitempty"`
	Theme             string  `json:"theme,omitempty"`
}

// GetConfigDir returns the path to the sim-cli configuration directory.
func GetConfigDir() string {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return os.TempDir()
	}

	return filepath.Join(homeDir, ".sim-cli")
}

// GetConfigPath returns the full path to the configuration file.
func GetConfigPath() string {
	return filepath.Join(GetConfigDir(), "config.json")
}

// LoadConfig reads and returns the current application configuration.
func LoadConfig() (*Config, error) {
	configPath := GetConfigPath()

	if err := os.MkdirAll(GetConfigDir(), 0o755); err != nil {
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

// SaveConfig writes the given configuration to disk with secure permissions.
func SaveConfig(config *Config) error {
	configPath := GetConfigPath()

	if err := os.MkdirAll(GetConfigDir(), 0o755); err != nil {
		return err
	}

	data, err := json.MarshalIndent(config, "", "  ")
	if err != nil {
		return err
	}

	return os.WriteFile(configPath, data, 0o600)
}

// SaveLastStartedDevice persists the last started device to the configuration file.
func SaveLastStartedDevice(device *Device) error {
	config, err := LoadConfig()
	if err != nil {
		config = &Config{}
	}

	config.LastStartedDevice = device

	return SaveConfig(config)
}

// GetLastStartedDevice retrieves the last started device from the configuration file.
func GetLastStartedDevice() (*Device, error) {
	config, err := LoadConfig()
	if err != nil {
		return nil, err
	}

	return config.LastStartedDevice, nil
}
