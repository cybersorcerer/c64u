package tui

import (
	"fmt"
	"sort"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/cybersorcerer/c64.nvim/tools/c64u/internal/api"
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
}

// The SoftIEC drive is listed among the drives but is enabled through the
// device configuration rather than the drives endpoint.
const (
	softIECDrive    = "softiec"
	softIECCategory = "SoftIEC Drive Settings"
	softIECItem     = "IEC Drive"
)

// DriveItem represents a row in the drive list
type DriveItem struct {
	Name        string
	Letter      string // "a", "b"
	Description string
	Mounted     string
	Mode        string
	Enabled     bool
}

func NewDrivesModel(client *api.Client) *DrivesModel {
	return &DrivesModel{
		client:  client,
		loading: true,
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

				letter := strings.ToLower(strings.TrimSpace(strings.TrimPrefix(name, "Drive")))
				if strings.Contains(strings.ToLower(name), "iec") {
					letter = "softiec"
				}

				enabled, _ := info["enabled"].(bool)
				image, _ := info["image_file"].(string)
				mode, _ := info["type"].(string) // "type" often holds the mode like 1541

				desc := mode
				if !enabled {
					desc += " (Disabled)"
				}

				items = append(items, DriveItem{
					Name:        name,
					Letter:      letter,
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

	if drive.Letter == softIECDrive {
		// SoftIEC is DOS emulation, not a floppy: no image, no ROM, no mode.
		if drive.Enabled {
			actions = append(actions, SelectorItem{Label: "Disable", Value: "off", Description: "Turn off SoftIEC"})
		} else {
			actions = append(actions, SelectorItem{Label: "Enable", Value: "on", Description: "Turn on SoftIEC"})
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

func (m *DrivesModel) performActionCmd(action string) tea.Cmd {
	return func() tea.Msg {
		drive := m.selectedDrive

		if action == "loadrom" {
			DebugLog("DrivesModel: Emit FilePickerRequestMsg for %s", drive)
			return FilePickerRequestMsg{Drive: drive}
		}

		// SoftIEC is switched through the device configuration. The drives
		// endpoint accepts /v1/drives/softiec:on and answers success, but the
		// drive stays disabled.
		if drive == softIECDrive && (action == "on" || action == "off") {
			value := "Enabled"
			if action == "off" {
				value = "Disabled"
			}
			if err := m.client.SetConfigItem(softIECCategory, softIECItem, value); err != nil {
				return statusMsg("Error: " + err.Error())
			}
			return statusMsg("SoftIEC " + strings.ToLower(value))
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

		stateStr := drive.Mode
		if !drive.Enabled {
			stateStr = "Disabled"
		} else if drive.Mounted != "" {
			stateStr += " • " + drive.Mounted
		} else {
			stateStr += " • Empty"
		}

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

	b.WriteString("\n" + StatusBarStyle.Width(m.width).Render("↑/↓: select  Enter: actions  ?: help"))

	return b.String()
}
