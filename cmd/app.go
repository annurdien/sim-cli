package cmd

import (
	"fmt"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/spf13/cobra"
)

var installCmd = &cobra.Command{
	Use:     "install [device-name-or-udid] <path-to-app>",
	Aliases: []string{"i"},
	Short:   "Install an app on a device",
	Long: `Install an app (.apk for Android, .app or .ipa for iOS) on a running iOS simulator or Android emulator.
	
If no device is specified, the first booted device is used automatically based on the app file extension.`,
	ValidArgsFunction: validDeviceAndFileArgs,
	Args:              cobra.RangeArgs(1, 2),
	RunE: func(cmd *cobra.Command, args []string) error {
		var deviceID, appPath string

		if len(args) == 1 {
			appPath = args[0]
			deviceID = ""
		} else {
			deviceID = args[0]
			appPath = args[1]
		}

		return InstallApp(deviceID, appPath)
	},
}

var uninstallCmd = &cobra.Command{
	Use:     "uninstall [device-name-or-udid] <bundle-id-or-package>",
	Aliases: []string{"u", "remove"},
	Short:   "Uninstall an app from a device",
	Long: `Uninstall an app from a running iOS simulator or Android emulator by its bundle ID (iOS) or package name (Android).

If no device is specified, the first booted device is used automatically.`,
	ValidArgsFunction: validDeviceArgs,
	Args:              cobra.RangeArgs(1, 2),
	RunE: func(cmd *cobra.Command, args []string) error {
		var deviceID, appID string

		if len(args) == 1 {
			appID = args[0]
			deviceID = ""
		} else {
			deviceID = args[0]
			appID = args[1]
		}

		return UninstallApp(deviceID, appID)
	},
}

func findDeviceForAppInstall(deviceID, ext string) (udid, name string, isAndroid bool, err error) {
	switch ext {
	case ExtAPK:
		u, n := FindRunningAndroidEmulator(deviceID)
		if u == "" {
			if deviceID == "" {
				return "", "", true, ErrAndroidEmulatorNotRunning
			}

			return "", "", true, fmt.Errorf("device %q: %w", deviceID, ErrAndroidEmulatorNotRunning)
		}

		return u, n, true, nil

	case ExtApp, ExtIPA:
		if runtime.GOOS != DarwinOS {
			return "", "", false, ErrIOSMacOnly
		}

		u, n, isA, err := FindRunningDevice(deviceID)
		if err != nil {
			return "", "", false, err
		}
		if isA {
			return "", "", false, ErrIOSSimulatorNotRunning
		}

		return u, n, false, nil

	default:
		return "", "", false, fmt.Errorf("%w: %s", ErrUnsupportedAppFormat, ext)
	}
}

func InstallApp(deviceID, appPath string) error {
	ext := strings.ToLower(filepath.Ext(appPath))

	udid, name, isAndroid, err := findDeviceForAppInstall(deviceID, ext)
	if err != nil {
		return err
	}

	if isAndroid {
		err = RunSpinner(fmt.Sprintf("Installing %s on Android emulator '%s'...", filepath.Base(appPath), name), func() error {
			if errExec := packageExecutor.Run(CmdAdb, "-s", udid, "install", appPath); errExec != nil {
				return fmt.Errorf("%w on Android emulator: %w", ErrInstallFailed, errExec)
			}

			return nil
		})
	} else {
		err = RunSpinner(fmt.Sprintf("Installing %s on iOS simulator '%s'...", filepath.Base(appPath), name), func() error {
			if errExec := packageExecutor.Run(CmdXCrun, CmdSimctl, "install", udid, appPath); errExec != nil {
				return fmt.Errorf("%w on iOS simulator: %w", ErrInstallFailed, errExec)
			}

			return nil
		})
	}

	if err == nil {
		PrintSuccess(fmt.Sprintf("App successfully installed on '%s'.", name))
	}

	return err
}

func findDeviceForUninstall(deviceID string) (udid, name string, isAndroid bool, err error) {
	return FindRunningDevice(deviceID)
}

func UninstallApp(deviceID, appID string) error {
	udid, name, isAndroid, err := findDeviceForUninstall(deviceID)
	if err != nil {
		return err
	}

	if isAndroid {
		err = RunSpinner(fmt.Sprintf("Uninstalling %s from Android emulator '%s'...", appID, name), func() error {
			if errExec := packageExecutor.Run(CmdAdb, "-s", udid, "uninstall", appID); errExec != nil {
				return fmt.Errorf("%w on Android emulator: %w", ErrUninstallFailed, errExec)
			}

			return nil
		})
	} else {
		err = RunSpinner(fmt.Sprintf("Uninstalling %s from iOS simulator '%s'...", appID, name), func() error {
			if errExec := packageExecutor.Run(CmdXCrun, CmdSimctl, "uninstall", udid, appID); errExec != nil {
				return fmt.Errorf("%w on iOS simulator: %w", ErrUninstallFailed, errExec)
			}

			return nil
		})
	}

	if err == nil {
		PrintSuccess(fmt.Sprintf("App '%s' successfully uninstalled from '%s'.", appID, name))
	}

	return err
}
