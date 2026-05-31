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

type deviceInfo struct {
	Hostname string
	Firmware string
}

var tabs = []struct {
	Label string
	State ViewState
}{
	{"Files", ViewFileBrowser},
	{"Drives", ViewDrives},
	{"Machine", ViewMachineControl},
	{"Config", ViewSettings},
}

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

	selector  *Selector // Main menu selector
	activeTab int
	device    deviceInfo

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
		viewState: ViewFileBrowser,
		activeTab: 0,
		browser:   NewBrowserModel(client),
		drives:    NewDrivesModel(client),
		machine:   NewMachineModel(client),
		config:    NewConfigModel(client),
		viewer:    NewViewerModel(),
	}
	return m
}

func (m *MainModel) Init() tea.Cmd {
	return tea.Batch(
		m.browser.Init(),
		m.fetchDeviceInfoCmd(),
	)
}

func (m *MainModel) fetchDeviceInfoCmd() tea.Cmd {
	return func() tea.Msg {
		resp, err := m.client.GetInfo()
		if err != nil {
			return deviceInfoMsg{}
		}
		hostname, _ := resp.Data["hostname"].(string)
		firmware, _ := resp.Data["firmware"].(string)
		return deviceInfoMsg{Hostname: hostname, Firmware: firmware}
	}
}

func (m *MainModel) initActiveView() tea.Cmd {
	switch m.viewState {
	case ViewFileBrowser:
		return m.browser.Init()
	case ViewDrives:
		return m.drives.Init()
	case ViewMachineControl:
		return m.machine.Init()
	case ViewSettings:
		return m.config.Init()
	}
	return nil
}

func (m *MainModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
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

		// Tab navigation
		switch msg.String() {
		case "tab":
			m.activeTab = (m.activeTab + 1) % len(tabs)
			m.viewState = tabs[m.activeTab].State
			return m, m.initActiveView()
		case "shift+tab":
			m.activeTab = (m.activeTab - 1 + len(tabs)) % len(tabs)
			m.viewState = tabs[m.activeTab].State
			return m, m.initActiveView()
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

	case deviceInfoMsg:
		m.device = deviceInfo{Hostname: msg.Hostname, Firmware: msg.Firmware}
		return m, nil

	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		m.browser.width = msg.Width - 4
		m.browser.height = msg.Height - 6
		m.drives.width = msg.Width - 4
		m.drives.height = msg.Height - 6
		m.config.width = msg.Width - 4
		m.config.height = msg.Height - 6
		return m, nil
	}

	// Handle views
	switch m.viewState {
	case ViewFileBrowser:
		newBrowser, cmd := m.browser.Update(msg)
		m.browser = newBrowser.(*BrowserModel)
		cmds = append(cmds, cmd)

	case ViewDrives:
		newDrives, cmd := m.drives.Update(msg)
		m.drives = newDrives.(*DrivesModel)
		cmds = append(cmds, cmd)

	case ViewMachineControl:
		newMachine, cmd := m.machine.Update(msg)
		m.machine = newMachine.(*MachineModel)
		cmds = append(cmds, cmd)

	case ViewSettings:
		newConfig, cmd := m.config.Update(msg)
		m.config = newConfig.(*ConfigModel)
		cmds = append(cmds, cmd)

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
	if m.width == 0 {
		return "Loading..."
	}

	// Header
	title := "C64 Ultimate"
	if m.device.Hostname != "" {
		title += " — " + m.device.Hostname
	}
	if m.device.Firmware != "" {
		title += "  fw: " + m.device.Firmware
	}
	header := HeaderStyle.Width(m.width - 2).Render(title)

	// Tab bar
	var tabParts []string
	for i, t := range tabs {
		if i == m.activeTab {
			tabParts = append(tabParts, TabActiveStyle.Render(t.Label))
		} else {
			tabParts = append(tabParts, TabStyle.Render(t.Label))
		}
	}
	tabsW := 0
	for _, p := range tabParts {
		tabsW += lipgloss.Width(p)
	}
	padding := m.width - 2 - tabsW
	if padding < 0 {
		padding = 0
	}
	tabBar := TabBarStyle.Width(m.width - 2).Render(
		strings.Join(tabParts, "") + strings.Repeat(" ", padding),
	)

	// Content
	var content string
	if m.showHelp {
		content = m.helpView()
	} else {
		switch m.viewState {
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

	// Status bar
	statusMsg := " Tab/S-Tab: switch view • ?: help • Ctrl+C: quit"
	if m.statusMessage != "" {
		statusMsg = " " + m.statusMessage + " "
	}
	statusW := m.width - 2
	if len(statusMsg) < statusW {
		statusMsg += strings.Repeat(" ", statusW-len(statusMsg))
	}
	var statusBar string
	if m.statusIsError {
		statusBar = StatusErrorStyle.Width(m.width - 2).Render(statusMsg)
	} else {
		statusBar = StatusBarStyle.Width(m.width - 2).Render(statusMsg)
	}

	inner := lipgloss.JoinVertical(lipgloss.Left, header, tabBar, content, statusBar)
	return BoxStyle.Width(m.width).Render(inner)
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
		{"Tab / Shift+Tab", "Switch view"},
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
			{"Enter", "Open dir / View disk / Mount or run file"},
			{"← / Backspace / h", "Go to parent directory"},
			{"d", "Delete file or directory (with confirmation)"},
			{"r", "Rename file or directory"},
			{"m", "Create new directory"},
			{"n", "Create new disk image (D64/D71/D81)"},
			{"Esc", "Back to previous view"},
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
