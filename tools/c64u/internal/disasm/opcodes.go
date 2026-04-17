package disasm

// AddrMode represents a 6502 addressing mode
type AddrMode uint8

const (
	Implied    AddrMode = iota // no operand
	Accumulator                // A
	Immediate                  // #$nn
	ZeroPage                   // $nn
	ZeroPageX                  // $nn,X
	ZeroPageY                  // $nn,Y
	Absolute                   // $nnnn
	AbsoluteX                  // $nnnn,X
	AbsoluteY                  // $nnnn,Y
	Indirect                   // ($nnnn)
	IndirectX                  // ($nn,X)
	IndirectY                  // ($nn),Y
	Relative                   // $nn (signed branch offset)
	ImpliedPad                 // implied + 1 padding byte (BRK: 6502 fetches 2 bytes)
)

// operandBytes returns number of operand bytes for an addressing mode
func (m AddrMode) operandBytes() int {
	switch m {
	case Implied, Accumulator:
		return 0
	case Immediate, ZeroPage, ZeroPageX, ZeroPageY, IndirectX, IndirectY, Relative, ImpliedPad:
		return 1
	case Absolute, AbsoluteX, AbsoluteY, Indirect:
		return 2
	}
	return 0
}

// Instruction describes one 6502/6510 instruction
type Instruction struct {
	Mnemonic string
	Mode     AddrMode
	Illegal  bool // true for undocumented/illegal opcodes
}

// InstructionLength returns total byte length (opcode + operands)
func (i Instruction) InstructionLength() int {
	return 1 + i.Mode.operandBytes()
}

// opcodeTable maps opcode byte → Instruction
// Illegal opcodes marked with Illegal=true
// Source: "64doc" by John West & Marko Mäkelä, plus VICE source
var opcodeTable = [256]Instruction{
	// 0x00
	0x00: {"BRK", ImpliedPad, false},
	0x01: {"ORA", IndirectX, false},
	0x02: {"JAM", Implied, true},
	0x03: {"SLO", IndirectX, true},
	0x04: {"NOP", ZeroPage, true},
	0x05: {"ORA", ZeroPage, false},
	0x06: {"ASL", ZeroPage, false},
	0x07: {"SLO", ZeroPage, true},
	0x08: {"PHP", Implied, false},
	0x09: {"ORA", Immediate, false},
	0x0A: {"ASL", Accumulator, false},
	0x0B: {"ANC", Immediate, true},
	0x0C: {"NOP", Absolute, true},
	0x0D: {"ORA", Absolute, false},
	0x0E: {"ASL", Absolute, false},
	0x0F: {"SLO", Absolute, true},
	// 0x10
	0x10: {"BPL", Relative, false},
	0x11: {"ORA", IndirectY, false},
	0x12: {"JAM", Implied, true},
	0x13: {"SLO", IndirectY, true},
	0x14: {"NOP", ZeroPageX, true},
	0x15: {"ORA", ZeroPageX, false},
	0x16: {"ASL", ZeroPageX, false},
	0x17: {"SLO", ZeroPageX, true},
	0x18: {"CLC", Implied, false},
	0x19: {"ORA", AbsoluteY, false},
	0x1A: {"NOP", Implied, true},
	0x1B: {"SLO", AbsoluteY, true},
	0x1C: {"NOP", AbsoluteX, true},
	0x1D: {"ORA", AbsoluteX, false},
	0x1E: {"ASL", AbsoluteX, false},
	0x1F: {"SLO", AbsoluteX, true},
	// 0x20
	0x20: {"JSR", Absolute, false},
	0x21: {"AND", IndirectX, false},
	0x22: {"JAM", Implied, true},
	0x23: {"RLA", IndirectX, true},
	0x24: {"BIT", ZeroPage, false},
	0x25: {"AND", ZeroPage, false},
	0x26: {"ROL", ZeroPage, false},
	0x27: {"RLA", ZeroPage, true},
	0x28: {"PLP", Implied, false},
	0x29: {"AND", Immediate, false},
	0x2A: {"ROL", Accumulator, false},
	0x2B: {"ANC", Immediate, true},
	0x2C: {"BIT", Absolute, false},
	0x2D: {"AND", Absolute, false},
	0x2E: {"ROL", Absolute, false},
	0x2F: {"RLA", Absolute, true},
	// 0x30
	0x30: {"BMI", Relative, false},
	0x31: {"AND", IndirectY, false},
	0x32: {"JAM", Implied, true},
	0x33: {"RLA", IndirectY, true},
	0x34: {"NOP", ZeroPageX, true},
	0x35: {"AND", ZeroPageX, false},
	0x36: {"ROL", ZeroPageX, false},
	0x37: {"RLA", ZeroPageX, true},
	0x38: {"SEC", Implied, false},
	0x39: {"AND", AbsoluteY, false},
	0x3A: {"NOP", Implied, true},
	0x3B: {"RLA", AbsoluteY, true},
	0x3C: {"NOP", AbsoluteX, true},
	0x3D: {"AND", AbsoluteX, false},
	0x3E: {"ROL", AbsoluteX, false},
	0x3F: {"RLA", AbsoluteX, true},
	// 0x40
	0x40: {"RTI", Implied, false},
	0x41: {"EOR", IndirectX, false},
	0x42: {"JAM", Implied, true},
	0x43: {"SRE", IndirectX, true},
	0x44: {"NOP", ZeroPage, true},
	0x45: {"EOR", ZeroPage, false},
	0x46: {"LSR", ZeroPage, false},
	0x47: {"SRE", ZeroPage, true},
	0x48: {"PHA", Implied, false},
	0x49: {"EOR", Immediate, false},
	0x4A: {"LSR", Accumulator, false},
	0x4B: {"ALR", Immediate, true},
	0x4C: {"JMP", Absolute, false},
	0x4D: {"EOR", Absolute, false},
	0x4E: {"LSR", Absolute, false},
	0x4F: {"SRE", Absolute, true},
	// 0x50
	0x50: {"BVC", Relative, false},
	0x51: {"EOR", IndirectY, false},
	0x52: {"JAM", Implied, true},
	0x53: {"SRE", IndirectY, true},
	0x54: {"NOP", ZeroPageX, true},
	0x55: {"EOR", ZeroPageX, false},
	0x56: {"LSR", ZeroPageX, false},
	0x57: {"SRE", ZeroPageX, true},
	0x58: {"CLI", Implied, false},
	0x59: {"EOR", AbsoluteY, false},
	0x5A: {"NOP", Implied, true},
	0x5B: {"SRE", AbsoluteY, true},
	0x5C: {"NOP", AbsoluteX, true},
	0x5D: {"EOR", AbsoluteX, false},
	0x5E: {"LSR", AbsoluteX, false},
	0x5F: {"SRE", AbsoluteX, true},
	// 0x60
	0x60: {"RTS", Implied, false},
	0x61: {"ADC", IndirectX, false},
	0x62: {"JAM", Implied, true},
	0x63: {"RRA", IndirectX, true},
	0x64: {"NOP", ZeroPage, true},
	0x65: {"ADC", ZeroPage, false},
	0x66: {"ROR", ZeroPage, false},
	0x67: {"RRA", ZeroPage, true},
	0x68: {"PLA", Implied, false},
	0x69: {"ADC", Immediate, false},
	0x6A: {"ROR", Accumulator, false},
	0x6B: {"ARR", Immediate, true},
	0x6C: {"JMP", Indirect, false},
	0x6D: {"ADC", Absolute, false},
	0x6E: {"ROR", Absolute, false},
	0x6F: {"RRA", Absolute, true},
	// 0x70
	0x70: {"BVS", Relative, false},
	0x71: {"ADC", IndirectY, false},
	0x72: {"JAM", Implied, true},
	0x73: {"RRA", IndirectY, true},
	0x74: {"NOP", ZeroPageX, true},
	0x75: {"ADC", ZeroPageX, false},
	0x76: {"ROR", ZeroPageX, false},
	0x77: {"RRA", ZeroPageX, true},
	0x78: {"SEI", Implied, false},
	0x79: {"ADC", AbsoluteY, false},
	0x7A: {"NOP", Implied, true},
	0x7B: {"RRA", AbsoluteY, true},
	0x7C: {"NOP", AbsoluteX, true},
	0x7D: {"ADC", AbsoluteX, false},
	0x7E: {"ROR", AbsoluteX, false},
	0x7F: {"RRA", AbsoluteX, true},
	// 0x80
	0x80: {"NOP", Immediate, true},
	0x81: {"STA", IndirectX, false},
	0x82: {"NOP", Immediate, true},
	0x83: {"SAX", IndirectX, true},
	0x84: {"STY", ZeroPage, false},
	0x85: {"STA", ZeroPage, false},
	0x86: {"STX", ZeroPage, false},
	0x87: {"SAX", ZeroPage, true},
	0x88: {"DEY", Implied, false},
	0x89: {"NOP", Immediate, true},
	0x8A: {"TXA", Implied, false},
	0x8B: {"ANE", Immediate, true},
	0x8C: {"STY", Absolute, false},
	0x8D: {"STA", Absolute, false},
	0x8E: {"STX", Absolute, false},
	0x8F: {"SAX", Absolute, true},
	// 0x90
	0x90: {"BCC", Relative, false},
	0x91: {"STA", IndirectY, false},
	0x92: {"JAM", Implied, true},
	0x93: {"SHA", IndirectY, true},
	0x94: {"STY", ZeroPageX, false},
	0x95: {"STA", ZeroPageX, false},
	0x96: {"STX", ZeroPageY, false},
	0x97: {"SAX", ZeroPageY, true},
	0x98: {"TYA", Implied, false},
	0x99: {"STA", AbsoluteY, false},
	0x9A: {"TXS", Implied, false},
	0x9B: {"SHS", AbsoluteY, true},
	0x9C: {"SHY", AbsoluteX, true},
	0x9D: {"STA", AbsoluteX, false},
	0x9E: {"SHX", AbsoluteY, true},
	0x9F: {"SHA", AbsoluteY, true},
	// 0xA0
	0xA0: {"LDY", Immediate, false},
	0xA1: {"LDA", IndirectX, false},
	0xA2: {"LDX", Immediate, false},
	0xA3: {"LAX", IndirectX, true},
	0xA4: {"LDY", ZeroPage, false},
	0xA5: {"LDA", ZeroPage, false},
	0xA6: {"LDX", ZeroPage, false},
	0xA7: {"LAX", ZeroPage, true},
	0xA8: {"TAY", Implied, false},
	0xA9: {"LDA", Immediate, false},
	0xAA: {"TAX", Implied, false},
	0xAB: {"LXA", Immediate, true},
	0xAC: {"LDY", Absolute, false},
	0xAD: {"LDA", Absolute, false},
	0xAE: {"LDX", Absolute, false},
	0xAF: {"LAX", Absolute, true},
	// 0xB0
	0xB0: {"BCS", Relative, false},
	0xB1: {"LDA", IndirectY, false},
	0xB2: {"JAM", Implied, true},
	0xB3: {"LAX", IndirectY, true},
	0xB4: {"LDY", ZeroPageX, false},
	0xB5: {"LDA", ZeroPageX, false},
	0xB6: {"LDX", ZeroPageY, false},
	0xB7: {"LAX", ZeroPageY, true},
	0xB8: {"CLV", Implied, false},
	0xB9: {"LDA", AbsoluteY, false},
	0xBA: {"TSX", Implied, false},
	0xBB: {"LAE", AbsoluteY, true},
	0xBC: {"LDY", AbsoluteX, false},
	0xBD: {"LDA", AbsoluteX, false},
	0xBE: {"LDX", AbsoluteY, false},
	0xBF: {"LAX", AbsoluteY, true},
	// 0xC0
	0xC0: {"CPY", Immediate, false},
	0xC1: {"CMP", IndirectX, false},
	0xC2: {"NOP", Immediate, true},
	0xC3: {"DCP", IndirectX, true},
	0xC4: {"CPY", ZeroPage, false},
	0xC5: {"CMP", ZeroPage, false},
	0xC6: {"DEC", ZeroPage, false},
	0xC7: {"DCP", ZeroPage, true},
	0xC8: {"INY", Implied, false},
	0xC9: {"CMP", Immediate, false},
	0xCA: {"DEX", Implied, false},
	0xCB: {"SBX", Immediate, true},
	0xCC: {"CPY", Absolute, false},
	0xCD: {"CMP", Absolute, false},
	0xCE: {"DEC", Absolute, false},
	0xCF: {"DCP", Absolute, true},
	// 0xD0
	0xD0: {"BNE", Relative, false},
	0xD1: {"CMP", IndirectY, false},
	0xD2: {"JAM", Implied, true},
	0xD3: {"DCP", IndirectY, true},
	0xD4: {"NOP", ZeroPageX, true},
	0xD5: {"CMP", ZeroPageX, false},
	0xD6: {"DEC", ZeroPageX, false},
	0xD7: {"DCP", ZeroPageX, true},
	0xD8: {"CLD", Implied, false},
	0xD9: {"CMP", AbsoluteY, false},
	0xDA: {"NOP", Implied, true},
	0xDB: {"DCP", AbsoluteY, true},
	0xDC: {"NOP", AbsoluteX, true},
	0xDD: {"CMP", AbsoluteX, false},
	0xDE: {"DEC", AbsoluteX, false},
	0xDF: {"DCP", AbsoluteX, true},
	// 0xE0
	0xE0: {"CPX", Immediate, false},
	0xE1: {"SBC", IndirectX, false},
	0xE2: {"NOP", Immediate, true},
	0xE3: {"ISC", IndirectX, true},
	0xE4: {"CPX", ZeroPage, false},
	0xE5: {"SBC", ZeroPage, false},
	0xE6: {"INC", ZeroPage, false},
	0xE7: {"ISC", ZeroPage, true},
	0xE8: {"INX", Implied, false},
	0xE9: {"SBC", Immediate, false},
	0xEA: {"NOP", Implied, false},
	0xEB: {"SBC", Immediate, true},
	0xEC: {"CPX", Absolute, false},
	0xED: {"SBC", Absolute, false},
	0xEE: {"INC", Absolute, false},
	0xEF: {"ISC", Absolute, true},
	// 0xF0
	0xF0: {"BEQ", Relative, false},
	0xF1: {"SBC", IndirectY, false},
	0xF2: {"JAM", Implied, true},
	0xF3: {"ISC", IndirectY, true},
	0xF4: {"NOP", ZeroPageX, true},
	0xF5: {"SBC", ZeroPageX, false},
	0xF6: {"INC", ZeroPageX, false},
	0xF7: {"ISC", ZeroPageX, true},
	0xF8: {"SED", Implied, false},
	0xF9: {"SBC", AbsoluteY, false},
	0xFA: {"NOP", Implied, true},
	0xFB: {"ISC", AbsoluteY, true},
	0xFC: {"NOP", AbsoluteX, true},
	0xFD: {"SBC", AbsoluteX, false},
	0xFE: {"INC", AbsoluteX, false},
	0xFF: {"ISC", AbsoluteX, true},
}

// Lookup returns the Instruction for a given opcode byte
func Lookup(opcode uint8) Instruction {
	return opcodeTable[opcode]
}
