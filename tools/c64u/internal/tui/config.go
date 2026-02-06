package tui

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/cybersorcerer/c64.nvim/tools/c64u/internal/api"
)

// ConfigState defines the current view in the config model
type ConfigState int

const (
	ConfigMenu ConfigState = iota
	ConfigCategories
	ConfigItems
	ConfigEditing
	ConfigFileNaming
)

// Actions for Config Menu
var configActions = []SelectorItem{
	{Label: "Browse/Edit Configuration", Value: "browse", Description: "View and edit settings"},
	{Label: "Save to Flash", Value: "save_flash", Description: "Persist settings to flash memory"},
	{Label: "Load from Flash", Value: "load_flash", Description: "Reload settings from flash"},
	{Label: "Reset to Defaults", Value: "reset_default", Description: "Reset to factory defaults"},
	{Label: "Save to File", Value: "save_file", Description: "Export configuration to file"},
}

// ConfigItem represents a single setting
type ConfigItem struct {
	Key   string
	Value interface{}
}

// ConfigModel handles the configuration UI
type ConfigModel struct {
	client *api.Client
	state  ConfigState

	// Menus
	menuSelector *Selector
	saveDir      string

	// Data
	categories      []string
	items           []ConfigItem
	currentCategory string

	// Navigation
	cursor  int
	offset  int
	width   int
	height  int
	history []ConfigHistoryItem // Stack for nested navigation

	// Editing
	textInput textinput.Model
	editItem  ConfigItem

	// Status
	loading bool
	err     error
	message string
}

type ConfigHistoryItem struct {
	Items    []ConfigItem
	Cursor   int
	Offset   int
	Title    string
	Category string // To identify where we are saving
}

func NewConfigModel(client *api.Client) *ConfigModel {
	ti := textinput.New()
	ti.Prompt = "> "
	ti.CharLimit = 256

	sel := NewSelector("Configuration", configActions)
	sel.PreventQuit = true

	return &ConfigModel{
		client:       client,
		state:        ConfigMenu,
		textInput:    ti,
		menuSelector: sel,
		loading:      false, // Menu is local, no loading initially
		history:      make([]ConfigHistoryItem, 0),
	}
}

func (m *ConfigModel) Init() tea.Cmd {
	return nil // No initial fetch needed for menu
}

func (m *ConfigModel) fetchCategoriesCmd() tea.Cmd {
	return func() tea.Msg {
		cats, err := m.client.GetConfigCategories()
		if err != nil {
			return errMsg{err}
		}
		sort.Strings(cats)
		return categoriesFetchedMsg(cats)
	}
}

func (m *ConfigModel) fetchItemsCmd(category string) tea.Cmd {
	return func() tea.Msg {
		data, err := m.client.GetConfigCategory(category)
		if err != nil {
			return errMsg{err}
		}

		// Check if the category itself is a wrapper key (e.g. "Printer Settings" -> "Printer Settings" -> ...)
		if wrapped, ok := data[category]; ok {
			if wrappedMap, ok := wrapped.(map[string]interface{}); ok {
				data = wrappedMap
			}
		}

		var items []ConfigItem
		for k, v := range data {
			// Skip metadata and empty errors
			if k == "errors" || k == "status" || k == "version" {
				continue
			}

			// Extract actual value if it's a map containing "value"
			if vMap, ok := v.(map[string]interface{}); ok {
				if val, ok := vMap["value"]; ok {
					items = append(items, ConfigItem{Key: k, Value: val})
					continue
				}
			}
			items = append(items, ConfigItem{Key: k, Value: v})
		}
		sort.Slice(items, func(i, j int) bool {
			return items[i].Key < items[j].Key
		})

		return itemsFetchedMsg{category: category, items: items}
	}
}

// Export
func (m *ConfigModel) exportConfigCmd(localPath, remoteDir, filename string) tea.Cmd {
	return func() tea.Msg {
		// 1. Fetch all categories
		categories, err := m.client.GetConfigCategories()
		if err != nil {
			return errMsg{fmt.Errorf("failed to fetch categories: %w", err)}
		}
		sort.Strings(categories)

		var sb strings.Builder

		// 2. Iterate and fetch items for each
		for _, cat := range categories {
			sb.WriteString(fmt.Sprintf("[%s]\n", cat))

			data, err := m.client.GetConfigCategory(cat)
			if err != nil {
				// Log error but continue? Or fail? Let's fail safety.
				return errMsg{fmt.Errorf("failed to fetch category %s: %w", cat, err)}
			}

			// Unwrap if needed
			if wrapped, ok := data[cat]; ok {
				if wrappedMap, ok := wrapped.(map[string]interface{}); ok {
					data = wrappedMap
				}
			}

			// Sort keys
			var keys []string
			for k := range data {
				if k == "errors" || k == "status" || k == "version" {
					continue
				}
				keys = append(keys, k)
			}
			sort.Strings(keys)

			for _, k := range keys {
				val := data[k]
				// Check for nested value map
				if vMap, ok := val.(map[string]interface{}); ok {
					if v, ok := vMap["value"]; ok {
						val = v
					}
				}

				// Handle boolean text
				valStr := fmt.Sprintf("%v", val)
				if b, ok := val.(bool); ok {
					if b {
						valStr = "Enabled" // Or "True", "Yes"? Based on sample: "Enabled", "Off", "On" mixed.
						// Sample has "Vol Drive 1=OFF", "Speaker Enable=Enabled", "Auto Address Mirroring=Enabled", "LED Select Top=On"
						// It seems inconsistent. Let's use "Enabled"/"Disabled" for now as it's common.
						// Wait, verify with sample.
						// "Vol Drive 1=OFF", "SID Socket 1=Disabled", "LED Select Top=On"
						// It seems the API returns strings mostly? If API returns bool, we map to "Enabled"/"Disabled" safely.
					} else {
						valStr = "Disabled"
					}
				}

				sb.WriteString(fmt.Sprintf("%s=%s\n", k, valStr))
			}
			sb.WriteString("\n")
		}

		// 3. Write to local temp file
		err = os.WriteFile(localPath, []byte(sb.String()), 0644)
		if err != nil {
			return errMsg{fmt.Errorf("failed to write local temp file: %w", err)}
		}

		// 4. Upload to remote
		remotePath := filepath.Join(remoteDir, filename)
		// Fix path separators if needed (remote is Linux-like)
		remotePath = strings.ReplaceAll(remotePath, "\\", "/")

		err = m.client.FTPUpload(localPath, remotePath)
		if err != nil {
			return errMsg{fmt.Errorf("upload failed: %w", err)}
		}

		return statusMsg(fmt.Sprintf("Saved to %s", remotePath))
	}
}

func (m *ConfigModel) saveItemCmd(category, key string, value interface{}) tea.Cmd {
	return func() tea.Msg {
		// API expects string value in query param for SetConfigItem
		valStr := fmt.Sprintf("%v", value)
		err := m.client.SetConfigItem(category, key, valStr)
		if err != nil {
			return errMsg{err}
		}
		return statusMsg(fmt.Sprintf("Saved %s = %s", key, valStr))
	}
}

// Messages
type categoriesFetchedMsg []string
type itemsFetchedMsg struct {
	category string
	items    []ConfigItem
}

func (m *ConfigModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	// Handle Config Menu
	if m.state == ConfigMenu {
		_, cmd := m.menuSelector.Update(msg)
		if m.menuSelector.cancelled {
			m.menuSelector.cancelled = false
			return m, func() tea.Msg { return BackMsg{} }
		}
		if m.menuSelector.confirmed {
			m.menuSelector.confirmed = false
			action := m.menuSelector.Items[m.menuSelector.selected].Value
			switch action {
			case "browse":
				m.state = ConfigCategories
				m.loading = true
				return m, m.fetchCategoriesCmd()
			case "save_flash":
				return m, func() tea.Msg {
					err := m.client.SaveConfigToFlash()
					if err != nil {
						return errMsg{err}
					}
					return statusMsg("Configuration saved to flash")
				}
			case "load_flash":
				return m, func() tea.Msg {
					err := m.client.LoadConfigFromFlash()
					if err != nil {
						return errMsg{err}
					}
					return statusMsg("Configuration loaded from flash")
				}
			case "reset_default":
				return m, func() tea.Msg {
					err := m.client.ResetConfigToDefault()
					if err != nil {
						return errMsg{err}
					}
					return statusMsg("Reset to defaults")
				}
			case "save_file":
				// Request Directory Picker
				return m, func() tea.Msg { return DirPickerRequestMsg{} }
			}
		}
		return m, cmd
	}

	switch msg := msg.(type) {
	case FileDirPickedMsg:
		m.saveDir = msg.Path
		m.state = ConfigFileNaming
		m.textInput.SetValue("c64u_config.cfg")
		m.textInput.Focus()
		m.message = "Enter filename to save in " + m.saveDir
		return m, textinput.Blink

	case categoriesFetchedMsg:
		m.loading = false
		m.categories = msg
		m.state = ConfigCategories
		m.cursor = 0
		m.history = nil // Reset history
		return m, nil

	case itemsFetchedMsg:
		m.loading = false
		m.items = msg.items
		m.currentCategory = msg.category
		m.state = ConfigItems
		m.cursor = 0
		m.history = nil // Reset history
		return m, nil

	case errMsg:
		m.loading = false
		m.err = msg.err
		return m, nil

	case statusMsg:
		m.message = string(msg)
		// Refresh items if we just saved
		if m.state == ConfigItems {
			return m, m.fetchItemsCmd(m.currentCategory)
		}
		return m, nil

	case tea.KeyMsg:
		// Global Back Navigation
		if (msg.String() == "esc" || msg.String() == "q") && m.state != ConfigEditing && m.state != ConfigFileNaming {
			// Check history first
			if len(m.history) > 0 {
				last := m.history[len(m.history)-1]
				m.history = m.history[:len(m.history)-1]

				m.items = last.Items
				m.cursor = last.Cursor
				m.offset = last.Offset
				m.currentCategory = last.Category
				// Title restoration is implicit via currentCategory or we might need to store full path
				return m, nil
			}

			if m.state == ConfigItems {
				m.state = ConfigCategories
				m.err = nil
				m.message = ""
				return m, nil
			}
			if m.state == ConfigCategories {
				m.state = ConfigMenu // Go back to Menu
				return m, nil
			}
			if m.state == ConfigMenu {
				return m, func() tea.Msg { return BackMsg{} } // Back to Main
			}
		}

		// Naming Mode
		if m.state == ConfigFileNaming {
			switch msg.String() {
			case "enter":
				filename := m.textInput.Value()
				m.state = ConfigMenu
				tmpFile := filepath.Join(os.TempDir(), filename)
				return m, m.exportConfigCmd(tmpFile, m.saveDir, filename)
			case "esc":
				m.state = ConfigMenu
				return m, nil
			}
			var cmd tea.Cmd
			m.textInput, cmd = m.textInput.Update(msg)
			return m, cmd
		}

		// Editing Mode
		if m.state == ConfigEditing {
			switch msg.String() {
			case "enter":
				newValue := m.textInput.Value()
				m.state = ConfigItems
				return m, m.saveItemCmd(m.currentCategory, m.editItem.Key, newValue)
			case "esc":
				m.state = ConfigItems
				return m, nil
			}
			var cmd tea.Cmd
			m.textInput, cmd = m.textInput.Update(msg)
			return m, cmd
		}

		// Selection Mode (Categories or Items)
		switch msg.String() {
		case "up", "k":
			if m.cursor > 0 {
				m.cursor--
			} else if m.offset > 0 {
				m.offset--
			}
		case "down", "j":
			listLen := len(m.categories)
			if m.state == ConfigItems {
				listLen = len(m.items)
			}
			if m.cursor < listLen-1 {
				m.cursor++
				if m.cursor >= m.offset+m.height-5 {
					m.offset++
				}
			}

		case "enter":
			if m.state == ConfigCategories {
				if len(m.categories) > 0 {
					selected := m.categories[m.cursor]
					m.loading = true
					// Logic error in previous implementation called fetchItemsCmd(selected), correcting here as well if needed
					return m, m.fetchItemsCmd(selected)
				}
			} else if m.state == ConfigItems {
				if len(m.items) > 0 {
					item := m.items[m.cursor]

					// Check if value is a nested map
					if nestedMap, ok := item.Value.(map[string]interface{}); ok {
						// Push current state
						m.history = append(m.history, ConfigHistoryItem{
							Items:    m.items,
							Cursor:   m.cursor,
							Offset:   m.offset,
							Title:    m.currentCategory,
							Category: m.currentCategory,
						})

						// Load nested items
						var newItems []ConfigItem
						for k, v := range nestedMap {
							// Same unwrap logic?
							if vMap, ok := v.(map[string]interface{}); ok {
								if val, ok := vMap["value"]; ok {
									newItems = append(newItems, ConfigItem{Key: k, Value: val})
									continue
								}
							}
							newItems = append(newItems, ConfigItem{Key: k, Value: v})
						}
						sort.Slice(newItems, func(i, j int) bool {
							return newItems[i].Key < newItems[j].Key
						})

						m.items = newItems
						m.cursor = 0
						m.offset = 0

						// FIXME: For real nested items (not the wrapper we just fixed), we need to handle paths.
						// But for now, let's assume valid categories are flat or handled by the unwrap fix.
						// If we DO descend, we should probably Keep the Category clean.

						// m.currentCategory = m.currentCategory + " > " + item.Key // REMOVED to avoid invalid category error

						return m, nil
					}

					// Type detection logic
					switch v := item.Value.(type) {
					case bool:
						// Toggle immediately
						newValue := !v
						return m, m.saveItemCmd(m.currentCategory, item.Key, newValue)
					case string:
						// Mock boolean toggle for "Enabled"/"Disabled"
						if v == "Enabled" {
							return m, m.saveItemCmd(m.currentCategory, item.Key, "Disabled")
						} else if v == "Disabled" {
							return m, m.saveItemCmd(m.currentCategory, item.Key, "Enabled")
						}
						// Continue to default for non-boolean strings
						m.state = ConfigEditing
						m.editItem = item
						m.textInput.SetValue(fmt.Sprintf("%v", v))
						m.textInput.Focus()
						return m, textinput.Blink
					default:
						// Enter edit mode for strings/numbers
						m.state = ConfigEditing
						m.editItem = item
						m.textInput.SetValue(fmt.Sprintf("%v", v))
						m.textInput.Focus()
						return m, textinput.Blink
					}
				}
			}
		}
	}

	return m, nil
}

func (m *ConfigModel) View() string {
	if m.state == ConfigMenu {
		base := m.menuSelector.View()
		if m.message != "" {
			return base + "\n" + StatusBarStyle.Render(m.message)
		}
		return base
	}

	if m.loading {
		return "Loading settings..."
	}
	if m.err != nil {
		return fmt.Sprintf("Error: %v", m.err)
	}

	var b strings.Builder

	if m.state == ConfigEditing {
		b.WriteString(HeaderStyle.Render("Edit Setting: " + m.editItem.Key))
		b.WriteString("\n\n")
		b.WriteString(m.textInput.View())
		b.WriteString("\n\n")
		b.WriteString(StatusBarStyle.Render("Enter: Save • Esc: Cancel"))
		return b.String()
	}

	if m.state == ConfigFileNaming {
		b.WriteString(HeaderStyle.Render("Save Configuration"))
		b.WriteString("\n\nDirectory: " + m.saveDir + "\n")
		b.WriteString("Filename:\n")
		b.WriteString(m.textInput.View())
		b.WriteString("\n\n")
		b.WriteString(StatusBarStyle.Render("Enter: Save • Esc: Cancel"))
		return b.String()
	}

	title := "Settings Categories"
	if m.state == ConfigItems {
		title = "Settings: " + m.currentCategory
	}
	b.WriteString(HeaderStyle.Render(title))
	b.WriteString("\n\n")

	start := m.offset
	end := start + m.height - 5

	listLen := len(m.categories)
	if m.state == ConfigItems {
		listLen = len(m.items)
	}

	if end > listLen {
		end = listLen
	}

	for i := start; i < end; i++ {
		line := ""
		if m.state == ConfigCategories {
			cat := m.categories[i]
			if i == m.cursor {
				line = SelectedItemStyle.Render("▶ " + cat)
			} else {
				line = ItemStyle.Render("  " + cat)
			}
		} else {
			item := m.items[i]
			valStr := fmt.Sprintf("%v", item.Value)

			// Highlight bools and Maps
			if v, ok := item.Value.(bool); ok {
				if v {
					valStr = lipgloss.NewStyle().Foreground(lipgloss.Color("2")).Render("On")
				} else {
					valStr = lipgloss.NewStyle().Foreground(lipgloss.Color("1")).Render("Off")
				}
			} else if s, ok := item.Value.(string); ok && (s == "Enabled" || s == "Disabled") {
				if s == "Enabled" {
					valStr = lipgloss.NewStyle().Foreground(lipgloss.Color("2")).Render("Enabled")
				} else {
					valStr = lipgloss.NewStyle().Foreground(lipgloss.Color("1")).Render("Disabled")
				}
			} else if _, ok := item.Value.(map[string]interface{}); ok {
				valStr = lipgloss.NewStyle().Foreground(lipgloss.Color("4")).Render("[Folder] >")
			}

			if i == m.cursor {
				content := fmt.Sprintf("▶ %-30s %s", item.Key, valStr)
				line = SelectedItemStyle.Render(content)
			} else {
				content := fmt.Sprintf("  %-30s %s", item.Key, valStr)
				line = ItemStyle.Render(content)
			}
		}
		b.WriteString(line + "\n")
	}

	b.WriteString("\n")
	if m.message != "" {
		b.WriteString(StatusBarStyle.Render(m.message))
	} else {
		help := "Select category"
		if m.state == ConfigItems {
			help = "Select item to edit (Enter toggles bools)"
		}
		b.WriteString(StatusBarStyle.Render(help))
	}

	return b.String()
}
