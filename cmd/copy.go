package cmd

import (
	"fmt"
	"path/filepath"
	"runtime"

	"github.com/spf13/cobra"
)

var copyCmd = &cobra.Command{
	Use:   "copy",
	Short: "Copy files to or from a device",
	Long: `Copy files to or from a device.
For iOS, 'copy to' adds media to the Photos app. 'copy from' is currently not supported for iOS.
For Android, 'copy to' pushes to /sdcard/Download/ and 'copy from' pulls from the specified path.`,
}

var copyToCmd = &cobra.Command{
	Use:   "to [device-name-or-udid] <local-path>",
	Short: "Copy a file to a device",
	Args:  cobra.RangeArgs(1, 2),
	RunE: func(cmd *cobra.Command, args []string) error {
		var deviceID, localPath string
		if len(args) == 1 {
			localPath = args[0]
		} else {
			deviceID = args[0]
			localPath = args[1]
		}

		udid, name, isAndroid, err := FindRunningDevice(deviceID)
		if err != nil {
			return err
		}

		absPath, err := filepath.Abs(localPath)
		if err != nil {
			return fmt.Errorf("invalid local path: %w", err)
		}

		fmt.Printf("Copying %s to '%s'...\n", filepath.Base(absPath), name)

		if isAndroid {
			if err := packageExecutor.Run(CmdAdb, "-s", udid, "push", absPath, "/sdcard/Download/"); err != nil {
				return fmt.Errorf("failed to copy to Android: %w", err)
			}
			fmt.Println("File copied successfully to /sdcard/Download/")
		} else {
			if runtime.GOOS != DarwinOS {
				return ErrIOSMacOnly
			}
			if err := packageExecutor.Run(CmdXCrun, CmdSimctl, "addmedia", udid, absPath); err != nil {
				return fmt.Errorf("failed to add media to iOS simulator: %w", err)
			}
			fmt.Println("Media added successfully to Photos.")
		}

		return nil
	},
}

var copyFromCmd = &cobra.Command{
	Use:   "from [device-name-or-udid] <remote-path> [local-path]",
	Short: "Copy a file from a device (Android only)",
	Args:  cobra.RangeArgs(1, 3),
	RunE: func(cmd *cobra.Command, args []string) error {
		var deviceID, remotePath, localPath string

		// Parse args depending on length
		switch len(args) {
		case 1:
			remotePath = args[0]
			localPath = "."
		case 2:
			// If it matches a device, arg0 is device, arg1 is remote. Else arg0 is remote, arg1 is local.
			// Try to find device with arg0
			_, _, _, err := FindRunningDevice(args[0])
			if err == nil {
				deviceID = args[0]
				remotePath = args[1]
				localPath = "."
			} else {
				remotePath = args[0]
				localPath = args[1]
			}
		case 3:
			deviceID = args[0]
			remotePath = args[1]
			localPath = args[2]
		}

		udid, name, isAndroid, err := FindRunningDevice(deviceID)
		if err != nil {
			return err
		}

		if !isAndroid {
			return fmt.Errorf("copy from is not supported for iOS simulators") //nolint:err113
		}

		fmt.Printf("Copying %s from '%s'...\n", remotePath, name)

		if err := packageExecutor.Run(CmdAdb, "-s", udid, "pull", remotePath, localPath); err != nil {
			return fmt.Errorf("failed to pull from Android: %w", err)
		}

		fmt.Println("File copied successfully.")

		return nil
	},
}

func init() {
	copyCmd.AddCommand(copyToCmd)
	copyCmd.AddCommand(copyFromCmd)
}
