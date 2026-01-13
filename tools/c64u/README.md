# c64u - Commodore C64 Ultimate CLI

A command-line interface for controlling the [Commodore C64 Ultimate](https://commodore.net) via its REST API.

## Features

- **Complete REST API Coverage**: All C64 Ultimate API endpoints supported
- **Flexible Configuration**: Config file, environment variables, or CLI flags
- **Multiple Output Formats**: Human-readable text or JSON for scripting
- **Cross-Platform**: Builds for macOS, Linux, and Windows
- **Easy Integration**: Works seamlessly with c64.nvim, VSCode, and scripts

## Installation

### Download Pre-built Binaries (Recommended)

Download the latest release from [GitHub Releases](https://github.com/cybersorcerer/c64u/releases/latest):

- `c64u_Darwin_x86_64.tar.gz` - macOS (Intel)
- `c64u_Darwin_arm64.tar.gz` - macOS (Apple Silicon)
- `c64u_Linux_x86_64.tar.gz` - Linux (x86_64)
- `c64u_Linux_arm64.tar.gz` - Linux (ARM64)
- `c64u_Windows_x86_64.zip` - Windows

**Quick Install (macOS/Linux):**

```bash
# Download and extract (replace with your platform)
curl -L https://github.com/cybersorcerer/c64u/releases/latest/download/c64u_Darwin_arm64.tar.gz | tar xz

# Move binary to PATH
sudo mv c64u /usr/local/bin/

# Verify installation
c64u version
```

**Windows:**

1. Download `c64u_Windows_x86_64.zip`
2. Extract `c64u.exe`
3. Add the directory to your PATH

### From Source

```bash
# Clone the repository (or navigate to the tools directory)
cd /path/to/c64.nvim/tools/c64u

# Build and install
make install
```

This will install `c64u` to `~/bin` or `/usr/local/bin`.

### Manual Build

```bash
# Build for current platform
make build

# Binary will be in build/c64u
./build/c64u version
```

### Cross-Platform Builds

The project uses [GoReleaser](https://goreleaser.com) for cross-platform builds:

```bash
# Build for all platforms (uses GoReleaser if installed, otherwise falls back to manual build)
make release

# Manual build for all platforms
make release-manual

# Binaries will be in dist/
# - macOS (Intel & Apple Silicon)
# - Linux (x86_64 & ARM64)
# - Windows (x86_64)
```

**Install GoReleaser (optional):**

```bash
# macOS
brew install goreleaser

# Or via Go
go install github.com/goreleaser/goreleaser@latest
```

## Prerequisites

- Go 1.22 or later (for building)
- C64 Ultimate hardware on your network

## Quick Start

### 1. Create Configuration File

```bash
c64u cli-config init
```

This creates `~/.config/c64u/config.toml` with default settings.

### 2. Edit Configuration

Edit `~/.config/c64u/config.toml`:

```toml
# C64 Ultimate hostname or IP address
host = "192.168.1.100"

# HTTP port (default: 80)
port = 80
```

### 3. Test Connection

```bash
# Get API version
c64u about

# Show current CLI configuration
c64u cli-config show
```

## Configuration

### Priority Order

1. **CLI Flags** (highest priority)

   ```bash
   c64u --host 192.168.1.100 --port 80 about
   ```

2. **Environment Variables**

   ```bash
   export C64U_HOST=192.168.1.100
   export C64U_PORT=80
   c64u about
   ```

3. **Config File**
   `~/.config/c64u/config.toml`

4. **Defaults**
   - host: `localhost`
   - port: `80`

## Usage

### Global Flags

```bash
--host string      C64 Ultimate hostname/IP (env: C64U_HOST)
--port int         HTTP port (default: 80) (env: C64U_PORT)
--json             Output in JSON format
--verbose          Enable verbose output (shows HTTP requests)
```

### Commands

#### Version Information

```bash
# CLI tool version
c64u version

# C64 Ultimate API version
c64u about

# C64 Ultimate device information
c64u info
```

#### CLI Configuration Management

Manage c64u CLI tool configuration file (`~/.config/c64u/config.toml`):

```bash
# Create default CLI config file
c64u cli-config init

# Show current CLI configuration
c64u cli-config show
```

#### Runners - Media & Program Execution

```bash
# SID playback
c64u runners sidplay <file> [--song N]        # Play SID from C64U filesystem
c64u runners sidplay-upload <file> [--song N] # Upload and play SID

# MOD playback
c64u runners modplay <file>                    # Play MOD from C64U filesystem
c64u runners modplay-upload <file>             # Upload and play MOD

# PRG loading (no execution)
c64u runners load-prg <file>                   # Load PRG via DMA
c64u runners load-prg-upload <file>            # Upload and load PRG

# PRG loading and running
c64u runners run-prg <file>                    # Load and run PRG
c64u runners run-prg-upload <file>             # Upload and run PRG

# Cartridge
c64u runners run-crt <file>                    # Start cartridge
c64u runners run-crt-upload <file>             # Upload and start cartridge
```

#### Machine Control

```bash
# Control commands
c64u machine reset                             # Reset machine
c64u machine reboot                            # Reboot with cartridge reinit
c64u machine pause                             # Pause via DMA
c64u machine resume                            # Resume from pause
c64u machine poweroff                          # Power off (U64 only)
c64u machine menu-button                       # Simulate Menu button press

# Memory operations
c64u machine write-mem <addr> <data>           # Write hex data to memory
c64u machine write-mem-file <addr> <file>      # Write file to memory
c64u machine read-mem <addr> [--length N]      # Read memory (hex dump)

# Debug register (U64 only)
c64u machine debug-reg                         # Read debug register
c64u machine debug-reg-set <value>             # Write debug register
```

#### Drive Operations

```bash
# List and mount
c64u drives list                               # List all drives
c64u drives mount <drive> <image> [--type TYPE] [--mode MODE]
c64u drives mount-upload <drive> <file> [--type TYPE] [--mode MODE]
c64u drives unmount <drive>                    # Remove disk

# Control
c64u drives reset <drive>                      # Reset drive
c64u drives on <drive>                         # Enable drive
c64u drives off <drive>                        # Disable drive

# ROM and mode
c64u drives load-rom <drive> <file>            # Load custom ROM
c64u drives load-rom-upload <drive> <file>     # Upload and load ROM
c64u drives set-mode <drive> <mode>            # Set mode (1541/1571/1581)
```

**Mount types:** `d64`, `g64`, `d71`, `g71`, `d81`
**Mount modes:** `readwrite`, `readonly`, `unlinked`

#### Data Streams (U64 Only)

```bash
c64u streams start <stream> <ip>               # Start stream (video/audio/debug)
c64u streams stop <stream>                     # Stop stream
```

**Streams:** `video` (port 11000), `audio` (port 11001), `debug` (port 11002)

#### File Operations

```bash
# File information
c64u files info <path>                         # Get file info (supports wildcards)

# Create empty disk images on C64 Ultimate
c64u files create-d64 <path> [--tracks N] [--name NAME]
c64u files create-d71 <path> [--name NAME]
c64u files create-d81 <path> [--name NAME]
c64u files create-dnp <path> --tracks N [--name NAME]

# Pack local directory into disk image (EXPERIMENTAL)
c64u files pack-d64 <source-dir> <output-file> [--name NAME] [--id ID] [--tracks N]

# Upload to C64 Ultimate
c64u fs upload <local-file> <remote-path>
```

> **⚠️ EXPERIMENTAL FEATURE**: The `pack-d64` command is experimental. While basic functionality works, there may be edge cases or compatibility issues with certain files or disk configurations. Always test generated disk images before relying on them.

**Pack D64 Examples:**

```bash
# Create D64 from directory
c64u files pack-d64 ./myproject game.d64 --name "MY GAME" --id "01"

# Create 40-track D64
c64u files pack-d64 ./build output.d64 --tracks 40 --name "BIG DISK"

# Use directory name as disk name
c64u files pack-d64 ./demo-files demo.d64

# Create and upload to C64 Ultimate
c64u files pack-d64 ./myproject game.d64 && c64u fs upload game.d64 /SD/games/game.d64
```

**Supported file types:** `.prg`, `.seq`, `.usr`, `.rel` (and their P00/S00/U00/R00 variants)

#### Filesystem Operations (via FTP)

Complete filesystem access to C64 Ultimate via FTP (port 21, anonymous login):

```bash
# Directory listing
c64u fs ls [path]                              # List files and directories

# File transfer
c64u fs upload <local> <remote>                # Upload file to C64U
c64u fs download <remote> <local>              # Download file from C64U

# Directory operations
c64u fs mkdir <path>                           # Create directory
c64u fs rm <path>                              # Remove file or directory

# File operations
c64u fs mv <source> <dest>                     # Move/rename file or directory
c64u fs cp <source> <dest>                     # Copy file (download+upload)
c64u fs cat <path>                             # Show file information
```

**Examples:**

```bash
# List root directory
c64u fs ls /

# Upload PRG file
c64u fs upload myprogram.prg /Temp/myprogram.prg

# Download from SD card
c64u fs download /SD/games/game.prg ./game.prg

# Create directory
c64u fs mkdir /Temp/myproject

# Move file
c64u fs mv /Temp/old.prg /Temp/new.prg
```

#### Configuration Management

Manage C64 Ultimate configuration settings:

```bash
# List all categories
c64u config list

# Show all settings in a category
c64u config show "Drive A Settings"
c64u config show "drive a*"              # Wildcards supported

# Get detailed info about a setting
c64u config get "Drive A Settings" "Drive Type"
c64u config get "drive a*" "*bus*"       # Wildcards supported

# Set a configuration item
c64u config set "Drive A Settings" "Drive Type" "1581"
c64u config set "drive a*" "*bus*" "9"

# Set multiple settings from JSON file
c64u config set-multiple settings.json

# Persistence
c64u config save-to-flash                # Save to non-volatile memory
c64u config load-from-flash              # Load from flash (discards unsaved changes)
c64u config reset-to-default             # Reset to factory defaults

# Export/Import
c64u config export config-backup.json    # Export all settings to JSON
c64u config export                       # Print to stdout
```

**JSON format for set-multiple:**

```json
{
  "Drive A Settings": {
    "Drive": "Enabled",
    "Drive Type": "1581"
  },
  "Drive B Settings": {
    "Drive": "Disabled"
  }
}
```

## Output Formats

### Text Mode (Default)

Human-readable output:

```bash
$ c64u about
C64 Ultimate API version: 0.1
```

### JSON Mode

Machine-readable output for scripting:

```bash
$ c64u --json about
{
  "version": "0.1"
}
```

### Verbose Mode

Shows HTTP requests and responses:

```bash
$ c64u --verbose about
→ GET http://192.168.1.100:80/v1/version
← 200 200 OK
  Response: {
  "version" : "0.1",
  "errors" : [  ]
}
C64 Ultimate API version: 0.1
```

## Integration

### With c64.nvim

The c64u CLI integrates seamlessly with the [c64.nvim](../../README.md) plugin:

```lua
-- In your c64.nvim config
require("c64").setup({
  c64u = {
    enabled = true,
    host = "192.168.1.100",
    port = 80,
  }
})

-- Use <leader>ku to upload and run on C64 Ultimate
```

### With Shell Scripts

```bash
#!/bin/bash
# Assemble and run on C64 Ultimate

# Assemble
java -jar kickass.jar -o program.prg program.asm

# Upload and run
c64u runners run-prg-upload program.prg
```

### With VSCode

(Coming soon - VSCode extension in development)

## Development

### Project Structure

```text
c64u/
├── cmd/c64u/          # Main application entry point
├── internal/
│   ├── api/           # REST API client
│   ├── config/        # Configuration handling
│   └── output/        # Output formatting
├── go.mod             # Go module definition
├── Makefile           # Build automation
└── README.md          # This file
```

### Building

```bash
# Development build
make dev

# Run tests
make test

# Run with coverage
make test-coverage

# Format code
make fmt

# Run linter (requires golangci-lint)
make lint
```

### Adding New Commands

Commands are organized by API category. See the [implementation plan](../../.claude/plans/) for details.

## Releasing

For maintainers: To create a new release:

```bash
# Tag the release
git tag -a v0.4.0 -m "Release v0.4.0"
git push origin v0.4.0

# The GitHub Action will automatically:
# 1. Build binaries for all platforms
# 2. Create a GitHub release
# 3. Upload all binaries and checksums
```

Or manually with GoReleaser:

```bash
# Ensure GITHUB_TOKEN is set
export GITHUB_TOKEN=your_github_token

# Create release
make release-publish
```

## API Reference

The C64 Ultimate REST API documentation is available at:
<https://1541u-documentation.readthedocs.io/en/latest/api/api_calls.html>

## Troubleshooting

### Connection Issues

```bash
# Test if C64 Ultimate is reachable
ping 192.168.1.100

# Test HTTP connection
curl http://192.168.1.100/v1/version

# Use verbose mode to see HTTP details
c64u --verbose --host 192.168.1.100 api-version
```

### Configuration Issues

```bash
# Check current CLI configuration
c64u cli-config show

# Verify config file location
ls -la ~/.config/c64u/config.toml

# Override with environment variables
C64U_HOST=192.168.1.100 c64u api-version
```

## License

Apache 2.0

## Credits

- Built for the [Commodore C64 Ultimate](https://commodore.net)
- Based on Gideon's Logic Architectures Ultimate64 FPGA board
- Part of the [c64.nvim](../../README.md) project

## Contributing

Contributions are welcome! Please feel free to submit issues or pull requests.
