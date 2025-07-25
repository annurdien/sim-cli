package cmd

import (
	"fmt"

	"github.com/spf13/cobra"
)

const version = "1.2.0"

const asciiArt = `
 ███████╗██╗███╗   ███╗      ██████╗██╗     ██╗
 ██╔════╝██║████╗ ████║     ██╔════╝██║     ██║
 ███████╗██║██╔████╔██║     ██║     ██║     ██║
 ╚════██║██║██║╚██╔╝██║     ██║     ██║     ██║
 ███████║██║██║ ╚═╝ ██║     ╚██████╗███████╗██║
 ╚══════╝╚═╝╚═╝     ╚═╝      ╚═════╝╚══════╝╚═╝
                                                    
`

var rootCmd = &cobra.Command{
	Use:     "sim",
	Version: version,
	Short:   "CLI tool to manage iOS simulators and Android emulators",
	Long: `SIM-CLI is a command-line tool for managing iOS simulators and Android emulators.
	
It provides a simple interface to:
- List available simulators and emulators
- Start, stop, shutdown, and restart devices
- Delete simulators and emulators
- Take screenshots and record screen
- Manage device lifecycle efficiently`,
	Run: func(cmd *cobra.Command, args []string) {
		version, _ := cmd.Flags().GetBool("version")
		if version {
			fmt.Printf("SIM-CLI version %s\n", cmd.Version)
			return
		}
		fmt.Print(asciiArt)
		fmt.Print("iOS Simulator & Android Emulator Manager\n")
		fmt.Printf("Version: %s\n\n", cmd.Version)
		fmt.Println("Use 'sim help' to see available commands")
	},
}

func Execute() error {
	return rootCmd.Execute()
}

func init() {
	rootCmd.Flags().BoolP("version", "v", false, "Show version information")

	rootCmd.AddCommand(listCmd)
	rootCmd.AddCommand(startCmd)
	rootCmd.AddCommand(stopCmd)
	rootCmd.AddCommand(shutdownCmd)
	rootCmd.AddCommand(restartCmd)
	rootCmd.AddCommand(deleteCmd)
	rootCmd.AddCommand(lastCmd)
	rootCmd.AddCommand(ltsCmd)
	rootCmd.AddCommand(screenshotCmd)
	rootCmd.AddCommand(recordCmd)

	// Screenshot flags
	screenshotCmd.Flags().BoolP("copy", "c", false, "Copy the screenshot to the clipboard")

	// Record flags
	recordCmd.Flags().IntP("duration", "d", 0, "Duration of the recording in seconds (default: unlimited)")
	recordCmd.Flags().BoolP("gif", "g", false, "Convert the recording to a GIF")
	recordCmd.Flags().BoolP("copy", "c", false, "Copy the recording to the clipboard")
}
