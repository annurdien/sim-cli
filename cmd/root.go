package cmd

import (
	"fmt"

	"github.com/spf13/cobra"
)

const version = "1.0.0"

var rootCmd = &cobra.Command{
	Use:     "sim",
	Version: version,
	Short:   "CLI tool to manage iOS simulators and Android emulators",
	Long: `SIM-CLI is a command-line tool for managing iOS simulators and Android emulators.
	
It provides a simple interface to:
- List available simulators and emulators
- Start, stop, shutdown, and restart devices
- Take screenshots and record screen
- Manage device lifecycle efficiently`,
	Run: func(cmd *cobra.Command, args []string) {
		version, _ := cmd.Flags().GetBool("version")
		if version {
			fmt.Printf("SIM-CLI version %s\n", cmd.Version)
			return
		}
		fmt.Println("SIM-CLI - iOS Simulator & Android Emulator Manager")
		fmt.Printf("Version: %s\n", cmd.Version)
		fmt.Println("Use 'sim help' to see available commands")
	},
}

func Execute() error {
	return rootCmd.Execute()
}

func init() {
	rootCmd.Flags().BoolP("version", "v", false, "Show version information")
}
