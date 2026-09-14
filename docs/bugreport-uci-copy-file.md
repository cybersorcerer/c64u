# Bug report: `DOS_CMD_COPY_FILE` ($0B) never copies a file

## Status

Reported to Commodore. They answered that the bug is known to the developers and will be
fixed in an upcoming release.

Retested since on a second machine: a C64C with an **Ultimate II+L on firmware 3.15a**
(FPGA 125, released 2026-09-11, the newest at the time of writing) behaves exactly as
described below. With absolute names the reply is `FILE EXISTS` and no copy appears; with
bare names after `CHANGE_DIR` it is `PATH DOESN'T EXIST`. `RENAME_FILE` in the same
directory, over the same code path, answers `00,OK` and renames the file. So the fix is not
in 3.15a.

## Summary

The Ultimate Command Interface command `DOS_CMD_COPY_FILE` (`$0B`) does not copy anything on
firmware 1.1.0. It always answers with a filesystem error, and no destination file is ever
created. `DOS_CMD_RENAME_FILE` (`$0A`), which the documentation describes with an identical
command format, works correctly on the same machine, over the same code path, with the same
byte stream — only the command byte differs.

## Environment

| | |
|---|---|
| Product | C64 Ultimate |
| Firmware version | 1.1.0 |
| FPGA version | 122 |
| Core version | 1.49 |
| DOS target `IDENTIFY` reply | `ULTIMATE-II DOS V1.2` |
| KERNAL | JiffyDOS 6.01 and the stock KERNAL, same result on both |
| Filesystem under test | USB stick, `/USB0` |

## Expected behaviour

Per chapter 12.2, *Ultimate DOS Target*:

> `DOS_CMD_COPY_FILE (0x0b)`
> Command format: `$01 $0b <source> $00 <destination>`
> The "Copy File" command copies the file specified by `<source>` to the file specified by
> `<destination>`. This command does not return any data. The status channel will either read
> `00,OK` or it will contain the appropriate filesystem error message.

## Actual behaviour

No copy is ever made. The status channel reports an error that depends only on **the first**
of the two names:

- if the first name exists, `FILE EXISTS`
- if the first name does not exist, `FILE DOESN'T EXIST`
- with bare names and a current directory set through `CHANGE_DIR`, `PATH DOESN'T EXIST`

The second name makes no difference to the reply. Since a copy needs an existing source and a
non-existent destination, there is no combination of arguments that succeeds.

## Steps to reproduce

A minimal 6502 program that writes the command bytes directly to the interface registers, so
neither BASIC nor any wedge is involved:

```asm
        lda #$01            // target: Ultimate DOS
        sta $df1d
        lda #$0b            // DOS_CMD_COPY_FILE
        sta $df1d

        ldx #<source        // "/USB0/TEMP/RNTEST/SEVEN.PRG", exists
        ldy #>source
        jsr sendName        // sends the characters, not the terminator

        lda #$00            // the documented separator
        sta $df1d

        ldx #<dest          // "/USB0/TEMP/RNTEST/SIX.PRG", does not exist
        ldy #>dest
        jsr sendName

        lda #$01            // PUSH_CMD
        sta $df1c
!wait:  lda $df1c           // wait for the command to leave the busy state
        and #$30
        cmp #$10
        beq !wait-
!status:
        lda $df1c           // print the status channel
        and #$40
        beq !done+
        lda $df1f
        jsr $ffd2
        jmp !status-
!done:
        lda #$02            // DATA_ACC
        sta $df1c
```

Result: `FILE EXISTS`. `/USB0/TEMP/RNTEST/SIX.PRG` is not created.

Replacing only the command byte `$0b` with `$0a` (`DOS_CMD_RENAME_FILE`), leaving every other
byte untouched, answers `00,OK` and renames the file as documented.

## What was ruled out

Every variation below was run on the hardware, and the resulting directory was checked over
FTP rather than trusted from the status message:

| Argument order | Names | Separator | Reply | File created |
|---|---|---|---|---|
| source, destination | absolute | `$00` | `FILE EXISTS` | no |
| source, destination | bare, after `CHANGE_DIR` | `$00` | `PATH DOESN'T EXIST` | no |
| destination, source | absolute | `$00` | `FILE DOESN'T EXIST` | no |
| destination, source | bare, after `CHANGE_DIR` | `$00` | `FILE DOESN'T EXIST` | no |
| source, destination | absolute, destination also null terminated | `$00` | `FILE EXISTS` | no |
| source, destination | absolute | `$20` (blank) | `PATH DOESN'T EXIST` | no |
| source, existing directory as destination | absolute | `$00` | `FILE EXISTS` | no |

Further checks:

- The outgoing bytes were logged to memory while being written to `$DF1D` and read back over
  DMA afterwards. They match the documented format exactly:
  `<01><0b>/USB0/TEMP/RNTEST/EIGHT.PRG<00>/USB0/TEMP/RNTEST/SEVE…`
- `CHANGE_DIR` to the directory in question answers `00,OK` immediately before the copy, so
  the path the `PATH DOESN'T EXIST` reply refers to demonstrably exists.
- `RENAME_FILE`, `DELETE_FILE`, `CREATE_DIR`, `CHANGE_DIR`, `OPEN_FILE`, `READ_DATA` and
  `WRITE_DATA` all behave as documented on the same machine and the same directory.
- Reproduced both from a program and by typing the command at the BASIC prompt through a
  cartridge wedge. Repeated with the stock KERNAL in place of JiffyDOS: the copy reported
  `PATH DOESN'T EXIST` while the rename in the same run, in the same directory, reported
  `00,OK` and renamed the file.

## Requested outcome

Either the command performs the copy as chapter 12.2 describes, or the documentation stops
describing it. The current state is the worst of the two: the command is specified in full,
answers when it is sent, and silently does nothing — so anyone building against the interface
spends their time looking for a mistake in their own code. A line in the chapter saying that
`COPY_FILE` is not implemented in this firmware would have saved that entirely.

## Side observation

A failed `COPY_FILE` appears to leave state behind. A command sent immediately afterwards
returns a truncated status message, while the same sequence with `RENAME_FILE` in place of the
copy does not. Waiting for the interface to leave the busy state after `DATA_ACC` works around
it, but the asymmetry may be a useful hint.
