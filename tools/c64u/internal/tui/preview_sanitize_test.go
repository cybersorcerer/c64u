package tui

import (
	"strings"
	"testing"

	"github.com/charmbracelet/lipgloss"
)

// Line endings as they actually occur in C64 text files on the device: the
// Turbo Macro Pro documentation uses CR CR LF throughout.
func TestSanitizeTextLineEndings(t *testing.T) {
	cases := map[string]string{
		"a\r\r\nb":       "a\nb",   // CR CR LF, one break
		"a\r\nb":         "a\nb",   // CR LF
		"a\rb":           "a\nb",   // lone CR, classic Mac and PETSCII
		"a\nb":           "a\nb",   // plain LF, unchanged
		"a\r\r\n\r\r\nb": "a\n\nb", // blank line survives as a blank line
	}
	for in, want := range cases {
		if got := sanitizeText(in); got != want {
			t.Errorf("sanitizeText(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestSanitizeTextRemovesControlCharacters(t *testing.T) {
	got := sanitizeText("a\x00b\x1bc\x7fd")
	if strings.ContainsAny(got, "\x00\x1b\x7f") {
		t.Errorf("control characters survived: %q", got)
	}
	if got != "a.b.c.d" {
		t.Errorf("got %q, want %q", got, "a.b.c.d")
	}
	if got := sanitizeText("a\tb"); got != "a    b" {
		t.Errorf("tab not expanded: %q", got)
	}
	// UTF-8 must survive untouched.
	if got := sanitizeText("Mäkelä ×"); got != "Mäkelä ×" {
		t.Errorf("UTF-8 mangled: %q", got)
	}
}

// The preview sits in a column next to the two file panes. A line wider than
// the column, or a stray carriage return, paints over them.
func TestPreviewTextStaysInsideItsColumn(t *testing.T) {
	const width = 20
	content := []byte(
		"short\r\r\n" +
			strings.Repeat("x", 200) + "\r\r\n" +
			"tail\r\r\n")

	out := renderPreview(fileItem{Name: "notes.txt", Size: int64(len(content))}, content, width, 10)

	if strings.ContainsRune(out, '\r') {
		t.Error("preview still contains a carriage return")
	}
	for i, line := range strings.Split(out, "\n") {
		if w := lipgloss.Width(line); w > width {
			t.Errorf("line %d is %d cells wide, column is %d: %q", i, w, width, line)
		}
	}
}

func TestPreviewHonoursHeight(t *testing.T) {
	content := []byte(strings.Repeat("line\n", 100))
	out := renderPreview(fileItem{Name: "notes.txt"}, content, 40, 8)
	if got := strings.Count(out, "\n") + 1; got > 8 {
		t.Errorf("preview is %d lines, height is 8", got)
	}
}

func TestPreviewDirectoryAndBinaryAlsoBounded(t *testing.T) {
	const width = 12
	long := strings.Repeat("d", 100)

	for _, out := range []string{
		renderPreview(fileItem{Name: long, IsDir: true}, nil, width, 5),
		renderPreview(fileItem{Name: long + ".bin", Size: 4711}, []byte{0x00, 0x01}, width, 5),
	} {
		for _, line := range strings.Split(out, "\n") {
			if w := lipgloss.Width(line); w > width {
				t.Errorf("line is %d cells wide, column is %d: %q", w, width, line)
			}
		}
	}
}
