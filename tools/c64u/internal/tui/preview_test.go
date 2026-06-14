package tui

import (
	"strings"
	"testing"

	"github.com/cybersorcerer/c64.nvim/tools/c64u/internal/diskimage"
)

func TestRenderPreview_D64(t *testing.T) {
	d, err := diskimage.NewD64("TESTDISK", "01", 35)
	if err != nil {
		t.Fatalf("NewD64: %v", err)
	}
	data, _ := d.Bytes()

	out := renderPreview(fileItem{Name: "test.d64", Size: int64(len(data))}, data, 40, 20)
	if !strings.Contains(out, "TESTDISK") {
		t.Errorf("preview should contain disk name, got:\n%s", out)
	}
	if !strings.Contains(out, "BLOCKS") {
		t.Errorf("preview should contain blocks line, got:\n%s", out)
	}
}

func TestRenderPreview_Text(t *testing.T) {
	out := renderPreview(fileItem{Name: "readme.txt", Size: 11}, []byte("hello world"), 40, 20)
	if !strings.Contains(out, "hello world") {
		t.Errorf("text preview should contain content, got:\n%s", out)
	}
}

func TestRenderPreview_Dir(t *testing.T) {
	out := renderPreview(fileItem{Name: "games", IsDir: true}, nil, 40, 20)
	if !strings.Contains(strings.ToLower(out), "director") {
		t.Errorf("dir preview should say directory, got:\n%s", out)
	}
}
