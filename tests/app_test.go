package tests

import (
	"runtime"
	"strings"
	"testing"

	"github.com/annurdien/sim-cli/cmd"
)

// TestAppInstall_iOS verifies install command generation for iOS.
func TestAppInstall_iOS(t *testing.T) {
	if runtime.GOOS != "darwin" {
		t.Skip("iOS app install only on macOS")
	}

	_ = NewTestHelpers(t)

	const (
		udid    = "TEST-UDID"
		appPath = "my_app.app"
	)

	var recorded [][]string
	exec := &recordingExecutor{
		onOutput: func(name string, args []string) ([]byte, error) {
			recorded = append(recorded, append([]string{name}, args...))
			if name == "xcrun" && len(args) >= 2 && args[1] == "list" {
				return iosSimulatorJSON("iPhone 15", udid, "Booted"), nil
			}

			return []byte{}, nil
		},
	}
	cmd.SetExecutor(exec)
	t.Cleanup(func() { cmd.SetExecutor(&cmd.OSCommandExecutor{}) })

	err := cmd.InstallApp("", appPath)
	if err != nil {
		t.Fatalf("InstallApp failed: %v", err)
	}

	foundInstall := false
	for _, call := range recorded {
		if call[0] == "xcrun" && len(call) >= 3 && call[2] == "install" {
			foundInstall = true
			if call[3] != udid {
				t.Errorf("expected udid %s, got %s", udid, call[3])
			}
			if call[4] != appPath {
				t.Errorf("expected appPath %s, got %s", appPath, call[4])
			}
		}
	}

	if !foundInstall {
		t.Errorf("xcrun simctl install was not called")
	}
}

// TestAppInstall_Android verifies install command generation for Android.
func TestAppInstall_Android(t *testing.T) {
	_ = NewTestHelpers(t)

	const (
		emulatorSerial = "emulator-5554"
		appPath        = "my_app.apk"
	)

	adbDevicesOutput := "List of devices attached\n" + emulatorSerial + "\tdevice\n"
	adbNameOutput := "Pixel_7\nOK\n"

	var recorded [][]string
	exec := &recordingExecutor{
		onOutput: func(name string, args []string) ([]byte, error) {
			recorded = append(recorded, append([]string{name}, args...))
			if name == "adb" {
				joined := strings.Join(args, " ")
				switch {
				case joined == "devices":
					return []byte(adbDevicesOutput), nil
				case strings.Contains(joined, "avd name"):
					return []byte(adbNameOutput), nil
				}
			}

			return []byte{}, nil
		},
	}
	cmd.SetExecutor(exec)
	t.Cleanup(func() { cmd.SetExecutor(&cmd.OSCommandExecutor{}) })

	err := cmd.InstallApp("", appPath)
	if err != nil {
		t.Fatalf("InstallApp failed: %v", err)
	}

	foundInstall := false
	for _, call := range recorded {
		if call[0] == "adb" && len(call) >= 3 && call[3] == "install" {
			foundInstall = true
			if call[2] != emulatorSerial {
				t.Errorf("expected serial %s, got %s", emulatorSerial, call[2])
			}
			if call[4] != appPath {
				t.Errorf("expected appPath %s, got %s", appPath, call[4])
			}
		}
	}

	if !foundInstall {
		t.Errorf("adb install was not called")
	}
}

// TestAppUninstall_Android verifies uninstall command generation for Android.
func TestAppUninstall_Android(t *testing.T) {
	_ = NewTestHelpers(t)

	const (
		emulatorSerial = "emulator-5554"
		appID          = "com.example.app"
	)

	adbDevicesOutput := "List of devices attached\n" + emulatorSerial + "\tdevice\n"
	adbNameOutput := "Pixel_7\nOK\n"

	var recorded [][]string
	exec := &recordingExecutor{
		onOutput: func(name string, args []string) ([]byte, error) {
			recorded = append(recorded, append([]string{name}, args...))
			if name == "adb" {
				joined := strings.Join(args, " ")
				switch {
				case joined == "devices":
					return []byte(adbDevicesOutput), nil
				case strings.Contains(joined, "avd name"):
					return []byte(adbNameOutput), nil
				}
			}

			return []byte{}, nil
		},
	}
	cmd.SetExecutor(exec)
	t.Cleanup(func() { cmd.SetExecutor(&cmd.OSCommandExecutor{}) })

	// Pass the specific device ID to force Android uninstall flow in tests
	// (otherwise macOS prioritizes iOS if there's no booted simulator, which might fail).
	err := cmd.UninstallApp(emulatorSerial, appID)
	if err != nil {
		t.Fatalf("UninstallApp failed: %v", err)
	}

	foundUninstall := false
	for _, call := range recorded {
		if call[0] == "adb" && len(call) >= 3 && call[3] == "uninstall" {
			foundUninstall = true
			if call[2] != emulatorSerial {
				t.Errorf("expected serial %s, got %s", emulatorSerial, call[2])
			}
			if call[4] != appID {
				t.Errorf("expected appID %s, got %s", appID, call[4])
			}
		}
	}

	if !foundUninstall {
		t.Errorf("adb uninstall was not called")
	}
}

// TestAppInstall_UnsupportedFormat verifies unsupported extension behavior.
func TestAppInstall_UnsupportedFormat(t *testing.T) {
	_ = NewTestHelpers(t)

	err := cmd.InstallApp("", "my_app.txt")
	if err == nil {
		t.Fatalf("expected error for unsupported app format")
	}
	if !strings.Contains(err.Error(), cmd.ErrUnsupportedAppFormat.Error()) {
		t.Errorf("expected ErrUnsupportedAppFormat, got %v", err)
	}
}
