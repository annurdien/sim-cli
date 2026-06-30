package tests

import (
	"testing"

	"github.com/annurdien/sim-cli/cmd"
)

func TestGenerateFilename_Screenshot(t *testing.T) {
	deviceID := "test-device"
	filename := cmd.GenerateFilename("screenshot", deviceID, ".png")

	if filename == "" {
		t.Error("Generated filename should not be empty")
	}

	// Should contain the sanitized device ID (spaces → underscores).
	if !containsSubstring(filename, deviceID) {
		t.Errorf("Filename should contain device ID %q, got %s", deviceID, filename)
	}

	if !hasSuffix(filename, ".png") {
		t.Errorf("Filename should end with .png, got %s", filename)
	}

	if !hasPrefix(filename, "screenshot_") {
		t.Errorf("Filename should start with 'screenshot_', got %s", filename)
	}
}

func TestGenerateFilename_Recording(t *testing.T) {
	deviceID := "test-device"
	filename := cmd.GenerateFilename("recording", deviceID, ".mp4")

	if !hasSuffix(filename, ".mp4") {
		t.Errorf("Filename should end with .mp4, got %s", filename)
	}

	if !hasPrefix(filename, "recording_") {
		t.Errorf("Filename should start with 'recording_', got %s", filename)
	}
}

func TestGenerateFilename_SpacesReplaced(t *testing.T) {
	filename := cmd.GenerateFilename("screenshot", "iPhone 15 Pro", ".png")

	if containsSubstring(filename, " ") {
		t.Errorf("Filename should not contain spaces, got %s", filename)
	}

	if !containsSubstring(filename, "iPhone_15_Pro") {
		t.Errorf("Filename should contain sanitized device ID, got %s", filename)
	}
}

func TestEnsureExtension_PNG(t *testing.T) {
	cases := []struct {
		input    string
		expected string
	}{
		{"test", "test.png"},
		{"test.jpg", "test.png"},
		{"test.png", "test.png"},
		{"test.PNG", "test.png"},
		{"path/to/test", "path/to/test.png"},
	}

	for _, tc := range cases {
		result := cmd.EnsureExtension(tc.input, ".png")
		if result != tc.expected {
			t.Errorf("EnsureExtension(%q, \".png\") = %q, want %q", tc.input, result, tc.expected)
		}
	}
}

func TestEnsureExtension_MP4(t *testing.T) {
	cases := []struct {
		input    string
		expected string
	}{
		{"test", "test.mp4"},
		{"test.avi", "test.mp4"},
		{"test.mp4", "test.mp4"},
		{"test.MP4", "test.mp4"},
		{"path/to/test", "path/to/test.mp4"},
	}

	for _, tc := range cases {
		result := cmd.EnsureExtension(tc.input, ".mp4")
		if result != tc.expected {
			t.Errorf("EnsureExtension(%q, \".mp4\") = %q, want %q", tc.input, result, tc.expected)
		}
	}
}

func TestValidateRecordingDuration(t *testing.T) {
	cases := []struct {
		duration  int
		expectErr bool
	}{
		{0, false},   // 0 means manual stop
		{5, false},   // 5 seconds
		{60, false},  // 1 minute
		{-1, true},   // negative is invalid
		{-100, true}, // very negative
	}

	for _, tc := range cases {
		err := cmd.ValidateRecordingDuration(tc.duration)
		if tc.expectErr && err == nil {
			t.Errorf("Expected error for duration %d, got nil", tc.duration)
		}

		if !tc.expectErr && err != nil {
			t.Errorf("Expected no error for duration %d, got: %v", tc.duration, err)
		}
	}
}

func TestCommandExists_Known(t *testing.T) {
	// "ls" or "echo" should exist on all platforms.
	if !cmd.CommandExists("ls") && !cmd.CommandExists("echo") {
		t.Error("Expected at least one of 'ls' or 'echo' to exist in PATH")
	}
}

func TestCommandExists_Unknown(t *testing.T) {
	exists := cmd.CommandExists("this-command-definitely-does-not-exist-xyz-abc-123")
	if exists {
		t.Error("Expected false for non-existent command")
	}
}

// --- string helpers (avoid importing strings just for tests) ---

func containsSubstring(s, sub string) bool {
	return len(s) >= len(sub) && (s == sub || len(sub) == 0 ||
		func() bool {
			for i := 0; i <= len(s)-len(sub); i++ {
				if s[i:i+len(sub)] == sub {
					return true
				}
			}

			return false
		}())
}

func hasSuffix(s, suffix string) bool {
	return len(s) >= len(suffix) && s[len(s)-len(suffix):] == suffix
}

func hasPrefix(s, prefix string) bool {
	return len(s) >= len(prefix) && s[:len(prefix)] == prefix
}
