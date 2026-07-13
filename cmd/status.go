package cmd

import (
	"fmt"
	"strings"

	"github.com/spf13/cobra"
)

var statusCmd = &cobra.Command{
	Use:   "status",
	Short: "Show a dashboard of all running devices",
	Run: func(cmd *cobra.Command, args []string) {
		devices := append(GetIOSSimulators(), GetAndroidEmulators()...)
		var running []Device
		for _, d := range devices {
			if strings.EqualFold(d.State, StateBooted) || strings.EqualFold(d.State, "device") {
				running = append(running, d)
			}
		}

		if len(running) == 0 {
			fmt.Println("No running devices found.")
			return
		}

		var rows [][]string

		for _, d := range running {
			version := FormatRuntime(d.Runtime)

			// Simplify platform type
			platform := "Unknown"
			switch d.Type {
			case TypeIOSSimulator:
				platform = "iOS"
			case TypeAndroidEmulator:
				platform = "Android"
			}

			// Format state and platform with lipgloss
			state := FormatState(d.State)
			platformStyled := FormatPlatform(platform)

			// For Android, udid can be emulator-5554. For iOS, it's a long UUID.
			id := d.UDID
			if len(id) > 20 {
				id = id[:8] + "..." + id[len(id)-4:]
			}

			rows = append(rows, []string{d.Name, platformStyled, version, state, id})
		}

		headers := []string{"Name", "Platform", "OS Version", "State", "ID"}
		RenderTable(headers, rows)
	},
}
