package tui

import (
	"fmt"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/cybersorcerer/c64.nvim/tools/c64u/internal/api"
)

// MachineModel handles the machine control view
type MachineModel struct {
	client   *api.Client
	selector *Selector
	message  string
	err      error
}

// NewMachineModel creates a new machine control view
func NewMachineModel(client *api.Client) *MachineModel {
	m := &MachineModel{
		client: client,
	}

	m.selector = NewSelector("Machine Control", []SelectorItem{
		{Label: "Reset", Value: "reset", Description: "Reset the machine"},
		{Label: "Reboot", Value: "reboot", Description: "Restart with cartridge reinit"},
		{Label: "Pause", Value: "pause", Description: "Pause execution (DMA)"},
		{Label: "Resume", Value: "resume", Description: "Resume from pause"},
		{Label: "Menu Button", Value: "menu", Description: "Simulate Menu button"},
		{Label: "Power Off", Value: "poweroff", Description: "Power off machine (U64 only)"},
	})
	m.selector.PreventQuit = true

	return m
}

func (m *MachineModel) Init() tea.Cmd {
	return nil
}

func (m *MachineModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	// Update selector
	_, cmd := m.selector.Update(msg)

	if m.selector.confirmed {
		m.selector.confirmed = false // Reset
		selected := m.selector.Items[m.selector.selected]
		return m, m.performAction(selected.Value)
	}

	return m, cmd
}

func (m *MachineModel) performAction(action string) tea.Cmd {
	return func() tea.Msg {
		var err error
		var resp *api.Response

		switch action {
		case "reset":
			resp, err = m.client.MachineReset()
		case "reboot":
			resp, err = m.client.MachineReboot()
		case "pause":
			resp, err = m.client.MachinePause()
		case "resume":
			resp, err = m.client.MachineResume()
		case "menu":
			resp, err = m.client.MachineMenuButton()
		case "poweroff":
			resp, err = m.client.MachinePowerOff()
		default:
			return statusMsg("Unknown action")
		}

		if err != nil {
			return statusMsg(fmt.Sprintf("Error: %v", err))
		}
		if resp != nil && resp.HasErrors() {
			return statusMsg(fmt.Sprintf("API Error: %v", resp.Errors))
		}

		return statusMsg(fmt.Sprintf("Action '%s' successful", action))
	}
}

func (m *MachineModel) View() string {
	var s string
	s += m.selector.View()

	if m.message != "" {
		s += "\n\n" + StatusBarStyle.Render(m.message)
	}

	return s
}
