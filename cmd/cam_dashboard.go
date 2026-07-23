//go:build darwin

package cmd

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"runtime"
	"sort"
	"strings"
	"syscall"
	"time"

	"github.com/charmbracelet/bubbles/list"
	"github.com/charmbracelet/bubbles/spinner"
	"github.com/charmbracelet/bubbles/table"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

type focusPane int

const (
	paneSimulators focusPane = iota
	paneCameras
	paneApps
)

type SimCamApp struct {
	BundleID        string `json:"CFBundleIdentifier"`
	DisplayName     string `json:"CFBundleDisplayName"`
	Name            string `json:"CFBundleName"`
	ApplicationType string `json:"ApplicationType"`
}

func (a SimCamApp) Title() string {
	if a.DisplayName != "" {
		return a.DisplayName
	}
	if a.Name != "" {
		return a.Name
	}
	return a.BundleID
}

type camSimDevice struct {
	Device
	CamStatus string
	CamSource string
	CamFPS    int
}

type ResolutionPreset struct {
	title       string
	description string
	width       int
	height      int
}

func (r ResolutionPreset) Title() string       { return r.title }
func (r ResolutionPreset) Description() string { return r.description }
func (r ResolutionPreset) FilterValue() string { return r.title }

type camDashboardModel struct {
	focused         focusPane
	simTable        table.Model
	appTable        table.Model
	cameraTable     table.Model
	presetList      list.Model
	filterInput     textinput.Model
	spinner         spinner.Model
	simulators      []camSimDevice
	allApps         []SimCamApp
	filteredApps    []SimCamApp
	cameras         []CameraInfo
	selectedSimUDID string
	selectedSimName string
	showSystemApps  bool
	showPopup       bool
	camWidth        int
	camHeight       int
	msg             string
	loading         bool
	width           int
	height          int
}

type (
	camRefreshMsg     []camSimDevice
	appsFetchedMsg    []SimCamApp
	camerasFetchedMsg []CameraInfo
)

type CameraInfo struct {
	UniqueID      string `json:"uniqueID"`
	LocalizedName string `json:"localizedName"`
	TypeLabel     string `json:"typeLabel"`
}

func fetchBootedSimulatorsWithCamStatus() []camSimDevice {
	if runtime.GOOS != DarwinOS {
		return nil
	}

	allSims := GetIOSSimulators()
	var booted []camSimDevice

	for _, sim := range allSims {
		if sim.State == StateBooted || sim.State == "Booted" {
			dev := camSimDevice{
				Device:    sim,
				CamStatus: "Stopped",
				CamSource: "-",
				CamFPS:    0,
			}

			// Read status file if running
			statusPath := statusFilePath(sim.UDID)
			if data, err := os.ReadFile(statusPath); err == nil {
				var st camFrameLoopStatus
				if json.Unmarshal(data, &st) == nil && st.Running {
					dev.CamStatus = "Running"
					dev.CamSource = st.Source
					dev.CamFPS = st.FPS
				}
			}

			booted = append(booted, dev)
		}
	}

	sort.Slice(booted, func(i, j int) bool {
		return booted[i].Name < booted[j].Name
	})

	return booted
}

func fetchInstalledApps(udid string) ([]SimCamApp, error) {
	cmd := fmt.Sprintf("xcrun simctl listapps %s | plutil -convert json - -o -", udid)
	out, err := exec.Command("sh", "-c", cmd).Output()
	if err != nil {
		return nil, fmt.Errorf("failed to list apps: %w", err)
	}

	var rawMap map[string]SimCamApp
	if err := json.Unmarshal(out, &rawMap); err != nil {
		return nil, fmt.Errorf("failed to parse apps JSON: %w", err)
	}

	var apps []SimCamApp
	for bundleID, app := range rawMap {
		if app.BundleID == "" {
			app.BundleID = bundleID
		}
		if strings.HasPrefix(app.BundleID, "com.apple.Process") || strings.HasPrefix(app.BundleID, "com.apple.CoreSimulator") {
			continue
		}
		apps = append(apps, app)
	}

	sort.Slice(apps, func(i, j int) bool {
		return strings.ToLower(apps[i].Title()) < strings.ToLower(apps[j].Title())
	})

	return apps, nil
}

func refreshCamDevicesCmd() tea.Cmd {
	return func() tea.Msg {
		return camRefreshMsg(fetchBootedSimulatorsWithCamStatus())
	}
}

func refreshCamStatusOnlyCmd(sims []camSimDevice) tea.Cmd {
	return func() tea.Msg {
		booted := make([]camSimDevice, 0, len(sims))
		for _, dev := range sims {
			dev.CamStatus = "Stopped"
			dev.CamSource = "-"
			dev.CamFPS = 0

			statusPath := statusFilePath(dev.UDID)
			if data, err := os.ReadFile(statusPath); err == nil {
				var st camFrameLoopStatus
				if json.Unmarshal(data, &st) == nil && st.Running {
					dev.CamStatus = "Running"
					dev.CamSource = st.Source
					dev.CamFPS = st.FPS
				}
			}
			booted = append(booted, dev)
		}
		return camRefreshMsg(booted)
	}
}

func fetchAppsCmd(udid string) tea.Cmd {
	return func() tea.Msg {
		apps, err := fetchInstalledApps(udid)
		if err != nil {
			return actionDoneMsg{msg: "Error fetching apps: " + err.Error()}
		}
		return appsFetchedMsg(apps)
	}
}

func getAvailableCameras() ([]CameraInfo, error) {
	irisDirVal := getIrisDir()
	bin := frameHostBin(irisDirVal)
	if _, err := os.Stat(bin); err != nil {
		return nil, fmt.Errorf("FrameHost binary not found")
	}

	out, err := exec.Command(bin, "--list-cameras", "--json").Output()
	if err != nil {
		return nil, fmt.Errorf("failed to list cameras: %w", err)
	}

	var cameras []CameraInfo
	if err := json.Unmarshal(out, &cameras); err != nil {
		return nil, fmt.Errorf("failed to parse cameras: %w", err)
	}
	return cameras, nil
}

func fetchCamerasCmd() tea.Cmd {
	return func() tea.Msg {
		cams, err := getAvailableCameras()
		if err != nil {
			return actionDoneMsg{msg: "Error fetching cameras: " + err.Error()}
		}
		return camerasFetchedMsg(cams)
	}
}

func (m camDashboardModel) Init() tea.Cmd {
	var cmds []tea.Cmd
	cmds = append(cmds, m.spinner.Tick, refreshCamDevicesCmd(), camTickCmd())
	if len(m.simulators) > 0 {
		cmds = append(cmds, fetchCamerasCmd(), fetchAppsCmd(m.selectedSimUDID))
	}
	return tea.Batch(cmds...)
}

type camTickMsg time.Time

func camTickCmd() tea.Cmd {
	return tea.Tick(time.Second*5, func(t time.Time) tea.Msg {
		return camTickMsg(t)
	})
}

func updateTableFocus(m *camDashboardModel) {
	m.simTable.Focus()
	m.cameraTable.Focus()
	m.appTable.Focus()

	if m.focused != paneSimulators {
		m.simTable.Blur()
	}
	if m.focused != paneCameras {
		m.cameraTable.Blur()
	}
	if m.focused != paneApps {
		m.appTable.Blur()
	}
}

func (m *camDashboardModel) updateFilteredApps() {
	var filtered []SimCamApp
	query := strings.ToLower(m.filterInput.Value())

	for _, app := range m.allApps {
		if !m.showSystemApps && app.ApplicationType == "System" {
			continue
		}
		if query != "" {
			titleMatch := strings.Contains(strings.ToLower(app.Title()), query)
			bundleMatch := strings.Contains(strings.ToLower(app.BundleID), query)
			if !titleMatch && !bundleMatch {
				continue
			}
		}
		filtered = append(filtered, app)
	}
	m.filteredApps = filtered

	rows := make([]table.Row, 0, len(filtered))
	for _, app := range filtered {
		appType := app.ApplicationType
		if appType == "" {
			appType = "User"
		}
		rows = append(rows, table.Row{
			app.Title(),
			app.BundleID,
			appType,
		})
	}
	m.appTable.SetRows(rows)
}

func (m camDashboardModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmds []tea.Cmd

	switch msg := msg.(type) {
	case tea.MouseMsg:
		if m.showPopup {
			break
		}
		if msg.Action == tea.MouseActionRelease && msg.Button == tea.MouseButtonLeft {
			availWidth := m.width - 8
			if availWidth < 90 {
				availWidth = 90
			}
			w1 := int(float64(availWidth)*0.35) + 2
			w2 := int(float64(availWidth)*0.30) + 2

			oldFocus := m.focused
			if msg.X < w1 {
				m.focused = paneSimulators
			} else if msg.X < w1+w2 {
				m.focused = paneCameras
			} else {
				m.focused = paneApps
			}
			if oldFocus != m.focused {
				updateTableFocus(&m)
			}
		}

		var cmd tea.Cmd
		switch m.focused {
		case paneSimulators:
			m.simTable, cmd = m.simTable.Update(msg)
		case paneCameras:
			m.cameraTable, cmd = m.cameraTable.Update(msg)
		case paneApps:
			m.appTable, cmd = m.appTable.Update(msg)
		}
		cmds = append(cmds, cmd)

	case tea.KeyMsg:
		if m.showPopup {
			switch msg.String() {
			case "q", "ctrl+c":
				return m, tea.Quit
			case "esc":
				m.showPopup = false
				return m, nil
			case "enter":
				if i, ok := m.presetList.SelectedItem().(ResolutionPreset); ok {
					m.camWidth = i.width
					m.camHeight = i.height
					m.msg = fmt.Sprintf("Resolution set to %dx%d", i.width, i.height)
				}
				m.showPopup = false
				return m, nil
			}
			var cmd tea.Cmd
			m.presetList, cmd = m.presetList.Update(msg)
			cmds = append(cmds, cmd)
			return m, tea.Batch(cmds...)
		}

		if m.focused == paneApps && m.filterInput.Focused() {
			if msg.String() == "enter" || msg.String() == "esc" {
				m.filterInput.Blur()
			} else {
				var inputCmd tea.Cmd
				m.filterInput, inputCmd = m.filterInput.Update(msg)
				m.updateFilteredApps()
				cmds = append(cmds, inputCmd)
				return m, tea.Batch(cmds...)
			}
		} else {
			switch msg.String() {
			case "q", "ctrl+c":
				return m, tea.Quit
			case "tab", "l", "right":
				m.focused = (m.focused + 1) % 3
				updateTableFocus(&m)
				return m, nil
			case "shift+tab", "h", "left":
				m.focused = (m.focused + 2) % 3
				updateTableFocus(&m)
				return m, nil
			case "o":
				m.showPopup = true
				return m, nil
			case "x":
				if m.selectedSimUDID != "" {
					m.loading = true
					m.msg = "Stopping camera feed for " + m.selectedSimName + "..."
					cmds = append(cmds, doActionCmd(func() error {
						return stopFrameHost(m.selectedSimUDID)
					}, "Stopped camera feed for "+m.selectedSimName))
				}
			case "r":
				m.loading = true
				m.msg = "Refreshing..."
				cmds = append(cmds, refreshCamDevicesCmd())
			case "t":
				if m.focused == paneApps {
					m.showSystemApps = !m.showSystemApps
					m.updateFilteredApps()
				}
			case "/":
				if m.focused == paneApps {
					m.filterInput.Focus()
				}
			case "enter":
				switch m.focused {
				case paneSimulators:
					m.focused = paneCameras
					updateTableFocus(&m)
				case paneCameras:
					idx := m.cameraTable.Cursor()
					if idx >= 0 && idx < len(m.cameras) {
						camID := m.cameras[idx].UniqueID
						m.loading = true
						m.msg = "Starting camera feed for " + m.selectedSimName + "..."
						cmds = append(cmds, doActionCmd(func() error {
							return startCameraForDevice(m.selectedSimUDID, true, "", camID, m.camWidth, m.camHeight)
						}, "Started camera feed for "+m.selectedSimName))
					}
				case paneApps:
					idx := m.appTable.Cursor()
					if idx >= 0 && idx < len(m.filteredApps) {
						bundleID := m.filteredApps[idx].BundleID
						appName := m.filteredApps[idx].Title()
						m.loading = true
						m.msg = fmt.Sprintf("Launching %s...", appName)
						udid := m.selectedSimUDID
						cmds = append(cmds, doActionCmd(func() error {
							return launchAppWithCam(udid, bundleID)
						}, fmt.Sprintf("Launched %s!", appName)))
					}
				}
			}
		}

	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height

		tHeight := m.height - 8
		if tHeight < 5 {
			tHeight = 5
		}

		availWidth := m.width - 8
		if availWidth < 90 {
			availWidth = 90
		}

		w1 := int(float64(availWidth) * 0.35)
		w2 := int(float64(availWidth) * 0.30)
		w3 := availWidth - (w1 + w2)

		m.simTable.SetHeight(tHeight - 2)
		m.cameraTable.SetHeight(tHeight - 2)
		m.appTable.SetHeight(tHeight - 3)

		sw1 := int(float64(w1) * 0.5)
		sw2 := int(float64(w1) * 0.25)
		sw3 := w1 - (sw1 + sw2) - 4
		m.simTable.SetColumns([]table.Column{
			{Title: "Name", Width: sw1},
			{Title: "Status", Width: sw2},
			{Title: "Source", Width: sw3},
		})

		cw1 := int(float64(w2) * 0.5)
		cw2 := int(float64(w2) * 0.3)
		cw3 := w2 - (cw1 + cw2) - 4
		m.cameraTable.SetColumns([]table.Column{
			{Title: "Camera", Width: cw1},
			{Title: "Type", Width: cw2},
			{Title: "ID", Width: cw3},
		})

		aw1 := int(float64(w3) * 0.4)
		aw2 := int(float64(w3) * 0.4)
		aw3 := w3 - (aw1 + aw2) - 4
		m.appTable.SetColumns([]table.Column{
			{Title: "App Name", Width: aw1},
			{Title: "Bundle ID", Width: aw2},
			{Title: "Type", Width: aw3},
		})

		m.presetList.SetSize(60, 20)

	case camRefreshMsg:
		rows := make([]table.Row, 0, len(msg))
		for _, d := range msg {
			rows = append(rows, table.Row{
				d.Name,
				d.CamStatus,
				d.CamSource,
			})
		}
		m.simTable.SetRows(rows)
		m.simulators = msg
		m.loading = false
		if m.msg == "Refreshing..." {
			m.msg = "Refreshed list."
		}

		if len(m.simulators) > 0 && m.selectedSimUDID == "" {
			m.selectedSimUDID = m.simulators[0].UDID
			m.selectedSimName = m.simulators[0].Name
			cmds = append(cmds, fetchCamerasCmd(), fetchAppsCmd(m.selectedSimUDID))
		}

	case appsFetchedMsg:
		m.allApps = msg
		m.updateFilteredApps()
		m.loading = false
		m.msg = fmt.Sprintf("Loaded %d apps.", len(msg))

	case camerasFetchedMsg:
		m.cameras = msg
		rows := make([]table.Row, 0, len(msg))
		for _, cam := range msg {
			displayID := cam.UniqueID
			if len(displayID) > 8 {
				displayID = displayID[:8] + "…"
			}
			rows = append(rows, table.Row{
				cam.LocalizedName,
				cam.TypeLabel,
				displayID,
			})
		}
		m.cameraTable.SetRows(rows)
		m.loading = false
		m.msg = fmt.Sprintf("Found %d cameras.", len(msg))

	case actionDoneMsg:
		m.msg = msg.msg
		m.loading = false
		cmds = append(cmds, refreshCamStatusOnlyCmd(m.simulators))

	case camTickMsg:
		cmds = append(cmds, camTickCmd(), refreshCamDevicesCmd())

	case spinner.TickMsg:
		var cmd tea.Cmd
		m.spinner, cmd = m.spinner.Update(msg)
		cmds = append(cmds, cmd)
	}

	oldSimUDID := m.selectedSimUDID

	if m.focused == paneSimulators && !m.showPopup {
		var tableCmd tea.Cmd
		m.simTable, tableCmd = m.simTable.Update(msg)
		cmds = append(cmds, tableCmd)

		idx := m.simTable.Cursor()
		if idx >= 0 && idx < len(m.simulators) {
			m.selectedSimUDID = m.simulators[idx].UDID
			m.selectedSimName = m.simulators[idx].Name
		}
	} else if m.focused == paneCameras && !m.showPopup {
		var camTableCmd tea.Cmd
		m.cameraTable, camTableCmd = m.cameraTable.Update(msg)
		cmds = append(cmds, camTableCmd)
	} else if m.focused == paneApps && !m.showPopup {
		var appTableCmd tea.Cmd
		m.appTable, appTableCmd = m.appTable.Update(msg)
		cmds = append(cmds, appTableCmd)
	}

	if oldSimUDID != m.selectedSimUDID && m.selectedSimUDID != "" {
		m.loading = true
		m.msg = "Loading details for " + m.selectedSimName + "..."
		cmds = append(cmds, fetchCamerasCmd(), fetchAppsCmd(m.selectedSimUDID))
	}

	return m, tea.Batch(cmds...)
}

func (m camDashboardModel) View() string {
	activeColor := lipgloss.Color("62")    // Purple highlight
	inactiveColor := lipgloss.Color("240") // Gray

	borderStyle := func(pane focusPane) lipgloss.Style {
		c := inactiveColor
		if m.focused == pane {
			c = activeColor
		}
		return lipgloss.NewStyle().
			BorderStyle(lipgloss.NormalBorder()).
			BorderForeground(c).
			Height(m.height - 4)
	}

	pane1 := borderStyle(paneSimulators).Render(
		lipgloss.NewStyle().Bold(true).PaddingLeft(1).Render("Booted Simulators") + "\n\n" + m.simTable.View(),
	)

	pane2 := borderStyle(paneCameras).Render(
		lipgloss.NewStyle().Bold(true).PaddingLeft(1).Render("Cameras") + "\n\n" + m.cameraTable.View(),
	)

	appHeader := "Apps"
	if m.showSystemApps {
		appHeader += " [All]"
	} else {
		appHeader += " [User]"
	}
	pane3 := borderStyle(paneApps).Render(
		lipgloss.NewStyle().Bold(true).PaddingLeft(1).Render(appHeader) + "\n" +
			" Search (/): " + m.filterInput.View() + "\n" +
			m.appTable.View(),
	)

	mainContent := lipgloss.JoinHorizontal(lipgloss.Top, pane1, pane2, pane3)

	footer := " Controls: [tab/h/l] Switch Pane • [j/k] Navigate • [enter] Action • [x] Stop Cam • [o] Resolution • [t] Toggle SysApps • [r] Refresh • [q] Quit\n "
	if m.loading {
		footer += m.spinner.View() + " " + lipgloss.NewStyle().Foreground(ColorIOS).Render(m.msg)
	} else if m.msg != "" {
		footer += "ℹ " + lipgloss.NewStyle().Foreground(ColorIOS).Render(m.msg)
	}

	ui := lipgloss.JoinVertical(lipgloss.Left, mainContent, footer)

	if m.showPopup {
		popupStyle := lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(lipgloss.Color("62")).
			Padding(1, 2).
			Background(lipgloss.Color("236")).
			Foreground(lipgloss.Color("252"))

		popupView := popupStyle.Render(m.presetList.View())

		// Create a full screen container and place the popup in the center
		ui = lipgloss.Place(m.width, m.height, lipgloss.Center, lipgloss.Center, popupView, lipgloss.WithWhitespaceChars(" "))
	}

	return ui
}

func startCameraForDevice(udid string, useCamera bool, imagePath string, cameraID string, width, height int) error {
	irisDirVal := getIrisDir()
	bin := frameHostBin(irisDirVal)
	if _, err := os.Stat(bin); err != nil {
		return fmt.Errorf("FrameHost binary not found")
	}

	_ = stopFrameHost(udid)
	_ = os.Remove(statusFilePath(udid))

	args := []string{
		"--udid", udid,
		"--width", fmt.Sprintf("%d", width),
		"--height", fmt.Sprintf("%d", height),
		"--fps", fmt.Sprintf("%d", DefaultCamFPS),
	}

	if useCamera {
		args = append(args, "--camera")
		if cameraID != "" {
			args = append(args, "--camera-id", cameraID)
		}
	} else if imagePath != "" {
		args = append(args, "--image", imagePath)
	} else {
		args = append(args, "--bars")
	}

	c := exec.Command(bin, args...)
	if !useCamera {
		c.SysProcAttr = &syscall.SysProcAttr{Setsid: true}
	}
	if err := c.Start(); err != nil {
		return err
	}

	setGlobalSimEnv(udid, DefaultCamFPS)

	return waitForFrameHostReady(c, statusFilePath(udid))
}

func launchAppWithCam(udid, bundleID string) error {
	irisDirVal := getIrisDir()
	dylib := injectorDylib(irisDirVal)
	if _, err := os.Stat(dylib); err != nil {
		return fmt.Errorf("IrisInject.dylib not found")
	}

	shm := shmPath(udid)
	fps := frameHostFPS(udid)

	c := exec.Command("xcrun", "simctl", "launch", udid, bundleID)
	c.Env = append(os.Environ(),
		"SIMCTL_CHILD_DYLD_INSERT_LIBRARIES="+dylib,
		"SIMCTL_CHILD_IRIS_PATH="+shm,
		"SIMCTL_CHILD_IRIS_FPS="+fmt.Sprintf("%d", fps),
	)
	return c.Run()
}

func runCamDashboard() error {
	initialSims := fetchBootedSimulatorsWithCamStatus()

	simCols := []table.Column{
		{Title: "Name", Width: 20},
		{Title: "Status", Width: 15},
		{Title: "Source", Width: 15},
	}

	simRows := make([]table.Row, 0, len(initialSims))
	for _, d := range initialSims {
		simRows = append(simRows, table.Row{
			d.Name,
			d.CamStatus,
			d.CamSource,
		})
	}

	tSim := table.New(
		table.WithColumns(simCols),
		table.WithRows(simRows),
		table.WithFocused(true),
		table.WithHeight(12),
	)

	tApp := table.New(
		table.WithColumns([]table.Column{
			{Title: "App Name", Width: 20},
			{Title: "Bundle ID", Width: 20},
			{Title: "Type", Width: 10},
		}),
		table.WithFocused(false),
		table.WithHeight(10),
	)

	tCam := table.New(
		table.WithColumns([]table.Column{
			{Title: "Camera Name", Width: 20},
			{Title: "Type", Width: 15},
			{Title: "Unique ID", Width: 15},
		}),
		table.WithFocused(false),
		table.WithHeight(10),
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

	tSim.SetStyles(s)
	tApp.SetStyles(s)
	tCam.SetStyles(s)

	sp := spinner.New()
	sp.Spinner = spinner.Dot
	sp.Style = lipgloss.NewStyle().Foreground(ColorHeader)

	ti := textinput.New()
	ti.Placeholder = "Search..."
	ti.CharLimit = 156
	ti.Width = 20

	presetItems := []list.Item{
		ResolutionPreset{title: "720p (16:9)", description: "1280 x 720 (Default)", width: 1280, height: 720},
		ResolutionPreset{title: "1080p (16:9)", description: "1920 x 1080", width: 1920, height: 1080},
		ResolutionPreset{title: "4K (16:9)", description: "3840 x 2160", width: 3840, height: 2160},
		ResolutionPreset{title: "Vertical HD (9:16)", description: "1080 x 1920", width: 1080, height: 1920},
		ResolutionPreset{title: "Square (1:1)", description: "1080 x 1080", width: 1080, height: 1080},
	}
	presetList := list.New(presetItems, list.NewDefaultDelegate(), 60, 20)
	presetList.Title = "Select Camera Resolution & Aspect Ratio"
	presetList.SetShowHelp(false)
	presetList.SetShowStatusBar(false)

	m := camDashboardModel{
		focused:     paneSimulators,
		simTable:    tSim,
		appTable:    tApp,
		cameraTable: tCam,
		presetList:  presetList,
		filterInput: ti,
		spinner:     sp,
		msg:         "Ready.",
		simulators:  initialSims,
		camWidth:    DefaultCamWidth,
		camHeight:   DefaultCamHeight,
	}

	if len(initialSims) > 0 {
		m.selectedSimUDID = initialSims[0].UDID
		m.selectedSimName = initialSims[0].Name
	}

	if _, err := tea.NewProgram(m, tea.WithAltScreen(), tea.WithMouseCellMotion()).Run(); err != nil {
		return err
	}

	return nil
}
