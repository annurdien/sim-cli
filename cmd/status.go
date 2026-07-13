package cmd

import (
	"fmt"
	"os"
	"strings"
	"text/tabwriter"

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

		w := tabwriter.NewWriter(os.Stdout, 0, 0, 4, ' ', 0)
		fmt.Fprintln(w, "NAME\tPLATFORM\tOS VERSION\tSTATE\tID")

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

			// For Android, udid can be emulator-5554. For iOS, it's a long UUID.
			id := d.UDID
			if len(id) > 20 {
				id = id[:8] + "..." + id[len(id)-4:]
			}

			fmt.Fprintf(w, "%s\t%s\t%s\t%s\t%s\n", d.Name, platform, version, d.State, id)
		}

		_ = w.Flush()
	},
}
