package tui

import "github.com/charmbracelet/lipgloss"

// Styles for the TUI
var (
	// Colors - Use standard ANSI colors to respect terminal theme
	// 4 = Blue, 12 = Bright Blue. These map to the user's terminal theme.
	themePrimary   = lipgloss.Color("4")  // Blue
	themeSecondary = lipgloss.Color("12") // Bright Blue

	// Text Styles
	TitleStyle = lipgloss.NewStyle().
			Foreground(themeSecondary).
			Background(themePrimary).
			Padding(0, 1).
			Bold(true)

	// Consistent Header Style
	HeaderStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("0")). // Black text
			Background(themeSecondary).      // Bright Blue background
			Padding(0, 1).
			Bold(true)

	StatusBarStyle = lipgloss.NewStyle().
			Reverse(true) // Use reverse video to adapt to any theme

	// List Styles
	ItemStyle = lipgloss.NewStyle().
			PaddingLeft(2).
			Foreground(lipgloss.Color("7")) // Standard white/gray

	// Unified Selection Style (Reverse Video)
	SelectedItemStyle = lipgloss.NewStyle().
				PaddingLeft(1).
				Bold(true).
				Reverse(true).
				Foreground(themeSecondary). // Color gets inverted
				SetString("▶ ")

	// Box Styles
	BoxStyle = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(themePrimary).
			Padding(0, 1)

	// Status Line Styles (inside box, colored background, full width)
	StatusSuccessStyle = lipgloss.NewStyle().
				Background(lipgloss.Color("#005f00")). // Dark green background
				Foreground(lipgloss.Color("#ffffff")). // Pure white text
				Bold(true)

	StatusErrorStyle = lipgloss.NewStyle().
				Background(lipgloss.Color("#870000")). // Dark red background
				Foreground(lipgloss.Color("#ffffff")). // Pure white text
				Bold(true)

	// Help Overlay Styles
	HelpKeyStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(themeSecondary)

	HelpDescStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("7"))
)
