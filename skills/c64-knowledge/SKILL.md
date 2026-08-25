---
name: c64-knowledge
description: Commodore 64 hardware and toolchain reference - memory map and banking, VIC-II, CIA keyboard/joystick/timers, SID, PETSCII and screen codes, 6502 opcodes and cycle counts, graphics and bitmap formats, disk image layout, BASIC pitfalls, Kick Assembler syntax, and REU/RAM Expansion programming. Use when writing, reviewing, or debugging C64 code in BASIC or 6502 assembly, when picking register addresses or bit values, when counting cycles for raster timing, when converting text to screen codes or images to bitmaps, or when computing SID frequencies.
---

# C64 Knowledge

Grounding for C64 programming and for driving real hardware through the `c64u` CLI.

## How to use this skill

Read **one** reference file - the one that covers the register or subsystem at hand.
Do not read them all; each is a standalone quickref.

| Question | Read |
|---|---|
| Zero page, `$01` banking, KERNAL entry points, I/O layout, VIC bank select | `references/memory-map.md` |
| Screen modes, sprites, raster IRQs, `$D011`/`$D018` bits, PAL vs NTSC timing | `references/vic-ii.md` |
| Keyboard matrix, joysticks, CIA timers, interrupt masks, serial bus | `references/cia.md` |
| Voice registers, waveforms, ADSR, filter, frequency formula | `references/sid.md` |
| Screen codes vs PETSCII, control codes, colour RAM, text output | `references/petscii.md` |
| Opcodes, addressing modes, cycle counts, page-crossing penalties | `references/opcodes.md` |
| Bitmap addressing, image conversion, charsets, sprite data pipelines | `references/graphics-pipeline.md` |
| PRG load address, D64/D71/D81 layout, BAM, directory entries | `references/disk-formats.md` |
| Tokenizer, line format, quote mode, variable-name traps, memory limits | `references/basic-pitfalls.md` |
| Directives, macros, pseudocommands, segments, `BasicUpstart2`, CLI options | `references/kickassembler.md` |
| REC registers `$DF00-$DF0A`, STASH/FETCH/SWAP/VERIFY, detection, DMA traps | `references/reu.md` |

`references/opcodes.md` is generated from the disassembler's own table in `tools/c64u`;
regenerate it with `make -C skills/c64-knowledge opcodes` rather than editing it.

For driving real hardware with the `c64u` CLI, use the `c64u-cli` skill instead.

## Working code

`examples/` holds assembled-and-runnable Kick Assembler sources.

**Do not read `examples/` to answer a question.** The reference files carry the
register tables and the rules; the examples exist to be built and run. Open one only when:

- the user asks for working code or a starting point,
- you need to modify or debug that specific program,
- a reference file explicitly points at it and the register table alone did not settle the question.

Preferring `build then run` over `read` keeps them out of context entirely.

| File | Shows |
|---|---|
| `examples/hello-border.asm` | Smallest useful program: upstart line, border flash |
| `examples/raster-irq.asm` | Raster interrupt under `$01 = $35`, hardware vector at `$FFFE` |
| `examples/sprite-setup.asm` | Sprite pointer, position, colour, X MSB |
| `examples/sid-note.asm` | Triangle note with best-practice ADSR, PAL/NTSC frequency pick |
| `examples/charset-copy.asm` | Copy character ROM to RAM, repoint `$D018` |
| `examples/hires-bitmap.asm` | 320x200 bitmap mode, colour map, plotted sine curve |
| `examples/reu-detect.asm` | Probe for a RAM Expansion Unit without triggering a DMA |
| `examples/reu-screen-stash.asm` | Save and restore screen RAM through STASH/FETCH |

## Toolchain

Assemble with Kick Assembler (Java):

```sh
java -jar KickAss.jar program.asm -o program.prg
```

Run on the connected C64 Ultimate:

```sh
c64u runners run-prg-upload program.prg    # upload and start
c64u machine read-mem 0400 --length 16     # verify what the program changed
c64u machine reset                         # back to a clean BASIC
```

The `c64u-cli` skill covers the CLI itself - all commands, verified workflows, and the
limits that catch people out.

## Rules that prevent most C64 bugs

1. **Check the video standard before any timing or pitch maths.** `$02A6` holds `1` for
   PAL, `0` for NTSC. SID frequency values and raster line counts differ between them.
   See `references/vic-ii.md` and `references/sid.md`.
2. **Screen RAM stores screen codes, not PETSCII.** `$FFD2` takes PETSCII; a direct
   `sta $0400` takes a screen code. They are different encodings.
   See `references/petscii.md`.
3. **Banking out ROM means banking out the IRQ handler.** Any write to `$01` that removes
   the KERNAL must be wrapped in `sei` / `cli`, with the hardware vector at `$FFFE` set
   up first. See `references/memory-map.md`.
4. **Acknowledge VIC interrupts.** A raster IRQ that does not clear `$D019` fires forever.
5. **Derive REU command bytes from the bit fields, not from copied constants.** Published
   listings disagree on the unused bits. See `references/reu.md`.

## Sources

Written from the *Commodore 64 Programmer's Reference Guide*, *Mapping the Commodore 64*
(Sheldon Leemon), the Kick Assembler manual (Mads Nielsen), and the REU article at
retro-programming.de. No text was copied from those sources.
