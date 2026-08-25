package disasm

import "testing"

// Spot checks against well-known cycle counts. The value of these is that they
// cover one opcode per access pattern, so a wrong classification in cycles.go
// shows up here rather than in a raster routine that drifts by a cycle.
func TestCycles(t *testing.T) {
	cases := []struct {
		opcode    uint8
		name      string
		cycles    int
		pageCross bool
	}{
		{0xA9, "LDA #", 2, false},
		{0xA5, "LDA $nn", 3, false},
		{0xB5, "LDA $nn,X", 4, false},
		{0xAD, "LDA $nnnn", 4, false},
		{0xBD, "LDA $nnnn,X", 4, true},
		{0xB9, "LDA $nnnn,Y", 4, true},
		{0xA1, "LDA ($nn,X)", 6, false},
		{0xB1, "LDA ($nn),Y", 5, true},

		{0x85, "STA $nn", 3, false},
		{0x9D, "STA $nnnn,X", 5, false},
		{0x91, "STA ($nn),Y", 6, false},

		{0x0A, "ASL A", 2, false},
		{0x06, "ASL $nn", 5, false},
		{0x16, "ASL $nn,X", 6, false},
		{0x0E, "ASL $nnnn", 6, false},
		{0x1E, "ASL $nnnn,X", 7, false},
		{0xFE, "INC $nnnn,X", 7, false},

		{0x4C, "JMP $nnnn", 3, false},
		{0x6C, "JMP ($nnnn)", 5, false},
		{0x20, "JSR", 6, false},
		{0x60, "RTS", 6, false},
		{0x40, "RTI", 6, false},
		{0x00, "BRK", 7, false},
		{0x48, "PHA", 3, false},
		{0x28, "PLP", 4, false},
		{0xEA, "NOP", 2, false},
		{0xAA, "TAX", 2, false},

		{0xF0, "BEQ", 2, true},
		{0xD0, "BNE", 2, true},

		// Illegal opcodes follow the same rules as their legal shape.
		{0x03, "SLO ($nn,X)", 8, false},
		{0x07, "SLO $nn", 5, false},
		{0xA7, "LAX $nn", 3, false},
		{0xBF, "LAX $nnnn,Y", 4, true},
		{0x87, "SAX $nn", 3, false},
		{0x9C, "SHY $nnnn,X", 5, false},
		{0x4B, "ALR #", 2, false},
	}

	for _, c := range cases {
		cycles, pageCross := Cycles(c.opcode)
		if cycles != c.cycles || pageCross != c.pageCross {
			t.Errorf("$%02X %s: got %d cycles pageCross=%v, want %d pageCross=%v",
				c.opcode, c.name, cycles, pageCross, c.cycles, c.pageCross)
		}
	}
}

// JAM opcodes hang the CPU, so no cycle count applies.
func TestCyclesJam(t *testing.T) {
	cycles, pageCross := Cycles(0x02)
	if cycles != 0 || pageCross {
		t.Errorf("JAM: got %d cycles pageCross=%v, want 0 false", cycles, pageCross)
	}
}

// Every opcode that is not a JAM must have a plausible count.
func TestCyclesCoverage(t *testing.T) {
	for i := 0; i < 256; i++ {
		op := uint8(i)
		cycles, _ := Cycles(op)
		if Lookup(op).Mnemonic == "JAM" {
			continue
		}
		if cycles < 2 || cycles > 8 {
			t.Errorf("$%02X %s: implausible cycle count %d",
				op, Lookup(op).Mnemonic, cycles)
		}
	}
}
