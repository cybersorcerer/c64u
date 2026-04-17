# c64u - Commodore C64 Ultimate CLI

A command-line interface for controlling the [Commodore C64 Ultimate](https://commodore.net) via its REST API written in Go. This project is currently work in progress, so there may still be some bugs. c64u is primarily intended for those of you who want to develop for the Commodore C64 Ultimate with VS Code or Neovim, or who want to create small automations with scripting languages. A development environment with full CLI integration in Neovim can be found in the [c64.nvim project](https://github.com/cybersorcerer/c64.nvim). A plugin for VSCode including a Tree Browser is in the making.

## Features

- **Complete REST API Coverage**: All C64 Ultimate API endpoints supported
- **Interactive TUI**: Full-screen terminal UI for browsing, mounting, and controlling
- **Live Video Stream**: Display C64 video output in a native window (U64 only)
- **Live Audio Stream**: Play back C64 audio in real time (U64 only)
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

1. Download `c64u_Windows_x86_64.zip`
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

### Commands

#### Interactive TUI

```bash
c64u ui
```

Full-screen terminal UI with:

- **File Browser**: Navigate filesystem, mount disk images, run programs
- **Drive Management**: Mount/unmount, load ROMs, enable/disable drives
- **Machine Control**: Reset, reboot, pause/resume, power off
- **Configuration**: Browse and edit device settings, save/load from flash

| Key | Action |
| --- | --- |
| `↑/↓` or `j/k` | Navigate |
| `Enter` | Select / Confirm |
| `←/Backspace/h` | Parent directory (File Browser) |
| `Tab` | Toggle Text/Hex view (File Viewer) |
| `PgUp/PgDn` | Scroll (File Viewer) |
| `?` | Help overlay |
| `Esc` | Back / Close |
| `Ctrl+C` | Quit |

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
| `w` | Watchpoint hinzufügen (Eingabe: `D020` oder `D020=05`) |
| `W` | Letzten Watchpoint entfernen |

**Watchpoints** überwachen beliebig viele Speicheradressen live:

- Adresse allein (`D020`): protokolliert jeden Zugriff mit Wert und Zähler
- Mit Wertbedingung (`D020=05`): pausiert automatisch wenn genau dieser Wert geschrieben wird
- Das Watchpoint-Panel zeigt Adresse, letzten Wert, Hit-Zähler und Condition-Status

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
c64u machine debug-reg                         # Read debug register (U64 only)
c64u machine debug-reg-set <value>             # Write debug register (U64 only)
```

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

```bash
git tag v0.6.0
git push origin v0.6.0
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
