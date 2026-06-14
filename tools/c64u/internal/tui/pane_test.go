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
	if len(p.items) != 3 {
		t.Fatalf("expected 3 items, got %d", len(p.items))
	}
	if p.cursor != 0 {
		t.Errorf("cursor should start at 0, got %d", p.cursor)
	}
	p.handleNav("j")
	if p.cursor != 1 {
		t.Errorf("after j cursor should be 1, got %d", p.cursor)
	}
	p.handleNav("G")
	if p.cursor != 2 {
		t.Errorf("after G cursor should be 2, got %d", p.cursor)
	}
}

func TestPaneModel_MarkToggle(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "a.txt"), []byte("x"), 0o644)

	p := newPaneModel(&localSource{}, "LOCAL", dir)
	p.reload()
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
	cur, ok := p.currentItem()
	if !ok || cur.Name != "only.txt" {
		t.Errorf("currentItem = %+v ok=%v", cur, ok)
	}
}
