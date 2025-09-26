# macOS Build Instructions

## Overview

TreeOS now targets Podman for container management and no longer links against the Docker Compose SDK. As a result, there are no macOS-specific CGO dependencies that block cross compilation. The build steps below remain for reference when building natively on macOS or troubleshooting platform differences.

## The Issue

The previous limitation was caused by the `github.com/fsnotify/fsevents` package, which was pulled in by the Docker Compose SDK. That dependency has been removed.

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

## Cross Compilation

With the Podman migration the codebase no longer depends on macOS-only CGO packages. Cross-compiling from Linux should work with a recent Go toolchain and the standard macOS SDK headers. Native builds on macOS continue to work using the steps above.
