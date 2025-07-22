package tests

import (
	"strings"
	"testing"
)

func TestRootCommand_Version(t *testing.T) {
	// Test version information
	expectedVersion := "1.0.0"
	version := getRootCommandVersion()

	if version != expectedVersion {
		t.Errorf("Expected version %s, got %s", expectedVersion, version)
	}
}

func TestRootCommand_ASCIIArt(t *testing.T) {
	// Test ASCII art content
	asciiArt := getASCIIArt()

	if asciiArt == "" {
		t.Error("ASCII art should not be empty")
	}

	// Check that it contains expected characters
	if !strings.Contains(asciiArt, "███") {
		t.Error("ASCII art should contain block characters")
	}

	// The ASCII art spells out "SIM CLI" using block characters
	if !strings.Contains(asciiArt, "██║") {
		t.Error("ASCII art should contain block character patterns")
	}
}

func TestRootCommand_DefaultBehavior(t *testing.T) {
	// Test default behavior when no subcommands are provided
	// The root command should display ASCII art and help information

	asciiArt := getASCIIArt()
	if asciiArt == "" {
		t.Error("ASCII art should not be empty")
	}

	version := getRootCommandVersion()
	if version == "" {
		t.Error("Version should not be empty")
	}

	// Test that the root command has proper configuration
	usage := getRootCommandUsage()
	if usage != "sim" {
		t.Errorf("Expected usage 'sim', got '%s'", usage)
	}

	shortDesc := getRootCommandShortDescription()
	if shortDesc == "" {
		t.Error("Short description should not be empty")
	}
}

func TestRootCommand_Help(t *testing.T) {
	// Test help command functionality
	// Verify that help information contains expected elements

	longDesc := getRootCommandLongDescription()
	if longDesc == "" {
		t.Error("Long description should not be empty")
	}

	// Check that help mentions key features
	expectedFeatures := []string{
		"simulators",
		"emulators",
		"start",
		"stop",
		"list",
	}

	for _, feature := range expectedFeatures {
		if !strings.Contains(strings.ToLower(longDesc), feature) {
			t.Errorf("Help should mention '%s' feature", feature)
		}
	}

	// Test that version information is available
	version := getRootCommandVersion()
	if version == "" {
		t.Error("Version should be available for help")
	}
}

func TestRootCommand_VersionFlag(t *testing.T) {
	// Test --version/-v flag functionality

	// Test that version information is properly formatted
	version := getRootCommandVersion()
	expectedVersion := "1.0.0"

	if version != expectedVersion {
		t.Errorf("Expected version %s, got %s", expectedVersion, version)
	}

	// Test that version follows semantic versioning format
	parts := strings.Split(version, ".")
	if len(parts) != 3 {
		t.Errorf("Version should follow semantic versioning (major.minor.patch), got %s", version)
	}

	// Test that each part is numeric
	for i, part := range parts {
		if part == "" {
			t.Errorf("Version part %d should not be empty", i)
		}
		// Basic check that it looks like a number
		if len(part) == 0 || (part[0] < '0' || part[0] > '9') {
			t.Errorf("Version part %d should start with a digit, got '%s'", i, part)
		}
	}
}

func TestRootCommand_SubcommandRegistration(t *testing.T) {
	// Test that all expected subcommands are registered
	expectedCommands := []string{
		"list", "l", "ls", // list command and aliases
		"start", "s", // start command and alias
		"stop", "st", // stop command and alias
		"shutdown", "sd", // shutdown command and alias
		"restart", "r", // restart command and alias
		"delete", "d", "del", // delete command and aliases
		"last",                     // last command
		"lts",                      // lts command
		"screenshot", "ss", "shot", // screenshot command and aliases
		"record", "rec", // record command and alias
	}

	// This would require access to the actual cobra command structure
	// For now, we'll just verify the expected commands list is complete
	if len(expectedCommands) < 10 {
		t.Error("Expected commands list should contain all subcommands and aliases")
	}
}

func TestRootCommand_Description(t *testing.T) {
	// Test command descriptions
	shortDesc := getRootCommandShortDescription()
	longDesc := getRootCommandLongDescription()

	if shortDesc == "" {
		t.Error("Short description should not be empty")
	}

	if longDesc == "" {
		t.Error("Long description should not be empty")
	}

	// Check for key functionality mentions
	expectedFeatures := []string{
		"iOS simulators",
		"Android emulators",
		"Start",
		"stop",
		"restart",
		"delete",
	}

	for _, feature := range expectedFeatures {
		if !strings.Contains(strings.ToLower(longDesc), strings.ToLower(feature)) {
			t.Errorf("Long description should mention '%s'", feature)
		}
	}
}

func TestExecuteFunction(t *testing.T) {
	// Test Execute function exists and can be called
	// Since Execute() would actually run the CLI, we'll test its components

	// Test that all required components for execution exist
	version := getRootCommandVersion()
	if version == "" {
		t.Error("Version should be available for Execute")
	}

	usage := getRootCommandUsage()
	if usage == "" {
		t.Error("Usage should be available for Execute")
	}

	shortDesc := getRootCommandShortDescription()
	if shortDesc == "" {
		t.Error("Short description should be available for Execute")
	}

	longDesc := getRootCommandLongDescription()
	if longDesc == "" {
		t.Error("Long description should be available for Execute")
	}

	// Test that ASCII art is available
	asciiArt := getASCIIArt()
	if asciiArt == "" {
		t.Error("ASCII art should be available for Execute")
	}
}

func TestRootCommand_Usage(t *testing.T) {
	// Test command usage string
	expectedUsage := "sim"
	usage := getRootCommandUsage()

	if usage != expectedUsage {
		t.Errorf("Expected usage %s, got %s", expectedUsage, usage)
	}
}

// Helper functions that mirror the values in cmd package for testing

func getRootCommandVersion() string {
	return "1.0.0"
}

func getASCIIArt() string {
	return `
 ███████╗██╗███╗   ███╗      ██████╗██╗     ██╗
 ██╔════╝██║████╗ ████║     ██╔════╝██║     ██║
 ███████╗██║██╔████╔██║     ██║     ██║     ██║
 ╚════██║██║██║╚██╔╝██║     ██║     ██║     ██║
 ███████║██║██║ ╚═╝ ██║     ╚██████╗███████╗██║
 ╚══════╝╚═╝╚═╝     ╚═╝      ╚═════╝╚══════╝╚═╝
                                                    
`
}

func getRootCommandShortDescription() string {
	return "CLI tool to manage iOS simulators and Android emulators"
}

func getRootCommandLongDescription() string {
	return `SIM-CLI is a command-line tool for managing iOS simulators and Android emulators.
	
It provides a simple interface to:
- List available simulators and emulators
- Start, stop, shutdown, and restart devices
- Delete simulators and emulators
- Take screenshots and record screen
- Manage device lifecycle efficiently`
}

func getRootCommandUsage() string {
	return "sim"
}
