# Ultimate Command Interface

The UCI lets a program **running on the C64** talk to the Ultimate's management application
through cartridge-port registers - open files on the Ultimate's filesystem, mount disk images,
use the network stack, load and save REU contents, freeze, reboot.

It is the counterpart to the REST API: REST drives the device from outside over the network,
the UCI drives it from inside the running C64 program. For the REST side see the `c64u-cli`
skill.

## Enabling it

The registers are mapped in only when the feature is switched on:

```sh
c64u config set "C64 and Cartridge Settings" "Command Interface" Enabled
```

With it off, the identification register does not read `$C9` and every command is ignored.
Check for that before doing anything else.

## Registers

Five bytes at `$DF1B-$DF1F`, in cartridge I/O space:

| Address | Read | Write |
|---|---|---|
| `$DF1B` | SoftwareIEC bus ID | - |
| `$DF1C` | Status register | Control register |
| `$DF1D` | Identification, `$C9` | Command data |
| `$DF1E` | Response data | - |
| `$DF1F` | Status data | - |

**Trap:** this block overlaps the REU's register mirror. A program using both the UCI and an REU
must keep them apart - see `reu.md`.

**Trap:** the identification byte reads `$C9` normally, but `$49` while the interface is
asserting an interrupt (bit 7 cleared). Test for `$C9` only when interrupts are not in use.

### Control register (write to `$DF1C`)

| Bit | Name | Meaning |
|---|---|---|
| 7 | DMA | Enter DMA mode immediately when the command is pushed |
| 6 | TRIGGER | Enter DMA mode once `$FF00` is written |
| 5 | IRQ | Raise an interrupt when the command completes (firmware 3.15+) |
| 4 | - | reserved |
| 3 | CLR_ERR | Clear the state-error flag |
| 2 | ABORT | Ask the Ultimate to abort and return to idle |
| 1 | DATA_ACC | All data has been read and accepted |
| 0 | PUSH_CMD | Hand the buffered command over |

### Status register (read from `$DF1C`)

| Bit | Name | Meaning |
|---|---|---|
| 7 | DATA_AV | A response byte is waiting at `$DF1E` |
| 6 | STAT_AV | A status byte is waiting at `$DF1F` |
| 5-4 | STATE | `00` idle, `01` command busy, `10` data last, `11` data more |
| 3 | ERROR | A command was pushed while not idle |
| 2 | ABORT_P | An abort is still pending |
| 1 | DATA_ACC | The acceptance was registered |
| 0 | CMD_BUSY | A command is pending in the command memory |

## Protocol

1. **Idle.** Write the command byte by byte to `$DF1D`. The first byte is the target.
2. **Push.** Write `PUSH_CMD` to `$DF1C`. The state moves to *command busy*.
3. **Wait.** Poll the status register until STATE leaves `01`.
4. **Read.** While DATA_AV is set, read `$DF1E`. While STAT_AV is set, read `$DF1F`.
5. **Accept.** Write `DATA_ACC` to `$DF1C`. On *data last* this returns to idle; on *data more*
   it goes back to *command busy* for the next block.

**Trap:** DATA_ACC is not optional. Skipping it leaves the state machine out of idle, and the
next command fails with the ERROR bit set. `CLR_ERR` clears that flag.

**Trap:** writing DATA_ACC also flushes both reply queues. Read everything you want before
acknowledging.

The command length is implicit - the queue contents define it, so strings need no terminator.

### Queue sizes

| Queue | Size |
|---|---|
| Command | 896 bytes (`$380`) |
| Response data | 896 bytes (`$380`) |
| Status | 256 bytes (`$100`) |

These are the per-command transfer limits. Bulk reads come back in 512-byte chunks, each
acknowledged separately.

## Targets

The first command byte selects the target:

| Target | Module |
|---|---|
| `$01`, `$02` | Ultimate DOS - two independent instances, so two files or directories can be open at once |
| `$03` | Network |
| `$04` | Control |
| `$05` | Software IEC |
| `$06` | HTTP (firmware 3.15+) |

Every target implements `IDENTIFY` as command `$01` and answers with a version string, which
makes it the safe probe for what a given firmware supports.

## Ultimate DOS (`$01` / `$02`)

| Command | Code | Format |
|---|---|---|
| IDENTIFY | `$01` | `$01 $01` |
| OPEN_FILE | `$02` | `$01 $02 <attrib> <filename>` |
| CLOSE_FILE | `$03` | `$01 $03` |
| READ_DATA | `$04` | `$01 $04 <len_lo> <len_hi>` |
| WRITE_DATA | `$05` | `$01 $05 <dummy> <dummy> <data...>` |
| FILE_SEEK | `$06` | `$01 $06 <pos 32 bit, LSB first>` |
| FILE_INFO | `$07` | `$01 $07` |
| FILE_STAT | `$08` | |
| DELETE_FILE | `$09` | |
| RENAME_FILE | `$0A` | |
| COPY_FILE | `$0B` | |
| CHANGE_DIR | `$11` | |
| GET_PATH | `$12` | |
| OPEN_DIR | `$13` | |
| READ_DIR | `$14` | |
| COPY_UI_PATH | `$15` | |
| CREATE_DIR | `$16` | |
| COPY_HOME_PATH | `$17` | |
| LOAD_REU | `$21` | |
| SAVE_REU | `$22` | `$01 $22 <addr 32 bit> <len 32 bit>` |
| MOUNT_DISK | `$23` | |
| UMOUNT_DISK | `$24` | |
| SWAP_DISK | `$25` | |
| GET_TIME | `$26` | |
| SET_TIME | `$27` | |
| ECHO | `$F0` | |

File open attribute flags, added together:

| Flag | Value | Meaning |
|---|---|---|
| FA_READ | `$01` | Open for reading |
| FA_WRITE | `$02` | Open for writing, keeping existing contents |
| FA_CREATE_NEW | `$04` | Create and truncate to zero |
| FA_CREATE_ALWAYS | `$08` | Overwrite an existing file |

`$0E` is the usual "write, replacing whatever is there" combination.

READ_DATA takes a 16-bit total length, maximum 65535, delivered in 512-byte chunks that must
each be acknowledged. FILE_INFO returns a packed struct: `uint32` size, `uint16` date,
`uint16` time, 3-byte extension, attribute byte, then the filename.

Status strings follow the Commodore convention, `00,OK` on success, and errors such as
`84,NO FILE TO CLOSE` or `85,NO FILE OPEN`.

## Control (`$04`)

| Command | Code | Format |
|---|---|---|
| IDENTIFY | `$01` | `$04 $01` |
| FINISH_CAPTURE | `$03` | `$04 $03` |
| FREEZE | `$05` | `$04 $05` |
| REBOOT | `$06` | `$04 $06` |
| LOAD_REU / SAVE_REU | `$08` / `$09` | `$04 $08 <filename>` |
| U64_SAVEMEM | `$0F` | `$04 $0F <filename>` |
| DECODE_TRACK | `$11` | `$04 $11 <trk> <max_sec> <gcr_addr> <bin_addr> <gcr_len>` |
| EASYFLASH_ERASE | `$20` | `$04 $20 $00 <bank> <baseaddr>` |
| GET_DRVINFO | `$29` | `$04 $29 <effective_addr_flag>` |
| ENABLE/DISABLE DRIVE A | `$30` / `$31` | `$04 $30` |
| ENABLE/DISABLE DRIVE B | `$32` / `$33` | `$04 $32` |
| GET_DRIVE_A/B_POWER | `$34` / `$35` | `$04 $34` |
| GET_MP3_RAMDISKINFO | `$40` | `$04 $40` |

`GET_HWINFO` (`$28`) is deprecated.

## Network (`$03`)

| Command | Code |
|---|---|
| IDENTIFY | `$01` |
| GET_INTERFACE_COUNT | `$02` |
| GET_NETADDR | `$04` |
| GET_IPADDR | `$05` |
| SET_IPADDR | `$06` |
| OPEN_TCP | `$07` |
| OPEN_UDP | `$08` |
| CLOSE_SOCKET | `$09` |
| READ_SOCKET | `$10` |
| WRITE_SOCKET | `$11` |

A full sockets interface, so a C64 program can reach the network without any C64-side TCP/IP
stack.

## Software IEC (`$05`)

Bypasses the IEC layer to load, save and access directories directly.

| Format | Purpose |
|---|---|
| `$05 $10 <sec_addr> <verify> <addr_lo> <addr_hi> <name>` | Load |
| `$05 $11 <sec_addr> <verify>` | Continue load |
| `$05 $12 <verify> <sec_addr> <start_lo/hi> <end_lo/hi> <name>` | Save |
| `$05 $13 <sec_addr> $00 <name>` | Open |
| `$05 $14 <sec_addr>` | Get more data |
| `$05 $15 <sec_addr>` | Close |
| `$05 $16 <sec_addr> $00 <data...>` | Write |
| `$05 $20 <index> <ident> ':' <path>` | Set partition path |
| `$05 $22 <channel> <iec_name>` | Convert IEC name |

Status codes are numeric rather than Commodore strings: `$00` OK, `$01` file not found,
`$02` save error, `$03` no input channel, `$04` unknown command, `$05` IEC module not loaded,
`$06` invalid parameters, `$07` invalid name, `$08` invalid partition, `$09` invalid directory.

File types in directory replies: `$00` DEL, `$01` PRG, `$02` SEQ, `$03` USR, `$04` REL,
`$05` folder.

## HTTP (`$06`, firmware 3.15+)

Builds a request incrementally, then performs the exchange. Headers are managed with
`HEADER_CREATE` (`$11`), `HEADER_FREE` (`$12`), `HEADER_ADD` (`$13`), `HEADER_QUERY` (`$14`),
`HEADER_LIST` (`$15`). Bodies are assembled as a JSON tree with `BODY_CREATE` (`$21`),
typed adders `BODY_ADD_INT` (`$23`), `BODY_ADD_BOOL` (`$24`), `BODY_ADD_STRING` (`$25`),
`BODY_ADD_OBJECT` (`$26`), `BODY_ADD_ARRAY` (`$27`), plus `BODY_UP` (`$28`) to leave the
current level, and `BODY_QUERY` (`$2A`), `BODY_MOVE` (`$2B`), `BODY_ADD_BINARY` (`$2C`),
`BODY_REMOVE` (`$29`), `BODY_CLEAR` (`$2E`). The exchange itself is `DO_EXCHANGE_OBJ` (`$31`)
or `DO_EXCHANGE_RAW` (`$32`); `FREE_ALL` (`$10`) releases every handle.

## Verified

`examples/uci-identify.asm` performs the full round trip and was run on a C64 Ultimate
(core 1.49, firmware 1.1.0). The DOS target answered `ULTIMATE-II DOS V1.2` on the data queue
and `00,OK` on the status queue.
