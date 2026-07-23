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



func copyToClipboard(text string) error {
	return clipboard.WriteAll(text)
}

// copyFileToClipboard copies the contents of the file at filePath to the system clipboard.
// On macOS, image files are copied as image data; on Linux, xclip is used when available.
// On other platforms (Windows), the file path is copied as text with a warning.
func copyFileToClipboard(filePath string) error {
	ext := strings.ToLower(filepath.Ext(filePath))

	switch runtime.GOOS {
	case DarwinOS:
		return copyFileToClipboardDarwin(filePath, ext)
	case "linux":
		return copyFileToClipboardLinux(filePath, ext)
	default:
		PrintInfo( // Windows and other platforms: fall back to copying the file path as text.
			fmt.Sprintf("Warning: clipboard file copy not supported on %s; copying file path instead.", runtime.GOOS))
		return copyToClipboard(filePath)
	}
}

// copyFileToClipboardDarwin copies a file to the macOS clipboard via AppleScript.
func copyFileToClipboardDarwin(filePath, ext string) error {
	// Escape double quotes to prevent AppleScript injection.
	safePath := strings.ReplaceAll(filePath, `"`, `\"`)

	var script string
	if ext == ExtPNG {
		script = fmt.Sprintf(`set the clipboard to (read (POSIX file "%s") as TIFF picture)`, safePath)
	} else {
		script = fmt.Sprintf(`set the clipboard to POSIX file "%s"`, safePath)
	}

	cmd := exec.Command(CmdOsaScript, "-e", script)
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed to copy file to clipboard: %w", err)
	}

	return nil
}

// copyFileToClipboardLinux copies a file to the Linux clipboard via xclip.
// Falls back to copying the file path as text if xclip is not installed.
func copyFileToClipboardLinux(filePath, ext string) error {
	if !CommandExists(CmdXclip) {
		PrintInfo("Warning: xclip not found; copying file path to clipboard instead. Install xclip for full clipboard support.")
		return copyToClipboard(filePath)
	}

	var mimeType string
	switch ext {
	case ExtPNG:
		mimeType = "image/png"
	case ".jpg", ".jpeg":
		mimeType = "image/jpeg"
	case ExtGIF:
		mimeType = "image/gif"
	default:
		PrintInfo( // For MP4 and other non-image files, copy the path as text.
			fmt.Sprintf("Note: copying file path to clipboard (binary clipboard not supported for %s files on Linux).", ext))
		return copyToClipboard(filePath)
	}

	cmd := exec.Command(CmdXclip, "-selection", "clipboard", "-t", mimeType, "-i", filePath)
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed to copy file to clipboard via xclip: %w", err)
	}

	return nil
}



// CommandExists reports whether the named executable exists in the system PATH.
func CommandExists(cmd string) bool {
	_, err := exec.LookPath(cmd)

	return err == nil
}



// ValidateRecordingDuration returns an error if duration is negative.
func ValidateRecordingDuration(duration int) error {
	if duration < 0 {
		return fmt.Errorf("recording duration must be non-negative, got %d: %w", duration, ErrInvalidDuration)
	}

	return nil
}



func convertToGIF(inputFile, outputFile string, fps, scale int) error {
	if !CommandExists(CmdFFmpeg) {
		return ErrFFmpegNotInstalled
	}

	PrintInfo("Converting to GIF...")
	vf := fmt.Sprintf("fps=%d,scale=%d:-1:flags=lanczos", fps, scale)

	if output, err := packageExecutor.Output(CmdFFmpeg, "-i", inputFile, "-vf", vf, "-c", "gif", "-f", "gif", outputFile); err != nil {
		return fmt.Errorf("failed to convert to GIF: %w\nOutput: %s", err, string(output))
	}
	PrintInfo(fmt.Sprintf("GIF saved to: %s", outputFile))

	return nil
}
