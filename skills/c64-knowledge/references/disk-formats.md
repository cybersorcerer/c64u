# Disk Images and File Formats

## PRG - the load address is the first two bytes

A `.prg` file starts with a little-endian load address, then the raw bytes:

```
$01 $08  <program bytes...>      -> loads to $0801, the BASIC start
```

Those two bytes are **not** part of the program. Two consequences:

- `LOAD"NAME",8` ignores the address and loads to the BASIC start, relinking as it goes.
  `LOAD"NAME",8,1` honours it. Machine code almost always needs `,1`.
- A raw binary without the header is a `.bin`, not a `.prg`. Kick Assembler emits a prg by
  default and a bin with `-binfile`.

## D64 - 1541 disk image

| Variant | Size | Tracks |
|---|---|---|
| Standard | 174848 bytes | 35 |
| Extended | 196608 bytes | 40 |

Sectors per track vary by zone, because outer tracks are physically longer:

| Tracks | Sectors each |
|---|---|
| 1-17 | 21 |
| 18-24 | 19 |
| 25-30 | 18 |
| 31-35 (and 36-40) | 17 |

Every sector is 256 bytes. A 35-track image holds 683 sectors, of which **664 are usable** -
track 18 is reserved for the BAM and directory.

The byte offset of a sector is the sum of the sectors on all preceding tracks, times 256. There
is no shortcut formula; walk the table.

### Sector chaining

The first two bytes of every data sector point at the next one:

| Bytes | Meaning |
|---|---|
| `0` | Next track, or `$00` when this is the last sector |
| `1` | Next sector - or, when the track byte is `$00`, the number of bytes used in this sector |
| `2-255` | 254 bytes of payload |

### BAM - track 18, sector 0

| Bytes | Contents |
|---|---|
| `$00-$01` | Track and sector of the first directory sector - normally 18, 1 |
| `$02` | DOS version, `$41` ("A") |
| `$04-$8F` | Free-sector map, 4 bytes per track: a count followed by a 24-bit bitmap |
| `$90-$9F` | Disk name, 16 characters in PETSCII, padded with `$A0` |
| `$A0-$A1` | `$A0` |
| `$A2-$A3` | Disk ID, two characters |
| `$A5-$A6` | DOS type, `"2A"` |

### Directory - track 18, sectors 1 onward

Each sector holds **8 entries of 32 bytes**, giving 144 entries over 18 sectors. Only the first
entry of a sector carries the link to the next directory sector; the other seven start at their
file type byte.

| Bytes | Field |
|---|---|
| `$00-$01` | Next directory track/sector - first entry only, `$00 $FF` marks the last |
| `$02` | File type |
| `$03-$04` | Track and sector of the first data block |
| `$05-$14` | Filename, 16 characters, padded with `$A0` |
| `$15-$16` | REL side sector track/sector |
| `$17` | REL record length |
| `$1E-$1F` | Size in blocks, low/high |

File type byte:

| Value | Type |
|---|---|
| `$00` | DEL - scratched |
| `$81` | SEQ |
| `$82` | PRG |
| `$83` | USR |
| `$84` | REL |

Bit 7 set means the file was closed properly; a PRG therefore normally reads `$82`. A type byte
with bit 7 clear shows as a splat file (`*PRG`) in the directory - the file was never closed and
its block chain may be broken. Bit 6 marks the file locked (`<`).

### Interleave

Consecutive blocks of a file are not stored adjacently. The 1541 leaves a gap - conventionally
10 sectors - so the drive has time to process one block before the next passes the head. Writing
a file with interleave 1 makes it load several times slower.

## D71, D81, DNP

| Format | Drive | Size | Layout |
|---|---|---|---|
| D71 | 1571 | 349696 bytes | 70 tracks - a D64 doubled onto two sides, BAM extended into track 53 |
| D81 | 1581 | 819200 bytes | 80 tracks, 40 sectors each, uniform; BAM on track 40 |
| DNP | CMD native partition | variable | Track/sector geometry chosen at creation, sized in tracks |

## Creating and using images with c64u

```sh
c64u files create-d64 mydisk.d64 --name "MY DISK"      # empty image
c64u files create-d71 mydisk.d71 --name "MY DISK"
c64u files create-d81 mydisk.d81 --name "MY DISK"
c64u files create-dnp mydisk.dnp --tracks 40 --name "MY DISK"

c64u files pack-d64 ./programs mydisk.d64 --name "MY DISK" --id 01
c64u files info mydisk.d64

c64u drives mount-upload a mydisk.d64
```

Then from the C64 side:

```basic
LOAD"$",8      : REM directory
LIST
LOAD"*",8,1    : REM first program, honouring its load address
RUN
```

For a single program, `c64u runners run-prg-upload` skips the disk entirely and is faster.

## Filenames

Disk filenames are PETSCII, up to 16 characters, padded with `$A0` rather than spaces. They may
contain characters that are awkward from a shell - including `*` and `?`, which the drive treats
as wildcards when loading. `LOAD"*",8,1` means "the first file", not a file literally named `*`.
