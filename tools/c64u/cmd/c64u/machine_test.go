package main

import "testing"

func TestParseAddress(t *testing.T) {
	good := map[string]int{
		"0400":   0x0400,
		"$0400":  0x0400,
		"0x0400": 0x0400,
		"0X0400": 0x0400,
		"d020":   0xD020,
		"D020":   0xD020,
		" 0400 ": 0x0400,
		"0":      0,
		"ffff":   0xFFFF,
	}
	for in, want := range good {
		got, err := parseAddress(in)
		if err != nil {
			t.Errorf("parseAddress(%q) failed: %v", in, err)
			continue
		}
		if got != want {
			t.Errorf("parseAddress(%q) = $%04X, want $%04X", in, got, want)
		}
	}

	// The device answers "zzzz" with data from somewhere rather than an error,
	// so these have to be caught before the request goes out.
	for _, bad := range []string{"zzzz", "", "   ", "$", "0x", "12g4", "10000", "-1"} {
		if got, err := parseAddress(bad); err == nil {
			t.Errorf("parseAddress(%q) accepted it as $%04X", bad, got)
		}
	}
}

func TestRangeLengthIsInclusive(t *testing.T) {
	// Screen RAM: $0400 through $07E7 is 1000 bytes, the classic 40x25.
	if got, err := rangeLength(0x0400, "07e7"); err != nil || got != 1000 {
		t.Errorf("rangeLength($0400, 07e7) = %d, %v; want 1000", got, err)
	}
	// A single byte.
	if got, err := rangeLength(0xD020, "d020"); err != nil || got != 1 {
		t.Errorf("rangeLength($D020, d020) = %d, %v; want 1", got, err)
	}
	// Whole address space.
	if got, err := rangeLength(0x0000, "ffff"); err != nil || got != 65536 {
		t.Errorf("rangeLength($0000, ffff) = %d, %v; want 65536", got, err)
	}
}

func TestRangeLengthRejectsBadRanges(t *testing.T) {
	if _, err := rangeLength(0x0500, "0400"); err == nil {
		t.Error("an end below the start was accepted")
	}
	if _, err := rangeLength(0x0400, "zzzz"); err == nil {
		t.Error("a non-hexadecimal end address was accepted")
	}
	if _, err := rangeLength(0x0400, "10000"); err == nil {
		t.Error("an end beyond $FFFF was accepted")
	}
}

// Firmware 1.2.1 writes "100" as the single byte $10 and reports success, so
// the data is checked here.
func TestCheckWriteData(t *testing.T) {
	for _, good := range []string{"00", "01020304", "aBcD"} {
		if err := checkWriteData(good); err != nil {
			t.Errorf("checkWriteData(%q) rejected it: %v", good, err)
		}
	}
	for _, bad := range []string{"", "100", "zz", "0x10", "01 02"} {
		if err := checkWriteData(bad); err == nil {
			t.Errorf("checkWriteData(%q) accepted it", bad)
		}
	}
}

// A full mouse queue (429) or a refused batch (400) is not a missing endpoint.
func TestInputErrorTitle(t *testing.T) {
	cases := map[int]string{
		404: "Input is not available on this device",
		501: "Input is not available on this device",
		429: "The device refused the input",
		400: "The device refused the input",
	}
	for status, want := range cases {
		if got := inputErrorTitle(status); got != want {
			t.Errorf("inputErrorTitle(%d) = %q, want %q", status, got, want)
		}
	}
}

// --disasm reads up to two bytes more so the last instruction is whole, but
// never past $FFFF.
func TestDisasmReadLength(t *testing.T) {
	cases := []struct{ addr, length, want int }{
		{0xC000, 16, 18},
		{0xFFFA, 6, 6},
		{0xFFF0, 15, 16},
		{0x0000, 0x10000, 0x10000},
	}
	for _, c := range cases {
		if got := disasmReadLength(c.addr, c.length); got != c.want {
			t.Errorf("disasmReadLength($%04X, %d) = %d, want %d", c.addr, c.length, got, c.want)
		}
	}
}
