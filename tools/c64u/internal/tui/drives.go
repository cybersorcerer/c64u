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
	message string

	// Action Menu State
	actionSelector *Selector
	selectedDrive  string // "a" or "b"
}

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
		m.message = string(msg)
		// Refresh list after action implies state change
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
		var err error
		drive := m.selectedDrive

		if strings.HasPrefix(action, "mode_") {
			mode := strings.TrimPrefix(action, "mode_")
			_, err = m.client.DrivesSetMode(drive, mode)
			if err == nil {
				return statusMsg(fmt.Sprintf("Drive %s set to %s", strings.ToUpper(drive), mode))
			}
		} else {
			switch action {
			case "unmount":
				_, err = m.client.DrivesRemove(drive)
				if err == nil {
					return statusMsg(fmt.Sprintf("Drive %s unmounted", strings.ToUpper(drive)))
				}
			case "reset":
				_, err = m.client.DrivesReset(drive)
				if err == nil {
					return statusMsg(fmt.Sprintf("Drive %s reset", strings.ToUpper(drive)))
				}
			case "off":
				_, err = m.client.DrivesOff(drive)
				if err == nil {
					return statusMsg(fmt.Sprintf("Drive %s disabled", strings.ToUpper(drive)))
				}
			case "on":
				_, err = m.client.DrivesOn(drive)
				if err == nil {
					return statusMsg(fmt.Sprintf("Drive %s enabled", strings.ToUpper(drive)))
				}
			case "loadrom":
				DebugLog("DrivesModel: Emit FilePickerRequestMsg for %s", drive)
				return FilePickerRequestMsg{Drive: drive}
			}
		}

		if err != nil {
			return statusMsg(fmt.Sprintf("Error: %v", err))
		}
		return statusMsg("Action executed")
	}
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

	footer := m.message
	if footer == "" {
		footer = "↑/↓: select  Enter: actions  ?: help"
	}
	b.WriteString("\n" + StatusBarStyle.Width(m.width).Render(footer))

	return b.String()
}
