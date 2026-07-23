//go:build darwin && !cam_embed

package cmd

// ensureExtractedAssets is a no-op when built without cam_embed tag.
// The CLI falls back to local Iris/.build/ paths.
func ensureExtractedAssets() (string, error) {
	return "", nil
}
