package tests

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/annurdien/sim-cli/cmd"
)

// --- doctor command tests ---

// TestDoctor_CommandExists_True verifies that CommandExists returns true for
// a binary that is actually present in PATH (we use the test binary itself).
func TestDoctor_CommandExists_True(t *testing.T) {
	// "go" is always in PATH when running `go test`.
	if !cmd.CommandExists("go") {
		t.Error("CommandExists should return true for 'go' which is in PATH during testing")
	}
}

// TestDoctor_CommandExists_False verifies that CommandExists returns false for
// a binary that does not exist.
func TestDoctor_CommandExists_False(t *testing.T) {
	const fakeCmd = "sim-cli-fake-binary-that-does-not-exist-xyz123"
	if cmd.CommandExists(fakeCmd) {
		t.Errorf("CommandExists should return false for non-existent binary %q", fakeCmd)
	}
}

// TestDoctor_CommandExists_ScriptInTempDir verifies that a script placed in a
// temp dir and added to PATH is correctly detected.
func TestDoctor_CommandExists_ScriptInTempDir(t *testing.T) {
	h := NewTestHelpers(t)

	// Create a fake binary in a temp bin directory.
	binDir := filepath.Join(h.TempDir, "bin")
	if err := os.MkdirAll(binDir, 0o755); err != nil {
		t.Fatalf("failed to create bin dir: %v", err)
	}

	fakeBinaryPath := filepath.Join(binDir, "fake-sim-tool")
	if err := os.WriteFile(fakeBinaryPath, []byte("#!/bin/sh\nexit 0\n"), 0o755); err != nil {
		t.Fatalf("failed to create fake binary: %v", err)
	}

	// Prepend binDir to PATH.
	originalPath := os.Getenv("PATH")
	t.Setenv("PATH", binDir+":"+originalPath)

	if !cmd.CommandExists("fake-sim-tool") {
		t.Error("CommandExists should return true for binary in modified PATH")
	}
}

// TestDoctor_CommandExists_NotExecutable verifies that a file without execute
// permission is NOT detected as a valid command.
func TestDoctor_CommandExists_NotExecutable(t *testing.T) {
	h := NewTestHelpers(t)

	binDir := filepath.Join(h.TempDir, "bin")
	if err := os.MkdirAll(binDir, 0o755); err != nil {
		t.Fatalf("failed to create bin dir: %v", err)
	}

	// Write a non-executable file.
	nonExecPath := filepath.Join(binDir, "not-executable-tool")
	if err := os.WriteFile(nonExecPath, []byte("#!/bin/sh\n"), 0o644); err != nil {
		t.Fatalf("failed to create non-executable file: %v", err)
	}

	originalPath := os.Getenv("PATH")
	t.Setenv("PATH", binDir+":"+originalPath)

	// exec.LookPath checks executability; non-executable files are not found.
	if cmd.CommandExists("not-executable-tool") {
		t.Error("CommandExists should return false for non-executable file")
	}
}

// TestDoctor_RequiredConstants verifies that all dependency name constants
// used by the doctor command are exported with expected values.
func TestDoctor_RequiredConstants(t *testing.T) {
	tests := []struct {
		name  string
		value string
		want  string
	}{
		{name: "CmdXCrun", value: cmd.CmdXCrun, want: "xcrun"},
		{name: "CmdAdb", value: cmd.CmdAdb, want: "adb"},
		{name: "CmdEmulator", value: cmd.CmdEmulator, want: "emulator"},
		{name: "CmdFFmpeg", value: cmd.CmdFFmpeg, want: "ffmpeg"},
		{name: "CmdAvdManager", value: cmd.CmdAvdManager, want: "avdmanager"},
		{name: "CmdXclip", value: cmd.CmdXclip, want: "xclip"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if tc.value != tc.want {
				t.Errorf("%s = %q, want %q", tc.name, tc.value, tc.want)
			}
		})
	}
}

// TestDoctor_AllDependencyConstantsDefined verifies that the constants used
// to check dependencies in doctor.go are all non-empty.
func TestDoctor_AllDependencyConstantsDefined(t *testing.T) {
	constants := map[string]string{
		"CmdXCrun":    cmd.CmdXCrun,
		"CmdAdb":      cmd.CmdAdb,
		"CmdEmulator": cmd.CmdEmulator,
		"CmdFFmpeg":   cmd.CmdFFmpeg,
	}

	for name, value := range constants {
		if value == "" {
			t.Errorf("constant %s should not be empty", name)
		}
	}
}

// TestDoctor_CommandExists_EmptyString verifies that passing an empty string
// does not panic and returns false.
func TestDoctor_CommandExists_EmptyString(t *testing.T) {
	// Should not panic.
	result := cmd.CommandExists("")
	// exec.LookPath("") returns an error on all platforms.
	if result {
		t.Error("CommandExists(\"\") should return false")
	}
}
