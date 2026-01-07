# Creating a GitHub Release for c64u

## Prerequisites

1. GitHub CLI (`gh`) installed
   ```bash
   # macOS
   brew install gh

   # Login
   gh auth login
   ```

2. All binaries built
   ```bash
   cd tools/c64u
   make release
   ```

## Release Process

### 1. Create a Git Tag

```bash
# From the c64.nvim root directory
git tag -a v0.1.0 -m "Release v0.1.0: c64u CLI for C64 Ultimate"
git push origin v0.1.0
```

### 2. Create GitHub Release with Binaries

```bash
cd tools/c64u

# Create release and upload all binaries
gh release create v0.1.0 \
  --title "c64u v0.1.0 - C64 Ultimate CLI" \
  --notes "First release of c64u CLI tool for C64 Ultimate hardware control.

## Features
- Complete REST API coverage (45+ commands)
- Configuration via TOML file, environment variables, or CLI flags
- Multiple output formats (text/JSON/verbose)
- Neovim integration
- Cross-platform support

## Installation

Download the appropriate binary for your platform and follow the instructions in the [README](https://github.com/cybersorcerer/c64.nvim#installing-c64u-cli).

## Binaries

- **macOS Apple Silicon**: c64u-darwin-arm64
- **macOS Intel**: c64u-darwin-amd64
- **Linux x86_64**: c64u-linux-amd64
- **Linux ARM64**: c64u-linux-arm64
- **Windows**: c64u-windows-amd64.exe" \
  dist/c64u-darwin-arm64 \
  dist/c64u-darwin-amd64 \
  dist/c64u-linux-amd64 \
  dist/c64u-linux-arm64 \
  dist/c64u-windows-amd64.exe
```

### 3. Verify the Release

```bash
# List releases
gh release list

# View release details
gh release view v0.1.0 --web
```

## Release Checklist

- [ ] All binaries built successfully (`make release`)
- [ ] Binaries tested on at least one platform
- [ ] README.md updated with installation instructions
- [ ] CHANGELOG updated (if exists)
- [ ] Git tag created and pushed
- [ ] GitHub release created with all binaries
- [ ] Release notes complete
- [ ] Installation instructions tested

## Future Releases

For subsequent releases:

1. Update version in code (if needed)
2. Create new tag: `git tag -a v0.1.1 -m "Release v0.1.1: ..."`
3. Push tag: `git push origin v0.1.1`
4. Rebuild binaries: `make release`
5. Create GitHub release with new binaries

## Quick Release Script

You can also use this one-liner (adjust version number):

```bash
VERSION=v0.1.0 && \
git tag -a $VERSION -m "Release $VERSION" && \
git push origin $VERSION && \
cd tools/c64u && \
make release && \
gh release create $VERSION \
  --title "c64u $VERSION" \
  --notes "See [README](https://github.com/cybersorcerer/c64.nvim#installing-c64u-cli) for installation instructions." \
  dist/*
```
