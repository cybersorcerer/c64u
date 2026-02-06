package tui

import (
	"fmt"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/cybersorcerer/c64.nvim/tools/c64u/internal/api"
)

// ViewState represents the currently active view
type ViewState int

const (
	ViewMainMenu ViewState = iota
	ViewFileBrowser
	ViewDrives
	ViewMachineControl
	ViewSettings
	ViewFileViewer
)

// MainModel is the top-level bubble tea model
type MainModel struct {
	client    *api.Client
	viewState ViewState
	width     int
	height    int

	// Child models
	browser *BrowserModel
	drives  *DrivesModel
	machine *MachineModel
	config  *ConfigModel
	viewer  *ViewerModel

	selector *Selector // Main menu selector

	// Help overlay
	showHelp bool

	// Global status line
	statusMessage string
	statusIsError bool
	statusID      int // incremented per message, used for auto-clear
}

// NewMainModel creates the initial model
func NewMainModel(client *api.Client) *MainModel {
	m := &MainModel{
		client:    client,
		viewState: ViewMainMenu,
		browser:   NewBrowserModel(client),
		drives:    NewDrivesModel(client),
		machine:   NewMachineModel(client),
		config:    NewConfigModel(client),
		viewer:    NewViewerModel(),
	}

	// Initialize Main Menu
	m.selector = NewSelector("C64 Ultimate Control", []SelectorItem{
		{Label: "File Browser", Value: "files", Description: "Browse and mount images"},
		{Label: "Drive Management", Value: "drives", Description: "Control drives, mount/unmount"},
		{Label: "Machine Control", Value: "machine", Description: "Reset, Reboot, Pause, etc."},
		{Label: "Configuration", Value: "settings", Description: "Configure device settings"},
		{Label: "Quit", Value: "quit", Description: "Exit TUI"},
	})
	m.selector.PreventQuit = true

	return m
}

func (m *MainModel) Init() tea.Cmd {
	return nil
}

func (m *MainModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmd tea.Cmd
	var cmds []tea.Cmd

	// Global key handlers
	switch msg := msg.(type) {
	case tea.KeyMsg:
		if msg.String() == "ctrl+c" {
			DebugLog("MainModel: Ctrl+C pressed")
			return m, tea.Quit
		}

		// Help overlay toggle
		if msg.String() == "?" {
			m.showHelp = !m.showHelp
			return m, nil
		}
		if m.showHelp && msg.String() == "esc" {
			m.showHelp = false
			return m, nil
		}

		// If help is open, consume all other keys
		if m.showHelp {
			return m, nil
		}

	// Global status messages
	case statusMsg:
		m.statusID++
		m.statusMessage = string(msg)
		text := strings.ToLower(string(msg))
		m.statusIsError = strings.HasPrefix(text, "error") ||
			strings.HasPrefix(text, "unknown") ||
			strings.HasPrefix(text, "api error") ||
			strings.HasPrefix(text, "fail") ||
			strings.Contains(text, "failed") ||
			strings.HasPrefix(text, "no ")
		duration := 5 * time.Second
		if m.statusIsError {
			duration = 8 * time.Second
		}
		return m, m.clearStatusAfter(m.statusID, duration)

	case errMsg:
		m.statusID++
		m.statusMessage = fmt.Sprintf("Error: %v", msg.err)
		m.statusIsError = true
		return m, m.clearStatusAfter(m.statusID, 8*time.Second)

	case GlobalStatusMsg:
		m.statusID++
		m.statusMessage = msg.Message
		m.statusIsError = msg.IsError
		duration := 5 * time.Second
		if msg.IsError {
			duration = 8 * time.Second
		}
		return m, m.clearStatusAfter(m.statusID, duration)

	case ClearStatusMsg:
		if msg.ID == m.statusID {
			m.statusMessage = ""
			m.statusIsError = false
		}
		return m, nil

	// Handle File Picker Logic
	case FilePickerRequestMsg:
		DebugLog("MainModel received FilePickerRequestMsg for drive %s", msg.Drive)
		m.viewState = ViewFileBrowser
		m.browser.Update(msg) // Initialize picking mode in browser
		return m, m.browser.Init()

	case FilePickedMsg:
		DebugLog("MainModel received FilePickedMsg: %s", msg.File)
		m.viewState = ViewDrives
		// Perform the action (Load ROM)
		return m, func() tea.Msg {
			DebugLog("MainModel Executing Load ROM...")
			_, err := m.client.DrivesLoadROM(msg.Drive, msg.File)
			if err != nil {
				return statusMsg(fmt.Sprintf("Error loading ROM: %v", err))
			}
			return statusMsg("ROM loaded to Drive " + strings.ToUpper(msg.Drive))
		}

	case DirPickerRequestMsg:
		m.viewState = ViewFileBrowser
		return m, m.browser.Init()

	case FileDirPickedMsg:
		m.viewState = ViewSettings
		// Pass directly to config Update
		m.config.Update(msg)
		return m, nil

	case FileContentMsg:
		if msg.Err != nil {
			m.statusID++
			m.statusMessage = fmt.Sprintf("Error reading file: %v", msg.Err)
			m.statusIsError = true
			return m, m.clearStatusAfter(m.statusID, 8*time.Second)
		}
		m.viewer.SetContent(msg.Filename, msg.Content, m.width-4, m.height-4)
		m.viewState = ViewFileViewer
		return m, nil

	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		m.browser.width = msg.Width - 4 // Account for padding
		m.browser.height = msg.Height - 4
		m.drives.width = msg.Width - 4
		m.drives.height = msg.Height - 4
		m.config.width = msg.Width - 4
		m.config.height = msg.Height - 4

		// Propagate size to children if they need it
	}

	// Handle views
	switch m.viewState {
	case ViewMainMenu:
		_, cmd = m.selector.Update(msg)
		cmds = append(cmds, cmd)

		// Handle cancellation (ESC) to quit app
		if m.selector.cancelled {
			return m, tea.Quit
		}

		if m.selector.confirmed {
			m.selector.confirmed = false
			selected := m.selector.Items[m.selector.selected]

			switch selected.Value {
			case "files":
				m.viewState = ViewFileBrowser
				cmds = append(cmds, m.browser.Init())
			case "drives":
				m.viewState = ViewDrives
				cmds = append(cmds, m.drives.Init())
			case "machine":
				m.viewState = ViewMachineControl
			case "settings":
				m.viewState = ViewSettings
				cmds = append(cmds, m.config.Init())
			case "quit":
				return m, tea.Quit
			}
		}

	case ViewFileBrowser:
		if msg, ok := msg.(tea.KeyMsg); ok && msg.String() == "esc" {
			m.viewState = ViewMainMenu
			return m, nil
		}

		newBrowser, cmd := m.browser.Update(msg)
		m.browser = newBrowser.(*BrowserModel)
		cmds = append(cmds, cmd)

	case ViewDrives:
		newDrives, cmd := m.drives.Update(msg)
		m.drives = newDrives.(*DrivesModel)
		cmds = append(cmds, cmd)

		// Check for BackMsg from child
		if _, ok := msg.(BackMsg); ok {
			m.viewState = ViewMainMenu
		}

	case ViewMachineControl:
		if msg, ok := msg.(tea.KeyMsg); ok && (msg.String() == "esc" || msg.String() == "q") {
			m.viewState = ViewMainMenu
			return m, nil
		}

		newMachine, cmd := m.machine.Update(msg)
		m.machine = newMachine.(*MachineModel)
		cmds = append(cmds, cmd)

	case ViewSettings:
		newConfig, cmd := m.config.Update(msg)
		m.config = newConfig.(*ConfigModel)
		cmds = append(cmds, cmd)

		if _, ok := msg.(BackMsg); ok {
			m.viewState = ViewMainMenu
		}

	case ViewFileViewer:
		if msg, ok := msg.(tea.KeyMsg); ok && (msg.String() == "esc" || msg.String() == "q") {
			m.viewState = ViewFileBrowser
			return m, nil
		}

		newViewer, cmd := m.viewer.Update(msg)
		m.viewer = newViewer.(*ViewerModel)
		cmds = append(cmds, cmd)
	}

	return m, tea.Batch(cmds...)
}

func (m *MainModel) View() string {
	var content string

	if m.showHelp {
		content = m.helpView()
	} else {
		switch m.viewState {
		case ViewMainMenu:
			content = m.selector.View()
		case ViewFileBrowser:
			content = m.browser.View()
		case ViewDrives:
			content = m.drives.View()
		case ViewMachineControl:
			content = m.machine.View()
		case ViewSettings:
			content = m.config.View()
		case ViewFileViewer:
			content = m.viewer.View()
		}
	}

	// Append status line as part of content (inside the box)
	if m.statusMessage != "" {
		// Measure the widest line in content to match status width
		contentWidth := lipgloss.Width(content)
		msg := " " + m.statusMessage + " "
		if len(msg) < contentWidth {
			msg += strings.Repeat(" ", contentWidth-len(msg))
		}
		content += "\n"
		if m.statusIsError {
			content += StatusErrorStyle.Render(msg)
		} else {
			content += StatusSuccessStyle.Render(msg)
		}
	}

	// Apply box style
	return BoxStyle.Render(content)
}

// helpView renders the context-dependent help overlay
func (m *MainModel) helpView() string {
	var b strings.Builder

	b.WriteString(HeaderStyle.Render("Keyboard Shortcuts"))
	b.WriteString("\n\n")

	type helpEntry struct {
		key  string
		desc string
	}

	// Common keys
	common := []helpEntry{
		{"↑ / k", "Move up"},
		{"↓ / j", "Move down"},
		{"Enter", "Select / Confirm"},
		{"Ctrl+C", "Quit application"},
		{"?", "Toggle this help"},
	}

	// View-specific keys
	var specific []helpEntry
	var actions []helpEntry
	var viewName string

	switch m.viewState {
	case ViewMainMenu:
		viewName = "Main Menu"
		specific = []helpEntry{
			{"q / Esc", "Quit"},
		}
	case ViewFileBrowser:
		viewName = "File Browser"
		specific = []helpEntry{
			{"Enter", "Open directory / Mount or run file"},
			{"← / Backspace / h", "Go to parent directory"},
			{"Esc", "Back to main menu"},
		}
		actions = []helpEntry{
			{".d64 / .g64 / .d81", "Mount disk image (select target drive)"},
			{".prg", "Run program"},
			{".crt", "Run cartridge"},
			{".sid", "Play SID music"},
		}
	case ViewDrives:
		viewName = "Drive Management"
		specific = []helpEntry{
			{"Enter", "Open drive actions"},
			{"Esc / q", "Back to main menu"},
		}
		actions = []helpEntry{
			{"Unmount", "Eject mounted disk image"},
			{"Load Custom ROM", "Select ROM file from browser"},
			{"Reset Drive", "Soft reset the drive"},
			{"Enable / Disable", "Toggle drive on/off"},
			{"Mode: 1541/1571/1581", "Switch drive emulation mode"},
		}
	case ViewMachineControl:
		viewName = "Machine Control"
		specific = []helpEntry{
			{"Enter", "Execute selected action"},
			{"Esc / q", "Back to main menu"},
		}
		actions = []helpEntry{
			{"Reset", "Reset the machine"},
			{"Reboot", "Restart with cartridge reinit"},
			{"Pause / Resume", "Pause or resume execution (DMA)"},
			{"Menu Button", "Simulate pressing Menu button"},
			{"Power Off", "Power off machine (U64 only)"},
		}
	case ViewSettings:
		viewName = "Configuration"
		specific = []helpEntry{
			{"Enter", "Edit value / Toggle boolean / Open category"},
			{"Esc / q", "Back / Main menu"},
		}
		actions = []helpEntry{
			{"Browse/Edit", "View and edit device settings"},
			{"Save to Flash", "Persist settings to flash memory"},
			{"Load from Flash", "Reload settings from flash"},
			{"Reset to Defaults", "Reset to factory defaults"},
			{"Save to File", "Export configuration to remote file"},
		}
	case ViewFileViewer:
		viewName = "File Viewer"
		specific = []helpEntry{
			{"Tab", "Toggle Text / Hex view"},
			{"↑ / ↓ / j / k", "Scroll line by line"},
			{"PgUp / PgDn", "Scroll page by page"},
			{"Esc / q", "Back to file browser"},
		}
	}

	b.WriteString(HelpDescStyle.Render("View: " + viewName))
	b.WriteString("\n\n")

	// Render specific keys first (most relevant)
	for _, entry := range specific {
		b.WriteString("  " + HelpKeyStyle.Render(fmt.Sprintf("%-22s", entry.key)) + HelpDescStyle.Render(entry.desc) + "\n")
	}

	// Render available actions if any
	if len(actions) > 0 {
		b.WriteString("\n")
		b.WriteString(HelpDescStyle.Render("Available Actions:"))
		b.WriteString("\n")
		for _, entry := range actions {
			b.WriteString("  " + HelpKeyStyle.Render(fmt.Sprintf("%-22s", entry.key)) + HelpDescStyle.Render(entry.desc) + "\n")
		}
	}

	b.WriteString("\n")
	b.WriteString(HelpDescStyle.Render("General:"))
	b.WriteString("\n")

	for _, entry := range common {
		b.WriteString("  " + HelpKeyStyle.Render(fmt.Sprintf("%-22s", entry.key)) + HelpDescStyle.Render(entry.desc) + "\n")
	}

	b.WriteString("\n")
	b.WriteString(StatusBarStyle.Render("Press Esc or ? to close"))

	return b.String()
}

// clearStatusAfter returns a tea.Cmd that clears the status after a delay
func (m *MainModel) clearStatusAfter(id int, d time.Duration) tea.Cmd {
	return tea.Tick(d, func(t time.Time) tea.Msg {
		return ClearStatusMsg{ID: id}
	})
}

// Run starts the TUI program
func (m *MainModel) Run() error {
	p := tea.NewProgram(m, tea.WithAltScreen())
	_, err := p.Run()
	return err
}
