//go:build cam_embed

package cmd

import (
	"crypto/sha256"
	"embed"
	"fmt"
	"os"
	"path/filepath"
)

//go:embed assets/FrameHost assets/MiniCamInject.dylib
var embeddedAssets embed.FS

// ensureExtractedAssets extracts embedded assets to ~/.sim-cli/bin/ if valid.
// Returns the directory containing the extracted binaries.
func ensureExtractedAssets() (string, error) {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("failed to get user home directory: %w", err)
	}

	targetDir := filepath.Join(homeDir, ".sim-cli", "bin")
	if err := os.MkdirAll(targetDir, 0755); err != nil {
		return "", fmt.Errorf("failed to create target directory %s: %w", targetDir, err)
	}

	files := []struct {
		embeddedName string
		fileName     string
		mode         os.FileMode
	}{
		{"assets/FrameHost", "FrameHost", 0755},
		{"assets/MiniCamInject.dylib", "MiniCamInject.dylib", 0644},
	}

	for _, f := range files {
		data, err := embeddedAssets.ReadFile(f.embeddedName)
		if err != nil {
			return "", fmt.Errorf("failed to read embedded asset %s: %w", f.embeddedName, err)
		}

		targetPath := filepath.Join(targetDir, f.fileName)

		// Check if file already exists and matches hash
		if existingData, err := os.ReadFile(targetPath); err == nil {
			if sha256.Sum256(existingData) == sha256.Sum256(data) {
				continue // File already exists and is up to date
			}
		}

		// Write extracted binary
		tmpPath := targetPath + ".tmp"
		if err := os.WriteFile(tmpPath, data, f.mode); err != nil {
			return "", fmt.Errorf("failed to write asset to %s: %w", tmpPath, err)
		}
		if err := os.Rename(tmpPath, targetPath); err != nil {
			_ = os.Remove(tmpPath)
			return "", fmt.Errorf("failed to replace asset at %s: %w", targetPath, err)
		}
		_ = os.Chmod(targetPath, f.mode)
	}

	return targetDir, nil
}
