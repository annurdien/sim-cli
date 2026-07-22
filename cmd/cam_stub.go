//go:build !darwin

package cmd

import (
	"fmt"

	"github.com/spf13/cobra"
)

var camCmd = &cobra.Command{
	Use:   "cam",
	Short: "iOS Simulator camera injector (macOS only)",
	RunE: func(cmd *cobra.Command, args []string) error {
		return fmt.Errorf("sim cam is only supported on macOS")
	},
}
