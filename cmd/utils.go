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

func generateFilename(prefix, deviceID, extension string) string {
	timestamp := time.Now().Format("20060102_150405")
	sanitizedDeviceID := strings.ReplaceAll(deviceID, " ", "_")
	return fmt.Sprintf("%s_%s_%s%s", prefix, sanitizedDeviceID, timestamp, extension)
}

func ensureExtension(filename, ext string) string {
	if !strings.HasSuffix(strings.ToLower(filename), ext) {
		return strings.TrimSuffix(filename, filepath.Ext(filename)) + ext
	}
	return filename
}

// --- Clipboard Operations ---

func copyToClipboard(text string) error {
	return clipboard.WriteAll(text)
}

func copyFileToClipboard(filePath string) error {
	var cmd *exec.Cmd
	ext := strings.ToLower(filepath.Ext(filePath))

	switch runtime.GOOS {
	case DarwinOS:
		var script string
		if ext == ExtPNG {
			script = fmt.Sprintf("set the clipboard to (read (POSIX file \"%s\") as TIFF picture)", filePath)
		} else {
			script = fmt.Sprintf("set the clipboard to POSIX file \"%s\"", filePath)
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

func commandExists(cmd string) bool {
	_, err := exec.LookPath(cmd)
	return err == nil
}

// --- Video Conversion ---

func convertToGIF(inputFile, outputFile string) error {
	if !commandExists(CmdFFmpeg) {
		return fmt.Errorf("'%s' is not installed. Please install ffmpeg to use the GIF conversion feature", CmdFFmpeg)
	}

	fmt.Println("Converting to GIF...")
	cmd := exec.Command(CmdFFmpeg, "-i", inputFile, "-vf", "fps=10,scale=480:-1:flags=lanczos", "-c", "gif", "-f", "gif", outputFile)
	if output, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("failed to convert to GIF: %w\nOutput: %s", err, string(output))
	}
	fmt.Printf("GIF saved to: %s\n", outputFile)
	return nil
}
