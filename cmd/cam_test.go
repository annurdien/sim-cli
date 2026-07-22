//go:build darwin

package cmd

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"
)

// TestCamShmPaths verifies that shmPath / statusFilePath / pidFilePath produce
// the expected string patterns without touching the filesystem.
func TestCamShmPaths(t *testing.T) {
	udid := "ABC-123"

	tests := []struct {
		fn   func(string) string
		want string
	}{
		{shmPath, "/tmp/minisimcam.ABC-123.frames"},
		{statusFilePath, "/tmp/minisimcam.ABC-123.status"},
		{pidFilePath, "/tmp/minisimcam.ABC-123.pid"},
	}

	for _, tt := range tests {
		if got := tt.fn(udid); got != tt.want {
			t.Errorf("got %q, want %q", got, tt.want)
		}
	}
}

// TestCamStatusParsing verifies that a status JSON file written by FrameHost
// can be round-tripped through camFrameLoopStatus.
func TestCamStatusParsing(t *testing.T) {
	sample := camFrameLoopStatus{
		UDID:           "TEST-UDID",
		Source:         "test-card.png",
		Width:          1280,
		Height:         720,
		FPS:            30,
		FramesProduced: 900,
		HostPID:        12345,
		StartedAt:      "2026-01-01T00:00:00Z",
		LastFrameAgeMs: 12.5,
		Running:        true,
	}

	data, err := json.Marshal(sample)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}

	var parsed camFrameLoopStatus
	if err := json.Unmarshal(data, &parsed); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	if parsed.FramesProduced != 900 {
		t.Errorf("FramesProduced: got %d, want 900", parsed.FramesProduced)
	}
	if parsed.Width != 1280 {
		t.Errorf("Width: got %d, want 1280", parsed.Width)
	}
	if !parsed.Running {
		t.Error("Running should be true")
	}
}

func TestFrameHostFPS(t *testing.T) {
	udid := fmt.Sprintf("frame-host-fps-test-%d", time.Now().UnixNano())
	path := filepath.Join(os.TempDir(), fmt.Sprintf("minisimcam.%s.status", udid))
	t.Cleanup(func() { _ = os.Remove(path) })

	status := camFrameLoopStatus{FPS: 60}
	data, err := json.Marshal(status)
	if err != nil {
		t.Fatalf("marshal status: %v", err)
	}
	if err := os.WriteFile(path, data, 0o600); err != nil {
		t.Fatalf("write status: %v", err)
	}
	if got := frameHostFPS(udid); got != 60 {
		t.Errorf("frameHostFPS() = %d, want 60", got)
	}
}

// TestStopFrameHostNoFile verifies stopFrameHost returns nil when no PID file exists.
func TestStopFrameHostNoFile(t *testing.T) {
	// Use a non-existent UDID — no PID file should exist.
	err := stopFrameHost("non-existent-udid-xyz")
	if err != nil {
		t.Errorf("expected nil error, got: %v", err)
	}
}

// TestMiniSimCamDirFallback verifies that miniSimCamDir() returns a path ending in "MiniSimCam".
func TestMiniSimCamDirFallback(t *testing.T) {
	dir := miniSimCamDir()
	if filepath.Base(dir) != "MiniSimCam" {
		t.Errorf("expected base to be MiniSimCam, got %q", filepath.Base(dir))
	}
}
