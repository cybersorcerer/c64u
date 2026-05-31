package diskimage_test

import (
	"testing"

	"github.com/cybersorcerer/c64.nvim/tools/c64u/internal/diskimage"
)

func TestReadD64Directory_EmptyDisk(t *testing.T) {
	d, err := diskimage.NewD64("TESTDISK", "01", 35)
	if err != nil {
		t.Fatalf("NewD64: %v", err)
	}
	data, err := d.Bytes()
	if err != nil {
		t.Fatalf("Bytes: %v", err)
	}
	entries, name, blocksUsed, err := diskimage.ReadD64Directory(data)
	if err != nil {
		t.Fatalf("ReadD64Directory: %v", err)
	}
	if name != "TESTDISK" {
		t.Errorf("disk name: got %q, want %q", name, "TESTDISK")
	}
	if len(entries) != 0 {
		t.Errorf("entries: got %d, want 0", len(entries))
	}
	_ = blocksUsed
}

func TestReadD64Directory_InvalidSize(t *testing.T) {
	_, _, _, err := diskimage.ReadD64Directory([]byte{0x00, 0x01})
	if err == nil {
		t.Fatal("expected error for invalid data, got nil")
	}
}
