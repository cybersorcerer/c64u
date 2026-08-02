package tui

import (
	"fmt"
	"strings"
	"unicode/utf8"

	"github.com/cybersorcerer/c64.nvim/tools/c64u/internal/diskimage"
)

// renderPreview produces a preview string for the given item. data is the file
// content (nil for directories or when not loaded). width/height bound the output.
func renderPreview(it fileItem, data []byte, width, height int) string {
	if it.IsDir {
		return ItemStyle.MaxWidth(width).Render(fmt.Sprintf("Directory: %s", it.Name))
	}

	ext := strings.ToLower(extOf(it.Name))
	switch ext {
	case ".d64", ".d71", ".d81":
		return previewD64(data, width, height)
	default:
		if looksLikeText(data) {
			return previewText(data, width, height)
		}
		return ItemStyle.MaxWidth(width).Render(fmt.Sprintf("%s\n%d bytes", it.Name, it.Size))
	}
}

func extOf(name string) string {
	idx := strings.LastIndex(name, ".")
	if idx < 0 {
		return ""
	}
	return name[idx:]
}

func previewD64(data []byte, width, height int) string {
	if len(data) == 0 {
		return ItemStyle.MaxWidth(width).Render("(disk not loaded)")
	}
	entries, name, blocksUsed, err := diskimage.ReadD64Directory(data)
	if err != nil {
		return ItemStyle.MaxWidth(width).Render("cannot read disk: " + err.Error())
	}
	var b strings.Builder
	b.WriteString(ItemStyle.Render(fmt.Sprintf("%q", strings.ToUpper(name))))
	b.WriteString("\n")
	max := height - 4
	if max < 1 {
		max = 1
	}
	for i, e := range entries {
		if i >= max {
			b.WriteString(ItemStyle.Render("..."))
			b.WriteString("\n")
			break
		}
		b.WriteString(ItemStyle.Render(fmt.Sprintf("%-16s %s", e.Name, e.FileType)))
		b.WriteString("\n")
	}
	b.WriteString(ItemStyle.Render(fmt.Sprintf("%d BLOCKS USED.", blocksUsed)))
	return ItemStyle.MaxWidth(width).Render(b.String())
}

func previewText(data []byte, width, height int) string {
	lines := strings.Split(sanitizeText(string(data)), "\n")
	max := height - 1
	if max < 1 {
		max = 1
	}
	if len(lines) > max {
		lines = lines[:max]
	}
	return ItemStyle.MaxWidth(width).Render(strings.Join(lines, "\n"))
}

// sanitizeText makes arbitrary file content safe to draw next to other panes.
// A carriage return sends the cursor back to column one, so the padding that
// follows it wipes out whatever the panes to the left had drawn on that row —
// which is what a C64 text file, with its CR CR LF line endings, did to the
// file browser. Other control characters are equally unwelcome, and tabs are
// expanded so a column's width stays predictable.
//
// Bytes above 0x1F pass through untouched, so UTF-8 sequences survive.
func sanitizeText(s string) string {
	var b strings.Builder
	b.Grow(len(s))
	for i := 0; i < len(s); {
		switch c := s[i]; {
		case c == '\r':
			// One line break, however it is spelled: CR, CR LF or CR CR LF.
			for i < len(s) && s[i] == '\r' {
				i++
			}
			if i < len(s) && s[i] == '\n' {
				i++
			}
			b.WriteByte('\n')
		case c == '\n':
			b.WriteByte(c)
			i++
		case c == '\t':
			b.WriteString("    ")
			i++
		case c < 0x20 || c == 0x7F:
			b.WriteByte('.')
			i++
		default:
			b.WriteByte(c)
			i++
		}
	}
	return b.String()
}

// looksLikeText reports whether data is likely UTF-8 text.
func looksLikeText(data []byte) bool {
	if len(data) == 0 {
		return false
	}
	n := len(data)
	if n > 1024 {
		n = 1024
	}
	for _, b := range data[:n] {
		if b == 0 {
			return false
		}
	}
	return utf8.Valid(data[:n])
}
