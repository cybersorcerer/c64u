# Memory Map, Banking, and KERNAL

## Overall layout

| Range | Contents |
|---|---|
| `$0000-$00FF` | Zero page - CPU port, KERNAL and BASIC working storage |
| `$0100-$01FF` | Processor stack (also used by BASIC for GOSUB/FOR state) |
| `$0200-$03FF` | BASIC input buffer, file tables, vectors, cassette buffer |
| `$0400-$07E7` | Default screen RAM (1000 bytes) |
| `$07F8-$07FF` | Default sprite pointers |
| `$0800` | Must be `$00` - BASIC program text starts at `$0801` |
| `$0801-$9FFF` | BASIC program + variables (38911 bytes free at startup) |
| `$A000-$BFFF` | BASIC ROM, or RAM |
| `$C000-$CFFF` | Free RAM, 4 KB, never used by ROM - good home for small ML |
| `$D000-$DFFF` | I/O, or character ROM, or RAM |
| `$E000-$FFFF` | KERNAL ROM, or RAM |

## The 6510 CPU port

| Address | Meaning |
|---|---|
| `$0000` | Data direction register, default `$2F` (bits 0-5 output, 6-7 input) |
| `$0001` | Port register, default `$37` |

`$0001` bit meanings:

| Bit | Name | Function |
|---|---|---|
| 0 | LORAM | `1` = BASIC ROM at `$A000`, `0` = RAM |
| 1 | HIRAM | `1` = KERNAL ROM at `$E000`, `0` = RAM |
| 2 | CHAREN | `1` = I/O at `$D000`, `0` = character ROM at `$D000` |
| 3 | - | Cassette write line |
| 4 | - | Cassette switch sense (input) |
| 5 | - | Cassette motor control |

Useful configurations:

| Value | `$A000` | `$D000` | `$E000` |
|---|---|---|---|
| `$37` | BASIC ROM | I/O | KERNAL ROM (default) |
| `$36` | RAM | I/O | KERNAL ROM |
| `$35` | RAM | I/O | RAM |
| `$34` | RAM | RAM | RAM |
| `$33` | BASIC ROM | Character ROM | KERNAL ROM |

**Trap:** `$35`, `$34`, and `$33` all remove the KERNAL from `$E000`, which removes the IRQ
handler that `$FFFE` points at. Disable interrupts with `sei` and install your own handler
at `$FFFE`/`$FFFF` **before** writing `$01`, or the next raster/timer interrupt jumps into
whatever bytes happen to sit there. See `examples/raster-irq.asm`.

**Trap:** the CPU sees character ROM at `$D000` only when CHAREN is `0`. The VIC-II sees it
somewhere else entirely - see the VIC bank rules below.

## Zero page worth knowing

| Address | Meaning |
|---|---|
| `$002B/$2C` | Start of BASIC program text (`$0801`) |
| `$002D/$2E` | Start of BASIC variables |
| `$002F/$30` | Start of arrays |
| `$0031/$32` | End of arrays |
| `$0033/$34` | Bottom of string heap (grows down) |
| `$0037/$38` | Highest RAM address available to BASIC (`$A000`) |
| `$0039/$3A` | Current BASIC line number |
| `$0061-$0066` | Floating point accumulator 1 |
| `$0090` | KERNAL I/O status byte (`ST`) |
| `$0091` | Stop key column scan |
| `$0093` | LOAD/VERIFY flag |
| `$0099` | Current input device |
| `$009A` | Current output device |
| `$00A0-$00A2` | Jiffy clock, 24-bit, increments 60x/second (`TI`) |
| `$00C5` | Key currently pressed, `$40` = none |
| `$00C6` | Number of characters waiting in the keyboard buffer |
| `$00CC` | Cursor blink enable, `0` = blinking on |
| `$00D1/$D2` | Pointer to the current screen line |
| `$00D3` | Cursor column |
| `$00D4` | Quote mode flag, non-zero while inside quotes |
| `$00D6` | Cursor row |
| `$00F3/$F4` | Pointer to the current colour RAM line |

**Trap:** the jiffy clock at `$00A0` is maintained by the KERNAL IRQ. Bank out the KERNAL or
take over the IRQ vector and `TI`/`TI$` stop advancing. The same is true of keyboard scanning
and cursor blink.

## Page 2 and 3

| Address | Meaning |
|---|---|
| `$0200-$0258` | BASIC input buffer, 88 bytes |
| `$0277-$0280` | Keyboard buffer, **10 bytes only** |
| `$0286` | Current text colour |
| `$0288` | Screen memory page, high byte, default `$04` |
| `$028A` | Key repeat flag (`$80` = repeat all keys) |
| `$0291` | Shift/Commodore case-toggle disable (`$80` = disabled) |
| `$02A6` | **Video standard: `1` = PAL, `0` = NTSC** |
| `$0300/$01` | Error message vector |
| `$0302/$03` | BASIC main loop vector |
| `$0304/$05` | BASIC tokenizer (crunch) vector |
| `$0306/$07` | BASIC list vector |
| `$0308/$09` | BASIC execute vector |
| `$0314/$15` | **IRQ vector**, default `$EA31` |
| `$0316/$17` | BRK vector, default `$FE66` |
| `$0318/$19` | NMI vector, default `$FE47` |
| `$0326/$27` | BSOUT vector, default `$F1CA` |
| `$032A/$2B` | GETIN vector |
| `$033C-$03FB` | Cassette buffer, 192 bytes - a classic home for small ML routines |

**`$0314` versus `$FFFE`:** with the KERNAL banked in, the hardware vector `$FFFE` points into
ROM at a handler that saves registers and jumps through `$0314`. Hooking `$0314` is simpler and
keeps KERNAL housekeeping alive; hooking `$FFFE` is faster and mandatory once the KERNAL is
banked out, but then you must save and restore A/X/Y yourself.

## I/O area, `$D000-$DFFF` (CHAREN = 1)

| Range | Chip |
|---|---|
| `$D000-$D3FF` | VIC-II, 47 registers mirrored every `$40` |
| `$D400-$D7FF` | SID, 29 registers mirrored every `$20` |
| `$D800-$DBE7` | Colour RAM, **lower nibble only** |
| `$DC00-$DCFF` | CIA 1 - keyboard, joysticks, timers, IRQ source |
| `$DD00-$DDFF` | CIA 2 - serial bus, user port, VIC bank select, NMI source |
| `$DE00-$DEFF` | I/O area 1, cartridge expansion |
| `$DF00-$DFFF` | I/O area 2, cartridge expansion - **the REU lives here** |

**Trap:** colour RAM reads return the low nibble in bits 0-3 and undefined values in bits 4-7.
Always mask with `and #$0f` after reading it.

## VIC bank selection

The VIC-II addresses only 16 KB at a time. CIA 2 port A selects which one, and the bits are
**inverted**:

| `$DD00` bits 1-0 | Bank | VIC sees |
|---|---|---|
| `%11` | 0 | `$0000-$3FFF` (default) |
| `%10` | 1 | `$4000-$7FFF` |
| `%01` | 2 | `$8000-$BFFF` |
| `%00` | 3 | `$C000-$FFFF` |

Change it without disturbing the other bits:

```asm
        lda $dd02
        ora #%00000011      // make bits 0-1 outputs
        sta $dd02
        lda $dd00
        and #%11111100
        ora #%00000010      // bank 1: $4000-$7FFF
        sta $dd00
```

**Trap:** the VIC-II sees the character ROM mirrored at `$1000-$1FFF` and `$9000-$9FFF`
regardless of `$01`. That means banks 0 and 2 cannot hold graphics data in those 4 KB windows,
and banks 1 and 3 have no character ROM at all - a custom charset is mandatory there.

## KERNAL entry points

| Address | Name | Function |
|---|---|---|
| `$FFD2` | CHROUT | Print the PETSCII character in A to the current output |
| `$FFCF` | CHRIN | Read one character from the current input |
| `$FFE4` | GETIN | Read a character from the keyboard buffer, `0` if empty |
| `$FFC6` | CHKIN | Set input to a logical file |
| `$FFC9` | CHKOUT | Set output to a logical file |
| `$FFCC` | CLRCHN | Restore default input/output |
| `$FFC0` | OPEN | Open a logical file |
| `$FFC3` | CLOSE | Close a logical file |
| `$FFBA` | SETLFS | Set logical file, device, secondary address |
| `$FFBD` | SETNAM | Set filename (A = length, X/Y = address) |
| `$FFD5` | LOAD | Load from device |
| `$FFD8` | SAVE | Save to device |
| `$FFE1` | STOP | Test the RUN/STOP key, Z set if pressed |
| `$FFE7` | CLALL | Close all files |
| `$FFF0` | PLOT | Read (C set) or set (C clear) cursor row/column in X/Y |
| `$FF81` | SCINIT | Initialise screen editor and VIC |
| `$FF8A` | RESTOR | Restore the default `$0314-$0333` vectors |
| `$FF9F` | SCNKEY | Scan the keyboard |
| `$FFEA` | UDTIM | Advance the jiffy clock |

`$E544` clears the screen, `$E566` homes the cursor. These are BASIC/KERNAL internals, not
part of the stable jump table - prefer `$FFD2` with PETSCII `147` for a clear.

**Trap:** every routine above needs the KERNAL banked in (`$01` bit 1 set) and, for anything
touching I/O, `$01` bit 2 set as well.
