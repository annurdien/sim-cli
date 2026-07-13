package cmd

import (
	"errors"
	"fmt"

	"github.com/charmbracelet/huh"
)

var (
	ErrNoDevicesAvailable = errors.New("no devices available to select")
	ErrSelectionCancelled = errors.New("device selection cancelled")
)

// PromptDeviceSelector shows an interactive TUI to pick a device.
// The filter parameter specifies what kind of devices to include ("all", "booted", "shutdown").
func PromptDeviceSelector(filter string) (string, error) {
	devices := append(GetIOSSimulators(), GetAndroidEmulators()...)

	var options []huh.Option[string]
	for _, d := range devices {
		// Apply filter
		if filter == "booted" && d.State != StateBooted {
			continue
		}
		if filter == "shutdown" && d.State != StateShutdown {
			continue
		}

		label := fmt.Sprintf("%-20s (%s) - %s", d.Name, FormatRuntime(d.Runtime), d.State)
		options = append(options, huh.NewOption(label, d.UDID))
	}

	if len(options) == 0 {
		return "", ErrNoDevicesAvailable
	}

	var selectedUDID string

	form := huh.NewForm(
		huh.NewGroup(
			huh.NewSelect[string]().
				Title("Select a Device").
				Options(options...).
				Value(&selectedUDID).
				Filtering(true).
				Height(10), // Show up to 10 items
		),
	)

	err := form.Run()
	if err != nil {
		if errors.Is(err, huh.ErrUserAborted) {
			return "", ErrSelectionCancelled
		}

		return "", fmt.Errorf("failed to prompt device: %w", err)
	}

	return selectedUDID, nil
}
