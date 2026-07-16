package cmd

import (
	"errors"
	"fmt"
	"path/filepath"
	"runtime"
	"strings"

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

		if isAndroid {
			err = RunSpinner(fmt.Sprintf("Copying %s to '%s'...", filepath.Base(absPath), name), func() error {
				if pushErr := packageExecutor.Run(CmdAdb, "-s", udid, "push", absPath, "/sdcard/Download/"); pushErr != nil {
					return fmt.Errorf("failed to copy to Android: %w", pushErr)
				}

				return nil
			})
			if err == nil {
				PrintSuccess("File copied successfully to /sdcard/Download/")
			}
		} else {
			if runtime.GOOS != DarwinOS {
				return ErrIOSMacOnly
			}
			err = RunSpinner(fmt.Sprintf("Copying %s to '%s'...", filepath.Base(absPath), name), func() error {
				if addErr := packageExecutor.Run(CmdXCrun, CmdSimctl, "addmedia", udid, absPath); addErr != nil {
					return fmt.Errorf("failed to add media to iOS simulator: %w", addErr)
				}

				return nil
			})
			if err == nil {
				PrintSuccess("Media added successfully to Photos.")
			}
		}

		return err
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
			if strings.Contains(args[0], "/") || strings.Contains(args[0], "\\") {
				// arg0 looks like a path (remote or local), so arg0=remote, arg1=local
				remotePath = args[0]
				localPath = args[1]
			} else {
				// arg0 might be a device
				_, _, _, err := FindRunningDevice(args[0])
				if err == nil {
					deviceID = args[0]
					remotePath = args[1]
					localPath = "."
				} else if !errors.Is(err, ErrDeviceNotRunning) && !errors.Is(err, ErrDeviceNotFound) && !errors.Is(err, ErrNoActiveDevice) {
					// Transient error during device lookup
					return err
				} else {
					// Fallback to path
					remotePath = args[0]
					localPath = args[1]
				}
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

		err = RunSpinner(fmt.Sprintf("Copying %s from '%s'...", remotePath, name), func() error {
			if pullErr := packageExecutor.Run(CmdAdb, "-s", udid, "pull", remotePath, localPath); pullErr != nil {
				return fmt.Errorf("failed to pull from Android: %w", pullErr)
			}

			return nil
		})

		if err == nil {
			PrintSuccess("File copied successfully.")
		}

		return err
	},
}

func init() {
	copyCmd.AddCommand(copyToCmd)
	copyCmd.AddCommand(copyFromCmd)
}
