---
name: c64u-cli
description: Driving a Commodore C64 Ultimate from the command line with the c64u CLI - uploading and running programs, reading and writing memory over DMA, injecting keystrokes, mounting disk images, managing device configuration, and live video/audio/debug streams. Use when automating or scripting against real C64 Ultimate hardware, when verifying that a program did what it should, or when a c64u command does not behave as expected.
---

# c64u CLI

`c64u` controls a Commodore C64 Ultimate over its REST API. This skill covers how to drive it
and, more importantly, where its edges are.

For C64 hardware questions - register addresses, screen codes, SID frequencies - use the
`c64-knowledge` skill instead. This one is about the tool.

## How to use this skill

| Question | Read |
|---|---|
| Which command does X, what arguments, which JSON comes back | `references/commands.md` |
| How do I assemble, run and then prove it worked | `references/workflows.md` |
| Why did my write not stick, why can't I control a game, what is U64-only | `references/limits.md` |

## Connecting

Resolution order, highest first:

1. Flags: `--host 192.168.1.100 --port 80`
2. Environment: `C64U_HOST`, `C64U_PORT`, `C64U_DEVICE`
3. `-D, --device NAME`, naming an entry in the config file
4. Config file: `~/.config/c64u/config.toml`
5. Port defaults to 80

**There is no default host.** With nothing configured the command stops and
says so - the device is never this machine, so guessing `localhost` would only
produce a confusing timeout.

Several machines can live in the config file under names, and `--device` picks
one per command:

```toml
default = "living-room"

[devices.living-room]
host = "192.168.1.100"

[devices.attic]
host = "c64u-attic.local"
```

```sh
c64u -D attic info
```

`c64u cli-config show` lists them and marks the one in use. An undefined name is
an error that lists the defined ones; it never falls back to another machine.

```sh
c64u info            # device identity - the cheapest reachability check
c64u about           # API version reported by the device
```

If `c64u info` fails, nothing else will. Check that before debugging anything downstream.

## The four things worth knowing up front

1. **Verify with `read-mem`, do not assume.** Running a program tells you the upload succeeded,
   not that the program worked. Read the memory it should have changed:

   ```sh
   c64u runners run-prg-upload program.prg
   c64u machine read-mem 07f8 --length 8      # did the sprite pointer land?
   ```

2. **`--json` for anything a script consumes.** Every command takes it. Human output is aligned
   and coloured and will change; JSON is the contract. `--raw` on `read-mem` gives the bytes
   themselves for piping.

3. **A DMA write only persists if nothing else drives that address.** Writing `$D020` sticks.
   Writing `$DC00` does not, because the KERNAL keyboard scan overwrites it every frame. See
   `references/limits.md` - this is the single most common source of "the CLI did nothing".

4. **`sendkey` fills a 10-byte buffer.** It drives BASIC and the KERNAL, not games. Long input
   is chunked automatically, but the machine has to consume it between chunks.

5. **Disk images are usually the wrong tool.** SoftIEC serves a directory of the Ultimate
   filesystem as an IEC device, so the C64 can `LOAD` from it at the BASIC prompt without any
   `.d64` being built or mounted. It ships disabled — check `c64u drives softiec status` first.
   See `references/workflows.md`.

## Leaving the machine clean

`c64u machine reset` returns to a BASIC prompt. Anything that ran before it is gone, including
programs sitting in an endless loop. Use it between experiments rather than reasoning about what
the previous program left behind.
