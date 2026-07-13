package cmd

import (
	"encoding/json"
	"fmt"
	"strconv"
	"strings"

	"github.com/spf13/cobra"
)

var configCmd = &cobra.Command{
	Use:   "config",
	Short: "Manage sim-cli configuration",
	Long:  `View or modify the sim-cli configuration settings (e.g. outputDir, gifFps).`,
}

var configShowCmd = &cobra.Command{
	Use:   "show",
	Short: "Show current configuration",
	RunE: func(cmd *cobra.Command, args []string) error {
		config, err := LoadConfig()
		if err != nil {
			return err
		}

		data, err := json.MarshalIndent(config, "", "  ")
		if err != nil {
			return err
		}

		PrintInfo(string(data))

		return nil
	},
}

var configSetCmd = &cobra.Command{
	Use:   "set <key> <value>",
	Short: "Set a configuration value",
	Long: `Set a configuration value. Supported keys:
- defaultDevice (string)
- outputDir (string)
- gifFps (int)
- gifScale (int)`,
	Args: cobra.ExactArgs(2),
	RunE: func(cmd *cobra.Command, args []string) error {
		key := args[0]
		value := args[1]

		config, err := LoadConfig()
		if err != nil {
			config = &Config{}
		}

		switch strings.ToLower(key) {
		case "defaultdevice":
			config.DefaultDevice = value
		case "outputdir":
			config.OutputDir = value
		case "giffps":
			fps, err := strconv.Atoi(value)
			if err != nil {
				return fmt.Errorf("invalid integer value for gifFps: %s", value) //nolint:err113
			}
			config.GifFps = fps
		case "gifscale":
			scale, err := strconv.Atoi(value)
			if err != nil {
				return fmt.Errorf("invalid integer value for gifScale: %s", value) //nolint:err113
			}
			config.GifScale = scale
		default:
			return fmt.Errorf("unknown configuration key: %s", key) //nolint:err113
		}

		if err := SaveConfig(config); err != nil {
			return err
		}

		PrintSuccess(fmt.Sprintf("Set %s = %s", key, value))

		return nil
	},
}

var configResetCmd = &cobra.Command{
	Use:   "reset",
	Short: "Reset configuration to defaults",
	RunE: func(cmd *cobra.Command, args []string) error {
		fmt.Print("Are you sure you want to reset all configuration settings? [y/N]: ")
		var confirm string
		_, _ = fmt.Scanln(&confirm)
		if strings.ToLower(strings.TrimSpace(confirm)) != "y" {
			PrintInfo("Reset cancelled.")
			return nil
		}

		if err := SaveConfig(&Config{}); err != nil {
			return err
		}

		PrintSuccess("Configuration reset to defaults.")

		return nil
	},
}

var configPathCmd = &cobra.Command{
	Use:   "path",
	Short: "Print the path to the configuration file",
	RunE: func(cmd *cobra.Command, args []string) error {
		PrintInfo(GetConfigPath())

		return nil
	},
}

func init() {
	configCmd.AddCommand(configShowCmd)
	configCmd.AddCommand(configSetCmd)
	configCmd.AddCommand(configResetCmd)
	configCmd.AddCommand(configPathCmd)
}
