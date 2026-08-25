# Graphics Pipeline

Getting pixels from an image file or a formula onto a real C64, end to end.

## Choose the format first

| Format | Resolution | Colours | Memory | Use for |
|---|---|---|---|---|
| Text + custom charset | 40x25 cells | 1 per cell + background | 2 KB charset + 1 KB screen | UI, scrolling, anything that repeats |
| Hires bitmap | 320x200 | 2 per 8x8 cell | 8 KB bitmap + 1 KB colour map | Line art, logos, plots |
| Multicolour bitmap | 160x200 | 4 per 4x8 cell | 8 KB + 1 KB + 1 KB colour RAM | Photographic images |
| Sprites | 24x21 each | 1 or 3 + transparent | 64 bytes each | Moving objects, overlays |

Multicolour halves the horizontal resolution: pixels become 2 physical dots wide. A picture
converted to multicolour without accounting for that comes out stretched.

## Memory layout

Everything the VIC reads must be inside its current 16 KB bank - see `memory-map.md` for the
`$DD00` bank bits.

| Piece | Where | Selected by |
|---|---|---|
| Bitmap | 8 KB aligned in the bank | `$D018` bit 3: `0` = +`$0000`, `1` = +`$2000` |
| Video matrix / colour map | 1 KB aligned | `$D018` bits 7-4, in units of 1024 |
| Charset | 2 KB aligned | `$D018` bits 3-1, in units of 2048 |
| Colour RAM | always `$D800` | not selectable |

**Trap:** `$D018` bit 0 is unused and reads back as `1`. Writing `$18` and reading `$19` is
correct, not a failed write.

**Trap:** the VIC sees character ROM mirrored at `+$1000` and `+$9000`, so banks 0 and 2 cannot
hold graphics data in that 4 KB window.

## Bitmap addressing

The bitmap is stored in character-cell order, not scanline order. Eight consecutive bytes are
the eight rows of one 8x8 cell; the next eight are the cell to its right.

```
address = base + (y >> 3) * 320 + (x & $F8) + (y & 7)
bit     = $80 >> (x & 7)
```

`(x >> 3) * 8` collapses to `x & $F8`, which saves the shifts. A 25-entry table of
`base + row * 320` removes the multiply - `.lohifill 25, base + 320*i` builds it at assemble
time. See `examples/hires-bitmap.asm`.

In hires, the colour map byte for a cell holds the foreground in the high nibble and the
background in the low nibble. In multicolour, bit pattern `%01` comes from the high nibble,
`%10` from the low nibble, `%11` from colour RAM at `$D800`, and `%00` from `$D021`.

## Converting an image at assemble time

Kick Assembler reads GIF and JPG directly, so no external converter is needed.

```asm
*= $2000 "Charset"
.var logo = LoadPicture("logo.gif")
.fill $800, logo.getSinglecolorByte((i >> 3) & $1f, (i & 7) | (i >> 8) << 3)
```

Always pass an explicit colour table. Without one, Kick Assembler guesses the mapping from the
colours present in the image, and the guess changes when the image does:

```asm
.var pic = LoadPicture("pic.gif", List().add($444444, $6c6c6c, $959595, $000000))
.fill $800, pic.getMulticolorByte(i >> 7, i & $7f)
```

The list entries are RGB values mapped to bit patterns `%00`, `%01`, `%10`, `%11` in order. The
image must use exactly those RGB values - an anti-aliased export will not match, so save from
the editor with dithering off and a fixed palette.

Picture values also expose `width`, `height` and `getPixel(x, y)` for building a format the
built-ins do not cover.

`getSinglecolorByte(x, y)` takes `x` as a **byte index** (pixel divided by 8) and `y` in pixels.
`getMulticolorByte` ignores every second pixel to match the halved resolution.

## Sprites

Sprite data must start on a 64-byte boundary, and the pointer at video matrix + `$03F8` holds
`address / 64` within the bank. 63 bytes of data, three per row, 21 rows, one byte padding.

```asm
.var spr = LoadPicture("sprite.gif")
*= $0c00 "Sprite"
.fill 63, spr.getSinglecolorByte(i % 3, i / 3)
.byte 0
```

See `examples/sprite-setup.asm` for enabling and positioning.

## Custom charsets

Copy the ROM charset to RAM first if you only want to redefine some characters - see
`examples/charset-copy.asm`. For a fully custom set, generate it from an image as above, or
build it from `.byte` rows directly.

Remember the charset must be 2 KB aligned and inside the VIC bank.

## Onto the hardware

```sh
java -jar KickAss.jar picture.asm -o picture.prg
c64u runners run-prg-upload picture.prg
```

Verify without looking at the screen - useful in scripts and when the result is subtly wrong:

```sh
c64u machine read-mem d011 --length 1      # bit 5 set = bitmap mode active
c64u machine read-mem d018 --length 1      # memory pointers, bit 0 reads as 1
c64u machine read-mem 2f04 --length 8      # a byte you can compute in advance
```

Computing one expected byte and comparing it is the fastest way to tell "the picture is wrong"
from "the picture is not there at all". For a visual check while iterating:

```sh
c64u streams listen video
```

## Common failures

| Symptom | Cause |
|---|---|
| Black screen | Invalid mode combination in `$D011`/`$D016`, or DEN (bit 4 of `$D011`) cleared |
| Garbage instead of the image | Bitmap not 8 KB aligned, or outside the current VIC bank |
| Image visible but wrong colours | Colour map not filled, or the `LoadPicture` colour list does not match the image palette |
| Image stretched horizontally | Multicolour data displayed in hires mode, or converted without halving the width |
| Only the top eight rows are right | Address arithmetic treating the bitmap as scanline-ordered |
| Characters unchanged after a charset copy | `$D018` still pointing at the ROM charset, or the copy ran with I/O banked in |
