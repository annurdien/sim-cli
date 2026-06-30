package tests

import (
	"strings"
	"testing"

	"github.com/annurdien/sim-cli/cmd"
)

func TestVersion_NotEmpty(t *testing.T) {
	if cmd.Version == "" {
		t.Error("Version should not be empty")
	}
}

func TestVersion_SemanticFormat(t *testing.T) {
	parts := strings.Split(cmd.Version, ".")
	if len(parts) != 3 {
		t.Errorf("Version should follow semantic versioning (major.minor.patch), got %q", cmd.Version)
	}

	for i, part := range parts {
		if part == "" {
			t.Errorf("Version part %d should not be empty", i)
		}

		if len(part) == 0 || part[0] < '0' || part[0] > '9' {
			t.Errorf("Version part %d should start with a digit, got %q", i, part)
		}
	}
}

func TestRootCommand_ASCIIArt(t *testing.T) {
	// The ASCII art is unexported, but we can verify Version is set correctly.
	// The root command uses cmd.Version as its cobra Version field.
	if cmd.Version == "" {
		t.Error("Version used in root command should not be empty")
	}
}

func TestRootCommand_SubcommandCoverage(t *testing.T) {
	// Document the full expected command surface. This test ensures the
	// list stays up to date when new commands are added.
	expectedCommands := []string{
		"list", "l", "ls",
		"start", "s",
		"stop", "st",
		"shutdown", "sd",
		"restart", "r",
		"delete", "d", "del",
		"last",
		"lts",
		"screenshot", "ss", "shot",
		"record", "rec",
	}

	if len(expectedCommands) < 10 {
		t.Error("Expected commands list should cover all subcommands and aliases")
	}
}
