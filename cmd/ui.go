package cmd

import (
	"fmt"
	"os"

	"github.com/charmbracelet/huh/spinner"
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

// ApplyTheme applies the specified color theme to the UI elements.
func ApplyTheme(theme string) {
	switch theme {
	case "catppuccin":
		ColorBooted = lipgloss.Color("#A6DA95")   // Green
		ColorShutdown = lipgloss.Color("#5B6078") // Surface
		ColorIOS = lipgloss.Color("#8AADF4")      // Blue
		ColorAndroid = lipgloss.Color("#A6DA95")  // Green
		ColorHeader = lipgloss.Color("#C6A0F6")   // Mauve
		ColorError = lipgloss.Color("#ED8796")    // Red
		ColorSuccess = lipgloss.Color("#A6DA95")  // Green
	case "dracula":
		ColorBooted = lipgloss.Color("#50fa7b")
		ColorShutdown = lipgloss.Color("#6272a4")
		ColorIOS = lipgloss.Color("#8be9fd")
		ColorAndroid = lipgloss.Color("#50fa7b")
		ColorHeader = lipgloss.Color("#bd93f9")
		ColorError = lipgloss.Color("#ff5555")
		ColorSuccess = lipgloss.Color("#50fa7b")
	default:
		ColorBooted = lipgloss.Color("42")
		ColorShutdown = lipgloss.Color("240")
		ColorIOS = lipgloss.Color("33")
		ColorAndroid = lipgloss.Color("40")
		ColorHeader = lipgloss.Color("99")
		ColorError = lipgloss.Color("196")
		ColorSuccess = lipgloss.Color("46")
	}

	StyleBooted = lipgloss.NewStyle().Foreground(ColorBooted).Bold(true)
	StyleShutdown = lipgloss.NewStyle().Foreground(ColorShutdown)
	StyleIOS = lipgloss.NewStyle().Foreground(ColorIOS)
	StyleAndroid = lipgloss.NewStyle().Foreground(ColorAndroid)
	StyleHeader = lipgloss.NewStyle().Foreground(ColorHeader).Bold(true)
	StyleSuccess = lipgloss.NewStyle().Foreground(ColorSuccess).Bold(true)
	StyleError = lipgloss.NewStyle().Foreground(ColorError).Bold(true)

	dashboardBaseStyle = lipgloss.NewStyle().
		BorderStyle(lipgloss.NormalBorder()).
		BorderForeground(ColorShutdown)
}

// FormatState returns a styled string for device state.
func FormatState(state string) string {
	if state == StateBooted || state == "device" {
		if state == "device" {
			state = StateBooted // Normalize Android 'device' to 'Booted'
		}

		return StyleBooted.Render(state)
	}

	if state == "offline" {
		state = StateOffline
	}

	return StyleShutdown.Render(state)
}

// FormatPlatform returns a styled string for platform type.
func FormatPlatform(platform string) string {
	if platform == TypeIOSSimulator || platform == NameIOS {
		return StyleIOS.Render(platform)
	}
	if platform == TypeAndroidEmulator || platform == NameAndroid {
		return StyleAndroid.Render(platform)
	}

	return platform
}

// RenderTable builds and renders a beautiful lipgloss table.
func RenderTable(headers []string, rows [][]string) {
	re := lipgloss.NewRenderer(os.Stdout)
	headerStyle := re.NewStyle().Foreground(ColorHeader).Bold(true).Padding(0, 1)
	borderStyle := re.NewStyle().Foreground(lipgloss.Color("238"))

	t := table.New().
		Border(lipgloss.RoundedBorder()).
		BorderStyle(borderStyle).
		Headers(headers...).
		StyleFunc(func(row, col int) lipgloss.Style {
			if row == table.HeaderRow {
				return headerStyle
			}

			return lipgloss.NewStyle().Padding(0, 1)
		}).
		Rows(rows...)

	fmt.Println(t.Render())
}

// PrintSuccess prints a beautiful success message with a green checkmark.
func PrintSuccess(msg string) {
	fmt.Printf("%s %s\n", StyleSuccess.Render("✓"), lipgloss.NewStyle().Bold(true).Render(msg))
}

// PrintError prints a beautiful error message with a red cross.
func PrintError(msg string) {
	fmt.Printf("%s %s\n", StyleError.Render("✗"), lipgloss.NewStyle().Bold(true).Render(msg))
}

// PrintInfo prints a beautiful info message.
func PrintInfo(msg string) {
	fmt.Printf("%s %s\n", StyleIOS.Render("ℹ"), lipgloss.NewStyle().Render(msg))
}

// RunSpinner runs the provided action function while displaying a beautiful loading spinner.
func RunSpinner(title string, action func() error) error {
	var actionErr error

	err := spinner.New().
		Title(title).
		Action(func() {
			actionErr = action()
		}).
		Run()
	if err != nil {
		return err // Error initializing spinner
	}

	return actionErr // Error from the action itself
}
