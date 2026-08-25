# Agent instructions

Guidance for coding agents working in this repository. Harness-neutral - it applies whether you
run through Claude Code, opencode, pi, hermes, or anything else that reads this file.

## What this project is

`c64u` is a Go command-line interface for the Commodore C64 Ultimate, talking to the device over
its REST API. The Go sources live in `tools/c64u/`.

## Build and test

All builds go through make - do not invoke `go build` directly.

```sh
make -C tools/c64u build      # build the binary into tools/c64u/build/
make -C tools/c64u test       # run the test suite
make -C tools/c64u fmt        # format
make -C tools/c64u lint       # lint

make -C skills check          # assemble every example in every skill
make -C skills dist           # package the skills for release
```

## Knowledge skills

`skills/` holds knowledge packs for working on and with the C64. Read
`skills/README.md` for the layout and how to wire them into your own harness.

Currently available:

| Skill | Read it when |
|---|---|
| `skills/c64-knowledge` | Writing, reviewing or debugging C64 code in BASIC or 6502 assembly; picking register addresses or bit values; counting cycles; converting text to screen codes or images to bitmaps; computing SID frequencies |
| `skills/c64u-cli` | Driving the device from the command line; verifying that a program did what it should; a `c64u` command not behaving as expected |

Each skill's `SKILL.md` carries a routing table. Read the one reference file that covers your
question rather than the whole directory - they are written to be independent.

## A real device may be reachable

When a C64 Ultimate is on the network, prefer checking behaviour against it over reasoning about
it:

```sh
tools/c64u/build/c64u info --json
tools/c64u/build/c64u machine read-mem 0400 --length 16
tools/c64u/build/c64u runners run-prg-upload program.prg
```

`c64u machine reset` returns the machine to a clean state afterwards. Verify claims about
register values and encodings this way before writing them into a reference file.

## Conventions

- All user-facing output is US English. Comments may be in German; messages, help text and
  errors may not.
- Match the surrounding style rather than importing your own.
- Change only what the task requires. Point out unrelated problems instead of fixing them
  silently.
