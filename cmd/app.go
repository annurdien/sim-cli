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
		if deviceID == "" {
			emu, errEmu := getRunningAndroidEmulator()
			if errEmu != nil {
				return "", "", true, errEmu
			}

			return emu.udid, emu.name, true, nil
		}

		u, n := FindRunningAndroidEmulator(deviceID)
		if u == "" {
			return "", "", true, fmt.Errorf("device %q: %w", deviceID, ErrAndroidEmulatorNotRunning)
		}

		return u, n, true, nil

	case ExtApp, ExtIPA:
		if runtime.GOOS != DarwinOS {
			return "", "", false, ErrIOSMacOnly
		}

		if deviceID == "" {
			sim, errSim := getRunningIOSSimulator()
			if errSim != nil {
				return "", "", false, errSim
			}

			return sim.udid, sim.name, false, nil
		}

		device := FindIOSSimulatorByID(deviceID)
		if device == nil || device.State != StateBooted {
			return "", "", false, fmt.Errorf("device %q: %w", deviceID, ErrIOSSimulatorNotRunning)
		}

		return device.UDID, device.Name, false, nil

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
		fmt.Printf("Installing %s on Android emulator '%s'...\n", filepath.Base(appPath), name)

		if output, err := packageExecutor.Output(CmdAdb, "-s", udid, "install", appPath); err != nil {
			return fmt.Errorf("%w on Android emulator: %w\nOutput: %s", ErrInstallFailed, err, string(output))
		}
	} else {
		fmt.Printf("Installing %s on iOS simulator '%s'...\n", filepath.Base(appPath), name)

		if output, err := packageExecutor.Output(CmdXCrun, CmdSimctl, "install", udid, appPath); err != nil {
			return fmt.Errorf("%w on iOS simulator: %w\nOutput: %s", ErrInstallFailed, err, string(output))
		}
	}

	fmt.Printf("App successfully installed on '%s'.\n", name)

	return nil
}

func findDeviceForUninstall(deviceID string) (udid, name string, isAndroid bool, err error) {
	if deviceID == "" {
		if runtime.GOOS == DarwinOS {
			if sim, errSim := getRunningIOSSimulator(); errSim == nil {
				return sim.udid, sim.name, false, nil
			}
		}

		if emu, errEmu := getRunningAndroidEmulator(); errEmu == nil {
			return emu.udid, emu.name, true, nil
		}

		return "", "", false, ErrNoActiveDevice
	}

	if runtime.GOOS == DarwinOS {
		device := FindIOSSimulatorByID(deviceID)
		if device != nil && device.State == StateBooted {
			return device.UDID, device.Name, false, nil
		}
	}

	u, n := FindRunningAndroidEmulator(deviceID)
	if u != "" {
		return u, n, true, nil
	}

	return "", "", false, fmt.Errorf("device %q: %w", deviceID, ErrDeviceNotRunning)
}

func UninstallApp(deviceID, appID string) error {
	udid, name, isAndroid, err := findDeviceForUninstall(deviceID)
	if err != nil {
		return err
	}

	if isAndroid {
		fmt.Printf("Uninstalling %s from Android emulator '%s'...\n", appID, name)

		if output, err := packageExecutor.Output(CmdAdb, "-s", udid, "uninstall", appID); err != nil {
			return fmt.Errorf("%w on Android emulator: %w\nOutput: %s", ErrUninstallFailed, err, string(output))
		}
	} else {
		fmt.Printf("Uninstalling %s from iOS simulator '%s'...\n", appID, name)

		if output, err := packageExecutor.Output(CmdXCrun, CmdSimctl, "uninstall", udid, appID); err != nil {
			return fmt.Errorf("%w on iOS simulator: %w\nOutput: %s", ErrUninstallFailed, err, string(output))
		}
	}

	fmt.Printf("App '%s' successfully uninstalled from '%s'.\n", appID, name)

	return nil
}
