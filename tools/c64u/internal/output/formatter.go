package output

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/cybersorcerer/c64.nvim/tools/c64u/internal/api"
)

// OutputMode represents the output format mode
type OutputMode int

const (
	// ModeText outputs human-readable text
	ModeText OutputMode = iota
	// ModeJSON outputs JSON
	ModeJSON
)

// Color styles using lipgloss
var (
	// Success style - green with checkmark
	successStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("10")).
			Bold(true)

	// Error style - red with X
	errorStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("9")).
			Bold(true)

	// Warning style - yellow with warning sign
	warningStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("11")).
			Bold(true)

	// Info style - cyan
	infoStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("14"))

	// Label style - bright cyan, bold
	labelStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("14")).
			Bold(true)

	// Value style - white/default
	valueStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("15"))

	// Header style - blue gradient, bold
	headerStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("12")).
			Bold(true).
			Underline(true)

	// Dim style - for less important info
	dimStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("8"))

	// Highlight style - bright white, bold
	highlightStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("15")).
			Bold(true)
)

// Formatter handles output formatting
type Formatter struct {
	Mode     OutputMode
	NoColor  bool
}

// NewFormatter creates a new output formatter
func NewFormatter(jsonMode bool) *Formatter {
	mode := ModeText
	if jsonMode {
		mode = ModeJSON
	}
	return &Formatter{
		Mode:    mode,
		NoColor: false,
	}
}

// SetNoColor disables colored output
func (f *Formatter) SetNoColor(noColor bool) {
	f.NoColor = noColor
}

// Success prints a success message
func (f *Formatter) Success(message string, data map[string]interface{}) {
	if f.Mode == ModeJSON {
		output := map[string]interface{}{
			"success": true,
			"message": message,
		}
		if data != nil {
			output["data"] = data
		}
		f.printJSON(output)
	} else {
		if f.NoColor {
			fmt.Printf("✓ %s\n", message)
		} else {
			fmt.Printf("%s %s\n", successStyle.Render("✓"), message)
		}
		if data != nil && len(data) > 0 {
			for key, value := range data {
				if f.NoColor {
					fmt.Printf("  %s: %v\n", key, value)
				} else {
					fmt.Printf("  %s %s\n",
						labelStyle.Render(key+":"),
						valueStyle.Render(fmt.Sprintf("%v", value)))
				}
			}
		}
	}
}

// Error prints an error message and exits
func (f *Formatter) Error(message string, errors []string) {
	if f.Mode == ModeJSON {
		output := map[string]interface{}{
			"success": false,
			"message": message,
			"errors":  errors,
		}
		f.printJSON(output)
	} else {
		if f.NoColor {
			fmt.Fprintf(os.Stderr, "✗ Error: %s\n", message)
		} else {
			fmt.Fprintf(os.Stderr, "%s %s\n",
				errorStyle.Render("✗"),
				errorStyle.Render("Error: "+message))
		}
		if len(errors) > 0 {
			for _, err := range errors {
				if f.NoColor {
					fmt.Fprintf(os.Stderr, "  - %s\n", err)
				} else {
					fmt.Fprintf(os.Stderr, "  %s %s\n",
						dimStyle.Render("-"),
						err)
				}
			}
		}
	}
	os.Exit(1)
}

// PrintResponse formats and prints an API response
func (f *Formatter) PrintResponse(resp *api.Response, successMsg string) {
	if resp.HasErrors() {
		f.Error(successMsg+" failed", resp.Errors)
		return
	}

	f.Success(successMsg, resp.Data)
}

// PrintData prints arbitrary data
func (f *Formatter) PrintData(data interface{}) {
	if f.Mode == ModeJSON {
		f.printJSON(data)
	} else {
		// For text mode, format based on type
		switch v := data.(type) {
		case string:
			fmt.Println(v)
		case []string:
			for _, item := range v {
				fmt.Printf("  - %s\n", item)
			}
		case map[string]interface{}:
			for key, value := range v {
				fmt.Printf("  %s: %v\n", key, value)
			}
		default:
			fmt.Printf("%v\n", data)
		}
	}
}

// PrintTable prints data in a table format (text mode only)
func (f *Formatter) PrintTable(headers []string, rows [][]string) {
	if f.Mode == ModeJSON {
		// Convert table to JSON array of objects
		var jsonRows []map[string]string
		for _, row := range rows {
			jsonRow := make(map[string]string)
			for i, header := range headers {
				if i < len(row) {
					jsonRow[header] = row[i]
				}
			}
			jsonRows = append(jsonRows, jsonRow)
		}
		f.printJSON(jsonRows)
		return
	}

	// Calculate column widths
	widths := make([]int, len(headers))
	for i, h := range headers {
		widths[i] = len(h)
	}
	for _, row := range rows {
		for i, cell := range row {
			if i < len(widths) && len(cell) > widths[i] {
				widths[i] = len(cell)
			}
		}
	}

	// Print header
	for i, h := range headers {
		fmt.Printf("%-*s  ", widths[i], h)
	}
	fmt.Println()

	// Print separator
	for _, w := range widths {
		fmt.Print(strings.Repeat("-", w) + "  ")
	}
	fmt.Println()

	// Print rows
	for _, row := range rows {
		for i, cell := range row {
			if i < len(widths) {
				fmt.Printf("%-*s  ", widths[i], cell)
			}
		}
		fmt.Println()
	}
}

// printJSON marshals and prints JSON
func (f *Formatter) printJSON(data interface{}) {
	jsonData, err := json.MarshalIndent(data, "", "  ")
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error marshaling JSON: %v\n", err)
		os.Exit(1)
	}
	fmt.Println(string(jsonData))
}

// Info prints an informational message (text mode only, silent in JSON mode)
func (f *Formatter) Info(message string) {
	if f.Mode == ModeText {
		if f.NoColor {
			fmt.Printf("ℹ %s\n", message)
		} else {
			fmt.Printf("%s %s\n", infoStyle.Render("ℹ"), message)
		}
	}
}

// Warning prints a warning message
func (f *Formatter) Warning(message string) {
	if f.Mode == ModeJSON {
		output := map[string]interface{}{
			"warning": message,
		}
		f.printJSON(output)
	} else {
		if f.NoColor {
			fmt.Fprintf(os.Stderr, "⚠ Warning: %s\n", message)
		} else {
			fmt.Fprintf(os.Stderr, "%s %s\n",
				warningStyle.Render("⚠"),
				warningStyle.Render("Warning: "+message))
		}
	}
}

// PrintKeyValue prints a styled key-value pair
func (f *Formatter) PrintKeyValue(key, value string) {
	if f.Mode == ModeJSON {
		// In JSON mode, this is handled by PrintData
		return
	}

	if f.NoColor {
		fmt.Printf("  %-18s %s\n", key+":", value)
	} else {
		fmt.Printf("  %s %s\n",
			labelStyle.Render(fmt.Sprintf("%-18s", key+":")),
			valueStyle.Render(value))
	}
}

// PrintHeader prints a styled header
func (f *Formatter) PrintHeader(text string) {
	if f.Mode == ModeJSON {
		return
	}

	if f.NoColor {
		fmt.Println(text)
	} else {
		fmt.Println(headerStyle.Render(text))
	}
}

// GetTitleStyle returns a style for help titles
func (f *Formatter) GetTitleStyle() lipgloss.Style {
	if f.NoColor {
		return lipgloss.NewStyle()
	}
	return lipgloss.NewStyle().
		Foreground(lipgloss.Color("12")).
		Bold(true)
}

// GetSectionStyle returns a style for help section headers
func (f *Formatter) GetSectionStyle() lipgloss.Style {
	if f.NoColor {
		return lipgloss.NewStyle()
	}
	return lipgloss.NewStyle().
		Foreground(lipgloss.Color("14")).
		Bold(true)
}

// GetCommandStyle returns a style for command names in help
func (f *Formatter) GetCommandStyle() lipgloss.Style {
	if f.NoColor {
		return lipgloss.NewStyle()
	}
	return lipgloss.NewStyle().
		Foreground(lipgloss.Color("10")).
		Bold(true)
}

// GetFlagStyle returns a style for flag names in help
func (f *Formatter) GetFlagStyle() lipgloss.Style {
	if f.NoColor {
		return lipgloss.NewStyle()
	}
	return lipgloss.NewStyle().
		Foreground(lipgloss.Color("11"))
}
