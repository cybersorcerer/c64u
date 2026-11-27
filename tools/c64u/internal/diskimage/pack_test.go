package diskimage_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/cybersorcerer/c64.nvim/tools/c64u/internal/diskimage"
)

// The file type already says PRG or SEQ, so an extension that only names that
// type does not belong in the CBM name: LOAD"HELLO",8 has to find hello.prg.
// Other extensions say something the type does not, and stay.
func TestPackDirectoryDropsTypeExtensions(t *testing.T) {
	src := t.TempDir()
	files := map[string][]byte{
		"hello.prg":                 {0x01, 0x08, 0x00, 0x00},
		"notes.SEQ":                 []byte("text"),
		"data.bin":                  {0x00, 0x10, 0xff},
		"averyveryverylongname.prg": {0x01, 0x08},
	}
	for name, data := range files {
		if err := os.WriteFile(filepath.Join(src, name), data, 0o644); err != nil {
			t.Fatal(err)
		}
	}
	out := filepath.Join(t.TempDir(), "out.d64")
	if err := diskimage.PackDirectory(src, out, nil); err != nil {
		t.Fatalf("PackDirectory: %v", err)
	}
	data, err := os.ReadFile(out)
	if err != nil {
		t.Fatal(err)
	}
	entries, _, _, err := diskimage.ReadD64Directory(data)
	if err != nil {
		t.Fatalf("ReadD64Directory: %v", err)
	}
	got := map[string]string{}
	for _, e := range entries {
		got[e.Name] = e.FileType
	}
	want := map[string]string{
		"HELLO":            "PRG",
		"NOTES":            "SEQ",
		"DATA.BIN":         "PRG",
		"AVERYVERYVERYLON": "PRG",
	}
	for name, typ := range want {
		if got[name] != typ {
			t.Errorf("%s: type %q, want %q (directory %v)", name, got[name], typ, got)
		}
	}
}
