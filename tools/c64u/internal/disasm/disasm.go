// Package disasm provides a 6502/6510 disassembler including all
// undocumented/illegal opcodes used on the Commodore 64.
package disasm

import "fmt"

// Result holds one disassembled instruction
type Result struct {
	Address  uint16
	Bytes    []byte   // raw bytes (opcode + operands)
	Mnemonic string   // e.g. "LDA"
	Operand  string   // e.g. "$DC01,X"
	Comment  string   // optional annotation (e.g. known I/O register name)
	Illegal  bool
}

// String returns a human-readable representation
func (r Result) String() string {
	// Format raw bytes as hex
	hex := ""
	for _, b := range r.Bytes {
		hex += fmt.Sprintf("%02X ", b)
	}
	instr := r.Mnemonic
	if r.Operand != "" {
		instr += " " + r.Operand
	}
	illegal := ""
	if r.Illegal {
		illegal = "*"
	}
	s := fmt.Sprintf("$%04X  %-8s  %s%-13s", r.Address, hex, illegal, instr)
	if r.Comment != "" {
		s += "  ; " + r.Comment
	}
	return s
}

// Disassemble decodes one instruction from mem starting at addr.
// mem must contain at least 3 bytes starting at offset 0 (caller slices).
// Returns the Result and the number of bytes consumed.
func Disassemble(addr uint16, mem []byte) (Result, int) {
	if len(mem) == 0 {
		return Result{Address: addr, Bytes: nil, Mnemonic: "???", Operand: ""}, 1
	}

	opcode := mem[0]
	ins := Lookup(opcode)
	size := ins.InstructionLength()

	// Clamp to available bytes
	if size > len(mem) {
		size = len(mem)
	}

	raw := make([]byte, size)
	copy(raw, mem[:size])

	operand := formatOperand(ins, addr, raw)
	comment := ioComment(ins, raw)

	return Result{
		Address:  addr,
		Bytes:    raw,
		Mnemonic: ins.Mnemonic,
		Operand:  operand,
		Comment:  comment,
		Illegal:  ins.Illegal,
	}, size
}

// DisassembleBlock disassembles a block of memory and returns all instructions.
func DisassembleBlock(startAddr uint16, mem []byte) []Result {
	var results []Result
	addr := startAddr
	offset := 0
	for offset < len(mem) {
		r, n := Disassemble(addr, mem[offset:])
		results = append(results, r)
		addr += uint16(n)
		offset += n
	}
	return results
}

// formatOperand formats the operand string for an instruction
func formatOperand(ins Instruction, addr uint16, raw []byte) string {
	if len(raw) < 1 {
		return ""
	}

	switch ins.Mode {
	case Implied, ImpliedPad:
		return ""
	case Accumulator:
		return "A"
	case Immediate:
		if len(raw) < 2 {
			return "#$??"
		}
		return fmt.Sprintf("#$%02X", raw[1])
	case ZeroPage:
		if len(raw) < 2 {
			return "$??"
		}
		return fmt.Sprintf("$%02X", raw[1])
	case ZeroPageX:
		if len(raw) < 2 {
			return "$??,X"
		}
		return fmt.Sprintf("$%02X,X", raw[1])
	case ZeroPageY:
		if len(raw) < 2 {
			return "$??,Y"
		}
		return fmt.Sprintf("$%02X,Y", raw[1])
	case Absolute:
		if len(raw) < 3 {
			return "$????"
		}
		target := uint16(raw[1]) | uint16(raw[2])<<8
		return fmt.Sprintf("$%04X", target)
	case AbsoluteX:
		if len(raw) < 3 {
			return "$????,X"
		}
		target := uint16(raw[1]) | uint16(raw[2])<<8
		return fmt.Sprintf("$%04X,X", target)
	case AbsoluteY:
		if len(raw) < 3 {
			return "$????,Y"
		}
		target := uint16(raw[1]) | uint16(raw[2])<<8
		return fmt.Sprintf("$%04X,Y", target)
	case Indirect:
		if len(raw) < 3 {
			return "($????)"
		}
		target := uint16(raw[1]) | uint16(raw[2])<<8
		return fmt.Sprintf("($%04X)", target)
	case IndirectX:
		if len(raw) < 2 {
			return "($??,X)"
		}
		return fmt.Sprintf("($%02X,X)", raw[1])
	case IndirectY:
		if len(raw) < 2 {
			return "($??),Y"
		}
		return fmt.Sprintf("($%02X),Y", raw[1])
	case Relative:
		if len(raw) < 2 {
			return "$????"
		}
		offset := int8(raw[1])
		// Branch target: addr + 2 (next instruction) + signed offset
		target := uint16(int32(addr) + 2 + int32(offset))
		return fmt.Sprintf("$%04X", target)
	}
	return ""
}

// ioComment returns a human-readable annotation for well-known C64 I/O addresses
func ioComment(ins Instruction, raw []byte) string {
	if len(raw) < 3 {
		return ""
	}
	switch ins.Mode {
	case Absolute, AbsoluteX, AbsoluteY:
		addr := uint16(raw[1]) | uint16(raw[2])<<8
		if name, ok := ioRegisters[addr]; ok {
			return name
		}
	}
	return ""
}

// ioRegisters maps well-known C64 I/O addresses to their names
var ioRegisters = map[uint16]string{
	// VIC-II
	0xD000: "VIC SP0X", 0xD001: "VIC SP0Y",
	0xD002: "VIC SP1X", 0xD003: "VIC SP1Y",
	0xD004: "VIC SP2X", 0xD005: "VIC SP2Y",
	0xD006: "VIC SP3X", 0xD007: "VIC SP3Y",
	0xD008: "VIC SP4X", 0xD009: "VIC SP4Y",
	0xD00A: "VIC SP5X", 0xD00B: "VIC SP5Y",
	0xD00C: "VIC SP6X", 0xD00D: "VIC SP6Y",
	0xD00E: "VIC SP7X", 0xD00F: "VIC SP7Y",
	0xD010: "VIC MSIGX",
	0xD011: "VIC SCROLY",
	0xD012: "VIC RASTER",
	0xD013: "VIC LPENX",
	0xD014: "VIC LPENY",
	0xD015: "VIC SPENA",
	0xD016: "VIC SCROLX",
	0xD017: "VIC YXPAND",
	0xD018: "VIC VMCSB",
	0xD019: "VIC VICIRQ",
	0xD01A: "VIC IRQMASK",
	0xD01B: "VIC SPBGPR",
	0xD01C: "VIC SPMC",
	0xD01D: "VIC XXPAND",
	0xD01E: "VIC SPSPCL",
	0xD01F: "VIC SPBGCL",
	0xD020: "VIC EXTCOL",
	0xD021: "VIC BGCOL0",
	0xD022: "VIC BGCOL1",
	0xD023: "VIC BGCOL2",
	0xD024: "VIC BGCOL3",
	0xD025: "VIC SPMC0",
	0xD026: "VIC SPMC1",
	0xD027: "VIC SP0COL", 0xD028: "VIC SP1COL",
	0xD029: "VIC SP2COL", 0xD02A: "VIC SP3COL",
	0xD02B: "VIC SP4COL", 0xD02C: "VIC SP5COL",
	0xD02D: "VIC SP6COL", 0xD02E: "VIC SP7COL",
	// SID
	0xD400: "SID V1 FREQ LO", 0xD401: "SID V1 FREQ HI",
	0xD402: "SID V1 PW LO",   0xD403: "SID V1 PW HI",
	0xD404: "SID V1 CTRL",    0xD405: "SID V1 AD",
	0xD406: "SID V1 SR",
	0xD407: "SID V2 FREQ LO", 0xD408: "SID V2 FREQ HI",
	0xD409: "SID V2 PW LO",   0xD40A: "SID V2 PW HI",
	0xD40B: "SID V2 CTRL",    0xD40C: "SID V2 AD",
	0xD40D: "SID V2 SR",
	0xD40E: "SID V3 FREQ LO", 0xD40F: "SID V3 FREQ HI",
	0xD410: "SID V3 PW LO",   0xD411: "SID V3 PW HI",
	0xD412: "SID V3 CTRL",    0xD413: "SID V3 AD",
	0xD414: "SID V3 SR",
	0xD415: "SID FILT LO",    0xD416: "SID FILT HI",
	0xD417: "SID RESFILT",    0xD418: "SID SIGVOL",
	0xD419: "SID POTX",       0xD41A: "SID POTY",
	0xD41B: "SID OSC3",       0xD41C: "SID ENV3",
	// CIA1
	0xDC00: "CIA1 PRA",  0xDC01: "CIA1 PRB",
	0xDC02: "CIA1 DDRA", 0xDC03: "CIA1 DDRB",
	0xDC04: "CIA1 TA LO", 0xDC05: "CIA1 TA HI",
	0xDC06: "CIA1 TB LO", 0xDC07: "CIA1 TB HI",
	0xDC08: "CIA1 TOD 10THS", 0xDC09: "CIA1 TOD SEC",
	0xDC0A: "CIA1 TOD MIN",   0xDC0B: "CIA1 TOD HR",
	0xDC0C: "CIA1 SDR",
	0xDC0D: "CIA1 ICR",
	0xDC0E: "CIA1 CRA", 0xDC0F: "CIA1 CRB",
	// CIA2
	0xDD00: "CIA2 PRA",  0xDD01: "CIA2 PRB",
	0xDD02: "CIA2 DDRA", 0xDD03: "CIA2 DDRB",
	0xDD04: "CIA2 TA LO", 0xDD05: "CIA2 TA HI",
	0xDD06: "CIA2 TB LO", 0xDD07: "CIA2 TB HI",
	0xDD08: "CIA2 TOD 10THS", 0xDD09: "CIA2 TOD SEC",
	0xDD0A: "CIA2 TOD MIN",   0xDD0B: "CIA2 TOD HR",
	0xDD0C: "CIA2 SDR",
	0xDD0D: "CIA2 ICR",
	0xDD0E: "CIA2 CRA", 0xDD0F: "CIA2 CRB",
	// Zero page / stack well-known
	0x0000: "CPU DDR",
	0x0001: "CPU PORT",
	// Kernal vectors
	0xFFFA: "NMI VECTOR LO", 0xFFFB: "NMI VECTOR HI",
	0xFFFC: "RST VECTOR LO", 0xFFFD: "RST VECTOR HI",
	0xFFFE: "IRQ VECTOR LO", 0xFFFF: "IRQ VECTOR HI",
}
