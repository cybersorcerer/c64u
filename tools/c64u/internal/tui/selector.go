package tui

import (
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// SelectorItem represents a selectable item
type SelectorItem struct {
	Label       string
	Value       string
	Description string
}

// Selector is a simple list selector using Bubble Tea
type Selector struct {
	Title       string
	Items       []SelectorItem
	cursor      int
	selected    int
	confirmed   bool
	cancelled   bool
	width       int
	height      int
}

// NewSelector creates a new selector
func NewSelector(title string, items []SelectorItem) *Selector {
	return &Selector{
		Title:    title,
		Items:    items,
		cursor:   0,
		selected: -1,
		width:    80,
		height:   20,
	}
}

// Init initializes the selector
func (s *Selector) Init() tea.Cmd {
	return nil
}

// Update handles messages
func (s *Selector) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "ctrl+c", "q", "esc":
			s.cancelled = true
			return s, tea.Quit

		case "enter":
			if len(s.Items) > 0 {
				s.selected = s.cursor
				s.confirmed = true
			}
			return s, tea.Quit

		case "up", "k":
			if s.cursor > 0 {
				s.cursor--
			}

		case "down", "j":
			if s.cursor < len(s.Items)-1 {
				s.cursor++
			}
		}

	case tea.WindowSizeMsg:
		s.width = msg.Width
		s.height = msg.Height
	}

	return s, nil
}

// View renders the selector
func (s *Selector) View() string {
	if s.confirmed || s.cancelled {
		return ""
	}

	var b strings.Builder

	// Title - use adaptive colors
	titleStyle := lipgloss.NewStyle().
		Bold(true).
		Padding(0, 1)

	b.WriteString(titleStyle.Render(s.Title))
	b.WriteString("\n\n")

	// Items - use reverse video for selection (works with any terminal theme)
	selectedStyle := lipgloss.NewStyle().
		Reverse(true). // Reverse video works universally
		Bold(true)

	normalStyle := lipgloss.NewStyle()

	dimStyle := lipgloss.NewStyle().
		Faint(true) // Faint/dim works across themes

	// Calculate visible range
	maxVisible := s.height - 6
	if maxVisible < 5 {
		maxVisible = 5
	}

	start := 0
	if s.cursor >= maxVisible {
		start = s.cursor - maxVisible + 1
	}
	end := start + maxVisible
	if end > len(s.Items) {
		end = len(s.Items)
	}

	// Render items
	for i := start; i < end; i++ {
		item := s.Items[i]
		cursor := "  "
		if i == s.cursor {
			cursor = "▶ "
		}

		line := cursor + item.Label
		if item.Description != "" {
			line += dimStyle.Render(" - " + item.Description)
		}

		if i == s.cursor {
			b.WriteString(selectedStyle.Render(line))
		} else {
			b.WriteString(normalStyle.Render(line))
		}
		b.WriteString("\n")
	}

	// Help text
	b.WriteString("\n")
	helpStyle := dimStyle
	b.WriteString(helpStyle.Render("↑/k: up • ↓/j: down • enter: select • q/esc: cancel"))

	return b.String()
}

// Run runs the selector and returns the selected item
func (s *Selector) Run() (*SelectorItem, error) {
	p := tea.NewProgram(s)
	if _, err := p.Run(); err != nil {
		return nil, err
	}

	if s.cancelled {
		return nil, fmt.Errorf("cancelled")
	}

	if s.selected < 0 || s.selected >= len(s.Items) {
		return nil, fmt.Errorf("no selection")
	}

	return &s.Items[s.selected], nil
}
