package tests

import (
	"runtime"
	"testing"

	"github.com/annurdien/sim-cli/cmd"
)

func TestCreateCommand_iOS(t *testing.T) {
	if runtime.GOOS != "darwin" {
		t.Skip("iOS tests require macOS")
	}

	_ = NewTestHelpers(t)

	var recorded [][]string
	exec := &recordingExecutor{
		onOutput: func(name string, args []string) ([]byte, error) {
			recorded = append(recorded, append([]string{name}, args...))
			return []byte{}, nil
		},
		onRun: func(name string, args []string) error {
			recorded = append(recorded, append([]string{name}, args...))
			return nil
		},
	}
	cmd.SetExecutor(exec)
	t.Cleanup(func() { cmd.SetExecutor(&cmd.OSCommandExecutor{}) })

	err := cmd.CreateIOSDevice("Test Phone", "iPhone-15", "iOS-17-0")
	if err != nil {
		t.Fatalf("CreateIOSDevice failed: %v", err)
	}

	found := false
	for _, call := range recorded {
		if call[0] == "xcrun" && len(call) >= 6 && call[2] == "create" {
			found = true
			if call[3] != "Test Phone" || call[4] != "iPhone-15" || call[5] != "iOS-17-0" {
				t.Errorf("args mismatch: %v", call)
			}
		}
	}

	if !found {
		t.Errorf("xcrun simctl create was not called")
	}
}

// Android uses exec.Command directly for 'sh -c echo no | avdmanager', so we can't easily mock it with packageExecutor without refactoring.
// Skip Android create test for now.
