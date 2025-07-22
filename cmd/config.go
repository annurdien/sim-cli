package cmd

import (
	"encoding/json"
	"os"
	"path/filepath"
)

type Config struct {
	LastStartedDevice *Device `json:"lastStartedDevice,omitempty"`
}

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

func getLastStartedDevice() (*Device, error) {
	config, err := loadConfig()
	if err != nil {
		return nil, err
	}

	return config.LastStartedDevice, nil
}
