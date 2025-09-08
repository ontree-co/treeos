# macOS Build Instructions

## Overview

Due to a dependency on `fsevents` (a macOS-specific file system events library) through Docker Compose, cross-compiling macOS binaries from Linux is not currently supported in our automated release process.

## The Issue

The `github.com/fsnotify/fsevents` package:
- Is macOS-specific and uses CGO
- Is required by `github.com/docker/compose/v2/pkg/watch`
- Cannot be cross-compiled from Linux to macOS
- Has build constraints (`//go:build darwin`) that are not properly handled during cross-compilation

## Building for macOS Locally

macOS users can build the binary locally:

```bash
# Clone the repository
git clone https://github.com/stefanmunz/treeos.git
cd treeos

# Build for your platform
make build

# Or build specifically for macOS ARM64
GOOS=darwin GOARCH=arm64 make build

# Install to /usr/local/bin
sudo cp build/treeos /usr/local/bin/treeos
```

## Future Solutions

Potential solutions being considered:
1. Use GitHub Actions macOS runners for building macOS releases
2. Replace Docker Compose dependency with a lighter alternative
3. Create build tags to exclude watch functionality when not needed
4. Use a Docker-based cross-compilation solution with proper macOS SDK

## Linux Releases

Linux releases (AMD64 and ARM64) are fully automated and available from the GitHub releases page.