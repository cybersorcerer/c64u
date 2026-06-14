package tui

import (
	"os"
	"path/filepath"
	"testing"
)

func TestPaneModel_LoadAndNavigate(t *testing.T) {
	dir := t.TempDir()
	os.Mkdir(filepath.Join(dir, "sub"), 0o755)
	os.WriteFile(filepath.Join(dir, "a.txt"), []byte("x"), 0o644)
	os.WriteFile(filepath.Join(dir, "b.txt"), []byte("y"), 0o644)

	p := newPaneModel(&localSource{}, "LOCAL", dir)
	if err := p.reload(); err != nil {
		t.Fatalf("reload: %v", err)
	}
	// 3 real entries (sub/, a.txt, b.txt) plus a ".." entry (dir is not root).
	if len(p.items) != 4 {
		t.Fatalf("expected 4 items (incl. ..), got %d", len(p.items))
	}
	if p.items[0].Name != ".." {
		t.Errorf("first item should be .., got %q", p.items[0].Name)
	}
	if p.cursor != 0 {
		t.Errorf("cursor should start at 0, got %d", p.cursor)
	}
	p.handleNav("j")
	if p.cursor != 1 {
		t.Errorf("after j cursor should be 1, got %d", p.cursor)
	}
	p.handleNav("G")
	if p.cursor != 3 {
		t.Errorf("after G cursor should be 3, got %d", p.cursor)
	}
}

func TestPaneModel_MarkToggle(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "a.txt"), []byte("x"), 0o644)

	p := newPaneModel(&localSource{}, "LOCAL", dir)
	p.reload()
	p.handleNav("j") // skip ".." entry, land on a.txt
	p.toggleMark()
	sel := p.selected()
	if len(sel) != 1 || sel[0].Name != "a.txt" {
		t.Errorf("expected a.txt marked, got %+v", sel)
	}
	p.toggleMark()
	if len(p.selected()) != 0 {
		t.Error("expected no marks after second toggle")
	}
}

func TestPaneModel_CurrentItem(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "only.txt"), []byte("x"), 0o644)
	p := newPaneModel(&localSource{}, "LOCAL", dir)
	p.reload()
	p.handleNav("j") // skip ".." entry
	cur, ok := p.currentItem()
	if !ok || cur.Name != "only.txt" {
		t.Errorf("currentItem = %+v ok=%v", cur, ok)
	}
}
