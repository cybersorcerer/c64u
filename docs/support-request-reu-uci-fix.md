# Support request: when does the LOAD_REU / SAVE_REU fix reach C64 Ultimate firmware?

## Summary

`CTRL_CMD_LOAD_REU` (`$04 $08 <filename>`) never completes on a Commodore 64 Ultimate running
firmware 1.1.0. The Ultimate Command Interface stays busy, no documented control bit releases
it, and every later UCI command from the C64 hangs as well. Only a power cycle restores the
interface.

This is the bug already reported and fixed upstream as
[issue #740](https://github.com/GideonZ/1541ultimate/issues/740) /
[PR #741](https://github.com/GideonZ/1541ultimate/pull/741), where it was diagnosed on an
Ultimate 64 Elite with firmware 3.14d. This report adds the observation that it is present on
the C64 Ultimate as well, and asks one question: **which C64 Ultimate firmware version will
carry that fix, and when is it expected?**

## Environment

| | |
|---|---|
| Product | C64 Ultimate |
| Firmware version | 1.1.0 |
| FPGA version | 122 |
| Core version | 1.49 |
| DOS target `IDENTIFY` reply | `ULTIMATE-II DOS V1.2` |
| `RAM Expansion Unit` | Enabled, REU Size 16 MB, REU Preload Disabled |

## What was observed

A cartridge program sent the documented command `$04 $08` followed by the filename
`/USB0/TEMP/NOSUCH.REU`, then `PUSH_CMD`, and waited for the interface to leave the busy
state. It never did.

Register `$DF1C`, read over DMA from outside the machine:

| Point in time | `$DF1C` | Meaning |
|---|---|---|
| After `PUSH_CMD` | `$11` | busy, command pending |
| After 10, 20, 30 seconds | `$11` | unchanged |
| After writing `$04` (ABORT) | `$15` | abort pending, still busy |
| After writing `$02` (DATA_ACC) | `$15` | unchanged |
| After writing `$08` (CLR_ERR) | `$15` | unchanged |
| After a C64 reset | `$15` | unchanged |
| After `machine:reboot` over the REST API | `$15` | unchanged |

With the interface in that state, an unrelated command sent afterwards (a plain directory
listing through the DOS target) hung the C64 as well. A power cycle of the Ultimate was the
only way back.

This matches the recovery table in issue #740 exactly, including the `$11` → `$15` transition
and the fact that no control bit releases the busy state.

`CTRL_CMD_SAVE_REU` (`$09`) was **not** tested here, because the reporter of #740 notes that it
shares the same `case` block and the same `data_message.message + 4` argument, and because a
save on this machine would write a 16 MB image before anything could be learned from it.

## Why it matters here

We are building a cartridge wedge that reaches the Ultimate filesystem from the BASIC prompt
through the UCI - directory, load, save, mount, drive control and so on. The UCI is the whole
foundation of that work, so a command that can wedge it until the power is pulled is more
serious for us than a command that merely returns an error.

For context, the `DOS_CMD_COPY_FILE` report sent earlier came out of the same project; that one
was confirmed as known, with a fix planned for a coming release.

## Questions

1. **Which C64 Ultimate firmware version will include PR #741, and is there a rough date?**
   The fix was merged upstream on 2026-07-31, and the Ultimate-II / Ultimate 64 release notes
   for firmware 3.15 list "incorrect UCI reply lengths for REU and SoftIEC commands" and
   "problems that could cause commands to hang" among the fixes. The C64 Ultimate uses its own
   version numbering, and 1.1.0 clearly does not have it.

2. **Is there a newer, not yet published C64 Ultimate firmware build we could test?**
   We would be glad to run it and report back. Our testing is automated: the machine is driven
   over the REST API, commands are injected into the keyboard buffer, and results are read back
   out of screen memory and over DMA, so a regression run is repeatable and quick.

3. **Would a cross-check on an Ultimate II+L help?**
   We also have a C64C with an Ultimate II+L. Firmware 3.15 can be installed there, which would
   let us verify the fixed behaviour of `LOAD_REU` and `SAVE_REU` on that line and compare it
   against the C64 Ultimate. We are happy to run the same test set on both machines and share
   the results.

## A second observation, on the same firmware

`CTRL_CMD_U64_SAVEMEM` (`$04 $0F`) answers `00,OK` and writes no file. The documentation says
it saves the entire C64 RAM, and that the filename may be omitted, in which case
`/temp/c64_memory.bin` is used.

Tried on the C64 Ultimate with firmware 1.1.0:

| Argument | Status reply | File afterwards |
|---|---|---|
| `/USB0/TEMP/MEMDUMP.BIN` (absolute) | `00,OK` | none |
| `MEMDUMP.BIN` with the current directory set to `/USB0/TEMP` | `00,OK` | none |
| no filename at all | `00,OK` | no `/temp/c64_memory.bin` |

The directories were checked over FTP and through the Ultimate's own DOS target, and again a
minute later in case the write was deferred. The status is not left over from an earlier
command: sending `CHANGE_DIR` to a directory that does not exist first gives
`83,NO SUCH DIRECTORY`, and the very next `U64_SAVEMEM` still answers `00,OK`.

Unlike the REU command above this one leaves the interface idle and needs no power cycle - it
simply reports a success that did not happen. We could not find an existing report for it.

## What we can provide

- The exact byte sequence sent, and the register states after each step.
- A reproduction that needs no 6502 code at all: the command bytes can be pushed into `$DF1D`
  and `$DF1C` through `machine:writemem` over the REST API, as described in issue #740.
- Test results from both machines, in whatever form is most useful to you.
