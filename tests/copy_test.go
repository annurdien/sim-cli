package tests

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/annurdien/sim-cli/cmd"
)

func TestCopyCommand_To_Android(t *testing.T) {
	_ = NewTestHelpers(t)

	const (
		emulatorSerial = "emulator-5554"
	)

	filePath := filepath.Join(t.TempDir(), "test.txt")
	_ = os.WriteFile(filePath, []byte("hello"), 0o644)

	adbDevicesOutput := "List of devices attached\n" + emulatorSerial + "\tdevice\n"
	adbNameOutput := "Pixel_7\nOK\n"

	var recorded [][]string
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
		onRun: func(name string, args []string) error {
			recorded = append(recorded, append([]string{name}, args...))
			return nil
		},
	}
	cmd.SetExecutor(exec)
	t.Cleanup(func() { cmd.SetExecutor(&cmd.OSCommandExecutor{}) })

	// Using the root command to execute the copy to subcommand
	cmd.GetRootCmd().SetArgs([]string{"copy", "to", emulatorSerial, filePath})
	err := cmd.GetRootCmd().Execute()
	if err != nil {
		t.Fatalf("copy to failed: %v", err)
	}

	found := false
	for _, call := range recorded {
		if call[0] == "adb" && len(call) >= 5 && call[3] == "push" {
			found = true
			if !strings.Contains(call[4], "test.txt") {
				t.Errorf("expected local path in args, got %s", call[4])
			}
			if call[5] != "/sdcard/Download/" {
				t.Errorf("expected remote path in args, got %s", call[5])
			}
		}
	}

	if !found {
		t.Errorf("adb push was not called")
	}
}

func TestCopyCommand_From_Android(t *testing.T) {
	_ = NewTestHelpers(t)

	const (
		emulatorSerial = "emulator-5554"
	)

	adbDevicesOutput := "List of devices attached\n" + emulatorSerial + "\tdevice\n"
	adbNameOutput := "Pixel_7\nOK\n"

	var recorded [][]string
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
		onRun: func(name string, args []string) error {
			recorded = append(recorded, append([]string{name}, args...))
			return nil
		},
	}
	cmd.SetExecutor(exec)
	t.Cleanup(func() { cmd.SetExecutor(&cmd.OSCommandExecutor{}) })

	// Using the root command to execute the copy from subcommand
	cmd.GetRootCmd().SetArgs([]string{"copy", "from", emulatorSerial, "/sdcard/Download/test.txt", "."})
	err := cmd.GetRootCmd().Execute()
	if err != nil {
		t.Fatalf("copy from failed: %v", err)
	}

	found := false
	for _, call := range recorded {
		if call[0] == "adb" && len(call) >= 5 && call[3] == "pull" {
			found = true
			if call[4] != "/sdcard/Download/test.txt" {
				t.Errorf("expected remote path in args, got %s", call[4])
			}
		}
	}

	if !found {
		t.Errorf("adb pull was not called")
	}
}
