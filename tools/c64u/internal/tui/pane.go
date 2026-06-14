package tui

import (
	"fmt"
	"strings"
)

// PaneModel is one source-agnostic file list column.
type PaneModel struct {
	src    fileSource
	label  string
	curDir string
	items  []fileItem
	cursor int
	offset int
	marked map[string]bool
	width  int
	height int
	active bool
}

func newPaneModel(src fileSource, label, startDir string) *PaneModel {
	return &PaneModel{
		src:    src,
		label:  label,
		curDir: startDir,
		marked: map[string]bool{},
	}
}

// reload re-reads the current directory.
func (p *PaneModel) reload() error {
	items, err := p.src.List(p.curDir)
	if err != nil {
		return err
	}
	p.items = items
	if p.cursor >= len(p.items) {
		p.cursor = len(p.items) - 1
	}
	if p.cursor < 0 {
		p.cursor = 0
	}
	return nil
}

// join builds a child path using the source's path style.
func (p *PaneModel) join(name string) string {
	if p.src.IsLocal() {
		return localJoin(p.curDir, name)
	}
	return joinPath(p.curDir, name)
}

// parentDir returns the parent of curDir using forward-slash style.
func (p *PaneModel) parentDir() string {
	d := strings.TrimRight(p.curDir, "/")
	idx := strings.LastIndex(d, "/")
	if idx <= 0 {
		return "/"
	}
	return d[:idx]
}

// currentItem returns the entry under the cursor.
func (p *PaneModel) currentItem() (fileItem, bool) {
	if p.cursor < 0 || p.cursor >= len(p.items) {
		return fileItem{}, false
	}
	return p.items[p.cursor], true
}

// handleNav applies a vim navigation key and adjusts the scroll offset.
func (p *PaneModel) handleNav(key string) {
	page := p.height - 2
	if page < 1 {
		page = 10
	}
	p.cursor = applyListNav(key, p.cursor, len(p.items), page)
	p.adjustOffset()
}

func (p *PaneModel) adjustOffset() {
	visible := p.height - 2
	if visible < 1 {
		visible = 10
	}
	if p.cursor < p.offset {
		p.offset = p.cursor
	}
	if p.cursor >= p.offset+visible {
		p.offset = p.cursor - visible + 1
	}
	if p.offset < 0 {
		p.offset = 0
	}
}

// toggleMark toggles the mark on the current item.
func (p *PaneModel) toggleMark() {
	cur, ok := p.currentItem()
	if !ok {
		return
	}
	if p.marked[cur.Name] {
		delete(p.marked, cur.Name)
	} else {
		p.marked[cur.Name] = true
	}
}

// selected returns marked items (nil if none marked).
func (p *PaneModel) selected() []fileItem {
	if len(p.marked) == 0 {
		return nil
	}
	var out []fileItem
	for _, it := range p.items {
		if p.marked[it.Name] {
			out = append(out, it)
		}
	}
	return out
}

// clearMarks removes all marks.
func (p *PaneModel) clearMarks() {
	p.marked = map[string]bool{}
}

// enterDir navigates into a subdirectory and reloads.
func (p *PaneModel) enterDir(name string) error {
	p.curDir = p.join(name)
	p.cursor = 0
	p.offset = 0
	p.clearMarks()
	return p.reload()
}

// goParent navigates up one level and reloads.
func (p *PaneModel) goParent() error {
	p.curDir = p.parentDir()
	p.cursor = 0
	p.offset = 0
	p.clearMarks()
	return p.reload()
}

// View renders the pane.
func (p *PaneModel) View() string {
	var b strings.Builder

	header := fmt.Sprintf("%s %s", p.label, p.curDir)
	if p.active {
		b.WriteString(SelectedItemStyle.Width(p.width).Render(header))
	} else {
		b.WriteString(ItemStyle.Width(p.width).Render(header))
	}
	b.WriteString("\n")

	visible := p.height - 2
	if visible < 1 {
		visible = 10
	}
	end := p.offset + visible
	if end > len(p.items) {
		end = len(p.items)
	}

	for i := p.offset; i < end; i++ {
		it := p.items[i]
		name := it.Name
		if it.IsDir {
			name += "/"
		}
		mark := " "
		if p.marked[it.Name] {
			mark = "●"
		}
		line := fmt.Sprintf("%s %s", mark, name)
		if i == p.cursor && p.active {
			b.WriteString(SelectedItemStyle.Render("▶"+line) + "\n")
		} else if i == p.cursor {
			b.WriteString(ItemStyle.Render("▷"+line) + "\n")
		} else {
			b.WriteString(ItemStyle.Render(" "+line) + "\n")
		}
	}

	return b.String()
}
