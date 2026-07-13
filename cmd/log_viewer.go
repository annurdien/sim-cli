package cmd

import (
	"bufio"
	"fmt"
	"io"
	"strings"

	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

type logMsg string

type logViewerModel struct {
	viewport viewport.Model
	lines    []string
	ready    bool
	paused   bool
}

func runLogViewer(stdout io.ReadCloser) error {
	m := logViewerModel{
		lines: []string{},
	}
	p := tea.NewProgram(m, tea.WithAltScreen(), tea.WithMouseCellMotion())

	go func() {
		scanner := bufio.NewScanner(stdout)
		for scanner.Scan() {
			p.Send(logMsg(scanner.Text()))
		}
	}()

	_, err := p.Run()

	return err
}

func (m logViewerModel) Init() tea.Cmd {
	return nil
}

//nolint:gocyclo,cyclop
func (m logViewerModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var (
		cmd  tea.Cmd
		cmds = make([]tea.Cmd, 0, 1)
	)

	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		headerHeight := 2
		footerHeight := 0
		verticalMarginHeight := headerHeight + footerHeight

		if !m.ready {
			m.viewport = viewport.New(msg.Width, msg.Height-verticalMarginHeight)
			m.viewport.YPosition = headerHeight
			m.ready = true
		} else {
			m.viewport.Width = msg.Width
			m.viewport.Height = msg.Height - verticalMarginHeight
		}

	case tea.KeyMsg:
		switch msg.String() {
		case "q", "ctrl+c":
			return m, tea.Quit
		case " ":
			m.paused = !m.paused
		}

	case logMsg:
		if !m.paused {
			line := string(msg)
			// Apply simple coloring
			switch {
			case strings.Contains(line, " Error ") || strings.Contains(line, " E ") || strings.Contains(strings.ToLower(line), "error"):
				line = lipgloss.NewStyle().Foreground(ColorError).Render(line)
			case strings.Contains(line, " Warn ") || strings.Contains(line, " W ") || strings.Contains(strings.ToLower(line), "warn"):
				line = lipgloss.NewStyle().Foreground(lipgloss.Color("214")).Render(line)
			case strings.Contains(line, " Info ") || strings.Contains(line, " I "):
				line = lipgloss.NewStyle().Foreground(ColorIOS).Render(line)
			}

			m.lines = append(m.lines, line)
			// Keep history manageable
			if len(m.lines) > 1000 {
				m.lines = m.lines[1:]
			}
			m.viewport.SetContent(strings.Join(m.lines, "\n"))
			m.viewport.GotoBottom()
		}
	}

	m.viewport, cmd = m.viewport.Update(msg)
	cmds = append(cmds, cmd)

	return m, tea.Batch(cmds...)
}

func (m logViewerModel) View() string {
	if !m.ready {
		return "\n  Initializing..."
	}
	header := lipgloss.NewStyle().Foreground(ColorHeader).Bold(true).Render("SIM-CLI Log Viewer")
	status := "STREAMING"
	statusColor := ColorBooted
	if m.paused {
		status = "PAUSED"
		statusColor = lipgloss.Color("214")
	}

	header += fmt.Sprintf(" [%s] (Press Space to toggle, 'q' to quit)", lipgloss.NewStyle().Foreground(statusColor).Render(status))

	return fmt.Sprintf("%s\n\n%s", header, m.viewport.View())
}
