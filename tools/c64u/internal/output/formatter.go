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

	// Calculate column widths (accounting for actual cell content, not styled)
	// For styled cells, we need to strip ANSI codes to get real width
	widths := make([]int, len(headers))
	for i, h := range headers {
		// Use runeWidth for headers too, in case they contain emojis
		widths[i] = runeWidth(h)
	}
	for _, row := range rows {
		for i, cell := range row {
			if i < len(widths) {
				// Strip ANSI escape codes for width calculation
				cleanCell := stripAnsi(cell)
				// Account for emoji width (they take 2 terminal cells)
				displayWidth := runeWidth(cleanCell)
				if displayWidth > widths[i] {
					widths[i] = displayWidth
				}
			}
		}
	}

	// Table styles
	headerStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("14")).
		Bold(true)

	separatorStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("8"))

	// Print header
	if f.NoColor {
		for i, h := range headers {
			// Calculate padding needed
			headerWidth := runeWidth(h)
			padding := widths[i] - headerWidth
			if padding < 0 {
				padding = 0
			}
			fmt.Print(h)
			fmt.Print(strings.Repeat(" ", padding))
			fmt.Print("  ")
		}
		fmt.Println()
	} else {
		for i, h := range headers {
			// Calculate padding needed
			headerWidth := runeWidth(h)
			padding := widths[i] - headerWidth
			if padding < 0 {
				padding = 0
			}
			fmt.Print(headerStyle.Render(h))
			fmt.Print(strings.Repeat(" ", padding))
			fmt.Print("  ")
		}
		fmt.Println()
	}

	// Print separator
	separator := ""
	for i, w := range widths {
		if i > 0 {
			separator += "  "
		}
		separator += strings.Repeat("─", w)
	}

	if f.NoColor {
		fmt.Println(separator)
	} else {
		fmt.Println(separatorStyle.Render(separator))
	}

	// Print rows
	for _, row := range rows {
		for i, cell := range row {
			if i < len(widths) {
				// Calculate actual display width of cell content
				cleanCell := stripAnsi(cell)
				cellWidth := runeWidth(cleanCell)
				padding := widths[i] - cellWidth
				if padding < 0 {
					padding = 0
				}
				fmt.Print(cell)
				fmt.Print(strings.Repeat(" ", padding))
				fmt.Print("  ")
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

// FormatFileSize formats bytes in human-readable format
func FormatFileSize(bytes uint64) string {
	if bytes == 0 {
		return "0 B"
	}

	const unit = 1024
	if bytes < unit {
		return fmt.Sprintf("%d B", bytes)
	}

	div, exp := uint64(unit), 0
	for n := bytes / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}

	return fmt.Sprintf("%.1f %cB", float64(bytes)/float64(div), "KMGTPE"[exp])
}

// GetFileIcon returns a minimal symbol for file type
func GetFileIcon(filename string, isDir bool) string {
	if isDir {
		return "▸"
	}

	ext := strings.ToLower(filename[strings.LastIndex(filename, "."):])

	// Disk images
	if strings.HasSuffix(ext, ".d64") || strings.HasSuffix(ext, ".d71") ||
	   strings.HasSuffix(ext, ".d81") || strings.HasSuffix(ext, ".g64") ||
	   strings.HasSuffix(ext, ".g71") || strings.HasSuffix(ext, ".dnp") {
		return "●"
	}

	// Programs
	if strings.HasSuffix(ext, ".prg") || strings.HasSuffix(ext, ".p00") {
		return "▶"
	}

	// Music
	if strings.HasSuffix(ext, ".sid") {
		return "♪"
	}

	// Cartridges
	if strings.HasSuffix(ext, ".crt") {
		return "■"
	}

	// Text files
	if strings.HasSuffix(ext, ".seq") || strings.HasSuffix(ext, ".s00") ||
	   strings.HasSuffix(ext, ".txt") || strings.HasSuffix(ext, ".asm") ||
	   strings.HasSuffix(ext, ".sym") || strings.HasSuffix(ext, ".bas") ||
	   strings.HasSuffix(ext, ".md") || strings.HasSuffix(ext, ".log") {
		return "―"
	}

	// User files
	if strings.HasSuffix(ext, ".usr") || strings.HasSuffix(ext, ".u00") {
		return "◆"
	}

	// Tape archives
	if strings.HasSuffix(ext, ".t64") {
		return "□"
	}

	// Default
	return "·"
}

// GetFileTypeStyle returns lipgloss style for file type
func GetFileTypeStyle(filename string, isDir bool) lipgloss.Style {
	if isDir {
		// Directories: Blue, bold
		return lipgloss.NewStyle().
			Foreground(lipgloss.Color("12")).
			Bold(true)
	}

	ext := strings.ToLower(filename[strings.LastIndex(filename, "."):])

	// Disk images: Magenta
	if strings.HasSuffix(ext, ".d64") || strings.HasSuffix(ext, ".d71") ||
	   strings.HasSuffix(ext, ".d81") || strings.HasSuffix(ext, ".g64") ||
	   strings.HasSuffix(ext, ".g71") || strings.HasSuffix(ext, ".dnp") {
		return lipgloss.NewStyle().Foreground(lipgloss.Color("13"))
	}

	// Programs: Green
	if strings.HasSuffix(ext, ".prg") || strings.HasSuffix(ext, ".p00") {
		return lipgloss.NewStyle().Foreground(lipgloss.Color("10"))
	}

	// Music: Cyan
	if strings.HasSuffix(ext, ".sid") {
		return lipgloss.NewStyle().Foreground(lipgloss.Color("14"))
	}

	// Cartridges: Yellow
	if strings.HasSuffix(ext, ".crt") {
		return lipgloss.NewStyle().Foreground(lipgloss.Color("11"))
	}

	// Text files: White
	if strings.HasSuffix(ext, ".seq") || strings.HasSuffix(ext, ".s00") ||
	   strings.HasSuffix(ext, ".txt") || strings.HasSuffix(ext, ".asm") ||
	   strings.HasSuffix(ext, ".sym") || strings.HasSuffix(ext, ".bas") {
		return lipgloss.NewStyle().Foreground(lipgloss.Color("15"))
	}

	// Default: Dim gray
	return lipgloss.NewStyle().Foreground(lipgloss.Color("8"))
}

// GetFileTypeLabel returns human-readable type label
func GetFileTypeLabel(filename string, isDir bool) string {
	if isDir {
		return "DIR"
	}

	ext := strings.ToLower(filename[strings.LastIndex(filename, "."):])

	switch {
	case strings.HasSuffix(ext, ".d64"):
		return "D64"
	case strings.HasSuffix(ext, ".d71"):
		return "D71"
	case strings.HasSuffix(ext, ".d81"):
		return "D81"
	case strings.HasSuffix(ext, ".g64"):
		return "G64"
	case strings.HasSuffix(ext, ".g71"):
		return "G71"
	case strings.HasSuffix(ext, ".dnp"):
		return "DNP"
	case strings.HasSuffix(ext, ".prg"), strings.HasSuffix(ext, ".p00"):
		return "PRG"
	case strings.HasSuffix(ext, ".seq"), strings.HasSuffix(ext, ".s00"):
		return "SEQ"
	case strings.HasSuffix(ext, ".usr"), strings.HasSuffix(ext, ".u00"):
		return "USR"
	case strings.HasSuffix(ext, ".rel"), strings.HasSuffix(ext, ".r00"):
		return "REL"
	case strings.HasSuffix(ext, ".sid"):
		return "SID"
	case strings.HasSuffix(ext, ".crt"):
		return "CRT"
	case strings.HasSuffix(ext, ".t64"):
		return "T64"
	case strings.HasSuffix(ext, ".txt"):
		return "TXT"
	case strings.HasSuffix(ext, ".asm"):
		return "ASM"
	case strings.HasSuffix(ext, ".sym"):
		return "SYM"
	case strings.HasSuffix(ext, ".bas"):
		return "BAS"
	default:
		// Return uppercase extension without dot
		if len(ext) > 1 {
			return strings.ToUpper(ext[1:])
		}
		return "FILE"
	}
}

// stripAnsi removes ANSI escape codes from a string
func stripAnsi(str string) string {
	// Simple regex-free approach: remove ESC sequences
	var result strings.Builder
	i := 0
	for i < len(str) {
		if str[i] == '\x1b' && i+1 < len(str) && str[i+1] == '[' {
			// Found ANSI escape sequence, skip until 'm'
			i += 2
			for i < len(str) && str[i] != 'm' {
				i++
			}
			i++ // skip the 'm'
		} else {
			result.WriteByte(str[i])
			i++
		}
	}
	return result.String()
}

// runeWidth calculates the display width of a string accounting for emoji width
func runeWidth(str string) int {
	width := 0
	for _, r := range str {
		if r >= 0x1F300 && r <= 0x1F9FF { // Emoji ranges
			width += 2
		} else if r >= 0x2600 && r <= 0x26FF { // Miscellaneous Symbols
			width += 2
		} else if r >= 0x2700 && r <= 0x27BF { // Dingbats
			width += 2
		} else if r >= 0xFE00 && r <= 0xFE0F { // Variation Selectors
			// Don't count variation selectors
		} else if r >= 0x1F000 && r <= 0x1FAFF { // Various emoji blocks
			width += 2
		} else {
			width += 1
		}
	}
	return width
}
