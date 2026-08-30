# Command Reference

Every command accepts the global flags below. `<>` is required, `[]` optional.

## Global flags

| Flag | Effect |
|---|---|
| `--host string` | Device hostname or IP (env `C64U_HOST`) |
| `-D, --device string` | Name of a device defined in the config file (env `C64U_DEVICE`) |
| `--port int` | HTTP port, default 80 (env `C64U_PORT`) |
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
c64u machine write-mem <address> <hexdata>
c64u machine write-mem-file <address> <file>
c64u machine sendkey <string> [--delay ms]

c64u machine debug-reg                         # read $D7FF, U64 only
c64u machine debug-reg-set <value>             # write $D7FF, U64 only
```

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
c64u drives on | off | reset <drive>
c64u drives set-mode <drive> <mode>
c64u drives load-rom        <drive> <file>
c64u drives load-rom-upload <drive> <local-file>
c64u drives softiec                            # DOS emulation drive settings
```

`c64u drives list --json` returns an array of single-key objects, one per drive (`a`, `b`,
`IEC Drive`), each with `bus_id`, `enabled`, `type`, `rom`, `image_file`, `image_path`.

## Device configuration

```
c64u config list                               # categories
c64u config show <category>                    # all settings in one category
c64u config get  <category> <item>
c64u config set  <category> <item> <value>
c64u config set-multiple <json-file>
c64u config export [output-file]
c64u config save-to-flash                      # persist across power cycles
c64u config load-from-flash
c64u config reset-to-default
```

Settings changed with `set` apply immediately but are lost on power-off unless
`save-to-flash` follows.

The category names contain spaces and must be quoted:

```sh
c64u config show "C64 and Cartridge Settings"     # includes RAM Expansion Unit, REU Size
c64u config set "C64 and Cartridge Settings" "RAM Expansion Unit" Enabled
```

## Streams (U64 only)

```
c64u streams listen video      # native window, 768x544, keystrokes forwarded to the C64
c64u streams listen audio      # playback through local speakers
c64u streams listen debug      # raw debug stream to stdout, --raw to pipe
c64u streams start <stream> <ip>
c64u streams stop  <stream>
```

`listen` detects the local IP, starts the stream and stops it on exit, which is what you want
almost always. `start`/`stop` are the manual halves for cases where the receiver is elsewhere.

Ports: video 11000, audio 11001, debug 11002.

## CLI configuration and TUI

```
c64u cli-config init          # write ~/.config/c64u/config.toml
c64u cli-config show
c64u ui                       # full-screen terminal UI
c64u completion <shell>       # bash | zsh | fish | powershell
```
