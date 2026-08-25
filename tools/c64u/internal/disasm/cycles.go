package disasm

// Cycle counts are derived from the addressing mode plus the access pattern of
// the mnemonic, which is how the 6502 actually decides them: a read-modify-write
// always pays for the extra write cycles, a store never pays the page-crossing
// penalty because it writes unconditionally.
//
// Source: "64doc" by John West & Marko Mäkelä, matching the table already used
// for the mnemonics in opcodes.go.

// readModifyWrite instructions read a value, change it and write it back.
var readModifyWrite = map[string]bool{
	"ASL": true, "LSR": true, "ROL": true, "ROR": true,
	"INC": true, "DEC": true,
	// Illegal combinations built from an RMW plus an ALU operation.
	"SLO": true, "RLA": true, "SRE": true, "RRA": true,
	"DCP": true, "ISC": true,
}

// stores write without reading first, so an indexed address that crosses a page
// costs nothing extra - the cycle is already spent.
var stores = map[string]bool{
	"STA": true, "STX": true, "STY": true,
	"SAX": true, "SHA": true, "SHX": true, "SHY": true, "SHS": true,
}

// branches take 2 cycles when not taken, 3 when taken, 4 when the target is on
// another page.
var branches = map[string]bool{
	"BPL": true, "BMI": true, "BVC": true, "BVS": true,
	"BCC": true, "BCS": true, "BNE": true, "BEQ": true,
}

// stackAndJump instructions do not follow the addressing-mode defaults.
var stackAndJump = map[string]int{
	"PHA": 3, "PHP": 3,
	"PLA": 4, "PLP": 4,
	"JSR": 6, "RTS": 6, "RTI": 6, "BRK": 7,
}

// Cycles returns how many cycles an opcode takes and whether an additional
// cycle is paid when an indexed address crosses a page boundary.
//
// For branches the returned count is the not-taken case; a taken branch costs
// one more, and one more again when it lands on a different page - so the
// pageCross flag is set for those too.
//
// JAM opcodes hang the processor. They return 0 cycles.
func Cycles(opcode uint8) (cycles int, pageCross bool) {
	ins := Lookup(opcode)

	if ins.Mnemonic == "JAM" {
		return 0, false
	}
	if n, ok := stackAndJump[ins.Mnemonic]; ok {
		return n, false
	}
	if ins.Mnemonic == "JMP" {
		if ins.Mode == Indirect {
			return 5, false
		}
		return 3, false
	}
	if branches[ins.Mnemonic] {
		return 2, true
	}

	switch {
	case readModifyWrite[ins.Mnemonic]:
		switch ins.Mode {
		case Accumulator:
			return 2, false
		case ZeroPage:
			return 5, false
		case ZeroPageX, ZeroPageY:
			return 6, false
		case Absolute:
			return 6, false
		case AbsoluteX, AbsoluteY:
			return 7, false
		case IndirectX, IndirectY:
			return 8, false
		}

	case stores[ins.Mnemonic]:
		switch ins.Mode {
		case ZeroPage:
			return 3, false
		case ZeroPageX, ZeroPageY:
			return 4, false
		case Absolute:
			return 4, false
		case AbsoluteX, AbsoluteY:
			return 5, false
		case IndirectX:
			return 6, false
		case IndirectY:
			return 6, false
		}
	}

	// Everything else reads its operand.
	switch ins.Mode {
	case Implied, ImpliedPad, Accumulator, Immediate:
		return 2, false
	case ZeroPage:
		return 3, false
	case ZeroPageX, ZeroPageY:
		return 4, false
	case Absolute:
		return 4, false
	case AbsoluteX, AbsoluteY:
		return 4, true
	case IndirectX:
		return 6, false
	case IndirectY:
		return 5, true
	}

	return 2, false
}
