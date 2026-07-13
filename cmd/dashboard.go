package cmd

import (
	"runtime"
	"sort"
	"time"

	"github.com/charmbracelet/bubbles/spinner"
	"github.com/charmbracelet/bubbles/table"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

var dashboardBaseStyle = lipgloss.NewStyle().
	BorderStyle(lipgloss.NormalBorder()).
	BorderForeground(lipgloss.Color("240"))

type dashboardModel struct {
	table   table.Model
	spinner spinner.Model
	msg     string
	loading bool
	width   int
	height  int
}

type (
	refreshMsg    []Device
	actionDoneMsg struct{ msg string }
	tickMsg       time.Time
)

func (m dashboardModel) Init() tea.Cmd {
	return tea.Batch(m.spinner.Tick, tickCmd())
}

func tickCmd() tea.Cmd {
	return tea.Tick(time.Second*3, func(t time.Time) tea.Msg {
		return tickMsg(t)
	})
}

func doActionCmd(action func() error, successMsg string) tea.Cmd {
	return func() tea.Msg {
		err := action()
		if err != nil {
			return actionDoneMsg{msg: "Error: " + err.Error()}
		}

		return actionDoneMsg{msg: successMsg}
	}
}

func fetchDevices() []Device {
	var devices []Device
	if runtime.GOOS == DarwinOS {
		devices = append(devices, GetIOSSimulators()...)
	}
	devices = append(devices, GetAndroidEmulators()...)

	sort.Slice(devices, func(i, j int) bool {
		if devices[i].Type != devices[j].Type {
			return devices[i].Type < devices[j].Type
		}

		return devices[i].Name < devices[j].Name
	})

	return devices
}

func refreshDevicesCmd() tea.Cmd {
	return func() tea.Msg {
		return refreshMsg(fetchDevices())
	}
}

//nolint:gocyclo,cyclop
func (m dashboardModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmds []tea.Cmd

	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "q", "ctrl+c":
			return m, tea.Quit
		case "s", "enter":
			row := m.table.SelectedRow()
			if len(row) > 0 {
				deviceID := row[3] // UDID
				if deviceID == "N/A" {
					deviceID = row[1] // Name
				}
				m.loading = true
				m.msg = "Starting " + row[1] + "..."
				cmds = append(cmds, doActionCmd(func() error {
					return startDevice(deviceID, true)
				}, "Started "+row[1]))
			}
		case "x", "k":
			row := m.table.SelectedRow()
			if len(row) > 0 {
				deviceID := row[3]
				if deviceID == "N/A" {
					deviceID = row[1]
				}
				m.loading = true
				m.msg = "Stopping " + row[1] + "..."
				cmds = append(cmds, doActionCmd(func() error {
					if row[0] == "iOS Simulator" || row[0] == TypeIOSSimulator {
						_, err := shutdownIOSSimulator(deviceID)
						return err
					}
					_, err := stopAndroidEmulator(deviceID)

					return err
				}, "Stopped "+row[1]))
			}
		case "r":
			m.loading = true
			m.msg = "Refreshing..."
			cmds = append(cmds, refreshDevicesCmd())
		}
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height

		// Adjust table height
		headerHeight := 6 // Title + controls + borders + msg
		tHeight := m.height - headerHeight
		if tHeight < 5 {
			tHeight = 5
		}
		m.table.SetHeight(tHeight)

		// Adjust columns dynamically based on width
		// table has 5 columns. Default padding is 1 on left and right (2 per column -> 10 total).
		// dashboardBaseStyle adds 2 characters for borders.
		availWidth := m.width - 12
		if availWidth < 60 {
			availWidth = 60
		}

		w1 := int(float64(availWidth) * 0.15)
		w2 := int(float64(availWidth) * 0.25)
		w3 := int(float64(availWidth) * 0.10)
		w4 := int(float64(availWidth) * 0.35)
		w5 := availWidth - (w1 + w2 + w3 + w4) // Soak up the remaining width exactly

		cols := []table.Column{
			{Title: "Type", Width: w1},
			{Title: "Name", Width: w2},
			{Title: "State", Width: w3},
			{Title: "UDID", Width: w4},
			{Title: "Runtime", Width: w5},
		}
		m.table.SetColumns(cols)

	case refreshMsg:
		m.table.SetRows(devicesToRows(msg))
		if m.msg == "Refreshing..." {
			m.msg = "Refreshed list."
			m.loading = false
		}
	case actionDoneMsg:
		m.msg = msg.msg
		m.loading = false
		cmds = append(cmds, refreshDevicesCmd())
	case tickMsg:
		cmds = append(cmds, tickCmd(), refreshDevicesCmd())
	case spinner.TickMsg:
		var cmd tea.Cmd
		m.spinner, cmd = m.spinner.Update(msg)
		cmds = append(cmds, cmd)
	}

	var tableCmd tea.Cmd
	m.table, tableCmd = m.table.Update(msg)
	cmds = append(cmds, tableCmd)

	return m, tea.Batch(cmds...)
}

func (m dashboardModel) View() string {
	s := lipgloss.NewStyle().Bold(true).Foreground(ColorHeader).Render("SIM-CLI Dashboard")
	s += "\nControls: [up/down] Navigate • [s/enter] Start • [x/k] Stop • [r] Refresh • [q] Quit\n\n"
	s += dashboardBaseStyle.Render(m.table.View()) + "\n"

	footer := ""
	if m.loading {
		footer = m.spinner.View() + " " + lipgloss.NewStyle().Foreground(ColorIOS).Render(m.msg)
	} else if m.msg != "" {
		footer = lipgloss.NewStyle().Foreground(ColorIOS).Render("ℹ " + m.msg)
	}
	s += footer + "\n"

	return s
}

func normalizeState(state string) string {
	if state == StateBooted || state == "Booted" || state == "device" {
		return "Booted"
	}
	if state == "offline" {
		return "Offline"
	}

	return state
}

func devicesToRows(devices []Device) []table.Row {
	rows := make([]table.Row, 0, len(devices))
	for _, d := range devices {
		rows = append(rows, table.Row{
			d.Type,
			d.Name,
			normalizeState(d.State),
			d.UDID,
			FormatRuntime(d.Runtime),
		})
	}

	return rows
}

func runDashboard(initialDevices []Device) error {
	columns := []table.Column{
		{Title: "Type", Width: 20},
		{Title: "Name", Width: 35},
		{Title: "State", Width: 15},
		{Title: "UDID", Width: 40},
		{Title: "Runtime", Width: 20},
	}

	t := table.New(
		table.WithColumns(columns),
		table.WithRows(devicesToRows(initialDevices)),
		table.WithFocused(true),
		table.WithHeight(15),
	)

	s := table.DefaultStyles()
	s.Header = s.Header.
		BorderStyle(lipgloss.NormalBorder()).
		BorderForeground(lipgloss.Color("240")).
		BorderBottom(true).
		Bold(false)
	s.Selected = s.Selected.
		Foreground(lipgloss.Color("229")).
		Background(lipgloss.Color("57")).
		Bold(false)
	t.SetStyles(s)

	sp := spinner.New()
	sp.Spinner = spinner.Dot
	sp.Style = lipgloss.NewStyle().Foreground(ColorHeader)

	m := dashboardModel{
		table:   t,
		spinner: sp,
		msg:     "Ready.",
	}
	if _, err := tea.NewProgram(m, tea.WithAltScreen()).Run(); err != nil {
		return err
	}

	return nil
}
