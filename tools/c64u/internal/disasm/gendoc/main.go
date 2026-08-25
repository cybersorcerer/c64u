// Command gendoc renders the 6502/6510 opcode reference used by the
// c64-knowledge skill from the disassembler's own tables, so the documentation
// cannot drift away from the code.
//
//	go run ./internal/disasm/gendoc -o ../../skills/c64-knowledge/references/opcodes.md
package main

import (
	"flag"
	"fmt"
	"os"
	"sort"
	"strings"

	"github.com/cybersorcerer/c64.nvim/tools/c64u/internal/disasm"
)

func modeSyntax(m disasm.AddrMode) string {
	switch m {
	case disasm.Implied, disasm.ImpliedPad:
		return "implied"
	case disasm.Accumulator:
		return "A"
	case disasm.Immediate:
		return "#$nn"
	case disasm.ZeroPage:
		return "$nn"
	case disasm.ZeroPageX:
		return "$nn,X"
	case disasm.ZeroPageY:
		return "$nn,Y"
	case disasm.Absolute:
		return "$nnnn"
	case disasm.AbsoluteX:
		return "$nnnn,X"
	case disasm.AbsoluteY:
		return "$nnnn,Y"
	case disasm.Indirect:
		return "($nnnn)"
	case disasm.IndirectX:
		return "($nn,X)"
	case disasm.IndirectY:
		return "($nn),Y"
	case disasm.Relative:
		return "$nnnn"
	}
	return "?"
}

func cycleText(op uint8) string {
	cycles, pageCross := disasm.Cycles(op)
	if cycles == 0 {
		return "-"
	}
	if disasm.Lookup(op).Mode == disasm.Relative {
		return "2/3/4"
	}
	if pageCross {
		return fmt.Sprintf("%d+", cycles)
	}
	return fmt.Sprint(cycles)
}

type entry struct {
	Opcode uint8
	Ins    disasm.Instruction
}

func main() {
	out := flag.String("o", "", "output file (default stdout)")
	flag.Parse()

	var b strings.Builder

	b.WriteString(header)

	// Grid: 16 rows of 16, mnemonic plus cycle count.
	b.WriteString("\n## Opcode grid\n\n")
	b.WriteString("Cell shows the mnemonic and the cycle count. `+` means one more cycle when an\n")
	b.WriteString("indexed address crosses a page. Branches show not taken / taken / taken across a page.\n")
	b.WriteString("Illegal opcodes are marked with `*`.\n\n")
	b.WriteString("|  |")
	for col := 0; col < 16; col++ {
		fmt.Fprintf(&b, " x%X |", col)
	}
	b.WriteString("\n|---|")
	b.WriteString(strings.Repeat("---|", 16))
	b.WriteString("\n")
	for row := 0; row < 16; row++ {
		fmt.Fprintf(&b, "| **%Xx** |", row)
		for col := 0; col < 16; col++ {
			op := uint8(row*16 + col)
			ins := disasm.Lookup(op)
			mark := ""
			if ins.Illegal {
				mark = "*"
			}
			fmt.Fprintf(&b, " %s%s %s |", ins.Mnemonic, mark, cycleText(op))
		}
		b.WriteString("\n")
	}

	// Group by mnemonic.
	byMnemonic := map[string][]entry{}
	for i := 0; i < 256; i++ {
		op := uint8(i)
		ins := disasm.Lookup(op)
		byMnemonic[ins.Mnemonic] = append(byMnemonic[ins.Mnemonic], entry{op, ins})
	}
	names := make([]string, 0, len(byMnemonic))
	for n := range byMnemonic {
		names = append(names, n)
	}
	sort.Strings(names)

	writeGroup := func(title string, illegal bool) {
		fmt.Fprintf(&b, "\n## %s\n\n", title)
		b.WriteString("| Mnemonic | Mode | Opcode | Bytes | Cycles |\n")
		b.WriteString("|---|---|---|---|---|\n")
		for _, n := range names {
			entries := byMnemonic[n]
			if entries[0].Ins.Illegal != illegal {
				continue
			}
			sort.Slice(entries, func(i, j int) bool { return entries[i].Opcode < entries[j].Opcode })
			for _, e := range entries {
				fmt.Fprintf(&b, "| %s | %s | `$%02X` | %d | %s |\n",
					e.Ins.Mnemonic, modeSyntax(e.Ins.Mode), e.Opcode,
					e.Ins.InstructionLength(), cycleText(e.Opcode))
			}
		}
	}
	writeGroup("Documented instructions", false)
	writeGroup("Undocumented instructions", true)

	b.WriteString(footer)

	data := []byte(b.String())
	if *out == "" {
		os.Stdout.Write(data)
		return
	}
	if err := os.WriteFile(*out, data, 0o644); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	fmt.Fprintf(os.Stderr, "wrote %s (%d bytes)\n", *out, len(data))
}

const header = `# 6502 / 6510 Opcodes and Cycles

**Generated file - do not edit.** Produced from the disassembler's own opcode table by
` + "`go run ./internal/disasm/gendoc`" + ` in ` + "`tools/c64u`" + `, so it cannot drift away from the
code that decodes these bytes. Regenerate with ` + "`make -C skills/c64-knowledge opcodes`" + `.

## Timing basics

One cycle is one CPU clock: 985248 Hz on PAL, 1022727 Hz on NTSC. A PAL raster line is 63
cycles, an NTSC line 65.

Three things add cycles beyond the table:

1. **Page crossing.** An indexed read whose address crosses a 256-byte boundary costs one extra
   cycle, marked ` + "`+`" + ` below. Stores never pay it. Aligning tables with ` + "`.align $100`" + ` avoids it.
2. **Taken branches.** A branch costs 2 cycles when not taken, 3 when taken, 4 when the target
   is on another page.
3. **Badlines.** Every eighth raster line the VIC steals 40-43 cycles from the CPU, leaving
   about 20 of the usual 63. See ` + "`vic-ii.md`" + `.

Read-modify-write instructions write the unmodified value back before the modified one. That
extra write is visible to hardware: ` + "`inc $d019`" + ` acknowledges a VIC interrupt as a side effect,
which is why ` + "`asl $d019`" + ` is the idiom for acknowledging raster interrupts.
`

const footer = `
## Undocumented opcodes on real hardware

The 6510 in a C64 executes these reliably, and demos use them for the cycles they save - ` + "`LAX`" + `
loads A and X in one instruction, ` + "`SAX`" + ` stores ` + "`A AND X`" + `. Two caveats: ` + "`ANE`" + ` and ` + "`LXA`" + ` depend on
analogue behaviour and vary between chips and temperatures, and ` + "`SHA`" + `, ` + "`SHX`" + `, ` + "`SHY`" + ` and ` + "`SHS`" + `
behave unpredictably when the indexed address crosses a page. ` + "`JAM`" + ` hangs the CPU until reset.

Kick Assembler accepts them by default; ` + "`-excludeillegal`" + ` turns them off.
`
