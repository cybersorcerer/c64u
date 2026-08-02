package tui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
)

// ViewMode controls how file content is displayed
type ViewMode int

const (
	ViewModeText ViewMode = iota
	ViewModeHex
)

// ViewerModel displays file content in a scrollable viewport
type ViewerModel struct {
	filename string
	content  []byte
	mode     ViewMode
	viewport viewport.Model
	width    int
	height   int
	ready    bool
}

// NewViewerModel creates a new file viewer
func NewViewerModel() *ViewerModel {
	return &ViewerModel{
		mode: ViewModeText,
	}
}

// SetContent loads file content into the viewer
func (m *ViewerModel) SetContent(filename string, content []byte, width, height int) {
	m.filename = filename
	m.content = content
	m.width = width
	m.height = height

	// Create viewport sized for content area (viewer own header + footer = 2 lines)
	vpHeight := height - 2
	if vpHeight < 1 {
		vpHeight = 1
	}
	if vpHeight < 5 {
		vpHeight = 5
	}
	vpWidth := width
	if vpWidth < 20 {
		vpWidth = 80
	}

	m.viewport = viewport.New(vpWidth, vpHeight)
	m.viewport.SetContent(m.renderContent())
	m.ready = true
}

func (m *ViewerModel) Init() tea.Cmd {
	return nil
}

func (m *ViewerModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "tab":
			if m.mode == ViewModeText {
				m.mode = ViewModeHex
			} else {
				m.mode = ViewModeText
			}
			m.viewport.SetContent(m.renderContent())
			m.viewport.GotoTop()
			return m, nil
		}
	}

	var cmd tea.Cmd
	m.viewport, cmd = m.viewport.Update(msg)
	return m, cmd
}

func (m *ViewerModel) View() string {
	if !m.ready {
		return "Loading..."
	}

	var b strings.Builder

	// Header with filename and mode
	modeStr := "TEXT"
	if m.mode == ViewModeHex {
		modeStr = "HEX"
	}
	header := fmt.Sprintf("%s [%s] (%d bytes)", m.filename, modeStr, len(m.content))
	b.WriteString(HeaderStyle.Render(header))
	b.WriteString("\n")

	// Viewport
	b.WriteString(m.viewport.View())
	b.WriteString("\n")

	// Footer
	scrollPct := fmt.Sprintf("%3.f%%", m.viewport.ScrollPercent()*100)
	footer := fmt.Sprintf("Tab: toggle view • ↑/↓/PgUp/PgDn: scroll • %s • Esc: back", scrollPct)
	b.WriteString(StatusBarStyle.Width(m.width).Render(footer))

	return b.String()
}

// renderContent generates the display string based on current mode
func (m *ViewerModel) renderContent() string {
	if m.mode == ViewModeHex {
		return m.renderHex()
	}
	return m.renderText()
}

// renderText displays content as text with non-printable chars replaced
func (m *ViewerModel) renderText() string {
	var b strings.Builder
	for _, ch := range m.content {
		if ch == '\n' || ch == '\r' || ch == '\t' {
			b.WriteByte(ch)
		} else if ch < 32 || ch == 127 {
			b.WriteByte('.')
		} else {
			b.WriteByte(ch)
		}
	}
	return b.String()
}

// renderHex displays content as a classic hex dump
func (m *ViewerModel) renderHex() string {
	var b strings.Builder
	bytesPerLine := 16

	for offset := 0; offset < len(m.content); offset += bytesPerLine {
		// Offset
		b.WriteString(fmt.Sprintf("%08X  ", offset))

		// Hex bytes
		end := offset + bytesPerLine
		if end > len(m.content) {
			end = len(m.content)
		}

		for i := offset; i < offset+bytesPerLine; i++ {
			if i < end {
				b.WriteString(fmt.Sprintf("%02X ", m.content[i]))
			} else {
				b.WriteString("   ")
			}
			if i == offset+7 {
				b.WriteByte(' ')
			}
		}

		b.WriteString(" |")

		// ASCII
		for i := offset; i < end; i++ {
			ch := m.content[i]
			if ch >= 32 && ch < 127 {
				b.WriteByte(ch)
			} else {
				b.WriteByte('.')
			}
		}

		b.WriteString("|\n")
	}

	return b.String()
}
