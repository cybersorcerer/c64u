# PETSCII, Screen Codes, and Colour RAM

## Two different encodings

The C64 uses **two** character encodings and mixing them up is the single most common text bug.

| Encoding | Where it is used |
|---|---|
| **PETSCII** | `$FFD2` (CHROUT), BASIC `PRINT`, `CHR$()`, `ASC()`, the keyboard buffer, files |
| **Screen code** | Bytes written directly into screen RAM at `$0400`, and character set indices |

`lda #'A'` in an assembler emits PETSCII `65`. Storing that at `$0400` shows a spade suit, not
an `A`, because screen code `65` is a graphics character. `A` as a screen code is `1`.

Kick Assembler handles this with `.encoding`:

```asm
.encoding "screencode_upper"
        .text "HELLO"          // bytes 8,5,12,12,15 - ready for screen RAM
.encoding "petscii_upper"
        .text "HELLO"          // bytes 72,69,76,76,79 - ready for $FFD2
```

Supported names: `ascii`, `petscii_mixed`, `petscii_upper`, `screencode_mixed`,
`screencode_upper`. Default is `screencode_mixed`.

## Screen code table (uppercase/graphics set)

| Code | Character |
|---|---|
| `0` | `@` |
| `1-26` | `A-Z` |
| `27` | `[` |
| `28` | `£` |
| `29` | `]` |
| `30` | up arrow |
| `31` | left arrow |
| `32` | space |
| `33-47` | `!"#$%&'()*+,-./` |
| `48-57` | `0-9` |
| `58-63` | `:;<=>?` |
| `64-95` | Graphics characters (`64` = horizontal bar, `81` = filled circle, `95` = filled corner) |
| `96` | Shifted space (solid, but transparent to collision) |
| `97-127` | More graphics characters |
| `128-255` | Same glyphs as `0-127`, reversed |

In the mixed (lower/upper) set, `1-26` are lowercase `a-z` and `65-90` are uppercase `A-Z`.

## PETSCII control codes

| Code | Effect | Code | Effect |
|---|---|---|---|
| `5` | White | `144` | Black |
| `13` | Carriage return | `145` | Cursor up |
| `14` | Switch to lower/uppercase set | `146` | Reverse off |
| `17` | Cursor down | `147` | Clear screen |
| `18` | Reverse on | `148` | Insert |
| `19` | Home | `149` | Brown |
| `20` | Delete | `150` | Light red |
| `28` | Red | `151` | Dark grey |
| `29` | Cursor right | `152` | Grey |
| `30` | Green | `153` | Light green |
| `31` | Blue | `154` | Light blue |
| `142` | Switch to upper/graphics set | `155` | Light grey |
| | | `156` | Purple |
| | | `157` | Cursor left |
| | | `158` | Yellow |
| | | `159` | Cyan |
| `160` | Shifted space | | |

## Function keys

Their codes are **interleaved**, not sequential - the shifted keys occupy the second half of
the range:

| Key | Code | Key | Code |
|---|---|---|---|
| F1 | `133` | F2 | `137` |
| F3 | `134` | F4 | `138` |
| F5 | `135` | F6 | `139` |
| F7 | `136` | F8 | `140` |

F2, F4, F6 and F8 are Shift+F1, Shift+F3, Shift+F5 and Shift+F7 on the real keyboard, which is
where the ordering comes from. Assigning `133-140` to F1-F8 in order is a common bug: it happens
to be right for F1 and F8 and wrong for everything between.

Verified on hardware by reading `$FFE4` results back through `c64u machine read-mem`.

## Converting PETSCII to a screen code

| PETSCII range | Operation |
|---|---|
| `0-31` | `+128` |
| `32-63` | unchanged |
| `64-95` | `-64` |
| `96-127` | `-32` |
| `128-159` | `+64` |
| `160-191` | `-64` |
| `192-223` | `-128` |
| `224-254` | `-128` |
| `255` | `94` |

## Colours

| Value | Colour | Value | Colour |
|---|---|---|---|
| 0 | Black | 8 | Orange |
| 1 | White | 9 | Brown |
| 2 | Red | 10 | Light red |
| 3 | Cyan | 11 | Dark grey |
| 4 | Purple | 12 | Grey |
| 5 | Green | 13 | Light green |
| 6 | Blue | 14 | Light blue |
| 7 | Yellow | 15 | Light grey |

Kick Assembler predefines these as `BLACK`, `WHITE`, `RED`, `CYAN`, `PURPLE`, `GREEN`, `BLUE`,
`YELLOW`, `ORANGE`, `BROWN`, `LIGHT_RED`, `DARK_GRAY`/`DARK_GREY`, `GRAY`/`GREY`,
`LIGHT_GREEN`, `LIGHT_BLUE`, `LIGHT_GRAY`/`LIGHT_GREY`.

## Screen and colour RAM

| | Address | Notes |
|---|---|---|
| Screen RAM | `$0400-$07E7` | 40 x 25 = 1000 bytes, one screen code per cell |
| Sprite pointers | `$07F8-$07FF` | The 8 bytes after screen RAM |
| Colour RAM | `$D800-$DBE7` | **Fixed at `$D800`**, low nibble only |

Cell at column `x`, row `y` is at `$0400 + y*40 + x`, and its colour at `$D800 + y*40 + x`.

**Trap:** colour RAM does not move with the screen. Relocating screen RAM via `$D018` moves the
character cells but colour always comes from `$D800`.

**Trap:** colour RAM is 4 bits wide. Reading it returns garbage in bits 4-7; mask with
`and #$0f`.

**Trap:** a freshly written screen cell keeps whatever colour was already in colour RAM. Text
that "does not appear" is usually text drawn in the background colour.

## Quote mode

While the BASIC screen editor is inside a quoted string, control characters are not executed
but shown as reversed glyphs and stored literally. `$00D4` holds the quote-mode flag. This is
how `PRINT "{clr}"` works: the clear-screen code sits inside the string as a byte and executes
only when printed.

Consequence for `c64u machine sendkey`: cursor and colour controls typed inside quotes end up
as data rather than actions, which is usually what you want when building a `PRINT` line.

## Keyboard buffer

`$0277-$0280`, **10 bytes**, with the count in `$00C6`. Anything that stuffs keystrokes -
including `c64u machine sendkey` - is limited by this. Long input must be sent in chunks that
the machine consumes between sends.
