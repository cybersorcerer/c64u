# Command Reference

Every command accepts the global flags below. `<>` is required, `[]` optional.

## Global flags

| Flag | Effect |
|---|---|
| `--host string` | Device hostname or IP (env `C64U_HOST`) |
| `-D, --device string` | Name of a device defined in the config file (env `C64U_DEVICE`) |
| `--port int` | HTTP port, default 80 (env `C64U_PORT`) |
| `--password string` | Network password, when the device has one set (env `C64U_PASSWORD`, or `password` in the config file). Sent as `X-Password`; a 403 without it means the device wants one |
| `--json` | JSON output - use this for anything a script parses |
| `--verbose` | Show HTTP requests and responses |
| `-d, --debug` | Debug log to `~/.local/share/c64u/c64u.log` |
| `--no-color` | Disable coloured output |

## Device

```
c64u info                          # product, hostname, unique id, core/firmware/fpga versions
c64u about                         # API version reported by the device
c64u version                       # version of the CLI itself
```

```json
// c64u info --json
{ "core_version": "1.49", "firmware_version": "1.1.0", "fpga_version": "122",
  "hostname": "C64U", "product": "C64 Ultimate", "unique_id": "C4B420" }
```

## Machine control and memory

```
c64u machine reset                             # back to a BASIC prompt
c64u machine reboot
c64u machine pause | resume
c64u machine menu-button                       # as if the Menu button was pressed
c64u machine poweroff                          # U64 only

c64u machine read-mem <address> [--length N | --to ADDR] [-o FILE] [--raw]
c64u machine read-mem <address> [--length N | --to ADDR] --disasm   # 6502, illegal opcodes marked *
c64u machine write-mem <address> <hexdata>
c64u machine write-mem-file <address> <file>
c64u machine sendkey <string> [--delay ms]

c64u machine input [event...] [--show]         # keys, joysticks, mouse; C64U 1.2.0+ / U64 3.15+
c64u machine menu-screen                       # the firmware menu as text, C64U 1.2.0+ / 3.15+

c64u machine heap                              # firmware heap: free, min_ever_free, total (fw 1.2.1)
c64u machine debug-reg                         # read $D7FF, U64 only
c64u machine debug-reg-set <value>             # write $D7FF, U64 only
```

**`input`** is not a faster `sendkey`, it is a different mechanism. `sendkey` writes PETSCII into
the KERNAL keyboard buffer at `$0277`, so only programs that read that buffer see it - BASIC and
the KERNAL. `input` injects at the key matrix and the joystick ports, which is what games, the
firmware menu and the machine code monitor read. Each argument is one event and one call is one
batch, applied atomically:

```sh
c64u machine input tap:commodore+o             # C= and O together, as one keystroke
c64u machine input press:joy2:up+fire          # held until released
c64u machine input press:left_shift tap:a release:left_shift
c64u machine input move:300,-40 tap:mouse:left # mouse on port 1 (firmware 1.2.1)
c64u machine input wheel:-2                     # two detents toward the user
c64u machine input release-all
```

A batch holds 1 to 64 events and has to serialise to under 4096 bytes and at most 1024 JSON
values (about 330 mouse path steps; on 1.2.1 the keys count too - two paths of 337 steps in all
pass, 338 are refused); the CLI checks both before sending; `restore` must stand
alone and can only be tapped. The state that results is printed after every batch, as with
`--show`. Verified on a C64 Ultimate on firmware 1.2.1: `tap:h tap:i` types `HI` at the READY
prompt, and `press:joy2:up+fire` turns `$DC00` from `$7F` into `$6E` until `release-all`.
Two failure modes are worth telling apart, and the CLI does: **404** means the firmware predates
the endpoint (1.2.0 on the C64 Ultimate, 3.15 elsewhere), **501** means the product has no
Ultimate 64 input hardware - a cartridge in a real C64 never will, because the keyboard is the
C64's.

**Mouse** (firmware 1.2.1) is a 1351 on control port 1: `press:`/`release:`/`tap:mouse:` with up
to three of `left`, `right`, `middle`; `move:X,Y` (positive is right and down) and `wheel:V[,H]`
(-64..64 detents). A move within -127..127 is one report; larger ones, up to 32512 per axis, go
out as a path of up to 256 steps 20 ms apart. The device queues 1024 mouse reports and answers
429 when a batch does not fit - the CLI shows "The device refused the input" with the room left;
`pending` in the printed state counts what is still queued. `release-all`, a reset and a reboot
detach the mouse (`attached: false`). Verified on 1.2.1: `move:300,-40 tap:mouse:left` leaves 5
reports pending (3 path steps, press, release); five `move:32512,0` calls in a row, with the
queue draining one report per 20 ms meanwhile, reached 1000 pending and the fifth was refused
with room for 32.

**`menu-screen`** returns the menu as a 40x25 character matrix in ASCII plus a colour matrix.
Open the menu with `menu-button` first; with no menu on screen the device answers 404. Together
with `input` it drives and reads the menu without the video stream.

**`read-mem --disasm`** shows the range as 6502 assembly, one instruction per line, with illegal
opcodes marked `*` and I/O registers named in a comment. An instruction that starts inside the
range is shown whole (up to two bytes are read past it, never past `$FFFF`). It decodes from the
given address, so start on an instruction. `--json` gives `address`, `bytes`, `mnemonic`,
`operand`, `comment`, `illegal` per instruction; `--raw` and `-o` are refused with it. Checked on
1.2.1: `read-mem e5cd --length 16 --disasm` shows the KERNAL's `LDA $C6 / STA $CC / STA $0292`.

**Addresses** may be written `0400`, `$0400` or `0x0400`.

**`read-mem`** defaults to 256 bytes. `--to` is inclusive: `--to 07e7` includes `$07E7`.
Without `-o`/`--raw` it prints a hex dump. `--raw` writes the bytes to stdout and prints nothing
else, so a pipe stays clean. Both may be combined, which behaves like `tee`.

```json
// c64u machine read-mem 0400 --length 4 --json
{ "address": "$0400", "data": "41414141", "length": 4 }
```

```sh
c64u machine read-mem 0400 --to 07e7 -o screen.bin     # screen RAM to a file
c64u machine read-mem 0400 --length 1000 --raw | xxd   # pipe the bytes
```

**`write-mem`** takes an uppercase or lowercase hex string, two characters per byte:

```sh
c64u machine write-mem 0400 08050c0c0f      # "HELLO" in screen codes at the top left
```

**`sendkey`** converts a string to PETSCII and injects it into the keyboard buffer at `$0277`.
Escapes: `\n` Return, `\f1`-`\f8` function keys, `\clr`, `\del`, `\stop`, `\home`, `\cup`,
`\cdn`, `\cleft`, `\cright`. Strings longer than 10 characters are sent in chunks with
`--delay` milliseconds between them (default 100).

## Running programs

Every runner exists twice: without `-upload` the file must already be on the device's
filesystem, with `-upload` a local file is sent first.

```
c64u runners run-prg   <file>            # load and run
c64u runners load-prg  <file>            # load without running
c64u runners run-crt   <file>            # start a cartridge image
c64u runners sidplay   <file> [--song N]
c64u runners modplay   <file>

c64u runners run-prg-upload   <local-file>
c64u runners load-prg-upload  <local-file>
c64u runners run-crt-upload   <local-file>
c64u runners sidplay-upload   <local-file> [--song N]
c64u runners modplay-upload   <local-file>
```

## Filesystem (over FTP)

```
c64u fs ls [path]
c64u fs cat <path>
c64u fs cp <source> <destination>
c64u fs mv <source> <destination>
c64u fs rm <path>
c64u fs mkdir <path>
c64u fs upload   <local-path> <remote-path>
c64u fs download <remote-path> <local-path>
```

```json
// c64u fs ls / --json
[ { "Name": "SD", "Size": 0, "IsDir": true, "Type": "dir" },
  { "Name": "USB0", "Size": 0, "IsDir": true, "Type": "dir" } ]
```

Note the key casing here is PascalCase while the rest of the CLI emits snake_case. Handle both
if you parse several commands in one script.

## Disk images

**Which side of the wire each command works on is not uniform - check before use:**

| Command | Operates on |
|---|---|
| `files create-d64` / `-d71` / `-d81` / `-dnp` | the **device** filesystem |
| `files info` | the **device** filesystem |
| `files pack-d64` | **local** paths - reads a local directory, writes a local image |
| `drives ...` | the device |
| `fs ...` | the device |

Passing a local path to `create-d64` fails with `PATH DOESN'T EXIST`, which is the device
answering about its own filesystem, not a message about your disk. Use a device path such as
`/Temp/mydisk.d64`, or build locally with `pack-d64` and upload:

```sh
c64u files pack-d64 ./programs mydisk.d64 --name "MY DISK" --id 01
c64u fs upload mydisk.d64 /Temp/mydisk.d64
```

```
c64u files create-d64 <device-path> [--tracks N] [--name NAME]
c64u files create-d71 <device-path> [--name NAME]
c64u files create-d81 <device-path> [--name NAME]
c64u files create-dnp <device-path> --tracks N [--name NAME]
c64u files pack-d64 <local-dir> <local-file> [--name NAME] [--id ID] [--tracks N]
c64u files info <device-path>

c64u drives list
c64u drives mount        <drive> <image>      [--type TYPE] [--mode MODE]
c64u drives mount-upload <drive> <local-file> [--type TYPE] [--mode MODE]
c64u drives unmount <drive>
c64u drives unlink <drive>                     # a, b or softiec; firmware 1.2.1
c64u drives on | off | reset <drive>
c64u drives set-mode <drive> <mode>
c64u drives load-rom        <drive> <file>
c64u drives load-rom-upload <drive> <local-file>
c64u drives softiec status | enable | disable [--completely] | bus-id | root
```

`c64u drives list --json` returns an array of single-key objects, one per drive. On a C64
Ultimate on firmware 1.2.1:

| Key | Fields |
|---|---|
| `a`, `b` | `bus_id`, `enabled`, `type` (`1541`...), `rom`, `image_file`, `image_path` |
| `IEC Drive` (SoftIEC) | `bus_id`, `enabled`, `type` (`DOS emulation`), `last_error`, `partitions` |
| `Printer Emulation` | `bus_id`, `enabled` |

`last_error` is the SoftIEC DOS status string, e.g. `73,U64HD ULTIMATE DOS V2.0,00,00` after
power-on; `partitions` is a list of `{id, path}`. An empty drive has `image_file` set to `""`.
Parse by key, not by position, and expect entries to come and go with the configuration.

`unlink` keeps a mounted disk in the drive but stops writing to its file: on the device a
`SAVE` after `unlink` left the `.d64` unchanged, the same `SAVE` without it changed the file.
Mounting with `--mode unlinked` does the same from the start. `drives list` does not show
the unlinked state, and `unlink` on an empty drive also reports success.

## Device configuration

```
c64u config list                               # categories
c64u config show <category>                    # all settings in one category
c64u config get  <category> <item>
c64u config set  <category> <item> <value>
c64u config set-multiple <json-file>
c64u config export [output-file]
c64u config restore <file> [--dry-run]         # backup back onto the same device, saved to flash
c64u config import <file> [--dry-run]          # copy settings, keep the device identity
c64u config save-to-flash [category]           # persist across power cycles
c64u config load-from-flash [category]
c64u config reset-to-default [category]
```

Settings changed with `set` apply immediately but are lost on power-off unless
`save-to-flash` follows.

**`restore` vs `import`** - the purpose decides, not a flag. Both read the output of `export`
(each category nested twice, with an `errors` member) as well as the flat `set-multiple` format,
compare it with the device and send only what differs; `--dry-run` lists those changes per
category as `Item: old -> new` and writes nothing. `restore` sends everything, the device
identity last, then saves to flash. `import` skips the identity - `Ethernet Settings`,
`WiFi settings`, `Host Name` and `Unique ID` - and changes memory only, like `set`. The firmware
tries every setting in the body and names the ones it rejects; those land under `failed` and
the rest is applied. Checked on 1.2.1: a fresh export restored with `--dry-run` reports 0
changes; an import with an unknown item reported it as failed, still changed `Disk swap delay`
and left `Host Name` as it was.

`Network Password` goes last, after the identity: it takes effect at once (checked on 1.2.1 - the
next request without it got `Forbidden`), so `c64u` carries on with the new password and says so;
from then on pass it with `--password` or `C64U_PASSWORD`. The output never shows the password,
and GET returns it in plain text, so an export file holds it in plain text - keep it private.

With a category name or pattern (firmware 1.2.1) the three act on the matching categories
only and list them. `save-to-flash` writes only categories with unsaved changes, and the
firmware answers a name that matches nothing the same way as one with nothing to save:
`Nothing to save`. `load-from-flash` and `reset-to-default` fail when nothing matches.

The category names contain spaces and must be quoted:

```sh
c64u config show "C64 and Cartridge Settings"     # includes RAM Expansion Unit, REU Size
c64u config set "C64 and Cartridge Settings" "RAM Expansion Unit" Enabled
```

## Streams (U64 only)

```
c64u streams listen video      # native window, 768x544; keyboard, gamepads and mouse reach the C64
c64u streams listen audio      # playback through local speakers
c64u streams listen debug      # raw debug stream to stdout, --raw to pipe
c64u streams start <stream> <ip>
c64u streams stop  <stream>
```

`listen` detects the local IP, starts the stream and stops it on exit, which is what you want
almost always. `start`/`stop` are the manual halves for cases where the receiver is elsewhere.

Ports: video 11000, audio 11001, debug 11002.

`video` and `audio` need a window and the host's graphics and sound libraries; see `limits.md`.

In the video window (firmware 1.2.0 / 3.15) character mode taps each typed character; position
mode holds keys as long as you hold them, which games need. Hotkeys,
with Cmd on macOS and Ctrl+Shift elsewhere: Mod+K keyboard mode (character / position), Mod+J
keyboard joystick, Mod+P swap ports, Mod+L capture the mouse, Mod+U Ultimate menu, Mod+R reset
(twice). Left Alt is C=, right Alt (AltGr) types the layout's symbols, Esc is RUN/STOP, Page Up
is RESTORE. The command prints the list when it starts.

## CLI configuration and TUI

```
c64u cli-config init          # write ~/.config/c64u/config.toml
c64u cli-config show
c64u ui                       # full-screen terminal UI
c64u completion <shell>       # bash | zsh | fish | powershell
```
