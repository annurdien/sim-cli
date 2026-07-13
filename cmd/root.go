package cmd

import (
	"errors"
	"fmt"

	"github.com/charmbracelet/lipgloss"
	"github.com/spf13/cobra"
)

// Version is the current release version of SIM-CLI.
var Version = "dev"

var rootCmd = &cobra.Command{
	Use:           "sim",
	Version:       Version,
	SilenceErrors: true,
	PersistentPreRun: func(cmd *cobra.Command, args []string) {
		cmd.SilenceUsage = true
	},
	Short: "CLI tool to manage iOS simulators and Android emulators",
	Long: `SIM-CLI is a command-line tool for managing iOS simulators and Android emulators.
	
It provides a simple interface to:
- List available simulators and emulators
- Start, stop, shutdown, and restart devices
- Delete simulators and emulators
- Take screenshots and record screen
- Manage device lifecycle efficiently`,
	Run: func(cmd *cobra.Command, args []string) {
		showVersion, _ := cmd.Flags().GetBool("version")
		if showVersion {
			fmt.Printf("SIM-CLI version %s\n", cmd.Version)

			return
		}

		content := lipgloss.JoinVertical(lipgloss.Center,
			lipgloss.NewStyle().Foreground(lipgloss.Color("99")).Bold(true).Render("SIM-CLI"),
			lipgloss.NewStyle().Foreground(lipgloss.Color("212")).Render("iOS Simulator & Android Emulator Manager"),
			lipgloss.NewStyle().Foreground(lipgloss.Color("240")).Render(fmt.Sprintf("Version: %s", cmd.Version)),
			"",
			lipgloss.NewStyle().Foreground(lipgloss.Color("245")).Italic(true).Render("Use 'sim help' to see available commands"),
		)

		box := lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(lipgloss.Color("99")).
			Padding(1, 4).
			Margin(1, 0).
			Align(lipgloss.Center).
			Render(content)

		fmt.Println(box)
	},
}

// Execute is the entry point for the CLI.
func Execute() error {
	err := rootCmd.Execute()
	if err != nil {
		if errors.Is(err, ErrSelectionCancelled) {
			PrintInfo("Device selection cancelled.")
		} else {
			PrintError(err.Error())
		}
	}

	return err
}

func init() {
	rootCmd.Flags().BoolP("version", "v", false, "Show version information")

	rootCmd.AddCommand(listCmd)
	rootCmd.AddCommand(startCmd)
	rootCmd.AddCommand(stopCmd)
	rootCmd.AddCommand(shutdownCmd)
	rootCmd.AddCommand(completionCmd)
	rootCmd.AddCommand(installCmd)
	rootCmd.AddCommand(uninstallCmd)
	rootCmd.AddCommand(restartCmd)
	rootCmd.AddCommand(deleteCmd)
	rootCmd.AddCommand(eraseCmd)
	rootCmd.AddCommand(cloneCmd)
	rootCmd.AddCommand(lastCmd)
	rootCmd.AddCommand(ltsCmd)
	rootCmd.AddCommand(screenshotCmd)
	rootCmd.AddCommand(recordCmd)
	rootCmd.AddCommand(logsCmd)
	rootCmd.AddCommand(pushCmd)
	rootCmd.AddCommand(configCmd)
	rootCmd.AddCommand(createCmd)
	rootCmd.AddCommand(statusCmd)
	rootCmd.AddCommand(copyCmd)
	rootCmd.AddCommand(pairCmd)
	rootCmd.AddCommand(openCmd)
	rootCmd.AddCommand(doctorCmd)

	// deleteCmd flags
	deleteCmd.Flags().BoolP("force", "f", false, "Skip confirmation prompt")

	// eraseCmd flags
	eraseCmd.Flags().BoolP("force", "f", false, "Skip confirmation prompt")

	// startCmd flags
	startCmd.Flags().Bool("no-wait", false, "Skip waiting for Android emulator to fully boot (fire-and-forget)")

	// screenshotCmd flags
	screenshotCmd.Flags().BoolP("copy", "c", false, "Copy the screenshot to the clipboard")
	screenshotCmd.Flags().String("output-dir", "", "Directory to save the screenshot (default: current directory)")

	// recordCmd flags
	recordCmd.Flags().IntP("duration", "d", 0, "Duration of the recording in seconds (default: unlimited)")
	recordCmd.Flags().BoolP("gif", "g", false, "Convert the recording to a GIF")
	recordCmd.Flags().BoolP("copy", "c", false, "Copy the recording to the clipboard")
	recordCmd.Flags().String("output-dir", "", "Directory to save the recording (default: current directory)")
	recordCmd.Flags().Int("fps", 10, "Frame rate for GIF conversion (used with --gif)")
	recordCmd.Flags().Int("scale", 480, "Width in pixels for GIF conversion, -1 for auto height (used with --gif)")
}
