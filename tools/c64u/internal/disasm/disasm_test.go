package disasm

import (
	"strings"
	"testing"
)

func TestDisassemble_Implied(t *testing.T) {
	r, n := Disassemble(0x1000, []byte{0xEA}) // NOP
	if r.Mnemonic != "NOP" {
		t.Errorf("mnemonic = %q, want NOP", r.Mnemonic)
	}
	if r.Operand != "" {
		t.Errorf("operand = %q, want empty", r.Operand)
	}
	if n != 1 {
		t.Errorf("size = %d, want 1", n)
	}
}

func TestDisassemble_Immediate(t *testing.T) {
	r, n := Disassemble(0x0800, []byte{0xA9, 0x42}) // LDA #$42
	if r.Mnemonic != "LDA" {
		t.Errorf("mnemonic = %q, want LDA", r.Mnemonic)
	}
	if r.Operand != "#$42" {
		t.Errorf("operand = %q, want #$42", r.Operand)
	}
	if n != 2 {
		t.Errorf("size = %d, want 2", n)
	}
}

func TestDisassemble_Absolute(t *testing.T) {
	r, n := Disassemble(0x0800, []byte{0xAD, 0x00, 0xDC}) // LDA $DC00
	if r.Mnemonic != "LDA" {
		t.Errorf("mnemonic = %q, want LDA", r.Mnemonic)
	}
	if r.Operand != "$DC00" {
		t.Errorf("operand = %q, want $DC00", r.Operand)
	}
	if n != 3 {
		t.Errorf("size = %d, want 3", n)
	}
}

func TestDisassemble_AbsoluteX(t *testing.T) {
	r, _ := Disassemble(0x0800, []byte{0xBD, 0x00, 0x04}) // LDA $0400,X
	if r.Operand != "$0400,X" {
		t.Errorf("operand = %q, want $0400,X", r.Operand)
	}
}

func TestDisassemble_AbsoluteY(t *testing.T) {
	r, _ := Disassemble(0x0800, []byte{0xB9, 0x00, 0x04}) // LDA $0400,Y
	if r.Operand != "$0400,Y" {
		t.Errorf("operand = %q, want $0400,Y", r.Operand)
	}
}

func TestDisassemble_ZeroPage(t *testing.T) {
	r, n := Disassemble(0x0800, []byte{0x85, 0xFB}) // STA $FB
	if r.Operand != "$FB" {
		t.Errorf("operand = %q, want $FB", r.Operand)
	}
	if n != 2 {
		t.Errorf("size = %d, want 2", n)
	}
}

func TestDisassemble_ZeroPageX(t *testing.T) {
	r, _ := Disassemble(0x0800, []byte{0x95, 0x20}) // STA $20,X
	if r.Operand != "$20,X" {
		t.Errorf("operand = %q, want $20,X", r.Operand)
	}
}

func TestDisassemble_ZeroPageY(t *testing.T) {
	r, _ := Disassemble(0x0800, []byte{0x96, 0x20}) // STX $20,Y
	if r.Operand != "$20,Y" {
		t.Errorf("operand = %q, want $20,Y", r.Operand)
	}
}

func TestDisassemble_IndirectX(t *testing.T) {
	r, _ := Disassemble(0x0800, []byte{0xA1, 0x20}) // LDA ($20,X)
	if r.Operand != "($20,X)" {
		t.Errorf("operand = %q, want ($20,X)", r.Operand)
	}
}

func TestDisassemble_IndirectY(t *testing.T) {
	r, _ := Disassemble(0x0800, []byte{0xB1, 0x20}) // LDA ($20),Y
	if r.Operand != "($20),Y" {
		t.Errorf("operand = %q, want ($20),Y", r.Operand)
	}
}

func TestDisassemble_Indirect(t *testing.T) {
	r, _ := Disassemble(0x0800, []byte{0x6C, 0x00, 0x03}) // JMP ($0300)
	if r.Operand != "($0300)" {
		t.Errorf("operand = %q, want ($0300)", r.Operand)
	}
}

func TestDisassemble_Accumulator(t *testing.T) {
	r, n := Disassemble(0x0800, []byte{0x0A}) // ASL A
	if r.Operand != "A" {
		t.Errorf("operand = %q, want A", r.Operand)
	}
	if n != 1 {
		t.Errorf("size = %d, want 1", n)
	}
}

func TestDisassemble_Relative_Forward(t *testing.T) {
	// BNE $0804 (from $0800: offset=+2, target=$0804)
	r, n := Disassemble(0x0800, []byte{0xD0, 0x02})
	if r.Mnemonic != "BNE" {
		t.Errorf("mnemonic = %q, want BNE", r.Mnemonic)
	}
	if r.Operand != "$0804" {
		t.Errorf("operand = %q, want $0804", r.Operand)
	}
	if n != 2 {
		t.Errorf("size = %d, want 2", n)
	}
}

func TestDisassemble_Relative_Backward(t *testing.T) {
	// BNE $08B2 (from $08B7: offset=-7=0xF9, target=$08B7+2-7=$08B2)
	r, _ := Disassemble(0x08B7, []byte{0xD0, 0xF9})
	if r.Operand != "$08B2" {
		t.Errorf("operand = %q, want $08B2", r.Operand)
	}
}

func TestDisassemble_IllegalOpcode_LAX(t *testing.T) {
	r, _ := Disassemble(0x0800, []byte{0xAF, 0x00, 0xDC}) // LAX $DC00
	if r.Mnemonic != "LAX" {
		t.Errorf("mnemonic = %q, want LAX", r.Mnemonic)
	}
	if !r.Illegal {
		t.Error("expected Illegal=true for LAX")
	}
}

func TestDisassemble_IllegalOpcode_SLO(t *testing.T) {
	r, _ := Disassemble(0x0800, []byte{0x07, 0x20}) // SLO $20
	if r.Mnemonic != "SLO" {
		t.Errorf("mnemonic = %q, want SLO", r.Mnemonic)
	}
	if !r.Illegal {
		t.Error("expected Illegal=true for SLO")
	}
}

func TestDisassemble_IllegalOpcode_JAM(t *testing.T) {
	r, n := Disassemble(0x0800, []byte{0x02}) // JAM
	if r.Mnemonic != "JAM" {
		t.Errorf("mnemonic = %q, want JAM", r.Mnemonic)
	}
	if !r.Illegal {
		t.Error("expected Illegal=true for JAM")
	}
	if n != 1 {
		t.Errorf("size = %d, want 1", n)
	}
}

func TestDisassemble_IOComment_CIA1(t *testing.T) {
	r, _ := Disassemble(0x0800, []byte{0xAD, 0x01, 0xDC}) // LDA $DC01
	if !strings.Contains(r.Comment, "CIA1") {
		t.Errorf("comment = %q, expected CIA1 annotation", r.Comment)
	}
}

func TestDisassemble_IOComment_SID(t *testing.T) {
	r, _ := Disassemble(0x0800, []byte{0x8D, 0x04, 0xD4}) // STA $D404
	if !strings.Contains(r.Comment, "SID") {
		t.Errorf("comment = %q, expected SID annotation", r.Comment)
	}
}

func TestDisassemble_IOComment_VIC(t *testing.T) {
	r, _ := Disassemble(0x0800, []byte{0x8D, 0x11, 0xD0}) // STA $D011
	if !strings.Contains(r.Comment, "VIC") {
		t.Errorf("comment = %q, expected VIC annotation", r.Comment)
	}
}

func TestDisassemble_String(t *testing.T) {
	r, _ := Disassemble(0x0800, []byte{0xA9, 0x42}) // LDA #$42
	s := r.String()
	if !strings.Contains(s, "$0800") {
		t.Errorf("String() missing address: %q", s)
	}
	if !strings.Contains(s, "LDA") {
		t.Errorf("String() missing mnemonic: %q", s)
	}
	if !strings.Contains(s, "#$42") {
		t.Errorf("String() missing operand: %q", s)
	}
}

func TestDisassembleBlock(t *testing.T) {
	// LDA #$37 / STA $01 / NOP
	mem := []byte{0xA9, 0x37, 0x85, 0x01, 0xEA}
	results := DisassembleBlock(0x0800, mem)
	if len(results) != 3 {
		t.Fatalf("got %d instructions, want 3", len(results))
	}
	if results[0].Mnemonic != "LDA" {
		t.Errorf("[0] mnemonic = %q, want LDA", results[0].Mnemonic)
	}
	if results[1].Mnemonic != "STA" {
		t.Errorf("[1] mnemonic = %q, want STA", results[1].Mnemonic)
	}
	if results[2].Mnemonic != "NOP" {
		t.Errorf("[2] mnemonic = %q, want NOP", results[2].Mnemonic)
	}
	if results[1].Address != 0x0802 {
		t.Errorf("[1] address = $%04X, want $0802", results[1].Address)
	}
	if results[2].Address != 0x0804 {
		t.Errorf("[2] address = $%04X, want $0804", results[2].Address)
	}
}

func TestOpcodeTable_AllEntriesValid(t *testing.T) {
	for i := 0; i < 256; i++ {
		ins := Lookup(uint8(i))
		if ins.Mnemonic == "" {
			t.Errorf("opcode $%02X has empty mnemonic", i)
		}
		size := ins.InstructionLength()
		if size < 1 || size > 3 {
			t.Errorf("opcode $%02X has invalid size %d", i, size)
		}
	}
}

func TestLookup_NOP_EA(t *testing.T) {
	ins := Lookup(0xEA)
	if ins.Mnemonic != "NOP" {
		t.Errorf("$EA = %q, want NOP", ins.Mnemonic)
	}
	if ins.Illegal {
		t.Error("$EA NOP should not be illegal")
	}
}

func TestLookup_JSR(t *testing.T) {
	ins := Lookup(0x20)
	if ins.Mnemonic != "JSR" {
		t.Errorf("$20 = %q, want JSR", ins.Mnemonic)
	}
	if ins.InstructionLength() != 3 {
		t.Errorf("JSR length = %d, want 3", ins.InstructionLength())
	}
}

// An instruction that starts inside the range is shown whole, from the bytes
// past its end; one that starts after it is not shown.
func TestDisassembleRange(t *testing.T) {
	got := DisassembleRange(0xC000, []byte{0xA9, 0x00, 0x8D, 0x20, 0xD0}, 3)
	if len(got) != 2 {
		t.Fatalf("got %d instructions, want 2: %v", len(got), got)
	}
	if got[1].Address != 0xC002 || got[1].Mnemonic != "STA" || got[1].Operand != "$D020" || len(got[1].Bytes) != 3 {
		t.Errorf("second = %+v, want STA $D020 at $C002 with 3 bytes", got[1])
	}

	got = DisassembleRange(0xC000, []byte{0xA9, 0x00, 0xEA, 0xEA}, 2)
	if len(got) != 1 {
		t.Errorf("got %d instructions, want only the one starting in range", len(got))
	}
}

// At the end of memory there are no bytes past the range: the operand is cut.
func TestDisassembleRange_Truncated(t *testing.T) {
	got := DisassembleRange(0xFFFF, []byte{0x8D}, 1)
	if len(got) != 1 || len(got[0].Bytes) != 1 || got[0].Address != 0xFFFF {
		t.Errorf("got %+v, want one instruction of 1 byte at $FFFF", got)
	}
}

func TestDisassembleRange_Illegal(t *testing.T) {
	got := DisassembleRange(0x1000, []byte{0x02}, 1)
	if len(got) != 1 || !got[0].Illegal {
		t.Errorf("got %+v, want one illegal instruction", got)
	}
}

// The mnemonic column lines up whatever the instruction length.
func TestResultString_Aligned(t *testing.T) {
	one, _ := Disassemble(0x1000, []byte{0xEA})
	three, _ := Disassemble(0x1000, []byte{0x8D, 0x20, 0xD0})
	if a, b := strings.Index(one.String(), "NOP"), strings.Index(three.String(), "STA"); a != b {
		t.Errorf("mnemonic at column %d and %d:\n%q\n%q", a, b, one.String(), three.String())
	}
}
