package cmd

import (
	"fmt"
	"os"

	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/lipgloss/table"
)

var (
	// Colors for UI elements.
	ColorBooted   = lipgloss.Color("42")  // Bright Green
	ColorShutdown = lipgloss.Color("240") // Dark Gray
	ColorIOS      = lipgloss.Color("33")  // Blue
	ColorAndroid  = lipgloss.Color("40")  // Green
	ColorHeader   = lipgloss.Color("99")  // Purple
	ColorError    = lipgloss.Color("196") // Red
	ColorSuccess  = lipgloss.Color("46")  // Green

	// Styles for UI elements.
	StyleBooted   = lipgloss.NewStyle().Foreground(ColorBooted).Bold(true)
	StyleShutdown = lipgloss.NewStyle().Foreground(ColorShutdown)
	StyleIOS      = lipgloss.NewStyle().Foreground(ColorIOS)
	StyleAndroid  = lipgloss.NewStyle().Foreground(ColorAndroid)
	StyleHeader   = lipgloss.NewStyle().Foreground(ColorHeader).Bold(true)
	StyleSuccess  = lipgloss.NewStyle().Foreground(ColorSuccess).Bold(true)
	StyleError    = lipgloss.NewStyle().Foreground(ColorError).Bold(true)
)

// FormatState returns a styled string for device state.
func FormatState(state string) string {
	if state == StateBooted || state == "Booted" || state == "device" {
		if state == "device" {
			state = "Booted" // Normalize Android 'device' to 'Booted'
		}

		return StyleBooted.Render(state)
	}

	if state == "offline" {
		state = "Offline"
	}

	return StyleShutdown.Render(state)
}

// FormatPlatform returns a styled string for platform type.
func FormatPlatform(platform string) string {
	if platform == TypeIOSSimulator || platform == "iOS" {
		return StyleIOS.Render(platform)
	}
	if platform == TypeAndroidEmulator || platform == "Android" {
		return StyleAndroid.Render(platform)
	}

	return platform
}

// RenderTable builds and renders a beautiful lipgloss table.
func RenderTable(headers []string, rows [][]string) {
	re := lipgloss.NewRenderer(os.Stdout)
	headerStyle := re.NewStyle().Foreground(ColorHeader).Bold(true)
	borderStyle := re.NewStyle().Foreground(lipgloss.Color("238"))

	t := table.New().
		Border(lipgloss.RoundedBorder()).
		BorderStyle(borderStyle).
		Headers(headers...).
		StyleFunc(func(row, col int) lipgloss.Style {
			if row == 0 {
				return headerStyle
			}

			return lipgloss.NewStyle().Padding(0, 1)
		}).
		Rows(rows...)

	fmt.Println(t.Render())
}
