package tui

import (
	"fmt"
	"os"
	"strings"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/cybersorcerer/c64.nvim/tools/c64u/internal/api"
)

type browserState int

const (
	browserBrowsing browserState = iota
	browserDeleting
	browserRenaming
	browserMkdir
	browserNewDisk
	browserNewDiskNaming
	browserDiskAction
)

// BrowserModel is the dual-pane file browser (local | remote | preview).
type BrowserModel struct {
	client  *api.Client
	local   *PaneModel
	remote  *PaneModel
	activeP *PaneModel
	width   int
	height  int

	showPreview bool
	previewData []byte
	previewName string

	state   browserState
	ti      textinput.Model
	message string

	newDiskFormat   string
	newDiskSelector *Selector

	diskSelector *Selector
	diskPath     string
}

func NewBrowserModel(client *api.Client) *BrowserModel {
	home, err := os.UserHomeDir()
	if err != nil || home == "" {
		home = "/"
	}
	local := newPaneModel(&localSource{}, "LOCAL", home)
	remote := newPaneModel(&remoteSource{client: client}, "REMOTE", "/")

	ti := textinput.New()
	ti.CharLimit = 64

	m := &BrowserModel{
		client:      client,
		local:       local,
		remote:      remote,
		showPreview: true,
		state:       browserBrowsing,
		ti:          ti,
	}
	local.active = true
	m.activeP = local
	return m
}

func (m *BrowserModel) Init() tea.Cmd {
	return func() tea.Msg {
		m.local.reload()  //nolint:errcheck
		m.remote.reload() //nolint:errcheck
		return paneReloadMsg{side: "both"}
	}
}

func (m *BrowserModel) other() *PaneModel {
	if m.activeP == m.local {
		return m.remote
	}
	return m.local
}

func (m *BrowserModel) setActive(p *PaneModel) {
	m.local.active = false
	m.remote.active = false
	p.active = true
	m.activeP = p
	m.previewName = ""
}

func (m *BrowserModel) layoutPanes() {
	paneH := m.height - 1
	if paneH < 3 {
		paneH = 3
	}
	cols := 2
	if m.showPreview {
		cols = 3
	}
	colW := (m.width - (cols - 1)) / cols
	if colW < 10 {
		colW = 10
	}
	m.local.width, m.local.height = colW, paneH
	m.remote.width, m.remote.height = colW, paneH
}

func (m *BrowserModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case paneReloadMsg:
		if msg.err != nil {
			m.message = "error: " + msg.err.Error()
		}
		return m, m.refreshPreviewCmd()

	case transferDoneMsg:
		if msg.err != nil {
			m.message = "transfer failed: " + msg.err.Error()
		} else {
			m.message = fmt.Sprintf("Copied %d item(s)", msg.count)
		}
		m.activeP.clearMarks()
		m.other().reload() //nolint:errcheck
		return m, nil

	case previewLoadedMsg:
		m.previewData = msg.data
		m.previewName = msg.name
		return m, nil

	case diskDirLoadedMsg:
		if msg.err != nil {
			m.message = "cannot read disk: " + msg.err.Error()
			m.state = browserBrowsing
			return m, nil
		}
		m.diskPath = msg.path
		m.diskSelector = NewSelector("Disk: "+msg.name, diskActionItems(msg.hasPRG))
		m.state = browserDiskAction
		return m, nil

	case tea.KeyMsg:
		return m.handleKey(msg)
	}
	return m, nil
}

func (m *BrowserModel) handleKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	key := msg.String()

	switch m.state {
	case browserDeleting:
		switch key {
		case "y", "Y":
			cur, ok := m.activeP.currentItem()
			m.state = browserBrowsing
			m.message = ""
			if ok {
				return m, m.deleteCmd(cur)
			}
		case "n", "N", "esc":
			m.state = browserBrowsing
			m.message = ""
		}
		return m, nil

	case browserRenaming, browserMkdir, browserNewDiskNaming:
		return m.handleTextInput(msg)

	case browserNewDisk:
		return m.handleNewDiskSelect(msg)

	case browserDiskAction:
		return m.handleDiskAction(msg)
	}

	switch key {
	case "tab":
		m.setActive(m.other())
		return m, m.refreshPreviewCmd()
	case "h", "left":
		m.setActive(m.local)
		return m, m.refreshPreviewCmd()
	case "l":
		m.setActive(m.remote)
		return m, m.refreshPreviewCmd()
	case "p":
		m.showPreview = !m.showPreview
		return m, nil
	case "j", "k", "down", "up", "g", "G", "ctrl+d", "ctrl+u":
		m.activeP.handleNav(key)
		return m, m.refreshPreviewCmd()
	case "enter":
		return m.handleEnter()
	case "backspace":
		m.activeP.goParent() //nolint:errcheck
		return m, m.refreshPreviewCmd()
	case " ":
		if cur, ok := m.activeP.currentItem(); ok && cur.Name != ".." {
			m.activeP.toggleMark()
		}
		m.activeP.handleNav("j")
		return m, nil
	case "f5", "c":
		return m, m.copyCmd()
	case "d":
		cur, ok := m.activeP.currentItem()
		if ok && cur.Name != ".." {
			m.state = browserDeleting
			m.message = fmt.Sprintf("Delete %q? (y/n)", cur.Name)
		}
		return m, nil
	case "r":
		cur, ok := m.activeP.currentItem()
		if ok && cur.Name != ".." {
			m.state = browserRenaming
			m.ti.SetValue(cur.Name)
			m.ti.Focus()
			m.message = "Rename to:"
			return m, textinput.Blink
		}
		return m, nil
	case "m":
		m.state = browserMkdir
		m.ti.SetValue("")
		m.ti.Focus()
		m.message = "New folder:"
		return m, textinput.Blink
	case "n":
		if !m.activeP.src.IsLocal() {
			m.state = browserNewDisk
			m.newDiskSelector = NewSelector("New Disk Image", []SelectorItem{
				{Label: "D64 (1541)", Value: "d64", Description: "170KB, 35 tracks"},
				{Label: "D71 (1571)", Value: "d71", Description: "340KB, 70 tracks"},
				{Label: "D81 (1581)", Value: "d81", Description: "800KB, 80 tracks"},
			})
		} else {
			m.message = "New disk only on remote pane"
		}
		return m, nil
	}
	return m, nil
}

func (m *BrowserModel) handleEnter() (tea.Model, tea.Cmd) {
	cur, ok := m.activeP.currentItem()
	if !ok {
		return m, nil
	}
	if cur.Name == ".." {
		m.activeP.goParent() //nolint:errcheck
		return m, m.refreshPreviewCmd()
	}
	if cur.IsDir {
		m.activeP.enterDir(cur.Name) //nolint:errcheck
		return m, m.refreshPreviewCmd()
	}
	if !m.activeP.src.IsLocal() {
		ext := strings.ToLower(extOf(cur.Name))
		switch ext {
		case ".d64", ".d71", ".d81", ".g64", ".g71":
			return m, m.loadDiskDirCmd(m.activeP.join(cur.Name))
		case ".prg":
			return m, m.runPRGCmd(m.activeP.join(cur.Name))
		}
	}
	return m, nil
}

func (m *BrowserModel) handleTextInput(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "enter":
		val := strings.TrimSpace(m.ti.Value())
		st := m.state
		m.state = browserBrowsing
		m.message = ""
		m.ti.Blur()
		if val == "" {
			return m, nil
		}
		switch st {
		case browserRenaming:
			cur, ok := m.activeP.currentItem()
			if ok {
				return m, m.renameCmd(cur.Name, val)
			}
		case browserMkdir:
			return m, m.mkdirCmd(val)
		case browserNewDiskNaming:
			return m, m.createDiskCmd(m.newDiskFormat, val)
		}
		return m, nil
	case "esc":
		m.state = browserBrowsing
		m.message = ""
		m.ti.Blur()
		return m, nil
	default:
		var cmd tea.Cmd
		m.ti, cmd = m.ti.Update(msg)
		return m, cmd
	}
}

func (m *BrowserModel) handleNewDiskSelect(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	if msg.String() == "esc" {
		m.state = browserBrowsing
		m.newDiskSelector = nil
		return m, nil
	}
	m.newDiskSelector.Update(msg) //nolint:errcheck
	if m.newDiskSelector.cancelled {
		m.state = browserBrowsing
		m.newDiskSelector = nil
		return m, nil
	}
	if m.newDiskSelector.confirmed {
		m.newDiskFormat = m.newDiskSelector.Items[m.newDiskSelector.selected].Value
		m.newDiskSelector = nil
		m.state = browserNewDiskNaming
		m.ti.SetValue("disk." + m.newDiskFormat)
		m.ti.Focus()
		m.message = "Filename:"
		return m, textinput.Blink
	}
	return m, nil
}

func (m *BrowserModel) handleDiskAction(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	if msg.String() == "esc" {
		m.state = browserBrowsing
		m.diskSelector = nil
		return m, nil
	}
	m.diskSelector.Update(msg) //nolint:errcheck
	if m.diskSelector.cancelled {
		m.state = browserBrowsing
		m.diskSelector = nil
		return m, nil
	}
	if m.diskSelector.confirmed {
		action := m.diskSelector.Items[m.diskSelector.selected].Value
		m.diskSelector = nil
		m.state = browserBrowsing
		return m, m.diskActionCmd(action)
	}
	return m, nil
}

func (m *BrowserModel) View() string {
	m.layoutPanes()

	switch m.state {
	case browserNewDisk:
		if m.newDiskSelector != nil {
			return m.newDiskSelector.View()
		}
	case browserDiskAction:
		if m.diskSelector != nil {
			return m.diskSelector.View()
		}
	}

	leftView := m.local.View()
	rightView := m.remote.View()

	var cols string
	if m.showPreview {
		cols = joinColumns(leftView, rightView, m.previewView())
	} else {
		cols = joinColumns(leftView, rightView)
	}

	footer := m.message
	if footer == "" {
		if m.state == browserRenaming || m.state == browserMkdir || m.state == browserNewDiskNaming {
			footer = m.ti.View()
		} else {
			n := len(m.activeP.selected())
			footer = fmt.Sprintf("%d sel  Tab/h/l: pane  F5: copy  d/r/m/n  p: preview  ?: help", n)
		}
	} else if m.state == browserRenaming || m.state == browserMkdir || m.state == browserNewDiskNaming {
		footer = m.message + " " + m.ti.View()
	}

	return cols + "\n" + StatusBarStyle.Width(m.width).Render(footer)
}

func (m *BrowserModel) previewView() string {
	cur, ok := m.activeP.currentItem()
	if !ok {
		return ""
	}
	colW := (m.width - 2) / 3
	return renderPreview(cur, m.previewData, colW, m.height-1)
}
