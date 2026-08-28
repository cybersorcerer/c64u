# Commodore BASIC V2 Pitfalls

BASIC V2 is small, quirky, and unhelpful about errors. These are the traps that actually bite
when generating or uploading BASIC programs.

## How a program is stored

Program text starts at `$0801`. Each line is:

| Bytes | Meaning |
|---|---|
| 2 | Pointer to the next line, low/high. `$0000` marks the end of the program |
| 2 | Line number, low/high (0-63999) |
| n | Tokenised line text |
| 1 | `$00` terminator |

The byte at `$0800` must be `$00`. A minimal `10 SYS 2064`:

```
$0801: $0C $08     link to next line ($080C)
$0803: $0A $00     line number 10
$0805: $9E         SYS token
$0806: "2064"      as PETSCII digits
$080A: $00         end of line
$080B: $00 $00     end of program
```

Kick Assembler's `BasicUpstart2(label)` generates exactly this and computes the SYS address, so
never hand-assemble it.

**Trap:** the link pointers are absolute addresses. A program loaded to a different address
must be relinked - which is what LOAD without `,1` does, and what LOAD with `,1` does not.

## Tokenisation

- Keywords are stored as single tokens (`PRINT` = `$99`, `SYS` = `$9E`, `GOTO` = `$89`), so a
  program is shorter than its listing.
- Tokenisation happens **only outside quotes**. Text inside a string is stored verbatim.
- Abbreviations produce identical tokens: `?` is `PRINT`, `gO` is `GOTO`. Listing them shows
  the full keyword.
- The 80-character limit is a *screen editor* limit, not a file format limit: two 40-column
  screen lines. A program generated as bytes may contain longer lines and will run, but cannot
  be edited on screen without truncation.

**Trap:** typing a line longer than 80 characters silently loses the rest.

## Variable names

- **Only the first two characters are significant.** `COUNT` and `COST` are the same variable.
  This produces bugs that look like random corruption.
- **A variable name may not contain a keyword.** `TOTAL` contains `TO`, `SCORE` contains `OR`,
  `VALUE` contains `VAL` - all `?SYNTAX ERROR`. Safe practice: use names of one or two
  characters.
- Suffixes: none = float, `%` = 16-bit integer, `$` = string.
- **Integer variables are slower than floats** in loops, because BASIC converts them to float
  for every operation. Use `%` only to save memory in arrays.

## Strings and quotes

- There is no escape character. A `"` cannot appear inside a string literal - use
  `CHR$(34)`.
- Strings are limited to 255 characters.
- String garbage collection can freeze the machine for **seconds** when many strings have been
  created and discarded. It is not a crash.
- `A$ = A$ + "X"` in a loop is the classic way to trigger it.

## Memory

| | |
|---|---|
| Program + variables | `$0801-$9FFF`, 38911 bytes free at startup |
| Lower BASIC limit | `$0283/$0284` (pointer) |
| Upper BASIC limit | `$0037/$0038` (pointer) |
| Safe RAM for machine code | `$C000-$CFFF` (4 KB) and `$033C-$03FB` (192 bytes) |

To reserve space at the top for machine code, lower `$0037/$0038` **before** the program runs;
doing it afterwards corrupts variables.

**Trap:** machine code loaded into the BASIC area and then followed by `NEW` is the standard
way to clear the BASIC pointers - but `NEW` after loading ML at `$0801` also destroys the ML if
BASIC's own start pointer still points there.

## Common runtime errors

| Message | Real cause |
|---|---|
| `?SYNTAX ERROR` | Often a keyword hidden inside a variable name |
| `?UNDEF'D STATEMENT` | `GOTO`/`GOSUB` to a line number that does not exist |
| `?OUT OF MEMORY` | Frequently a `FOR` without `NEXT` or a `GOSUB` without `RETURN` filling the stack |
| `?REDIM'D ARRAY` | An array was used before `DIM`, which auto-dimensioned it to 10 |
| `?ILLEGAL QUANTITY` | An argument out of range, e.g. `POKE` with a value above 255 |
| `?DIVISION BY ZERO` | Also produced by underflow in some expressions |

`FOR` and `GOSUB` state lives on the 6502 stack at `$0100-$01FF`. Jumping out of a loop with
`GOTO` leaves its entry behind; do it often enough in one run and you get `?OUT OF MEMORY`
with plenty of RAM free.

## Extending BASIC from machine code

Hooking `IGONE` (`$0308`) is the usual way to add commands. Three things go
wrong there, none of which announce themselves:

**Operators are tokenised too.** By the time a line reaches the dispatcher, `/`
is `$AD` and the up arrow `$AE` - not `$2F` and `$5E`. `+ - * ^` are likewise
`$AA $AB $AC $AE`. Comparing against character codes never matches. `@` is not
an operator and does survive unchanged, which makes `@` commands work while `/`
silently falls through.

**CHRGET's flags are part of the contract.** The statement executor reads the
zero and carry flags CHRGET left: zero for end of statement, carry clear for a
digit. A `cmp` in a hook destroys them, and the result is not a crash but
misbehaviour - `PRINT` executes as `PRINT#` and reports `?FILE NOT OPEN`, `RUN`
reports `?UNDEF'D STATEMENT`. Wrap comparisons in `php`/`plp`.

**Some ROM routines are not subroutines.** `CLR` at `$A659` ends in
`PLA/TAY/PLA`; `jsr $A659` does not return to the caller. Set `VARTAB`,
`ARYTAB` and `STREND` directly instead. To start a freshly loaded program, point
`TXTPTR` at a one-byte tokenised `RUN` line (`$8A, $00`) and let BASIC execute
it, rather than jumping into the RUN routine and having to reproduce the flag
state it expects.

**After loading a BASIC program**, set `VARTAB`/`ARYTAB`/`STREND` to the end
address and call the relink routine at `$A533`, or `LIST` and `RUN` both
misbehave.

## Useful POKEs

| Address | Effect |
|---|---|
| `53280` (`$D020`) | Border colour |
| `53281` (`$D021`) | Background colour |
| `646` (`$0286`) | Current text colour |
| `650` (`$028A`) | `128` = repeat all keys |
| `657` (`$0291`) | `128` = disable Shift+Commodore case switching |
| `788/789` (`$0314`) | IRQ vector |
| `56334` (`$DC0E`) | `%11111110` disables the keyboard-scan interrupt |
| `1` (`$0001`) | Memory banking - see `memory-map.md` |

## Driving BASIC from c64u

`c64u machine sendkey` types into the 10-byte keyboard buffer at `$0277`. Practical rules:

- Send one line at a time and let the machine consume it; the buffer holds 10 characters.
- Terminate lines with a carriage return (PETSCII `13`).
- The screen editor is in uppercase/graphics mode after reset, so send uppercase.
- For anything longer than a few lines, assemble or tokenise a PRG and use
  `c64u runners run-prg-upload` instead. Typing is slow and lossy.
