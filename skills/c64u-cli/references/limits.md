# Limits and Surprising Behaviour

Everything here was checked against a physical C64 Ultimate, not inferred.

## A DMA write persists only if nothing else drives that address

`read-mem` and `write-mem` go through DMA and **do** reach the I/O area at `$D000-$DFFF`, both
reading and writing. What differs is whether the value survives.

| Target | Written value | Why |
|---|---|---|
| `$D020` border colour | sticks | Nothing rewrites it; the VIC just displays what is latched |
| `$0400` screen RAM | sticks | Plain RAM |
| `$DC00` CIA 1 port A | gone within a frame | The KERNAL keyboard scan rewrites it 60 times a second |

So "the write did nothing" almost always means something else owns that register. Before
concluding the CLI failed, ask what code is running on the machine and what it writes.

The corollary for reads: a value read back from a register reflects the hardware, not
necessarily what you wrote.

## Colour registers read back with the upper bits set

VIC colour registers are 4 bits wide and the unused bits read as `1`:

```sh
c64u machine write-mem d020 02
c64u machine read-mem  d020 --length 1     # D020: F2, not 02
```

Mask with `& 0x0F` before comparing. The same applies to colour RAM at `$D800-$DBE7`.

## Keyboard injection drives BASIC, not games

`sendkey` writes PETSCII into the keyboard buffer at `$0277` and updates the count at `$00C6`.
That is the path the KERNAL and BASIC read from, so it works for the BASIC prompt, `INPUT`,
`GET`, and typing commands.

Games and demos usually poll the keyboard matrix through CIA 1 (`$DC00`/`$DC01`) instead. Those
registers are driven by the hardware scan of the physical key lines: an injected value is
overwritten before any game reads it. A "press SPACE to start" title screen therefore cannot be
driven this way. This is a property of how the matrix works, not a missing feature.

The buffer is **10 bytes**. Longer strings are chunked with `--delay` between chunks, and the
machine has to consume each chunk. If the running program does not drain the buffer, the rest
is lost.

## Function key codes are interleaved

`\f1` through `\f8` map to `$85 $89 $86 $8A $87 $8B $88 $8C` - not sequentially - because F2,
F4, F6 and F8 are the shifted variants of F1, F3, F5 and F7. To inject a code the escapes do not
cover, write the buffer directly:

```sh
c64u machine write-mem 0277 86      # F3
c64u machine write-mem 00c6 01      # one character waiting
```

## Running a program proves the upload, not the result

`run-prg-upload` reports success once the machine has been told to start. Whether the program
did the right thing is a separate question, and `read-mem` is how you answer it. See
`workflows.md`.

A program that ends in an endless loop keeps the machine busy; `c64u machine reset` is the way
out.

## U64-only commands

These need an Ultimate 64, not a 1541 Ultimate II:

- `c64u streams` - video, audio, debug
- `c64u machine poweroff`
- `c64u machine debug-reg`, `debug-reg-set` (register `$D7FF`)

## Configuration changes are volatile by default

`c64u config set` takes effect at once but is lost on power-off. Follow it with
`c64u config save-to-flash` to persist. `reset-to-default` restores factory settings and is not
undoable from the CLI - export first:

```sh
c64u config export config-backup.json
```

## JSON key casing is not uniform

Most commands emit snake_case (`core_version`, `bus_id`). `c64u fs ls --json` emits PascalCase
(`Name`, `Size`, `IsDir`, `Type`). Scripts touching both need to handle both.

## The video window forwards keystrokes

While `c64u streams listen video` has focus, keys are forwarded to the C64 keyboard buffer by
the same DMA mechanism as `sendkey` - so the same BASIC-only limitation applies there.
