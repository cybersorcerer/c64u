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
		return ItemStyle.Render(fmt.Sprintf("Directory: %s", it.Name))
	}

	ext := strings.ToLower(extOf(it.Name))
	switch ext {
	case ".d64", ".d71", ".d81":
		return previewD64(data, height)
	default:
		if looksLikeText(data) {
			return previewText(data, height)
		}
		return ItemStyle.Render(fmt.Sprintf("%s\n%d bytes", it.Name, it.Size))
	}
}

func extOf(name string) string {
	idx := strings.LastIndex(name, ".")
	if idx < 0 {
		return ""
	}
	return name[idx:]
}

func previewD64(data []byte, height int) string {
	if len(data) == 0 {
		return ItemStyle.Render("(disk not loaded)")
	}
	entries, name, blocksUsed, err := diskimage.ReadD64Directory(data)
	if err != nil {
		return ItemStyle.Render("cannot read disk: " + err.Error())
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
	return b.String()
}

func previewText(data []byte, height int) string {
	lines := strings.SplitN(string(data), "\n", height)
	max := height - 1
	if max < 1 {
		max = 1
	}
	if len(lines) > max {
		lines = lines[:max]
	}
	return ItemStyle.Render(strings.Join(lines, "\n"))
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
