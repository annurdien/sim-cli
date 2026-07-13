package tests

import (
	"runtime"
	"testing"

	"github.com/annurdien/sim-cli/cmd"
)

func TestPairCommand(t *testing.T) {
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

	// Using the root command to execute the pair command with args
	cmd.GetRootCmd().SetArgs([]string{"pair", "watch-udid", "phone-udid"})
	err := cmd.GetRootCmd().Execute()
	if err != nil {
		t.Fatalf("pair failed: %v", err)
	}

	found := false
	for _, call := range recorded {
		if call[0] == "xcrun" && len(call) >= 5 && call[2] == "pair" {
			found = true
			if call[3] != "watch-udid" || call[4] != "phone-udid" {
				t.Errorf("args mismatch: %v", call)
			}
		}
	}

	if !found {
		t.Errorf("xcrun simctl pair was not called")
	}
}
