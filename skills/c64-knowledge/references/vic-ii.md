# VIC-II Video Chip

Base `$D000`. The 47 registers mirror every `$40` up to `$D3FF`; always use the `$D0xx` form.

## Register map

| Address | Function |
|---|---|
| `$D000-$D00E` | Sprite 0-7 X/Y, alternating (`$D000` = sprite 0 X, `$D001` = sprite 0 Y, ...) |
| `$D010` | Sprite X coordinate bit 8, one bit per sprite |
| `$D011` | Control register 1 |
| `$D012` | Raster line, low 8 bits (read: current line, write: IRQ compare line) |
| `$D013/$D014` | Light pen X/Y |
| `$D015` | Sprite enable, one bit per sprite |
| `$D016` | Control register 2 |
| `$D017` | Sprite Y expand |
| `$D018` | Memory pointers |
| `$D019` | Interrupt status |
| `$D01A` | Interrupt enable |
| `$D01B` | Sprite-to-background priority (`1` = sprite behind background) |
| `$D01C` | Sprite multicolour enable |
| `$D01D` | Sprite X expand |
| `$D01E` | Sprite-sprite collision (cleared on read) |
| `$D01F` | Sprite-background collision (cleared on read) |
| `$D020` | Border colour |
| `$D021` | Background colour 0 |
| `$D022-$D024` | Background colours 1-3 (multicolour/ECM) |
| `$D025/$D026` | Sprite multicolour 0/1 (shared by all sprites) |
| `$D027-$D02E` | Sprite 0-7 colour |

Colour registers use only bits 0-3; bits 4-7 read back as `1`.

## `$D011` - control register 1 (default `$1B`)

| Bit | Name | Function |
|---|---|---|
| 7 | RST8 | Bit 8 of the raster compare line |
| 6 | ECM | Extended colour mode |
| 5 | BMM | Bitmap mode |
| 4 | DEN | Display enable - **`0` blanks the screen entirely** |
| 3 | RSEL | `1` = 25 rows, `0` = 24 rows |
| 2-0 | YSCROLL | Vertical smooth scroll, default `%011` |

## `$D016` - control register 2 (default `$C8`)

| Bit | Name | Function |
|---|---|---|
| 7-6 | - | Unused, read as `1` |
| 5 | RES | Reset - leave at `0` |
| 4 | MCM | Multicolour mode |
| 3 | CSEL | `1` = 40 columns, `0` = 38 columns |
| 2-0 | XSCROLL | Horizontal smooth scroll, default `%000` |

## Screen modes

| ECM | BMM | MCM | Mode |
|---|---|---|---|
| 0 | 0 | 0 | Standard text |
| 0 | 0 | 1 | Multicolour text |
| 0 | 1 | 0 | Standard bitmap, 320x200 |
| 0 | 1 | 1 | Multicolour bitmap, 160x200 |
| 1 | 0 | 0 | Extended background colour text (only 64 characters usable) |
| any other combination | | | Invalid - screen goes black |

In multicolour text mode, a character cell is multicolour only when bit 3 of its colour RAM
nibble is set; that leaves 8 selectable foreground colours (0-7).

## `$D018` - memory pointers (default `$15`)

| Bits | Function |
|---|---|
| 7-4 | Video matrix base = bits x 1024 within the current VIC bank |
| 3-1 | Character generator base = bits x 2048 within the current VIC bank |
| 0 | Unused |

Default `$15` = `%0001 0101` -> video matrix at `+$0400`, character base at `+$1000`.

In bitmap mode, bit 3 of `$D018` alone selects the bitmap: `0` = `+$0000`, `1` = `+$2000`.
The video matrix still supplies the colour pairs, one byte per 8x8 cell.

All these offsets are relative to the 16 KB VIC bank chosen through `$DD00` - see
`memory-map.md`.

## Sprites

- 24x21 pixels, 63 bytes of data, stored 3 bytes per row.
- Sprite pointers sit at **video matrix base + `$03F8`** - with the default screen that is
  `$07F8-$07FF`. Each pointer is `data address / 64` within the VIC bank, so sprite data must
  start on a 64-byte boundary.
- X coordinate is 9 bits: `$D000+2n` plus the sprite's bit in `$D010`.
- Visible screen area starts around X = 24, Y = 50; X = 0 and Y = 0 are off-screen to the
  upper left.
- Multicolour sprites use `$D025`/`$D026` for bit patterns `%01` and `%11`, and the sprite's
  own colour register for `%10`. Horizontal resolution halves to 12 pixels.

**Trap:** collision registers `$D01E`/`$D01F` clear themselves on read. Read them once and
keep the value; reading twice loses the second result.

## Raster interrupts

```asm
        sei
        lda #$7f
        sta $dc0d          // disable CIA 1 timer IRQs
        sta $dd0d          // disable CIA 2 timer IRQs
        lda $dc0d          // acknowledge whatever was pending
        lda $dd0d

        lda #<irq
        sta $0314
        lda #>irq
        sta $0315

        lda #$80
        sta $d012          // compare line
        lda $d011
        and #$7f           // RST8 = 0, so the line is below 256
        sta $d011

        lda #$01
        sta $d01a          // enable raster IRQ
        asl $d019          // acknowledge any pending VIC IRQ
        cli
```

`$D019` bits: 0 = raster, 1 = sprite-background collision, 2 = sprite-sprite collision,
3 = light pen, 7 = "some VIC IRQ is pending". `$D01A` enables the same sources.

**Trap:** acknowledging is mandatory and is done by writing a `1` back to the bit.
`asl $d019` is the idiom - it shifts bit 6 into bit 7 and writes the result, setting bit 0.
Without it the interrupt re-fires immediately and the machine locks up.

**Trap:** the raster compare is 9 bits. Writing `$D012` alone only sets the low 8; lines 256-311
also need RST8 in `$D011` bit 7.

**Trap:** a raster IRQ triggers at the *start* of the line, but the CPU takes a variable 7-8
cycles to react, and a badline steals up to 43 more. For effects that must change a register at
an exact pixel, use a double-IRQ stabiliser or a timed `nop` chain.

## Badlines

Every 8th raster line inside the display window, the VIC fetches the video matrix and halts the
CPU for 40-43 cycles. A badline occurs when the display is enabled, the raster line is in
`$30-$F7`, and `(rasterline & 7) == (YSCROLL in $D011 & 7)`.

Consequence: a raster line normally offers 63 usable cycles (PAL), but only about 20 on a
badline. Timing-critical code must either account for this or shift YSCROLL so badlines land
elsewhere.

## PAL versus NTSC

| | PAL | NTSC |
|---|---|---|
| `$02A6` | `1` | `0` |
| Raster lines per frame | 312 | 263 |
| Cycles per raster line | 63 | 65 |
| CPU clock | 985248 Hz | 1022727 Hz |
| Frame rate | ~50.12 Hz | ~59.83 Hz |
| Cycles per frame | 19656 | 17095 |
| Visible raster lines | 284 | 235 |

Anything that counts raster lines, times a scroll, or converts a musical pitch to a SID
frequency must branch on `$02A6`. Reading a hardcoded PAL table on an NTSC machine produces
music that is audibly sharp - about two thirds of a semitone - and effects that tear.

The C64 Ultimate can be configured either way; do not assume. Check at runtime:

```asm
        lda $02a6
        beq ntsc
        // PAL path
```
