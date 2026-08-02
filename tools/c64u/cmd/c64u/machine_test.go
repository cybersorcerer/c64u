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
