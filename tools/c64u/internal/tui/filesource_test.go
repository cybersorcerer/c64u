package tui

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLocalSource_ListAndMkdirAndDelete(t *testing.T) {
	dir := t.TempDir()
	src := &localSource{}

	sub := filepath.Join(dir, "subdir")
	if err := src.Mkdir(sub); err != nil {
		t.Fatalf("Mkdir: %v", err)
	}

	f := filepath.Join(dir, "a.txt")
	if err := src.WriteFile(f, []byte("hello")); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	items, err := src.List(dir)
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(items) != 2 {
		t.Fatalf("expected 2 items, got %d", len(items))
	}

	data, err := src.ReadFile(f)
	if err != nil || string(data) != "hello" {
		t.Fatalf("ReadFile: %q %v", data, err)
	}

	f2 := filepath.Join(dir, "b.txt")
	if err := src.Rename(f, f2); err != nil {
		t.Fatalf("Rename: %v", err)
	}
	if _, err := os.Stat(f2); err != nil {
		t.Fatalf("renamed file missing: %v", err)
	}

	if err := src.Delete(f2, false); err != nil {
		t.Fatalf("Delete file: %v", err)
	}
	if err := src.Delete(sub, true); err != nil {
		t.Fatalf("Delete dir: %v", err)
	}
	if !src.IsLocal() {
		t.Error("localSource.IsLocal() should be true")
	}
}

func TestLocalSource_ListSortsDirsFirst(t *testing.T) {
	dir := t.TempDir()
	src := &localSource{}
	os.WriteFile(filepath.Join(dir, "zfile.txt"), []byte("x"), 0o644)
	os.Mkdir(filepath.Join(dir, "adir"), 0o755)

	items, err := src.List(dir)
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if !items[0].IsDir || items[0].Name != "adir" {
		t.Errorf("expected dir first, got %+v", items[0])
	}
}
