package cmd

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os/exec"
	"runtime"
	"sort"
	"strings"

	"github.com/charmbracelet/bubbles/spinner"
	"github.com/charmbracelet/bubbles/table"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/spf13/cobra"
)

// ---------------------------------------------------------------------------
// Cobra command
// ---------------------------------------------------------------------------

var defaultsCmd = &cobra.Command{
	Use:     "defaults [device-name-or-udid]",
	Aliases: []string{"df", "prefs"},
	Short:   "Interactively browse and edit UserDefaults on an iOS simulator",
	Long: `Open a TUI to browse and edit UserDefaults (plist) values for
apps installed on a booted iOS simulator.

If no device is specified, the first booted iOS simulator is used.

Controls inside the TUI:
  App list:  [↑/↓] navigate  [enter] select  [a] toggle system apps  [q] quit
  Key list:  [↑/↓] navigate  [e/enter] edit   [d] delete  [n] new key  [esc] back
  Edit mode: [enter] confirm  [esc] cancel`,
	Args: cobra.MaximumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		if runtime.GOOS != DarwinOS {
			return ErrIOSMacOnly
		}

		showAll, _ := cmd.Flags().GetBool("all")

		// Resolve device
		var udid, name string
		if len(args) == 1 {
			dev := FindIOSSimulatorByID(args[0])
			if dev == nil {
				return fmt.Errorf("device %q: %w", args[0], ErrDeviceNotFound)
			}
			if dev.State != StateBooted {
				return fmt.Errorf("device %q: %w", args[0], ErrDeviceNotRunning)
			}
			udid = dev.UDID
			name = dev.Name
		} else {
			mgr := &IOSManager{}
			u, n, found, err := mgr.FindRunningDevice("")
			if err != nil {
				return err
			}
			if !found {
				return ErrNoRunningIOSSimulator
			}
			udid = u
			name = n
		}

		m, err := newDefaultsModel(udid, name, showAll)
		if err != nil {
			return err
		}
		_, err = tea.NewProgram(m, tea.WithAltScreen()).Run()
		return err
	},
}

func init() {
	defaultsCmd.Flags().BoolP("all", "a", false, "Include Apple system apps in the list")
}

// ---------------------------------------------------------------------------
// Data types
// ---------------------------------------------------------------------------

// AppInfo holds basic information about an installed simulator app.
type AppInfo struct {
	BundleID    string
	DisplayName string
}

// DefaultEntry represents a single UserDefaults key-value pair.
type DefaultEntry struct {
	Key   string
	Type  string // "string", "integer", "float", "bool", "array", "dict", "data", "date"
	Value string // human-readable display string
	Raw   interface{}
}

// ---------------------------------------------------------------------------
// plutil backend functions
// ---------------------------------------------------------------------------

// listInstalledApps returns all installed apps on the simulator, optionally
// filtering out com.apple.* system apps.
func listInstalledApps(udid string, showAll bool) ([]AppInfo, error) {
	// xcrun simctl listapps outputs an old-style ASCII plist; convert to JSON via plutil.
	listOut, err := exec.Command(CmdXCrun, CmdSimctl, "listapps", udid).Output()
	if err != nil {
		return nil, fmt.Errorf("failed to list apps: %w", err)
	}

	// plutil reads from stdin when given "-" as the file argument.
	cmd := exec.Command(CmdPlutil, "-convert", "json", "-o", "-", "--", "-")
	cmd.Stdin = bytes.NewReader(listOut)
	jsonOut, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("failed to convert app list to JSON: %w", err)
	}

	var raw map[string]map[string]interface{}
	if err = json.Unmarshal(jsonOut, &raw); err != nil {
		return nil, fmt.Errorf("failed to parse app list JSON: %w", err)
	}

	apps := make([]AppInfo, 0, len(raw))
	for bundleID, meta := range raw {
		if !showAll && strings.HasPrefix(bundleID, "com.apple.") {
			continue
		}
		displayName := bundleID
		if v, ok := meta["CFBundleDisplayName"].(string); ok && v != "" {
			displayName = v
		} else if v, ok := meta["CFBundleName"].(string); ok && v != "" {
			displayName = v
		}
		apps = append(apps, AppInfo{BundleID: bundleID, DisplayName: displayName})
	}

	sort.Slice(apps, func(i, j int) bool {
		return apps[i].DisplayName < apps[j].DisplayName
	})

	return apps, nil
}

// getDefaultsPlistPath returns the path to the UserDefaults plist for the given app.
func getDefaultsPlistPath(udid, bundleID string) (string, error) {
	out, err := exec.Command(CmdXCrun, CmdSimctl, "get_app_container", udid, bundleID, "data").Output()
	if err != nil {
		return "", fmt.Errorf("failed to get data container for %q: %w", bundleID, err)
	}
	dataContainer := strings.TrimSpace(string(out))
	return fmt.Sprintf("%s/Library/Preferences/%s.plist", dataContainer, bundleID), nil
}

// readDefaults reads all key-value pairs from the plist at plistPath.
func readDefaults(plistPath string) ([]DefaultEntry, error) {
	out, err := exec.Command(CmdPlutil, "-convert", "json", "-o", "-", plistPath).Output()
	if err != nil {
		return nil, ErrDefaultsNotFound
	}

	var raw map[string]interface{}
	if err = json.Unmarshal(out, &raw); err != nil {
		return nil, fmt.Errorf("failed to parse plist: %w", err)
	}

	entries := make([]DefaultEntry, 0, len(raw))
	for k, v := range raw {
		t, disp := inferType(v)
		entries = append(entries, DefaultEntry{Key: k, Type: t, Value: disp, Raw: v})
	}
	sort.Slice(entries, func(i, j int) bool {
		return entries[i].Key < entries[j].Key
	})
	return entries, nil
}

// inferType returns a display type string and human-readable value for a parsed JSON value.
func inferType(v interface{}) (typeName, display string) {
	switch val := v.(type) {
	case bool:
		if val {
			return "bool", "true"
		}
		return "bool", "false"
	case float64:
		if val == float64(int64(val)) {
			return "integer", fmt.Sprintf("%d", int64(val))
		}
		return "float", fmt.Sprintf("%g", val)
	case string:
		return "string", val
	case []interface{}:
		b, _ := json.Marshal(val)
		return "array", string(b)
	case map[string]interface{}:
		b, _ := json.Marshal(val)
		return "dict", string(b)
	case nil:
		return "null", "<null>"
	default:
		return "unknown", fmt.Sprintf("%v", v)
	}
}

// parseValueForType converts a user-supplied string into the Go value for the
// given plist type name, ready to be marshalled back to JSON.
func parseValueForType(value, typeName string) (interface{}, error) {
	switch typeName {
	case "integer":
		var n int64
		if _, err := fmt.Sscanf(value, "%d", &n); err != nil {
			return nil, fmt.Errorf("invalid integer %q: %w", value, err)
		}
		return float64(n), nil // JSON numbers stored as float64
	case "float":
		var f float64
		if _, err := fmt.Sscanf(value, "%g", &f); err != nil {
			return nil, fmt.Errorf("invalid float %q: %w", value, err)
		}
		return f, nil
	case "bool":
		v := strings.ToLower(strings.TrimSpace(value))
		if v == "true" || v == "yes" || v == "1" {
			return true, nil
		}
		if v == "false" || v == "no" || v == "0" {
			return false, nil
		}
		return nil, fmt.Errorf("invalid bool %q: use true/false/yes/no/1/0", value)
	case "array", "dict":
		var v interface{}
		if err := json.Unmarshal([]byte(value), &v); err != nil {
			return nil, fmt.Errorf("invalid JSON for %s: %w", typeName, err)
		}
		return v, nil
	default: // string
		return value, nil
	}
}

// readPlistAsMap reads a plist file and returns its top-level keys as a
// map[string]interface{} by converting to JSON via plutil.
func readPlistAsMap(plistPath string) (map[string]interface{}, error) {
	out, err := exec.Command(CmdPlutil, "-convert", "json", "-o", "-", plistPath).Output()
	if err != nil {
		return nil, fmt.Errorf("failed to read plist: %w", err)
	}
	var raw map[string]interface{}
	if err = json.Unmarshal(out, &raw); err != nil {
		return nil, fmt.Errorf("failed to parse plist JSON: %w", err)
	}
	return raw, nil
}

// writePlistFromMap writes a map back to a plist file as JSON piped through
// plutil. This avoids any keypath interpretation issues.
func writePlistFromMap(plistPath string, raw map[string]interface{}) error {
	newJSON, err := json.Marshal(raw)
	if err != nil {
		return fmt.Errorf("failed to marshal plist data: %w", err)
	}
	// Use json → binary1 (native iOS simulator format). Fall back to xml1.
	for _, fmt2 := range []string{"binary1", "xml1"} {
		cmd := exec.Command(CmdPlutil, "-convert", fmt2, "-o", plistPath, "--", "-")
		cmd.Stdin = bytes.NewReader(newJSON)
		if out, err2 := cmd.CombinedOutput(); err2 == nil {
			_ = out
			return nil
		}
	}
	return fmt.Errorf("failed to write plist: could not convert to binary1 or xml1")
}

// writeDefaultValue replaces an existing key's value in the plist using a
// safe read → modify → write approach that avoids plutil keypath issues.
func writeDefaultValue(plistPath, key, typeName, value string) error {
	raw, err := readPlistAsMap(plistPath)
	if err != nil {
		return err
	}
	parsed, err := parseValueForType(value, typeName)
	if err != nil {
		return err
	}
	raw[key] = parsed
	return writePlistFromMap(plistPath, raw)
}

// deleteDefaultKey removes a key from the plist using a safe read → modify → write approach.
func deleteDefaultKey(plistPath, key string) error {
	raw, err := readPlistAsMap(plistPath)
	if err != nil {
		return err
	}
	delete(raw, key)
	return writePlistFromMap(plistPath, raw)
}

// addDefaultKey inserts a new key into the plist using a safe read → modify → write approach.
func addDefaultKey(plistPath, key, typeName, value string) error {
	raw, err := readPlistAsMap(plistPath)
	if err != nil {
		return err
	}
	if _, exists := raw[key]; exists {
		return fmt.Errorf("key %q already exists; use edit to change its value", key)
	}
	parsed, err := parseValueForType(value, typeName)
	if err != nil {
		return err
	}
	raw[key] = parsed
	return writePlistFromMap(plistPath, raw)
}

// ---------------------------------------------------------------------------
// TUI views
// ---------------------------------------------------------------------------

const (
	viewApps          = "apps"
	viewKeys          = "keys"
	viewEdit          = "edit"
	viewConfirmDelete = "confirmDelete"
	viewNewKeyName    = "newKeyName"
	viewNewKeyType    = "newKeyType"
	viewNewKeyValue   = "newKeyValue"
)

// ---------------------------------------------------------------------------
// Messages
// ---------------------------------------------------------------------------

type appsLoadedMsg struct{ apps []AppInfo }
type appsErrMsg struct{ err error }
type keysLoadedMsg struct {
	entries   []DefaultEntry
	plistPath string
}
type keysErrMsg struct{ err error }
type actionDoneDefaultsMsg struct{ err error }

// ---------------------------------------------------------------------------
// Model
// ---------------------------------------------------------------------------

type defaultsModel struct {
	// Config
	deviceUDID  string
	deviceName  string
	showAllApps bool

	// View state
	view string

	// App list
	apps     []AppInfo
	appTable table.Model

	// Key-value view
	selectedApp AppInfo
	plistPath   string
	entries     []DefaultEntry
	keyTable    table.Model

	// Edit / new key
	editKey     string // key being edited
	editType    string // type of key being edited
	textInput   textinput.Model
	newKeyName  string
	newKeyType  string
	typeChoices []string
	typeCursor  int

	// Feedback
	spinner   spinner.Model
	loading   bool
	statusMsg string

	// Terminal size
	width  int
	height int
}

func newDefaultsModel(udid, name string, showAll bool) (*defaultsModel, error) {
	sp := spinner.New()
	sp.Spinner = spinner.Dot
	sp.Style = lipgloss.NewStyle().Foreground(ColorHeader)

	ti := textinput.New()
	ti.CharLimit = 512

	appTbl := buildAppTable(nil, 80, 20)
	keyTbl := buildKeyTable(nil, 80, 20)

	m := &defaultsModel{
		deviceUDID:  udid,
		deviceName:  name,
		showAllApps: showAll,
		view:        viewApps,
		appTable:    appTbl,
		keyTable:    keyTbl,
		textInput:   ti,
		spinner:     sp,
		loading:     true,
		statusMsg:   "Loading apps...",
		typeChoices: []string{"string", "integer", "float", "bool"},
	}
	return m, nil
}

// ---------------------------------------------------------------------------
// Table builders
// ---------------------------------------------------------------------------

func buildAppTable(apps []AppInfo, width, height int) table.Model {
	availWidth := width - 4
	if availWidth < 40 {
		availWidth = 40
	}
	w1 := availWidth / 2
	w2 := availWidth - w1

	cols := []table.Column{
		{Title: "App Name", Width: w1},
		{Title: "Bundle ID", Width: w2},
	}

	rows := make([]table.Row, 0, len(apps))
	for _, a := range apps {
		rows = append(rows, table.Row{a.DisplayName, a.BundleID})
	}

	t := table.New(
		table.WithColumns(cols),
		table.WithRows(rows),
		table.WithFocused(true),
		table.WithHeight(height),
	)
	applyTableStyles(&t)
	return t
}

func buildKeyTable(entries []DefaultEntry, width, height int) table.Model {
	availWidth := width - 4
	if availWidth < 60 {
		availWidth = 60
	}
	w1 := int(float64(availWidth) * 0.35)
	w2 := int(float64(availWidth) * 0.12)
	w3 := availWidth - w1 - w2

	cols := []table.Column{
		{Title: "Key", Width: w1},
		{Title: "Type", Width: w2},
		{Title: "Value", Width: w3},
	}

	rows := make([]table.Row, 0, len(entries))
	for _, e := range entries {
		val := e.Value
		if len(val) > w3-2 {
			val = val[:w3-5] + "..."
		}
		rows = append(rows, table.Row{e.Key, e.Type, val})
	}

	t := table.New(
		table.WithColumns(cols),
		table.WithRows(rows),
		table.WithFocused(true),
		table.WithHeight(height),
	)
	applyTableStyles(&t)
	return t
}

func applyTableStyles(t *table.Model) {
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
}

// ---------------------------------------------------------------------------
// Init
// ---------------------------------------------------------------------------

func (m defaultsModel) Init() tea.Cmd {
	return tea.Batch(
		m.spinner.Tick,
		loadAppsCmd(m.deviceUDID, m.showAllApps),
	)
}

func loadAppsCmd(udid string, showAll bool) tea.Cmd {
	return func() tea.Msg {
		apps, err := listInstalledApps(udid, showAll)
		if err != nil {
			return appsErrMsg{err: err}
		}
		return appsLoadedMsg{apps: apps}
	}
}

func loadKeysCmd(udid, bundleID string) tea.Cmd {
	return func() tea.Msg {
		path, err := getDefaultsPlistPath(udid, bundleID)
		if err != nil {
			return keysErrMsg{err: err}
		}
		entries, err := readDefaults(path)
		if err != nil {
			return keysErrMsg{err: fmt.Errorf("%w (the app may not have any UserDefaults yet)", err)}
		}
		return keysLoadedMsg{entries: entries, plistPath: path}
	}
}

// ---------------------------------------------------------------------------
// Update
// ---------------------------------------------------------------------------

//nolint:gocyclo,cyclop,funlen
func (m defaultsModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmds []tea.Cmd

	switch msg := msg.(type) {

	// --- Window resize ---
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		m = m.recalcTableSizes()
		return m, nil

	// --- Spinner tick ---
	case spinner.TickMsg:
		var cmd tea.Cmd
		m.spinner, cmd = m.spinner.Update(msg)
		return m, cmd

	// --- Data loaded ---
	case appsLoadedMsg:
		m.apps = msg.apps
		m.loading = false
		if len(m.apps) == 0 {
			m.statusMsg = "No apps found. Press [a] to show system apps."
		} else {
			m.statusMsg = fmt.Sprintf("%d apps loaded.", len(m.apps))
		}
		m.appTable = buildAppTable(m.apps, m.width, m.tableHeight()-2)
		return m, nil

	case appsErrMsg:
		m.loading = false
		m.statusMsg = "Error: " + msg.err.Error()
		return m, nil

	case keysLoadedMsg:
		m.entries = msg.entries
		m.plistPath = msg.plistPath
		m.loading = false
		m.view = viewKeys
		if len(m.entries) == 0 {
			m.statusMsg = "No UserDefaults keys found for this app."
		} else {
			m.statusMsg = fmt.Sprintf("%d keys loaded. [e/enter] Edit • [d] Delete • [n] New key", len(m.entries))
		}
		m.keyTable = buildKeyTable(m.entries, m.width, m.tableHeight()-2)
		return m, nil

	case keysErrMsg:
		m.loading = false
		m.statusMsg = "Error: " + msg.err.Error()
		return m, nil

	case actionDoneDefaultsMsg:
		if msg.err != nil {
			m.statusMsg = "Error: " + msg.err.Error()
			m.loading = false
			return m, nil
		}
		// Reload keys after action
		m.loading = true
		m.statusMsg = "Reloading..."
		return m, tea.Batch(m.spinner.Tick, loadKeysCmd(m.deviceUDID, m.selectedApp.BundleID))

	// --- Keyboard ---
	case tea.KeyMsg:
		return m.handleKey(msg)
	}

	// Propagate to active table
	switch m.view {
	case viewApps:
		var cmd tea.Cmd
		m.appTable, cmd = m.appTable.Update(msg)
		cmds = append(cmds, cmd)
	case viewKeys:
		var cmd tea.Cmd
		m.keyTable, cmd = m.keyTable.Update(msg)
		cmds = append(cmds, cmd)
	case viewEdit, viewNewKeyName, viewNewKeyValue:
		var cmd tea.Cmd
		m.textInput, cmd = m.textInput.Update(msg)
		cmds = append(cmds, cmd)
	}

	return m, tea.Batch(cmds...)
}

//nolint:gocyclo,cyclop,funlen
func (m defaultsModel) handleKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch m.view {

	// -----------------------------------------------------------------------
	// App list view
	// -----------------------------------------------------------------------
	case viewApps:
		switch msg.String() {
		case "q", "ctrl+c":
			return m, tea.Quit

		case "a":
			m.showAllApps = !m.showAllApps
			m.loading = true
			m.statusMsg = "Loading apps..."
			return m, tea.Batch(m.spinner.Tick, loadAppsCmd(m.deviceUDID, m.showAllApps))

		case "enter":
			row := m.appTable.SelectedRow()
			if len(row) < 2 {
				return m, nil
			}
			bundleID := row[1]
			for _, a := range m.apps {
				if a.BundleID == bundleID {
					m.selectedApp = a
					break
				}
			}
			m.loading = true
			m.statusMsg = "Loading UserDefaults for " + m.selectedApp.BundleID + "..."
			return m, tea.Batch(m.spinner.Tick, loadKeysCmd(m.deviceUDID, m.selectedApp.BundleID))

		default:
			var cmd tea.Cmd
			m.appTable, cmd = m.appTable.Update(msg)
			return m, cmd
		}

	// -----------------------------------------------------------------------
	// Key-value view
	// -----------------------------------------------------------------------
	case viewKeys:
		switch msg.String() {
		case "q", "ctrl+c":
			return m, tea.Quit

		case "esc":
			m.view = viewApps
			m.statusMsg = fmt.Sprintf("%d apps. [enter] Select app • [a] Toggle system apps", len(m.apps))
			return m, nil

		case "e", "enter":
			row := m.keyTable.SelectedRow()
			if len(row) < 3 {
				return m, nil
			}
			// Find the full entry
			for _, e := range m.entries {
				if e.Key == row[0] {
					m.editKey = e.Key
					m.editType = e.Type
					m.view = viewEdit
					m.textInput.Reset()
					m.textInput.SetValue(e.Value)
					m.textInput.Focus()
					m.textInput.Prompt = fmt.Sprintf("Edit %q (%s): ", e.Key, e.Type)
					m.statusMsg = "[enter] Confirm • [esc] Cancel"
					break
				}
			}
			return m, textinput.Blink

		case "d":
			row := m.keyTable.SelectedRow()
			if len(row) < 1 {
				return m, nil
			}
			m.editKey = row[0]
			m.view = viewConfirmDelete
			m.statusMsg = fmt.Sprintf("Delete %q? [y] Yes • [n/esc] No", m.editKey)
			return m, nil

		case "n":
			m.view = viewNewKeyName
			m.textInput.Reset()
			m.textInput.Focus()
			m.textInput.Prompt = "New key name: "
			m.newKeyName = ""
			m.newKeyType = ""
			m.typeCursor = 0
			m.statusMsg = "[enter] Next • [esc] Cancel"
			return m, textinput.Blink

		default:
			var cmd tea.Cmd
			m.keyTable, cmd = m.keyTable.Update(msg)
			return m, cmd
		}

	// -----------------------------------------------------------------------
	// Edit view
	// -----------------------------------------------------------------------
	case viewEdit:
		switch msg.String() {
		case "ctrl+c":
			return m, tea.Quit
		case "esc":
			m.view = viewKeys
			m.statusMsg = fmt.Sprintf("%d keys. [e/enter] Edit • [d] Delete • [n] New key • [esc] Back", len(m.entries))
			m.textInput.Blur()
			return m, nil
		case "enter":
			newVal := m.textInput.Value()
			m.loading = true
			m.statusMsg = fmt.Sprintf("Saving %q...", m.editKey)
			m.view = viewKeys
			m.textInput.Blur()
			key, typeName := m.editKey, m.editType
			plistPath := m.plistPath
			return m, tea.Batch(m.spinner.Tick, func() tea.Msg {
				err := writeDefaultValue(plistPath, key, typeName, newVal)
				return actionDoneDefaultsMsg{err: err}
			})
		default:
			var cmd tea.Cmd
			m.textInput, cmd = m.textInput.Update(msg)
			return m, cmd
		}

	// -----------------------------------------------------------------------
	// Confirm delete view
	// -----------------------------------------------------------------------
	case viewConfirmDelete:
		switch msg.String() {
		case "ctrl+c":
			return m, tea.Quit
		case "y":
			key := m.editKey
			plistPath := m.plistPath
			m.loading = true
			m.statusMsg = fmt.Sprintf("Deleting %q...", key)
			m.view = viewKeys
			return m, tea.Batch(m.spinner.Tick, func() tea.Msg {
				err := deleteDefaultKey(plistPath, key)
				return actionDoneDefaultsMsg{err: err}
			})
		case "n", "esc":
			m.view = viewKeys
			m.statusMsg = fmt.Sprintf("%d keys. [e/enter] Edit • [d] Delete • [n] New key • [esc] Back", len(m.entries))
			return m, nil
		}

	// -----------------------------------------------------------------------
	// New key — name step
	// -----------------------------------------------------------------------
	case viewNewKeyName:
		switch msg.String() {
		case "ctrl+c":
			return m, tea.Quit
		case "esc":
			m.view = viewKeys
			m.textInput.Blur()
			m.statusMsg = fmt.Sprintf("%d keys. [e/enter] Edit • [d] Delete • [n] New key • [esc] Back", len(m.entries))
			return m, nil
		case "enter":
			name := strings.TrimSpace(m.textInput.Value())
			if name == "" {
				m.statusMsg = "Key name cannot be empty. [enter] Next • [esc] Cancel"
				return m, nil
			}
			m.newKeyName = name
			m.view = viewNewKeyType
			m.textInput.Blur()
			m.typeCursor = 0
			m.statusMsg = "[←/→] Choose type • [enter] Confirm • [esc] Cancel"
			return m, nil
		default:
			var cmd tea.Cmd
			m.textInput, cmd = m.textInput.Update(msg)
			return m, cmd
		}

	// -----------------------------------------------------------------------
	// New key — type step
	// -----------------------------------------------------------------------
	case viewNewKeyType:
		switch msg.String() {
		case "ctrl+c":
			return m, tea.Quit
		case "esc":
			m.view = viewNewKeyName
			m.textInput.Reset()
			m.textInput.Focus()
			m.textInput.Prompt = "New key name: "
			m.textInput.SetValue(m.newKeyName)
			m.statusMsg = "[enter] Next • [esc] Cancel"
			return m, textinput.Blink
		case "left", "h":
			if m.typeCursor > 0 {
				m.typeCursor--
			}
			return m, nil
		case "right", "l":
			if m.typeCursor < len(m.typeChoices)-1 {
				m.typeCursor++
			}
			return m, nil
		case "enter":
			m.newKeyType = m.typeChoices[m.typeCursor]
			m.view = viewNewKeyValue
			m.textInput.Reset()
			m.textInput.Focus()
			m.textInput.Prompt = fmt.Sprintf("Value (%s): ", m.newKeyType)
			m.statusMsg = "[enter] Confirm • [esc] Cancel"
			return m, textinput.Blink
		}

	// -----------------------------------------------------------------------
	// New key — value step
	// -----------------------------------------------------------------------
	case viewNewKeyValue:
		switch msg.String() {
		case "ctrl+c":
			return m, tea.Quit
		case "esc":
			m.view = viewNewKeyType
			m.textInput.Blur()
			m.statusMsg = "[←/→] Choose type • [enter] Confirm • [esc] Cancel"
			return m, nil
		case "enter":
			val := m.textInput.Value()
			key, typeName, plistPath := m.newKeyName, m.newKeyType, m.plistPath
			m.loading = true
			m.statusMsg = fmt.Sprintf("Adding %q...", key)
			m.view = viewKeys
			m.textInput.Blur()
			return m, tea.Batch(m.spinner.Tick, func() tea.Msg {
				err := addDefaultKey(plistPath, key, typeName, val)
				return actionDoneDefaultsMsg{err: err}
			})
		default:
			var cmd tea.Cmd
			m.textInput, cmd = m.textInput.Update(msg)
			return m, cmd
		}
	}

	return m, nil
}

// ---------------------------------------------------------------------------
// View
// ---------------------------------------------------------------------------

func (m defaultsModel) View() string {
	header := m.renderHeader()
	footer := m.renderFooter()
	body := m.renderBody()
	return lipgloss.JoinVertical(lipgloss.Left, header, body, footer)
}

func (m defaultsModel) renderHeader() string {
	title := lipgloss.NewStyle().
		Bold(true).
		Foreground(ColorHeader).
		Render("UserDefaults Editor")

	var sub string
	switch m.view {
	case viewApps:
		sub = fmt.Sprintf("Device: %s", m.deviceName)
	default:
		sub = fmt.Sprintf("Device: %s  │  App: %s", m.deviceName, m.selectedApp.BundleID)
	}

	subtitle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("245")).
		Render(sub)

	return lipgloss.JoinVertical(lipgloss.Left, title, subtitle, "")
}

func (m defaultsModel) renderFooter() string {
	msg := m.statusMsg

	var line string
	if m.loading {
		line = m.spinner.View() + " " + lipgloss.NewStyle().Foreground(ColorIOS).Render(msg)
	} else {
		line = lipgloss.NewStyle().Foreground(lipgloss.Color("245")).Render(msg)
	}

	note := ""
	if m.view == viewKeys || m.view == viewEdit {
		note = "\n" + lipgloss.NewStyle().
			Foreground(lipgloss.Color("240")).Italic(true).
			Render("ℹ Relaunch the app for changes to take effect.")
	}
	return "\n" + line + note
}

//nolint:gocyclo
func (m defaultsModel) renderBody() string {
	switch m.view {
	case viewApps:
		if m.loading {
			return ""
		}
		if len(m.apps) == 0 {
			noApps := lipgloss.NewStyle().Foreground(lipgloss.Color("240")).Render(
				"No apps found. Press [a] to toggle system apps.")
			return noApps
		}
		hint := lipgloss.NewStyle().Foreground(lipgloss.Color("245")).Render(
			"[↑/↓] Navigate  [enter] Select  [a] Toggle system apps  [q] Quit")
		return dashboardBaseStyle.Render(m.appTable.View()) + "\n" + hint

	case viewKeys, viewEdit, viewConfirmDelete, viewNewKeyName, viewNewKeyType, viewNewKeyValue:
		if m.loading {
			return ""
		}
		tbl := dashboardBaseStyle.Render(m.keyTable.View())

		var overlay string
		switch m.view {
		case viewEdit:
			overlay = m.renderInputOverlay("Editing: " + m.editKey)
		case viewConfirmDelete:
			overlay = m.renderConfirmOverlay()
		case viewNewKeyName:
			overlay = m.renderInputOverlay("New Key — Step 1/3: Name")
		case viewNewKeyType:
			overlay = m.renderTypeChooser()
		case viewNewKeyValue:
			overlay = m.renderInputOverlay(fmt.Sprintf("New Key — Step 3/3: Value (%s)", m.newKeyType))
		}

		hint := ""
		if m.view == viewKeys {
			hint = "\n" + lipgloss.NewStyle().Foreground(lipgloss.Color("245")).Render(
				"[↑/↓] Navigate  [e/enter] Edit  [d] Delete  [n] New key  [esc] Back  [q] Quit")
		}

		return tbl + hint + overlay

	default:
		return ""
	}
}

func (m defaultsModel) renderInputOverlay(label string) string {
	labelStr := lipgloss.NewStyle().Bold(true).Foreground(ColorHeader).Render(label)
	box := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(ColorHeader).
		Padding(0, 1).
		Width(m.width - 4).
		Render(labelStr + "\n" + m.textInput.View())
	return "\n" + box
}

func (m defaultsModel) renderConfirmOverlay() string {
	msg := fmt.Sprintf("Delete key %q? This cannot be undone.", m.editKey)
	choices := lipgloss.NewStyle().Foreground(lipgloss.Color("196")).Bold(true).Render("[y] Yes") +
		"  " + lipgloss.NewStyle().Foreground(lipgloss.Color("245")).Render("[n/esc] No")
	box := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color("196")).
		Padding(0, 1).
		Width(m.width - 4).
		Render(msg + "\n" + choices)
	return "\n" + box
}

func (m defaultsModel) renderTypeChooser() string {
	label := lipgloss.NewStyle().Bold(true).Foreground(ColorHeader).
		Render("New Key — Step 2/3: Choose Type")

	var tabs []string
	for i, t := range m.typeChoices {
		if i == m.typeCursor {
			tabs = append(tabs, lipgloss.NewStyle().
				Background(lipgloss.Color("57")).
				Foreground(lipgloss.Color("229")).
				Padding(0, 1).Bold(true).Render(t))
		} else {
			tabs = append(tabs, lipgloss.NewStyle().
				Foreground(lipgloss.Color("245")).
				Padding(0, 1).Render(t))
		}
	}

	hint := lipgloss.NewStyle().Foreground(lipgloss.Color("245")).
		Render("[←/→] Select  [enter] Confirm  [esc] Back")

	box := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(ColorHeader).
		Padding(0, 1).
		Width(m.width - 4).
		Render(label + "\n\n" + strings.Join(tabs, "  ") + "\n\n" + hint)
	return "\n" + box
}

// ---------------------------------------------------------------------------
// Layout helpers
// ---------------------------------------------------------------------------

func (m defaultsModel) tableHeight() int {
	// Reserve space for header (3 lines), footer (2-3 lines), overlays, etc.
	h := m.height - 8
	if h < 5 {
		h = 5
	}
	return h
}

func (m defaultsModel) recalcTableSizes() defaultsModel {
	h := m.tableHeight()
	w := m.width

	newAppTbl := buildAppTable(m.apps, w, h)
	// Preserve cursor position
	idx := m.appTable.Cursor()
	newAppTbl.SetCursor(idx)
	m.appTable = newAppTbl

	newKeyTbl := buildKeyTable(m.entries, w, h)
	kidx := m.keyTable.Cursor()
	newKeyTbl.SetCursor(kidx)
	m.keyTable = newKeyTbl

	m.textInput.Width = w - 8

	return m
}
