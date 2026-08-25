# Kick Assembler

A 6502 assembler with a full script language built in (Java-based). Written by Mads Nielsen.
Everything below is from the official manual.

## Running it

```sh
java -jar KickAss.jar program.asm -o program.prg
```

| Option | Effect |
|---|---|
| `-o file.prg` | Output file (default: source name with `.prg`) |
| `-odir dir` | Output directory |
| `-binfile` | Emit a raw binary without the two-byte load address |
| `-showmem` | Print a memory map after assembling |
| `-bytedump` | Write `ByteDump.txt` mapping bytes to source lines |
| `-vicesymbols` | Write a VICE label file |
| `-debugdump` | Write C64Debugger mapping info |
| `-symbolfile` | Write a `.sym` file importable by other sources |
| `-libdir path` | Extra search path for included files |
| `-define SYM` | Define a preprocessor symbol |
| `-execute "x64 +sound"` | Run a program (e.g. an emulator) with the output on success |
| `-afo` | Allow file output outside the output dir |
| `-aom` | Overlapping memory blocks warn instead of erroring |
| `-fillbyte 255` | Byte used to pad gaps between memory blocks |
| `-excludeillegal` | Drop the illegal opcodes from the instruction set |
| `:name=value` | Pass a string into the script, readable via `cmdLineVars` |

## Syntax basics

Numbers: decimal, `$` hex, `%` binary. Characters in single quotes: `'A'`.

Comments are `//` and `/* */`.

Labels end with `:` and are referenced bare:

```asm
loop:   inc $d020
        jmp loop
```

Multi-labels can repeat; refer forward with `+` and backward with `-`:

```asm
        ldx #100
!loop:  inc $d020
        dex
        bne !loop-
```

`*` is the current program counter, so `jmp *` is an endless loop and `jmp *-6` jumps back six
bytes.

Argument labels enable self-modifying code:

```asm
        stx tmpX
        ldx tmpX:#$00       // the operand byte is labelled tmpX
```

Forward references default to 16-bit addressing. Mark zero-page labels with `.zp { ... }` to
get the short form.

## Memory directives

```asm
*=$1000 "Program"           // set the PC, optionally naming the block for -showmem
*=$0400 "Buffers" virtual   // reserved but not written to the output file
.align $100                 // advance the PC to the next page boundary
.pseudopc $2000 { ... }     // assemble as if at $2000, store at the real PC
```

`virtual` blocks may overlap other blocks and are excluded from the file - the right tool for
buffers and tables that are filled at runtime.

`.align $100` before a lookup table removes the one-cycle page-crossing penalty on indexed
reads.

## Data directives

```asm
.byte $01,$02,$03           // aliases: .by
.word $2000,$1234           // little-endian; aliases: .wo
.dword $12341234            // aliases: .dw
.text "Hello"               // aliases: .te; encoding-dependent, see below

.fill 5, 0                  // 0,0,0,0,0
.fill 5, i                  // 0,1,2,3,4 - i is the iteration counter
.fill 4, [$10,$20]          // repeats the pattern
.fill 256, round(127.5 + 127.5*sin(toRadians(i*360/256)))   // sine table
.fillword 5, i*$80
.lohifill $100, 40*i        // two tables; access via label.lo and label.hi
```

`.fill` is faster to assemble than the equivalent `.for` plus `.byte`.

## Encoding

```asm
.encoding "screencode_upper"
```

Values: `ascii`, `petscii_mixed`, `petscii_upper`, `screencode_mixed` (default),
`screencode_upper`. It affects `.text`, character literals, and `.import text`.

Pick `screencode_*` for bytes written straight into screen RAM, `petscii_*` for bytes passed to
`$FFD2`. See `petscii.md`.

## Script language

```asm
.var x = 27                 // mutable
.const DELAY = 7            // immutable
.label color = $d020        // an address label
.eval x = x + 1
.enum { ON, OFF }

.if (x > 10) { ... } else { ... }
.for (var i = 0; i < 10; i++) { ... }
.while (i < 10) { ... }

.function area(h, w) { .return h * w }
.struct Point { x, y }
.print "value: " + x        // last pass
.printnow "immediately"
.error "not good"
.errorif x > 10, "too big"
```

Lists and hashtables exist too (`List().add(...)`, `Hashtable()`).

Math is the Java library: `abs ceil floor round min max pow sqrt sin cos tan atan2 log exp
toRadians toDegrees random signum mod`, plus constants `PI` and `E`.

## Macros

```asm
.macro SetColor(color) {
        lda #color
        sta $d020
}

SetColor(RED)               // the leading ':' is optional since v4.0
```

Each call gets its own scope, so internal labels never collide between calls. Macros may
recurse - provide a termination condition.

## Pseudocommands

Macros that take addressing-mode arguments, separated by `:`.

```asm
.pseudocommand mov src:tar {
        lda src
        sta tar
}

mov #10 : $1000             // lda #10 / sta $1000
mov source,x : target,y
```

Arguments arrive as `CmdValue` objects with `getType()` and `getValue()`. Type constants:
`AT_ABSOLUTE`, `AT_ABSOLUTEX`, `AT_ABSOLUTEY`, `AT_IMMEDIATE`, `AT_INDIRECT`, `AT_IZEROPAGEX`,
`AT_IZEROPAGEY`, `AT_NONE`. Build new ones with `CmdArgument(type, value)`.

A leading `:` lets a pseudocommand share a name with a real mnemonic:
`:adc #$20 : $10 : $20` calls the pseudocommand, `adc #$10` calls the opcode.

## Segments

```asm
.segmentdef Code [start=$1000]
.segmentdef Data [start=$3000]

.segment Code
        ...
.segment Data
        ...

.file [name="out.prg", segments="Code,Data"]
```

Segments let different code claim the same address range, which is how bank-switched
cartridges and drive code are built. `.segmentout [segments="X"]` splices an intermediate
segment's bytes into the current block; `.disk [...] { ... }` writes a D64.

## Imports

```asm
#import "lib.asm"           // preferred over the deprecated .import source
#importonce                 // guard at the top of a library file
.import binary "music.bin"
.import c64 "music.prg"     // like binary, but skips the two load-address bytes
.import text "scroll.txt"
```

## Graphics conversion

```asm
.var logo = LoadPicture("logo.gif")
.fill $800, logo.getSinglecolorByte((i>>3)&$1f, (i&7) | (i>>8)<<3)

.var pic = LoadPicture("pic.gif", List().add($444444,$6c6c6c,$959595,$000000))
.fill $800, pic.getMulticolorByte(i>>7, i&$7f)
```

Picture values expose `width`, `height`, `getPixel(x,y)`, `getSinglecolorByte(x,y)`, and
`getMulticolorByte(x,y)`. For the byte functions `x` is a **byte index** (pixel/8) and `y` is a
pixel row. The colour list maps RGB values to bit patterns; without it a default mapping is
guessed from the image, which is rarely what you want.

`getMulticolorByte` ignores every second pixel, matching the C64's halved multicolour
resolution.

## Basic upstart

```asm
BasicUpstart2(start)        // emits the SYS line and sets the PC after it
start:  inc $d020
        jmp start
```

`BasicUpstart(start)` is the older variant that only emits the line; `BasicUpstart2` also
handles the memory block. Use `BasicUpstart2` unless you have a reason not to.

## Debugging and testing

```asm
.break                      // breakpoint at the current position
.break "if y<5"             // argument passed through to VICE
.watch $d018
.watch $d000,$d00f,"store"

.assert "2+2", 2+2, 4       // assert an expression
.asserterror "bad index", List().get(27)
```

Breakpoints and watches emit no bytes - they only populate the VICE symbol file or the
C64Debugger dump, so they need `-vicesymbols` or `-debugdump` to have any effect.

## Built-in colour constants

`BLACK WHITE RED CYAN PURPLE GREEN BLUE YELLOW ORANGE BROWN LIGHT_RED DARK_GREY GREY
LIGHT_GREEN LIGHT_BLUE LIGHT_GREY` (with `GRAY` spellings accepted), values 0-15.

## Directive index

`* .align .assert .asserterror .break .by .byte .const .cpu .define .disk .dw .dword .encoding
.enum .error .errorif .eval .file .filemodify .filenamespace .fill .fillword .for .function .if
.import .importonce .label .lohifill .macro .memblock .modify .namespace .pc .plugin .print
.printnow .pseudocommand .pseudopc .return .segment .segmentdef .segmentout .struct .te .text
.var .watch .while .wo .word .zp`

Preprocessor: `#define #undef #if #elif #else #endif #import #importif #importonce`
