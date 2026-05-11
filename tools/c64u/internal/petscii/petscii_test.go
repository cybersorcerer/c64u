package petscii

import (
	"bytes"
	"testing"
)

func TestEncodeSimpleString(t *testing.T) {
	got, err := Encode("HELLO")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	want := []byte{0x48, 0x45, 0x4C, 0x4C, 0x4F}
	if !bytes.Equal(got, want) {
		t.Errorf("got %X, want %X", got, want)
	}
}

func TestEncodeLowercase(t *testing.T) {
	got, err := Encode("hello")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	want := []byte{0x48, 0x45, 0x4C, 0x4C, 0x4F}
	if !bytes.Equal(got, want) {
		t.Errorf("got %X, want %X", got, want)
	}
}

func TestEncodeDigits(t *testing.T) {
	got, err := Encode("123")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	want := []byte{0x31, 0x32, 0x33}
	if !bytes.Equal(got, want) {
		t.Errorf("got %X, want %X", got, want)
	}
}

func TestEncodeEscapeReturn(t *testing.T) {
	got, err := Encode(`\n`)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(got) != 1 || got[0] != 0x0D {
		t.Errorf("got %X, want [0D]", got)
	}
}

func TestEncodeEscapeF1(t *testing.T) {
	got, err := Encode(`\f1`)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(got) != 1 || got[0] != 0x85 {
		t.Errorf("got %X, want [85]", got)
	}
}

func TestEncodeEscapeF8(t *testing.T) {
	got, err := Encode(`\f8`)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(got) != 1 || got[0] != 0x8C {
		t.Errorf("got %X, want [8C]", got)
	}
}

func TestEncodeEscapeCLR(t *testing.T) {
	got, err := Encode(`\clr`)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(got) != 1 || got[0] != 0x93 {
		t.Errorf("got %X, want [93]", got)
	}
}

func TestEncodeEscapeDEL(t *testing.T) {
	got, err := Encode(`\del`)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(got) != 1 || got[0] != 0x14 {
		t.Errorf("got %X, want [14]", got)
	}
}

func TestEncodeEscapeSTOP(t *testing.T) {
	got, err := Encode(`\stop`)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(got) != 1 || got[0] != 0x03 {
		t.Errorf("got %X, want [03]", got)
	}
}

func TestEncodeEscapeHOME(t *testing.T) {
	got, err := Encode(`\home`)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(got) != 1 || got[0] != 0x13 {
		t.Errorf("got %X, want [13]", got)
	}
}

func TestEncodeMixed(t *testing.T) {
	got, err := Encode(`LOAD"*",8,1\n`)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// L=4C O=4F A=41 D=44 "=22 *=2A "=22 ,=2C 8=38 ,=2C 1=31 Return=0D
	want := []byte{0x4C, 0x4F, 0x41, 0x44, 0x22, 0x2A, 0x22, 0x2C, 0x38, 0x2C, 0x31, 0x0D}
	if !bytes.Equal(got, want) {
		t.Errorf("got %X, want %X", got, want)
	}
}

func TestEncodeUnknownEscape(t *testing.T) {
	_, err := Encode(`\xyz`)
	if err == nil {
		t.Error("expected error for unknown escape sequence")
	}
}

func TestEncodeUnknownChar(t *testing.T) {
	_, err := Encode("€")
	if err == nil {
		t.Error("expected error for unknown character")
	}
}

func TestEncodeEmpty(t *testing.T) {
	_, err := Encode("")
	if err == nil {
		t.Error("expected error for empty string")
	}
}

func TestEncodePunctuation(t *testing.T) {
	got, err := Encode(".,;:?!+-*/=()")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	want := []byte{0x2E, 0x2C, 0x3B, 0x3A, 0x3F, 0x21, 0x2B, 0x2D, 0x2A, 0x2F, 0x3D, 0x28, 0x29}
	if !bytes.Equal(got, want) {
		t.Errorf("got %X, want %X", got, want)
	}
}

func TestEncodeTrailingBackslash(t *testing.T) {
	_, err := Encode(`\`)
	if err == nil {
		t.Error("expected error for trailing backslash")
	}
}

func TestEncodeQuote(t *testing.T) {
	got, err := Encode(`"`)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(got) != 1 || got[0] != 0x22 {
		t.Errorf("got %X, want [22]", got)
	}
}
