package cmd

import (
	"fmt"
	"runtime"

	"github.com/spf13/cobra"
)

var pairCmd = &cobra.Command{
	Use:   "pair [watch-udid] [phone-udid]",
	Short: "Pair an Apple Watch simulator with an iPhone simulator",
	Long: `Pair an Apple Watch simulator with an iPhone simulator.
If arguments are not provided, an interactive prompt will allow you to select them.`,
	Args: cobra.MaximumNArgs(2),
	RunE: func(cmd *cobra.Command, args []string) error {
		if runtime.GOOS != DarwinOS {
			return ErrIOSMacOnly
		}

		var watchID, phoneID string

		if len(args) == 2 {
			watchID = args[0]
			phoneID = args[1]
		} else {
			fmt.Println("Select the Apple Watch Simulator:")
			selectedWatch, err := PromptDeviceSelector("all")
			if err != nil {
				return err
			}
			watchID = selectedWatch

			fmt.Println("Select the iPhone Simulator:")
			selectedPhone, err := PromptDeviceSelector("all")
			if err != nil {
				return err
			}
			phoneID = selectedPhone
		}

		fmt.Printf("Pairing Watch (%s) with Phone (%s)...\n", watchID, phoneID)

		if err := packageExecutor.Run(CmdXCrun, CmdSimctl, "pair", watchID, phoneID); err != nil {
			return fmt.Errorf("failed to pair devices: %w", err)
		}

		fmt.Println("Devices paired successfully.")

		return nil
	},
}
