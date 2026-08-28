package crt

import (
	"bytes"
	"encoding/binary"
	"testing"
)

func TestEightKRejectsWrongSize(t *testing.T) {
	if _, err := EightK("x", make([]byte, 100)); err == nil {
		t.Error("expected an error for a short ROM")
	}
	if _, err := EightK("x", make([]byte, 0x2000)); err != nil {
		t.Errorf("unexpected error for an 8K ROM: %v", err)
	}
}

// The Ultimate rejects an image whose header does not match the VICE layout,
// so the byte offsets are pinned here.
func TestWriteToHeaderLayout(t *testing.T) {
	rom := make([]byte, 0x2000)
	rom[0] = 0x09
	rom[1] = 0x80

	cart, err := EightK("ULTIMATE WEDGE", rom)
	if err != nil {
		t.Fatal(err)
	}

	var buf bytes.Buffer
	if _, err := cart.WriteTo(&buf); err != nil {
		t.Fatal(err)
	}
	out := buf.Bytes()

	if want := 0x40 + 0x10 + 0x2000; len(out) != want {
		t.Fatalf("image is %d bytes, want %d", len(out), want)
	}
	if got := string(out[0:16]); got != "C64 CARTRIDGE   " {
		t.Errorf("signature = %q", got)
	}
	if got := binary.BigEndian.Uint32(out[0x10:]); got != 0x40 {
		t.Errorf("header length = %d, want 64", got)
	}
	if got := binary.BigEndian.Uint16(out[0x14:]); got != 0x0100 {
		t.Errorf("version = %04X, want 0100", got)
	}
	if got := binary.BigEndian.Uint16(out[0x16:]); got != HardwareNormal {
		t.Errorf("hardware type = %d, want 0", got)
	}
	if out[0x18] != 0 || out[0x19] != 1 {
		t.Errorf("EXROM/GAME = %d/%d, want 0/1", out[0x18], out[0x19])
	}
	if got := string(bytes.TrimRight(out[0x20:0x40], "\x00")); got != "ULTIMATE WEDGE" {
		t.Errorf("name = %q", got)
	}

	chip := out[0x40:]
	if got := string(chip[0:4]); got != "CHIP" {
		t.Errorf("chip signature = %q", got)
	}
	if got := binary.BigEndian.Uint32(chip[4:]); got != 0x10+0x2000 {
		t.Errorf("packet length = %d", got)
	}
	if got := binary.BigEndian.Uint16(chip[0x0C:]); got != 0x8000 {
		t.Errorf("load address = %04X, want 8000", got)
	}
	if got := binary.BigEndian.Uint16(chip[0x0E:]); got != 0x2000 {
		t.Errorf("image size = %04X, want 2000", got)
	}
	if chip[0x10] != 0x09 || chip[0x11] != 0x80 {
		t.Errorf("ROM data not copied verbatim")
	}
}

// A name longer than the 32 byte field must be truncated, not overflow it.
func TestWriteToTruncatesLongName(t *testing.T) {
	cart, err := EightK("0123456789012345678901234567890123456789", make([]byte, 0x2000))
	if err != nil {
		t.Fatal(err)
	}
	var buf bytes.Buffer
	if _, err := cart.WriteTo(&buf); err != nil {
		t.Fatal(err)
	}
	if got := buf.Len(); got != 0x40+0x10+0x2000 {
		t.Errorf("image is %d bytes, header overflowed", got)
	}
}
