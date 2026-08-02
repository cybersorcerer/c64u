package tui

import (
	"fmt"
	"sort"
	"strconv"
	"strings"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/cybersorcerer/c64.nvim/tools/c64u/internal/api"
	"github.com/cybersorcerer/c64.nvim/tools/c64u/internal/softiec"
)

// DrivesModel handles the drives list view and management
type DrivesModel struct {
	client  *api.Client
	drives  []DriveItem // Simplified structure for list
	cursor  int
	width   int
	height  int
	loading bool
	err     error

	// Action Menu State
	actionSelector *Selector
	selectedDrive  string // "a" or "b"

	// Text entry for the SoftIEC device number and directory
	state    drivesState
	ti       textinput.Model
	prompt   string
	enabling bool // the bus ID being entered is part of enabling SoftIEC
}

type drivesState int

const (
	drivesBrowsing drivesState = iota
	drivesBusIDInput
	drivesRootInput
)

// Not every entry in the drives list is a floppy drive. SoftIEC and the printer
// are device features that happen to sit on the IEC bus; they are switched
// through the configuration. The drives endpoint accepts /v1/drives/<id>:on for
// them and answers success, but nothing changes.
const (
	softIECDrive = "softiec"
	printerDrive = "printer"
)

// configToggle locates the configuration item that enables such an entry.
type configToggle struct {
	category string
	item     string
	label    string // used in the status message
}

var configDrives = map[string]configToggle{
	softIECDrive: {"SoftIEC Drive Settings", "IEC Drive", "SoftIEC"},
	printerDrive: {"Printer Settings", "IEC printer", "Printer emulation"},
}

// driveLetter maps a name from the drives list ("a", "IEC Drive", "Printer
// Emulation") to the identifier the API and configDrives use.
func driveLetter(name string) string {
	switch lower := strings.ToLower(name); {
	case strings.Contains(lower, "iec"):
		return softIECDrive
	case strings.Contains(lower, "printer"):
		return printerDrive
	default:
		return strings.ToLower(strings.TrimSpace(strings.TrimPrefix(name, "Drive")))
	}
}

// driveStateText is what a drive row shows in brackets: its device number on
// the IEC bus, and what it currently holds — a disk image for the floppies, the
// served directory for SoftIEC.
func driveStateText(drive DriveItem) string {
	if !drive.Enabled {
		return "Disabled"
	}

	parts := []string{}
	if !isConfigDrive(drive.Letter) && drive.Mode != "" {
		parts = append(parts, drive.Mode)
	}
	if drive.BusID > 0 {
		parts = append(parts, fmt.Sprintf("#%d", drive.BusID))
	}

	switch {
	case drive.Letter == softIECDrive:
		if drive.Path != "" {
			parts = append(parts, drive.Path)
		}
	case isConfigDrive(drive.Letter):
		// Nothing can be inserted into the printer.
	case drive.Mounted != "":
		parts = append(parts, drive.Mounted)
	default:
		parts = append(parts, "Empty")
	}

	if len(parts) == 0 {
		return "Enabled"
	}
	return strings.Join(parts, " • ")
}

// firstPartitionPath returns the directory a SoftIEC drive currently serves.
func firstPartitionPath(info map[string]interface{}) string {
	parts, _ := info["partitions"].([]interface{})
	if len(parts) == 0 {
		return ""
	}
	p, ok := parts[0].(map[string]interface{})
	if !ok {
		return ""
	}
	path, _ := p["path"].(string)
	return path
}

// isConfigDrive reports whether a drive list entry is one of those features.
func isConfigDrive(letter string) bool {
	_, ok := configDrives[letter]
	return ok
}

// DriveItem represents a row in the drive list
type DriveItem struct {
	Name        string
	Letter      string // "a", "b"
	Description string
	Mounted     string
	Mode        string
	Enabled     bool
	BusID       int    // IEC device number the drive answers on
	Path        string // directory served, SoftIEC only
}

func NewDrivesModel(client *api.Client) *DrivesModel {
	ti := textinput.New()
	ti.CharLimit = 128
	return &DrivesModel{
		client:  client,
		loading: true,
		ti:      ti,
	}
}

func (m *DrivesModel) Init() tea.Cmd {
	return m.fetchDrivesCmd()
}

func (m *DrivesModel) fetchDrivesCmd() tea.Cmd {
	return func() tea.Msg {
		resp, err := m.client.DrivesList()
		if err != nil {
			return errMsg{err}
		}
		if resp.HasErrors() {
			return errMsg{fmt.Errorf("API errors: %v", resp.Errors)}
		}

		drivesList, ok := resp.Data["drives"].([]interface{})
		if !ok {
			return errMsg{fmt.Errorf("Invalid API response format")}
		}

		var items []DriveItem
		for _, d := range drivesList {
			driveMap, ok := d.(map[string]interface{})
			if !ok {
				continue
			}

			for name, infoInterface := range driveMap {
				info, ok := infoInterface.(map[string]interface{})
				if !ok {
					continue
				}

				letter := driveLetter(name)

				enabled, _ := info["enabled"].(bool)
				image, _ := info["image_file"].(string)
				mode, _ := info["type"].(string) // "type" often holds the mode like 1541

				desc := mode
				if !enabled {
					desc += " (Disabled)"
				}

				busID := 0
				if id, ok := info["bus_id"].(float64); ok {
					busID = int(id)
				}

				items = append(items, DriveItem{
					Name:        name,
					Letter:      letter,
					BusID:       busID,
					Path:        firstPartitionPath(info),
					Description: desc,
					Mounted:     image,
					Mode:        mode,
					Enabled:     enabled,
				})
			}
		}

		// Sort by letter
		sort.Slice(items, func(i, j int) bool {
			return items[i].Letter < items[j].Letter
		})

		return drivesFetchedMsg(items)
	}
}

type drivesFetchedMsg []DriveItem

func (m *DrivesModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	if m.state != drivesBrowsing {
		if key, ok := msg.(tea.KeyMsg); ok {
			return m.handleTextInput(key)
		}
		return m, nil
	}

	// If action menu is open, delegate to it
	if m.actionSelector != nil {
		_, cmd := m.actionSelector.Update(msg)

		if m.actionSelector.cancelled {
			m.actionSelector = nil
			return m, nil
		}

		if m.actionSelector.confirmed {
			selectedAction := m.actionSelector.Items[m.actionSelector.selected].Value
			m.actionSelector = nil
			if cmd, handled := m.startInput(selectedAction); handled {
				return m, cmd
			}
			return m, m.performActionCmd(selectedAction)
		}

		return m, cmd
	}

	switch msg := msg.(type) {
	case drivesFetchedMsg:
		m.loading = false
		m.drives = msg
		// Reset cursor if out of bounds?
		if m.cursor >= len(m.drives) {
			m.cursor = 0
		}
		return m, nil

	case errMsg:
		m.loading = false
		m.err = msg.err
		return m, nil

	case statusMsg:
		// An action changed the device state — reload so the list reflects it.
		// The message itself is rendered by MainModel's status bar.
		return m, m.fetchDrivesCmd()

	case tea.KeyMsg:
		switch msg.String() {
		case "up", "k", "down", "j", "g", "G", "ctrl+d", "ctrl+u":
			m.cursor = applyListNav(msg.String(), m.cursor, len(m.drives), m.height-3)
		case "enter":
			if len(m.drives) > 0 {
				m.selectedDrive = m.drives[m.cursor].Letter
				m.openActionMenu(m.drives[m.cursor])
			}
		case "esc", "q":
			return m, func() tea.Msg { return BackMsg{} }
		}
	}

	return m, nil
}

func (m *DrivesModel) openActionMenu(drive DriveItem) {
	// Build actions based on state
	var actions []SelectorItem

	if toggle, ok := configDrives[drive.Letter]; ok {
		// Not a floppy: no image, no ROM, no drive mode.
		switch {
		case drive.Letter != softIECDrive && drive.Enabled:
			actions = append(actions, SelectorItem{Label: "Disable", Value: "off", Description: "Turn off " + toggle.label})
		case drive.Letter != softIECDrive:
			actions = append(actions, SelectorItem{Label: "Enable", Value: "on", Description: "Turn on " + toggle.label})
		case drive.Enabled:
			actions = append(actions,
				SelectorItem{Label: "Set directory", Value: "softiec-root", Description: "Directory served over the IEC bus"},
				SelectorItem{Label: "Set device number", Value: "softiec-busid", Description: "IEC device number, currently #" + strconv.Itoa(drive.BusID)},
				SelectorItem{Label: "Disable", Value: "off", Description: "Turn off SoftIEC"},
			)
		default:
			actions = append(actions, SelectorItem{Label: "Enable", Value: "softiec-enable", Description: "Turn on SoftIEC and choose its device number"})
		}
		m.actionSelector = NewSelector(fmt.Sprintf("%s Actions", drive.Name), actions)
		m.actionSelector.PreventQuit = true
		return
	}

	if drive.Enabled {
		if drive.Mounted != "" {
			actions = append(actions, SelectorItem{Label: "Unmount", Value: "unmount", Description: "Eject disk"})
		}
		actions = append(actions, SelectorItem{Label: "Load Custom ROM", Value: "loadrom", Description: "Select ROM from file"})
		actions = append(actions, SelectorItem{Label: "Reset Drive", Value: "reset", Description: "Soft reset"})
		actions = append(actions, SelectorItem{Label: "Disable", Value: "off", Description: "Turn off drive"})

		// Mode switching
		actions = append(actions, SelectorItem{Label: "Mode: 1541", Value: "mode_1541"})
		actions = append(actions, SelectorItem{Label: "Mode: 1571", Value: "mode_1571"})
		actions = append(actions, SelectorItem{Label: "Mode: 1581", Value: "mode_1581"})
	} else {
		actions = append(actions, SelectorItem{Label: "Enable", Value: "on", Description: "Turn on drive"})
	}

	m.actionSelector = NewSelector(fmt.Sprintf("%s Actions", drive.Name), actions)
	m.actionSelector.PreventQuit = true
}

// startInput opens the text prompt for actions that need a value. It reports
// whether it took the action; anything else is a plain API call.
func (m *DrivesModel) startInput(action string) (tea.Cmd, bool) {
	current := m.currentDrive()

	switch action {
	case "softiec-root":
		m.state = drivesRootInput
		m.prompt = "Directory to serve:"
		m.ti.SetValue(current.Path)
	case "softiec-busid", "softiec-enable":
		m.state = drivesBusIDInput
		m.prompt = "IEC device number:"
		m.enabling = action == "softiec-enable"
		busID := current.BusID
		if busID == 0 {
			busID = 11
		}
		m.ti.SetValue(strconv.Itoa(busID))
	default:
		return nil, false
	}

	m.ti.CursorEnd()
	m.ti.Focus()
	return textinput.Blink, true
}

func (m *DrivesModel) currentDrive() DriveItem {
	if m.cursor < len(m.drives) {
		return m.drives[m.cursor]
	}
	return DriveItem{}
}

func (m *DrivesModel) handleTextInput(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "esc":
		m.state = drivesBrowsing
		m.ti.Blur()
		return m, nil

	case "enter":
		value := strings.TrimSpace(m.ti.Value())
		state := m.state
		enabling := m.enabling
		m.state = drivesBrowsing
		m.enabling = false
		m.ti.Blur()
		if value == "" {
			return m, nil
		}
		if state == drivesRootInput {
			return m, m.setSoftIECRootCmd(value)
		}
		return m, m.setSoftIECBusIDCmd(value, enabling)

	default:
		var cmd tea.Cmd
		m.ti, cmd = m.ti.Update(msg)
		return m, cmd
	}
}

// setSoftIECRootCmd points SoftIEC at a directory and confirms it took effect.
func (m *DrivesModel) setSoftIECRootCmd(path string) tea.Cmd {
	client := m.client
	busID := m.currentDrive().BusID
	return func() tea.Msg {
		status, err := softiec.SetRoot(client, busID, path)
		if err != nil {
			return statusMsg("Error: " + err.Error())
		}
		return statusMsg("SoftIEC now serves " + status.Path)
	}
}

// setSoftIECBusIDCmd changes the IEC device number, and enables the drive too
// when the number was asked for as part of enabling it.
func (m *DrivesModel) setSoftIECBusIDCmd(value string, enable bool) tea.Cmd {
	client := m.client
	return func() tea.Msg {
		busID, err := strconv.Atoi(value)
		if err != nil {
			return statusMsg(fmt.Sprintf("Error: %q is not a device number", value))
		}

		settings, err := softiec.LoadSettings(client)
		if err != nil {
			return statusMsg("Error: " + err.Error())
		}
		if err := softiec.SetBusID(client, settings, busID); err != nil {
			return statusMsg("Error: " + err.Error())
		}
		if !enable {
			return statusMsg(fmt.Sprintf("SoftIEC device number set to #%d", busID))
		}
		if err := softiec.SetEnabled(client, settings, true); err != nil {
			return statusMsg("Error: " + err.Error())
		}
		return statusMsg(fmt.Sprintf("SoftIEC enabled on #%d", busID))
	}
}

func (m *DrivesModel) performActionCmd(action string) tea.Cmd {
	return func() tea.Msg {
		drive := m.selectedDrive

		if action == "loadrom" {
			DebugLog("DrivesModel: Emit FilePickerRequestMsg for %s", drive)
			return FilePickerRequestMsg{Drive: drive}
		}

		if toggle, ok := configDrives[drive]; ok && (action == "on" || action == "off") {
			on := action == "on"
			if drive == softIECDrive {
				return statusMsg(setSoftIECEnabled(m.client, on))
			}
			value := "Enabled"
			if !on {
				value = "Disabled"
			}
			if err := m.client.SetConfigItem(toggle.category, toggle.item, value); err != nil {
				return statusMsg("Error: " + err.Error())
			}
			return statusMsg(toggle.label + " " + strings.ToLower(value))
		}

		var resp *api.Response
		var err error
		var okMsg string

		if mode := strings.TrimPrefix(action, "mode_"); mode != action {
			resp, err = m.client.DrivesSetMode(drive, mode)
			okMsg = fmt.Sprintf("Drive %s set to %s", strings.ToUpper(drive), mode)
		} else {
			switch action {
			case "unmount":
				resp, err = m.client.DrivesRemove(drive)
				okMsg = fmt.Sprintf("Drive %s unmounted", strings.ToUpper(drive))
			case "reset":
				resp, err = m.client.DrivesReset(drive)
				okMsg = fmt.Sprintf("Drive %s reset", strings.ToUpper(drive))
			case "off":
				resp, err = m.client.DrivesOff(drive)
				okMsg = fmt.Sprintf("Drive %s disabled", strings.ToUpper(drive))
			case "on":
				resp, err = m.client.DrivesOn(drive)
				okMsg = fmt.Sprintf("Drive %s enabled", strings.ToUpper(drive))
			default:
				return statusMsg(fmt.Sprintf("Error: unknown drive action %q", action))
			}
		}

		return statusMsg(driveActionResult(resp, err, okMsg))
	}
}

// setSoftIECEnabled switches SoftIEC through the configuration, resolving where
// that setting lives on this firmware, and returns the status line.
func setSoftIECEnabled(client *api.Client, on bool) string {
	settings, err := softiec.LoadSettings(client)
	if err != nil {
		return "Error: " + err.Error()
	}
	if err := softiec.SetEnabled(client, settings, on); err != nil {
		return "Error: " + err.Error()
	}
	if on {
		return "SoftIEC enabled"
	}
	return "SoftIEC disabled"
}

// driveActionResult turns an API call's outcome into a status line. The device
// reports a rejected request in the response body rather than as a transport
// error, so claiming success requires checking both.
func driveActionResult(resp *api.Response, err error, okMsg string) string {
	if err != nil {
		return "Error: " + err.Error()
	}
	if resp != nil && resp.HasErrors() {
		return "Error: " + strings.Join(resp.Errors, "; ")
	}
	return okMsg
}

func (m *DrivesModel) View() string {
	if m.actionSelector != nil {
		m.actionSelector.width = m.width
		return m.actionSelector.View()
	}

	if m.loading {
		return "Loading..."
	}
	if m.err != nil {
		return fmt.Sprintf("Error: %v", m.err)
	}

	var b strings.Builder

	b.WriteString(ItemStyle.Render("Drives"))
	b.WriteString("\n")

	for i, drive := range m.drives {
		// Build line content
		icon := "○" // Disabled
		if drive.Enabled {
			icon = "●" // Enabled
		}

		stateStr := driveStateText(drive)

		// Formatting
		nameStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("7"))
		descStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("8")) // Gray

		if !drive.Enabled {
			nameStyle = nameStyle.Faint(true)
			descStyle = descStyle.Faint(true)
		}

		lineContent := ""
		if i == m.cursor {
			// Selected - Apply Reverse Video to full line
			// We manually build the string then render it with SelectedItemStyle
			txt := fmt.Sprintf("▶ %s %s [%s]", icon, drive.Name, stateStr)
			b.WriteString(SelectedItemStyle.Render(txt) + "\n")
		} else {
			// Regular
			txt := fmt.Sprintf("  %s %s", icon, drive.Name)
			lineContent = nameStyle.Render(txt) + descStyle.Render(fmt.Sprintf(" [%s]", stateStr))
			b.WriteString(ItemStyle.Render(lineContent) + "\n")
		}
	}

	footer := "↑/↓: select  Enter: actions  ?: help"
	if m.state != drivesBrowsing {
		footer = m.prompt + " " + m.ti.View() + "   (Enter: apply, Esc: cancel)"
	}
	b.WriteString("\n" + StatusBarStyle.Width(m.width).Render(footer))

	return b.String()
}
