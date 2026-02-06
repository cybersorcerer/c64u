package tui

import (
	"fmt"
	"sort"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/cybersorcerer/c64.nvim/tools/c64u/internal/api"
	"github.com/cybersorcerer/c64.nvim/tools/c64u/internal/output"
)

// BrowserState represents the state of the browser
type BrowserState int

const (
	BrowserBrowsing BrowserState = iota
	BrowserSelectingDrive
)

// BrowserModel handles the file browser view
type BrowserModel struct {
	client     *api.Client
	state      BrowserState
	currentDir string
	files      []api.FileEntry
	cursor     int
	offset     int
	width      int
	height     int
	loading    bool
	err        error
	message    string // status message

	// Drive Selection
	driveSelector *Selector
	mountingFile  api.FileEntry

	// File Picking Mode
	pickingMode  bool
	pickingDir   bool
	pickingDrive string
}

// NewBrowserModel creates a new browser
func NewBrowserModel(client *api.Client) *BrowserModel {
	return &BrowserModel{
		client:     client,
		state:      BrowserBrowsing,
		currentDir: "/",
		cursor:     0,
		offset:     0,
		loading:    true,
	}
}

// Init loads the initial directory
func (m *BrowserModel) Init() tea.Cmd {
	return m.fetchFilesCmd(m.currentDir)
}

// fetchFilesCmd returns a command to fetch files
func (m *BrowserModel) fetchFilesCmd(path string) tea.Cmd {
	return func() tea.Msg {
		DebugLog("BrowserModel: fetching files for %s", path)
		if !strings.HasPrefix(path, "/") {
			path = "/" + path
		}

		files, err := m.client.FTPList(path)
		if err != nil {
			return errMsg{err}
		}

		// Sort files: Directories first, then alphabetical
		sort.Slice(files, func(i, j int) bool {
			if files[i].IsDir && !files[j].IsDir {
				return true
			}
			if !files[i].IsDir && files[j].IsDir {
				return false
			}
			return strings.ToLower(files[i].Name) < strings.ToLower(files[j].Name)
		})

		// Add ".." if not root
		if path != "/" {
			parent := api.FileEntry{
				Name:  "..",
				IsDir: true,
				Type:  "dir",
			}
			files = append([]api.FileEntry{parent}, files...)
		}

		return fileListMsg{path: path, files: files}
	}
}

// fetchDrivesCmd queries enabled drives
func (m *BrowserModel) fetchDrivesCmd() tea.Cmd {
	return func() tea.Msg {
		resp, err := m.client.DrivesList()
		if err != nil {
			return errMsg{err}
		}
		if resp.HasErrors() {
			return errMsg{fmt.Errorf("API errors: %v", resp.Errors)}
		}

		drives, ok := resp.Data["drives"].([]interface{})
		if !ok || len(drives) == 0 {
			return statusMsg("No drives found")
		}

		var items []SelectorItem
		for _, driveData := range drives {
			driveMap, ok := driveData.(map[string]interface{})
			if !ok {
				continue
			}

			for driveName, driveInfo := range driveMap {
				info, ok := driveInfo.(map[string]interface{})
				if !ok {
					continue
				}

				if enabled, ok := info["enabled"].(bool); !ok || !enabled {
					continue
				}

				// Copy logic from CLI: "Drive A" -> "a"
				driveLetter := strings.ToLower(strings.TrimSpace(strings.TrimPrefix(driveName, "Drive")))

				desc := ""
				if img, ok := info["image_file"].(string); ok && img != "" {
					desc = "Mounted: " + img
				}

				items = append(items, SelectorItem{
					Label:       driveName,
					Value:       driveLetter,
					Description: desc,
				})
			}
		}

		if len(items) == 0 {
			return statusMsg("No enabled drives found")
		}

		return driveListMsg(items)
	}
}

// Messages
type fileListMsg struct {
	path  string
	files []api.FileEntry
}

type driveListMsg []SelectorItem

type errMsg struct{ err error }

type statusMsg string

// Update handles browser events
func (m *BrowserModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case fileListMsg:
		m.loading = false
		m.currentDir = msg.path
		m.files = msg.files
		m.cursor = 0
		m.offset = 0
		return m, nil

	case driveListMsg:
		m.driveSelector = NewSelector("Select Target Drive", []SelectorItem(msg))
		m.driveSelector.PreventQuit = true
		m.state = BrowserSelectingDrive
		m.message = ""
		return m, nil

	case errMsg:
		m.loading = false
		m.err = msg.err
		return m, nil

	case statusMsg:
		m.message = string(msg)
		return m, nil

	case FilePickerRequestMsg:
		DebugLog("BrowserModel: FilePickerRequestMsg received")
		m.pickingMode = true
		m.pickingDrive = msg.Drive
		m.loading = true
		m.files = nil
		m.cursor = 0
		m.message = "Select a file for Drive " + strings.ToUpper(msg.Drive)
		return m, nil

	case DirPickerRequestMsg:
		m.pickingDir = true
		m.loading = true
		m.cursor = 0
		m.message = "Navigate to folder and press 's' to select"
		return m, nil

	case tea.KeyMsg:
		// Handle Drive Selection State
		if m.state == BrowserSelectingDrive {
			if msg.String() == "esc" {
				m.state = BrowserBrowsing
				m.message = "Mount cancelled"
				return m, nil
			}

			// Delegate
			_, cmd := m.driveSelector.Update(msg)

			if m.driveSelector.cancelled {
				m.driveSelector = nil
				m.state = BrowserBrowsing
				m.message = "Mount cancelled"
				return m, nil
			}

			if m.driveSelector.confirmed {
				m.driveSelector.confirmed = false
				selected := m.driveSelector.Items[m.driveSelector.selected]
				m.state = BrowserBrowsing
				return m, m.mountFileCmd(m.mountingFile, selected.Value)
			}
			return m, cmd
		}

		// Handle Browsing State
		switch msg.String() {
		case "up", "k":
			if m.cursor > 0 {
				m.cursor--
			} else if m.offset > 0 {
				m.offset--
			}
		case "down", "j":
			if m.cursor < len(m.files)-1 {
				m.cursor++
				if m.cursor >= m.offset+m.height-5 {
					m.offset++
				}
			}

		case "s":
			if m.pickingDir {
				// Pick current directory
				dir := m.currentDir
				m.pickingDir = false
				return m, func() tea.Msg { return FileDirPickedMsg{Path: dir} }
			}

		case "enter":
			DebugLog("BrowserModel: Enter key pressed. Files: %d, Picker: %v", len(m.files), m.pickingMode)
			if len(m.files) == 0 {
				return m, nil
			}
			file := m.files[m.cursor]

			if file.Name == ".." {
				// Go Up
				parent := m.currentDir[:strings.LastIndex(m.currentDir, "/")]
				if parent == "" {
					parent = "/"
				}
				m.loading = true
				return m, m.fetchFilesCmd(parent)
			}

			if file.IsDir {
				newPath := m.currentDir + "/" + file.Name
				if m.currentDir == "/" {
					newPath = "/" + file.Name
				}

				m.loading = true
				return m, m.fetchFilesCmd(newPath)
			} else {
				// Handle Picking Mode
				if m.pickingMode {
					fullPath := m.currentDir + "/" + file.Name
					if m.currentDir == "/" {
						fullPath = "/" + file.Name
					}

					// Return Picked Message
					msg := FilePickedMsg{
						Drive:    m.pickingDrive,
						File:     fullPath,
						Filename: file.Name,
					}

					// Reset mode
					m.pickingMode = false
					m.pickingDrive = ""

					return m, func() tea.Msg { return msg }
				}

				// Attempt mount/run
				var ext string
				if dotIdx := strings.LastIndex(file.Name, "."); dotIdx >= 0 {
					ext = strings.ToLower(file.Name[dotIdx:])
				}
				if ext == ".d64" || ext == ".g64" || ext == ".d81" || ext == ".d71" || ext == ".g71" {
					m.mountingFile = file
					m.message = "Fetching drives..."
					return m, m.fetchDrivesCmd()
				}
				// Default to simple run for others
				return m, m.mountFileCmd(file, "")
			}

		case "backspace", "left", "h":
			if m.currentDir != "/" {
				parent := m.currentDir[:strings.LastIndex(m.currentDir, "/")]
				if parent == "" {
					parent = "/"
				}
				m.loading = true
				return m, m.fetchFilesCmd(parent)
			}

		case "esc":
			// Handled by parent
		}
	}
	return m, nil
}

func (m *BrowserModel) mountFileCmd(file api.FileEntry, drive string) tea.Cmd {
	return func() tea.Msg {
		var ext string
		if dotIdx := strings.LastIndex(file.Name, "."); dotIdx >= 0 {
			ext = strings.ToLower(file.Name[dotIdx:])
		}
		fullPath := m.currentDir + "/" + file.Name
		if m.currentDir == "/" {
			fullPath = "/" + file.Name
		}

		if ext == ".d64" || ext == ".g64" || ext == ".d81" || ext == ".d71" || ext == ".g71" {
			if drive == "" {
				return statusMsg("No drive selected")
			}
			_, err := m.client.DrivesMount(drive, fullPath, "", "")
			if err != nil {
				return statusMsg(fmt.Sprintf("Error mounting: %v", err))
			}
			return statusMsg("Mounted " + file.Name + " to Drive " + strings.ToUpper(drive))
		}
		if ext == ".prg" {
			_, err := m.client.RunPRG(fullPath)
			if err != nil {
				return statusMsg(fmt.Sprintf("Error running: %v", err))
			}
			return statusMsg("Running " + file.Name)
		}
		if ext == ".crt" {
			_, err := m.client.RunCRT(fullPath)
			if err != nil {
				return statusMsg(fmt.Sprintf("Error running: %v", err))
			}
			return statusMsg("Running " + file.Name)
		}
		if ext == ".sid" {
			_, err := m.client.SidPlay(fullPath, 0)
			if err != nil {
				return statusMsg(fmt.Sprintf("Error playing SID: %v", err))
			}
			return statusMsg("Playing " + file.Name)
		}

		// Unknown file type: read via FTP and show in viewer
		data, err := m.client.FTPReadFile(fullPath)
		if err != nil {
			return FileContentMsg{Filename: file.Name, Err: err}
		}
		return FileContentMsg{Filename: file.Name, Content: data}
	}
}

func (m *BrowserModel) View() string {
	if m.state == BrowserSelectingDrive && m.driveSelector != nil {
		return m.driveSelector.View()
	}

	if m.loading {
		return "Loading..."
	}
	if m.err != nil {
		return fmt.Sprintf("Error: %v", m.err)
	}

	var b strings.Builder

	// Use Header Style for Directory
	b.WriteString(HeaderStyle.Render("DIR: " + m.currentDir))
	b.WriteString("\n\n")

	start := m.offset
	end := start + m.height - 5 // Ensure space for status bar
	if end > len(m.files) {
		end = len(m.files)
	}

	for i := start; i < end; i++ {
		file := m.files[i]

		// Icon logic
		icon := output.GetFileIcon(file.Name, file.IsDir)

		displayName := file.Name
		if file.IsDir {
			displayName += "/"
		}

		// Special styling for ".."
		if file.Name == ".." {
			icon = "⬆"
		}

		if i == m.cursor {
			// Selected Item: Use Unified SelectedItemStyle which is Reverse video
			// We construct the content and render it all together for full line highlight effect
			lineContent := fmt.Sprintf("▶ %s %s", icon, displayName)
			b.WriteString(SelectedItemStyle.Render(lineContent) + "\n")
		} else {
			// Regular item
			style := ItemStyle
			// Apply specific color to ".." if not selected
			if file.Name == ".." {
				style = style.Copy().Foreground(lipgloss.Color("12"))
			}

			lineContent := fmt.Sprintf("  %s %s", icon, displayName)
			b.WriteString(style.Render(lineContent) + "\n")
		}
	}

	if m.message != "" {
		b.WriteString("\n" + StatusBarStyle.Render(m.message))
	} else {
		helpText := fmt.Sprintf("%d items • ↑/↓/j/k: nav • Enter: select • ←/Back: up • ESC: menu", len(m.files))
		b.WriteString("\n" + StatusBarStyle.Render(helpText))
	}

	return b.String()
}
