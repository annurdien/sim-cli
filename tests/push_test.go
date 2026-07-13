package tests

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/annurdien/sim-cli/cmd"
)

func TestPushCommand_iOS(t *testing.T) {
	if runtime.GOOS != "darwin" {
		t.Skip("iOS push is only supported on macOS")
	}

	_ = NewTestHelpers(t)

	const (
		udid     = "TEST-UDID"
		bundleID = "com.example.app"
	)

	// Create a valid JSON payload file
	payloadPath := filepath.Join(t.TempDir(), "payload.json")
	err := os.WriteFile(payloadPath, []byte(`{"aps":{"alert":"test"}}`), 0o644)
	if err != nil {
		t.Fatalf("failed to create temp payload: %v", err)
	}

	var recorded [][]string
	exec := &recordingExecutor{
		onOutput: func(name string, args []string) ([]byte, error) {
			recorded = append(recorded, append([]string{name}, args...))
			if name == "xcrun" && len(args) >= 2 && args[1] == "list" {
				return iosSimulatorJSON("iPhone 15", udid, "Booted"), nil
			}

			return []byte{}, nil
		},
		onRun: func(name string, args []string) error {
			recorded = append(recorded, append([]string{name}, args...))
			return nil
		},
	}
	cmd.SetExecutor(exec)
	t.Cleanup(func() { cmd.SetExecutor(&cmd.OSCommandExecutor{}) })

	err = cmd.SendPushNotification("", bundleID, payloadPath)
	if err != nil {
		t.Fatalf("SendPushNotification failed: %v", err)
	}

	foundPush := false
	for _, call := range recorded {
		if call[0] == "xcrun" && len(call) >= 3 && call[2] == "push" {
			foundPush = true
			if call[3] != udid {
				t.Errorf("expected udid %s, got %s", udid, call[3])
			}
			if call[4] != bundleID {
				t.Errorf("expected bundleID %s, got %s", bundleID, call[4])
			}
			if !strings.Contains(call[5], "payload.json") {
				t.Errorf("expected payload path in args, got %s", call[5])
			}
		}
	}

	if !foundPush {
		t.Errorf("xcrun simctl push was not called")
	}
}

func TestPushCommand_InvalidJSON(t *testing.T) {
	if runtime.GOOS != "darwin" {
		t.Skip("iOS push is only supported on macOS")
	}

	_ = NewTestHelpers(t)

	payloadPath := filepath.Join(t.TempDir(), "invalid.json")
	err := os.WriteFile(payloadPath, []byte(`{"aps": "alert": "test"}}`), 0o644)
	if err != nil {
		t.Fatalf("failed to create temp payload: %v", err)
	}

	err = cmd.SendPushNotification("", "com.example.app", payloadPath)
	if err == nil || !strings.Contains(err.Error(), "invalid JSON") {
		t.Fatalf("expected invalid JSON error, got %v", err)
	}
}

func TestPushCommand_Android(t *testing.T) {
	if runtime.GOOS != "darwin" {
		t.Skip("iOS push checks runtime first, skip on linux/windows")
	}

	_ = NewTestHelpers(t)

	const (
		emulatorSerial = "emulator-5554"
		bundleID       = "com.example.app"
	)

	payloadPath := filepath.Join(t.TempDir(), "payload.json")
	_ = os.WriteFile(payloadPath, []byte(`{"aps":{"alert":"test"}}`), 0o644)

	adbDevicesOutput := "List of devices attached\n" + emulatorSerial + "\tdevice\n"
	adbNameOutput := "Pixel_7\nOK\n"

	exec := &recordingExecutor{
		onOutput: func(name string, args []string) ([]byte, error) {
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

	err := cmd.SendPushNotification(emulatorSerial, bundleID, payloadPath)
	if err == nil || !strings.Contains(err.Error(), "not supported for Android") {
		t.Fatalf("expected Android not supported error, got %v", err)
	}
}
