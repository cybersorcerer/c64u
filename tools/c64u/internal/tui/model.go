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
	ViewStreams
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
	{"Streams", ViewStreams},
}

// MainModel is the top-level bubble tea model
type MainModel struct {
	client    *api.Client
	host      string
	viewState ViewState
	width     int
	height    int

	// Child models
	browser *BrowserModel
	drives  *DrivesModel
	machine *MachineModel
	config  *ConfigModel
	viewer  *ViewerModel
	streams *StreamsModel

	selector  *Selector // Main menu selector
	activeTab int
	device    deviceInfo

	// Help overlay
	showHelp bool

	// Global status line
	statusMessage string
	statusIsError bool
	statusID      int // incremented per message, used for auto-clear

	// Set before tea.Quit to signal ui.go to launch a blocking stream
	PendingStream string

	// Shown in the status line once the TUI starts. Used by ui.go to report
	// what went wrong in a stream it ran while the TUI was suspended.
	InitialStatus string
}

// NewMainModel creates the initial model
func NewMainModel(client *api.Client, host string) *MainModel {
	m := &MainModel{
		client:    client,
		host:      host,
		viewState: ViewFileBrowser,
		activeTab: 0,
		browser:   NewBrowserModel(client),
		drives:    NewDrivesModel(client),
		machine:   NewMachineModel(client),
		config:    NewConfigModel(client),
		viewer:    NewViewerModel(),
		streams:   NewStreamsModel(client, host),
	}
	return m
}

func (m *MainModel) Init() tea.Cmd {
	cmds := []tea.Cmd{
		m.browser.Init(),
		m.fetchDeviceInfoCmd(),
	}
	if m.InitialStatus != "" {
		status := m.InitialStatus
		cmds = append(cmds, func() tea.Msg { return statusMsg(status) })
	}
	return tea.Batch(cmds...)
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
	case ViewStreams:
		return m.streams.Init()
	}
	return nil
}

// inModalState reports whether the active view is in a modal sub-state
// (text input or selector dialog), so global navigation shortcuts
// (1-5, Ctrl+h/l) must be suppressed and routed to the view instead.
func (m *MainModel) inModalState() bool {
	switch m.viewState {
	case ViewFileBrowser:
		switch m.browser.state {
		case browserDeleting, browserRenaming, browserMkdir,
			browserNewDisk, browserNewDiskNaming, browserDiskAction:
			return true
		}
	case ViewSettings:
		switch m.config.state {
		case ConfigEditing, ConfigFileNaming:
			return true
		}
	case ViewDrives:
		// Typing a device number or a directory, or picking an action.
		return m.drives.state != drivesBrowsing || m.drives.actionSelector != nil
	}
	return false
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

		// View navigation: number keys jump directly, Ctrl+h/l step.
		// Suppressed while a modal sub-state (text input / selector) is active,
		// so those keys reach the view instead of switching tabs.
		if !m.inModalState() {
			switch msg.String() {
			case "1", "2", "3", "4", "5":
				idx := int(msg.String()[0] - '1')
				if idx >= 0 && idx < len(tabs) {
					m.activeTab = idx
					m.viewState = tabs[idx].State
					return m, m.initActiveView()
				}
			case "ctrl+l":
				m.activeTab = (m.activeTab + 1) % len(tabs)
				m.viewState = tabs[m.activeTab].State
				return m, m.initActiveView()
			case "ctrl+h":
				m.activeTab = (m.activeTab - 1 + len(tabs)) % len(tabs)
				m.viewState = tabs[m.activeTab].State
				return m, m.initActiveView()
			}
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
		// Deliberately no early return: the active view has to see the message
		// too, so it can refresh state the action just changed on the device.
		cmds = append(cmds, m.clearStatusAfter(m.statusID, duration))

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
		contentH := msg.Height - 2 // header + tabbar
		m.browser.width = msg.Width
		m.browser.height = contentH
		m.drives.width = msg.Width
		m.drives.height = contentH

		m.machine.width = msg.Width
		m.streams.width = msg.Width
		m.streams.height = contentH
		m.viewer.width = msg.Width
		m.viewer.height = contentH
		m.config.width = msg.Width
		m.config.height = contentH
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

	case ViewStreams:
		newStreams, cmd := m.streams.Update(msg)
		m.streams = newStreams.(*StreamsModel)
		cmds = append(cmds, cmd)
	}

	// Route stream messages regardless of active view
	switch msg.(type) {
	case streamStartedMsg, streamStoppedMsg, streamErrMsg:
		newStreams, cmd := m.streams.Update(msg)
		m.streams = newStreams.(*StreamsModel)
		cmds = append(cmds, cmd)
	}

	// Blocking stream request: quit TUI, let ui.go take over
	if req, ok := msg.(requestStreamMsg); ok {
		m.PendingStream = req.id
		return m, tea.Quit
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
	header := HeaderStyle.Width(m.width).Render(title)

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
	padding := m.width - tabsW
	if padding < 0 {
		padding = 0
	}
	tabBar := TabBarStyle.Width(m.width).Render(
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
		case ViewStreams:
			content = m.streams.View()
		}
	}

	// Before the first WindowSizeMsg there is no height to fit into.
	if m.height <= 2 {
		return lipgloss.JoinVertical(lipgloss.Left, header, tabBar, content)
	}

	// The frame is always exactly m.height rows. A frame that shrinks — which is
	// what happened when a status message auto-cleared — leaves the previous
	// frame's trailing rows behind, so the view's own footer appeared to vanish.
	lines := fitLines(content, m.height-2)

	// Global status message (e.g. after an action): pinned to the bottom row.
	// Child views render their own footers; MainModel only owns this one line.
	if m.statusMessage != "" {
		msg := " " + m.statusMessage + " "
		if m.statusIsError {
			lines[len(lines)-1] = StatusErrorStyle.Width(m.width).Render(msg)
		} else {
			lines[len(lines)-1] = StatusSuccessStyle.Width(m.width).Render(msg)
		}
	}

	return strings.Join(append([]string{header, tabBar}, lines...), "\n")
}

// fitLines pads s with blank lines, or truncates it, so the result is exactly
// n lines.
func fitLines(s string, n int) []string {
	lines := strings.Split(s, "\n")
	if len(lines) > n {
		return lines[:n]
	}
	for len(lines) < n {
		lines = append(lines, "")
	}
	return lines
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
		{"1-5", "Jump to view"},
		{"Ctrl+h / Ctrl+l", "Previous / next view"},
		{"j / k", "Move down / up"},
		{"g / G", "Top / bottom"},
		{"Ctrl+d / Ctrl+u", "Half page down / up"},
		{"Enter", "Select / open"},
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
		viewName = "File Browser (dual-pane)"
		specific = []helpEntry{
			{"Tab / h / l", "Switch active pane (local/remote)"},
			{"Enter", "Open dir / disk actions / run PRG·CRT·SID·MOD"},
			{"Backspace", "Parent directory"},
			{"Space", "Mark file (batch)"},
			{"F5 / c", "Copy to other pane"},
			{"d / r / m", "Delete / rename / mkdir"},
			{"n", "New disk image (remote pane)"},
			{"u", "Apply to drive: mount disk / load ROM"},
			{"p", "Toggle preview column"},
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
			{"Set directory", "SoftIEC: serve this directory (C64 must be at BASIC)"},
			{"Set device number", "SoftIEC: IEC bus number to answer on"},
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
	b.WriteString(StatusBarStyle.Width(m.width).Render("Press Esc or ? to close"))

	return b.String()
}

// clearStatusAfter returns a tea.Cmd that clears the status after a delay
func (m *MainModel) clearStatusAfter(id int, d time.Duration) tea.Cmd {
	return tea.Tick(d, func(t time.Time) tea.Msg {
		return ClearStatusMsg{ID: id}
	})
}

// ActiveTab returns the index of the tab currently shown. A caller that has to
// restart the TUI — see ui.go and PendingStream — uses it together with SetTab
// to put the user back where they were.
func (m *MainModel) ActiveTab() int { return m.activeTab }

// SetTab selects a tab before the TUI starts. Out-of-range indices are ignored.
func (m *MainModel) SetTab(idx int) {
	if idx < 0 || idx >= len(tabs) {
		return
	}
	m.activeTab = idx
	m.viewState = tabs[idx].State
}

// Run starts the TUI program
func (m *MainModel) Run() error {
	p := tea.NewProgram(m, tea.WithAltScreen())
	_, err := p.Run()
	return err
}

// RequestStream signals the TUI to quit and hand control to a blocking stream.
func (m *MainModel) requestStream(id string) tea.Cmd {
	return func() tea.Msg {
		m.PendingStream = id
		return tea.Quit()
	}
}
