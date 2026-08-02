# c64u - Commodore C64 Ultimate CLI

A command-line interface for controlling the [Commodore C64 Ultimate](https://commodore.net) via its REST API written in Go. This project is currently work in progress, so there may still be some bugs. c64u is primarily intended for those of you who want to develop for the Commodore C64 Ultimate with VS Code or Neovim, or who want to create small automations with scripting languages. A development environment with full CLI integration in Neovim can be found in the [c64.nvim project](https://github.com/cybersorcerer/c64.nvim). A plugin for VSCode including a Tree Browser is in the making.

## Features

- **Complete REST API Coverage**: All C64 Ultimate API endpoints supported
- **Interactive TUI**: Full-screen terminal UI with a dual-pane file browser, drive, machine and config views
- **SoftIEC Control**: Enable the DOS emulation drive, set its device number, and point it at a directory
- **Live Video Stream**: Display C64 video output in a native window with keyboard forwarding — type into BASIC, load and run programs from your Mac keyboard (BASIC/KERNAL input only; games that poll the keyboard matrix directly are not supported)
- **Live Audio Stream**: Play back C64 audio in real time
- **FTP Integration**: Access the C64 Ultimate filesystem
- **Flexible Configuration**: Config file, environment variables, or CLI flags
- **Multiple Output Formats**: Human-readable text or JSON for scripting
- **Debug Logging**: Built-in debug mode for troubleshooting
- **Cross-Platform**: Builds for macOS, Linux, and Windows
- **Easy Integration**: Works seamlessly with c64.nvim, VSCode, and scripts

## Installation

### Download Pre-built Binaries (Recommended)

Download the latest release from [GitHub Releases](https://github.com/cybersorcerer/c64u/releases/latest):

- `c64u_Darwin_x86_64.tar.gz` - macOS (Intel)
- `c64u_Darwin_arm64.tar.gz` - macOS (Apple Silicon)
- `c64u_Linux_x86_64.tar.gz` - Linux (x86_64)
- `c64u_Linux_arm64.tar.gz` - Linux (ARM64)
- `c64u_Windows_x86_64.tar.gz` - Windows

**Quick Install (macOS/Linux):**

```bash
# Download and extract (replace with your platform)
curl -L https://github.com/cybersorcerer/c64u/releases/latest/download/c64u_Darwin_arm64.tar.gz | tar xz

# Move binary to PATH
mv c64u ~/.local/bin/

# Verify installation
c64u version
```

**Windows:**

1. Download `c64u_Windows_x86_64.tar.gz`
2. Extract `c64u.exe`
3. Add the directory to your PATH

### From Source

```bash
git clone https://github.com/cybersorcerer/c64u.git
cd c64u/tools/c64u

# Build and install to ~/.local/bin
make install
```

## Prerequisites

- Go 1.25 or later (for building from source)
- C64 Ultimate hardware on your network

## Quick Start

### 1. Create Configuration File

```bash
c64u cli-config init
```

This creates `~/.config/c64u/config.toml` with default settings.

### 2. Edit Configuration

```toml
# ~/.config/c64u/config.toml
host = "192.168.1.100"
port = 80
```

### 3. Test Connection

```bash
c64u about        # C64 Ultimate API version
c64u info         # Device information
```

## Configuration

### Priority Order

1. **CLI Flags** `--host 192.168.1.100 --port 80`
2. **Environment Variables** `C64U_HOST`, `C64U_PORT`
3. **Config File** `~/.config/c64u/config.toml`
4. **Defaults** host=`localhost`, port=`80`

## Usage

### Global Flags

```bash
--host string      C64 Ultimate hostname/IP (env: C64U_HOST)
--port int         HTTP port, default 80 (env: C64U_PORT)
--json             Output in JSON format
--verbose          Show HTTP requests/responses
--debug, -d        Debug logging to ~/.local/share/c64u/c64u.log
--no-color         Disable colored output
```

### Shell Completion

```bash
c64u completion zsh > "${fpath[1]}/_c64u"        # zsh
c64u completion bash > /etc/bash_completion.d/c64u
c64u completion fish > ~/.config/fish/completions/c64u.fish
```

### Commands

#### Interactive TUI

```bash
c64u ui
```

Full-screen terminal UI, organised as five tabs:

- **Files**: Dual-pane browser — your machine on the left, the C64 Ultimate on the right. Copy between them, run programs, mount images
- **Drives**: Mount/unmount, load ROMs, enable/disable drives, SoftIEC directory and device number
- **Machine**: Reset, reboot, pause/resume, menu button, power off
- **Config**: Browse and edit device settings, save/load from flash
- **Streams**: Start the video, audio or debug stream

Global keys, in every view:

| Key | Action |
| --- | --- |
| `1`–`5` | Jump to tab |
| `Ctrl+h` / `Ctrl+l` | Previous / next tab |
| `j` / `k`, `↑` / `↓` | Move down / up |
| `g` / `G` | Top / bottom |
| `Ctrl+d` / `Ctrl+u` | Half page down / up |
| `Enter` | Select / open |
| `?` | Help overlay |
| `Esc` | Back / close |
| `Ctrl+C` | Quit |

File browser:

| Key | Action |
| --- | --- |
| `Tab`, `h` / `l` | Switch active pane (local/remote) |
| `Enter` | Open directory, disk actions, or run PRG/CRT/SID/MOD |
| `Backspace` | Parent directory |
| `Space` | Mark file for a batch operation |
| `F5` or `c` | Copy marked files to the other pane |
| `d` / `r` / `m` | Delete / rename / make directory |
| `n` | New disk image (remote pane) |
| `u` | Apply to a drive: mount disk image, or load ROM |
| `p` | Toggle the preview column |

File viewer:

| Key | Action |
| --- | --- |
| `Tab` | Toggle text / hex view |
| `PgUp` / `PgDn` | Scroll page by page |
| `Esc` / `q` | Back to the file browser |

Starting a stream from the Streams tab suspends the TUI, hands the terminal to
the stream viewer, and returns you to the same tab when it exits.

#### Data Streams (U64 Only)

```bash
# Live listeners — auto-detect local IP, start stream, stop on Ctrl+C
c64u streams listen video                      # C64 video in a native window
c64u streams listen audio                      # C64 audio through speakers
c64u streams listen debug                      # Raw debug stream to stdout
c64u streams listen debug --raw | xxd          # Pipe to other tools

# Override local IP
c64u streams listen video --ip 192.168.1.50

# Manual start/stop
c64u streams start <stream> <ip>
c64u streams stop <stream>
```

**Streams:** `video` (port 11000), `audio` (port 11001), `debug` (port 11002)

**Video Stream**: Opens a native 768×544 window (2× scaled) with accurate VIC colors. PAL (384×272) and NTSC (384×240) supported.

While the video window is focused, keystrokes are forwarded to the C64 keyboard buffer via DMA — the window acts as a keyboard input device for the real hardware.

> **Important — BASIC only, not games:** Keyboard forwarding works only for input that the C64 KERNAL/BASIC reads from the keyboard buffer at `$0277` (the BASIC prompt, `INPUT`, `GET`, typing/loading programs). It writes to RAM via DMA. Most **games and demos poll the keyboard matrix hardware directly** (CIA registers `$DC00`/`$DC01`) — for example a "press SPACE to start" title screen. DMA cannot reach those I/O registers, so such keypresses have no effect. This is a hardware limitation of the Ultimate REST API, not a bug.

| Key                          | C64 action                                          |
| ---------------------------- | --------------------------------------------------- |
| Letters, digits, punctuation | Typed character (layout-aware, converted to PETSCII)|
| Return                       | Return                                              |
| Backspace / Delete           | DEL                                                 |
| F1–F8                        | F1–F8                                               |
| Cursor keys                  | Cursor Up / Down / Left / Right                     |
| Left Alt + Shift             | CBM+Shift (toggle graphics/text charset)            |
| Escape                       | — (see below)                                       |

**Escape key — hardware limitation:** RUN/STOP on a real C64 works by the BASIC ROM reading the CIA1 hardware register at `$DC01` directly — it does not go through the keyboard buffer. DMA writes (which the Ultimate API uses) only reach RAM, not I/O-mapped CIA registers. Therefore RUN/STOP cannot be injected via the API while a program is running.

As a workaround, pressing **Escape once** arms a reset confirmation — the window title changes to signal this state. Pressing **Escape a second time within 3 seconds** sends `machine:reset` to the C64. Any other key or waiting cancels the confirmation.

**Audio Stream**: Plays 48 kHz stereo 16-bit PCM from the C64 audio mixer with automatic gap compensation.

**Debug Stream**: Streams raw 6510/VIC/1541 CPU bus data for clock-cycle-accurate program tracing (firmware ≥ 3.7 required).

```bash
c64u streams listen debug --tui                # Live disassembler TUI
```

The `--tui` flag opens a full-screen live disassembler that decodes the 6510 bus trace in real time:

- **Live disassembly** of code running on the C64, including all illegal opcodes
- **CPU registers**: PC, SP and reconstructed status flags (N V B D I Z C)
- **I/O access log**: live VIC, SID and CIA register reads/writes
- Kernal ROM ($E000–$FFFF) and BASIC ROM ($A000–$BFFF) are filtered out to keep the trace focused on user code

| Key | Action |
| --- | --- |
| `Space` | Pause / resume |
| `↑/k`, `↓/j` | Scroll (paused) |
| `fn+↑/u`, `fn+↓/d` | Page up / down (paused) |
| `q` / `Ctrl+C` | Quit |
| `w` | Add watchpoint (enter: `D020` or `D020=05`) |
| `W` | Remove last watchpoint |

**Watchpoints** monitor any number of memory addresses live:

- Address only (`D020`): logs every write with value and hit counter
- With value condition (`D020=05`): auto-pauses when exactly that value is written
- The watchpoint panel shows address, last value, hit counter, and condition status

#### Runners — Media & Program Execution

```bash
c64u runners sidplay <file> [--song N]         # Play SID from C64U filesystem
c64u runners sidplay-upload <file> [--song N]  # Upload and play SID
c64u runners modplay <file>                    # Play MOD
c64u runners modplay-upload <file>             # Upload and play MOD
c64u runners load-prg <file>                   # Load PRG via DMA
c64u runners load-prg-upload <file>            # Upload and load PRG
c64u runners run-prg <file>                    # Load and run PRG
c64u runners run-prg-upload <file>             # Upload and run PRG
c64u runners run-crt <file>                    # Start cartridge
c64u runners run-crt-upload <file>             # Upload and start cartridge
```

#### Machine Control

```bash
c64u machine reset                             # Reset machine
c64u machine reboot                            # Reboot with cartridge reinit
c64u machine pause                             # Pause via DMA
c64u machine resume                            # Resume from pause
c64u machine poweroff                          # Power off (U64 only)
c64u machine menu-button                       # Simulate Menu button press
c64u machine write-mem <addr> <data>           # Write hex data to memory
c64u machine write-mem-file <addr> <file>      # Write file to memory
c64u machine read-mem <addr> [--length N]      # Read memory (hex dump)
c64u machine read-mem <addr> --to <addr>       # Read an address range, end inclusive
c64u machine read-mem <addr> --output <file>   # Write the raw bytes to a file (-o)
c64u machine read-mem <addr> --raw             # Write the raw bytes to stdout
c64u machine sendkey <string> [--delay <ms>]   # Send keystrokes to keyboard buffer
c64u machine debug-reg                         # Read debug register (U64 only)
c64u machine debug-reg-set <value>             # Write debug register (U64 only)
```

**Reading memory** (`read-mem`) shows a hex dump by default. `--output` (`-o`)
writes the bytes to a file and `--raw` writes them to standard output, both
exactly as the C64 returned them — neither adds or removes anything. Either
combines with `--length` or `--to`, in text and JSON mode alike:

```bash
c64u machine read-mem 0400 --to 07e7 --output screen.bin   # screen RAM, 1000 bytes
c64u machine read-mem 0000 --to ffff -o memory.bin         # whole address space
c64u machine read-mem 0400 --length 16 --raw | xxd         # pipe the bytes
```

`--to` is inclusive, so `0400 --to 07e7` is the full 1000-byte screen. Addresses
may be written as `0400`, `$0400` or `0x0400`, and are checked before the
request goes out — the device answers an invalid address with data from
somewhere rather than an error.

**Keyboard injection** (`sendkey`) converts ASCII strings to PETSCII and injects them into the C64 keyboard buffer via DMA. Strings longer than 10 characters are sent in chunks (default 100ms delay). Escape sequences: `\n` (Return), `\f1`–`\f8` (F1–F8), `\clr`, `\del`, `\home`, `\cup` (cursor up), `\cdn` (cursor down), `\cleft` (cursor left), `\cright` (cursor right).

> **Note — `\stop` (RUN/STOP):** The sequence `\stop` writes PETSCII `$03` into the keyboard buffer. This only works when the C64 is idle (e.g. at the BASIC prompt or waiting for `INPUT`). While a BASIC program is running, the BASIC ROM reads the RUN/STOP state directly from the CIA1 hardware register at `$DC01` — not from the keyboard buffer. DMA writes cannot reach I/O-mapped CIA registers, so `\stop` has no effect on a running program.

#### Drive Operations

```bash
c64u drives list                               # List all drives
c64u drives mount <drive> <image> [--type TYPE] [--mode MODE]
c64u drives mount-upload <drive> <file> [--type TYPE] [--mode MODE]
c64u drives unmount <drive>
c64u drives reset <drive>
c64u drives on <drive>
c64u drives off <drive>
c64u drives load-rom <drive> <file>
c64u drives load-rom-upload <drive> <file>
c64u drives set-mode <drive> <mode>            # 1541 / 1571 / 1581
```

**Mount types:** `d64`, `g64`, `d71`, `g71`, `d81` — **Modes:** `readwrite`, `readonly`, `unlinked`

#### SoftIEC — DOS Emulation Drive

SoftIEC serves a directory of the C64 Ultimate filesystem over the IEC bus, so
the C64 can `LOAD"$",11` straight into it without a disk image.

```bash
c64u drives softiec status                     # State, device number, directory
c64u drives softiec enable [--bus-id N]        # Enable, optionally on another bus ID
c64u drives softiec disable
c64u drives softiec bus-id <id>                # Change the device number (8-30)
c64u drives softiec root <path>                # Point the drive at a directory
```

SoftIEC is not a floppy drive, and the drives endpoints do not control it —
`/v1/drives/softiec:on` answers success while nothing changes. Enabling it and
its bus ID are device configuration settings, which these commands write. For
convenience `c64u drives on softiec` and `off` are routed the same way.

> **`root` types on the C64.** There is no API endpoint for the served
> directory. The command is sent as a CBM DOS `CD:` line through the keyboard
> buffer, so the **C64 must be sitting at the BASIC prompt** — if a program is
> running, the line goes nowhere. The drive is read back afterwards, and the
> command reports the directory actually being served rather than assuming it
> worked. From the C64 itself the equivalent is:
>
> ```basic
> OPEN 1,11,15,"CD:development":CLOSE 1
> ```

#### File Operations

```bash
c64u files info <path>                         # File info (wildcards supported)
c64u files create-d64 <path> [--tracks N] [--name NAME]
c64u files create-d71 <path> [--name NAME]
c64u files create-d81 <path> [--name NAME]
c64u files create-dnp <path> --tracks N [--name NAME]

# Pack local directory into D64 image (EXPERIMENTAL)
c64u files pack-d64 <source-dir> <output-file> [--name NAME] [--id ID] [--tracks N]
```

#### Filesystem Operations (FTP)

```bash
c64u fs ls [path]
c64u fs upload <local> <remote>
c64u fs download <remote> <local>
c64u fs mkdir <path>
c64u fs rm <path>
c64u fs mv <source> <dest>
c64u fs cp <source> <dest>
c64u fs cat <path>
```

#### Hardware Configuration

```bash
c64u config list                               # List all categories
c64u config show "Drive A Settings"            # Show category (wildcards supported)
c64u config get "Drive A Settings" "Drive Type"
c64u config set "Drive A Settings" "Drive Type" "1581"
c64u config set-multiple settings.json         # Set multiple from JSON
c64u config save-to-flash
c64u config load-from-flash
c64u config reset-to-default
c64u config export [file]                      # Export all settings to JSON
```

## Project Structure

```text
tools/c64u/
├── cmd/c64u/          # CLI entry point and command definitions
├── internal/
│   ├── api/           # REST API client
│   ├── audio/         # Audio stream receiver and playback
│   ├── config/        # Configuration handling
│   ├── debug/         # Debug logging
│   ├── debugger/      # Live disassembler TUI (bus trace decoder)
│   ├── disasm/        # 6502/6510 disassembler incl. illegal opcodes
│   ├── diskimage/     # Local disk image creation
│   ├── network/       # Local IP detection
│   ├── output/        # Output formatting
│   ├── petscii/       # ASCII to PETSCII conversion for keyboard injection
│   ├── softiec/       # SoftIEC drive: settings discovery and directory control
│   ├── tui/           # Interactive terminal UI (Bubble Tea)
│   └── video/         # Video stream receiver and rendering (Ebitengine)
├── go.mod
├── Makefile
└── README.md
```

## Integration

### With c64.nvim

```lua
require("c64").setup({
  c64u = {
    enabled = true,
    host = "192.168.1.100",
    port = 80,
  }
})
-- <leader>ku to upload and run on C64 Ultimate
```

### With Shell Scripts

```bash
java -jar kickass.jar -o program.prg program.asm
c64u runners run-prg-upload program.prg
```

## Building

```bash
make build     # Build for current platform
make install   # Build and install to ~/.local/bin
make dev       # Development build (verbose)
make test      # Run tests
make fmt       # Format code
make lint      # Run linter (requires golangci-lint)
```

## Releasing

The version `c64u version` reports comes from `RELEASE` in the Makefile. Bump it
first, then tag — GoReleaser takes its own version from the tag.

```bash
# 1. Bump RELEASE in tools/c64u/Makefile, commit it
# 2. Tag and push
git tag v0.9.0
git push origin v0.9.0
```

The GitHub Action builds automatically for all platforms and creates a GitHub Release:

- **macOS** (Intel + Apple Silicon): built on `macos-latest` with CGO — includes video and audio stream
- **Linux** (x86_64 + ARM64): built on `ubuntu-latest` without CGO
- **Windows** (x86_64): built on `ubuntu-latest` without CGO

> **Note:** Video and audio stream features are only available in the macOS binaries, as they require native frameworks (Metal, CoreAudio).

## Troubleshooting

```bash
# Test connection
c64u --verbose about

# Debug logging
c64u -d about
cat ~/.local/share/c64u/c64u.log

# Check configuration
c64u cli-config show
```

## API Reference

<https://1541u-documentation.readthedocs.io/en/latest/api/api_calls.html>

## License

Apache 2.0

## Credits

- Built for the [Commodore C64 Ultimate](https://commodore.net)
- Based on Gideon's Logic Architectures Ultimate64 FPGA board
- Part of the [c64.nvim](https://github.com/cybersorcerer/c64.nvim) project

## Contributing

Contributions are welcome! Please feel free to submit issues or pull requests.
