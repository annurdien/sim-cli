package cmd

import (
	"fmt"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"github.com/atotto/clipboard"
)

// --- File and Path Helpers ---

// GenerateFilename creates a timestamped filename with the given prefix, device ID, and extension.
func GenerateFilename(prefix, deviceID, extension string) string {
	timestamp := time.Now().Format("20060102_150405")
	sanitizedDeviceID := strings.ReplaceAll(deviceID, " ", "_")

	return fmt.Sprintf("%s_%s_%s%s", prefix, sanitizedDeviceID, timestamp, extension)
}

// EnsureExtension returns filename with ext as its extension, replacing any existing extension.
// The extension is normalized (e.g. ".PNG" becomes ".png").
func EnsureExtension(filename, ext string) string {
	return strings.TrimSuffix(filename, filepath.Ext(filename)) + ext
}

// --- Clipboard Operations ---

func copyToClipboard(text string) error {
	return clipboard.WriteAll(text)
}

// copyFileToClipboard copies the contents of the file at filePath to the system clipboard.
// On macOS, image files are copied as image data; all other files are copied by path.
func copyFileToClipboard(filePath string) error {
	var cmd *exec.Cmd

	ext := strings.ToLower(filepath.Ext(filePath))

	switch runtime.GOOS {
	case DarwinOS:
		var script string
		// Escape double quotes to prevent AppleScript injection.
		safePath := strings.ReplaceAll(filePath, `"`, `\"`)

		if ext == ExtPNG {
			script = fmt.Sprintf(`set the clipboard to (read (POSIX file "%s") as TIFF picture)`, safePath)
		} else {
			script = fmt.Sprintf(`set the clipboard to POSIX file "%s"`, safePath)
		}

		cmd = exec.Command(CmdOsaScript, "-e", script)
	default:
		return copyToClipboard(filePath)
	}

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed to copy file to clipboard: %w", err)
	}

	return nil
}

// --- Command Execution ---

// CommandExists reports whether the named executable exists in the system PATH.
func CommandExists(cmd string) bool {
	_, err := exec.LookPath(cmd)

	return err == nil
}

// --- Recording Duration Validation ---

// ValidateRecordingDuration returns an error if duration is negative.
func ValidateRecordingDuration(duration int) error {
	if duration < 0 {
		return fmt.Errorf("recording duration must be non-negative, got %d: %w", duration, ErrInvalidDuration)
	}

	return nil
}

// --- Video Conversion ---

func convertToGIF(inputFile, outputFile string, fps, scale int) error {
	if !CommandExists(CmdFFmpeg) {
		return ErrFFmpegNotInstalled
	}

	fmt.Println("Converting to GIF...")
	vf := fmt.Sprintf("fps=%d,scale=%d:-1:flags=lanczos", fps, scale)
	cmd := exec.Command(CmdFFmpeg, "-i", inputFile, "-vf", vf, "-c", "gif", "-f", "gif", outputFile)

	if output, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("failed to convert to GIF: %w\nOutput: %s", err, string(output))
	}

	fmt.Printf("GIF saved to: %s\n", outputFile)

	return nil
}
