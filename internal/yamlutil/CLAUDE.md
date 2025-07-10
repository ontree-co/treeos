# YAML Utilities Package Documentation

## Overview

The `yamlutil` package provides utilities for managing docker-compose.yml files with OnTree-specific metadata stored in the `x-ontree` extension field.

## Key Features

- Preserves YAML formatting and comments when updating files
- Thread-safe file operations with per-file locking
- Supports OnTree metadata in the `x-ontree` extension field
- Validates docker-compose.yml syntax and structure

## OnTreeMetadata Structure

```go
type OnTreeMetadata struct {
    Subdomain string `yaml:"subdomain,omitempty"`
    HostPort  int    `yaml:"host_port,omitempty"`
    IsExposed bool   `yaml:"is_exposed"`
    Emoji     string `yaml:"emoji,omitempty"`  // Added 2025-07-10
}
```

### Fields:
- **Subdomain**: The subdomain used when exposing the app via Caddy
- **HostPort**: The host port mapping for the application
- **IsExposed**: Whether the app is currently exposed via Caddy
- **Emoji**: Optional emoji to display with the app (added for UI improvements)

## Emoji Support (Added 2025-07-10)

### Validation
- Only emojis from the `AppEmojis` curated list are allowed
- Empty emoji is valid (optional field)
- Validation function: `IsValidEmoji(emoji string) bool`

### Curated Emoji List
The `AppEmojis` slice contains ~100 app-appropriate emojis organized by category:
- Development & Technology (💻, 🖥️, ⌨️, etc.)
- Data & Analytics (📊, 📈, 📉, etc.)
- Security & Monitoring (🔒, 🔓, 🔐, etc.)
- Communication & Media (📧, 📨, 📩, etc.)
- Storage & Database (🗃️, 🗳️, 📦, etc.)
- Nature & Science (🌍, 🌎, 🌏, etc.)

### Storage Format
Emojis are stored in the docker-compose.yml file under the `x-ontree` section:

```yaml
version: '3.8'
x-ontree:
  subdomain: myapp
  host_port: 8080
  is_exposed: true
  emoji: "🚀"
services:
  myapp:
    image: nginx:latest
```

## Key Functions

### ReadComposeWithMetadata(path string) (*ComposeFile, error)
Reads a docker-compose.yml file preserving formatting and comments.

### WriteComposeWithMetadata(path string, compose *ComposeFile) error
Writes a docker-compose.yml file preserving formatting and comments. Uses file locking to prevent race conditions.

### ReadComposeMetadata(appPath string) (*OnTreeMetadata, error)
Helper function that reads only the OnTree metadata from a compose file in the given app directory.

### UpdateComposeMetadata(appPath string, metadata *OnTreeMetadata) error
Helper function that updates only the OnTree metadata in a compose file, preserving all other content.

### ValidateComposeFile(content string) error
Validates the YAML syntax and basic structure of a docker-compose file.

### IsValidEmoji(emoji string) bool
Validates that an emoji is in the allowed list. Empty string is considered valid.

## File Locking

The package implements file-level locking to prevent race conditions during concurrent access:
- Each file path gets its own mutex
- Locks are acquired during write operations
- Ensures data consistency when multiple handlers access the same compose file

## Testing

Comprehensive test coverage includes:
- Basic read/write operations
- Comment and formatting preservation
- Metadata updates
- Empty metadata handling
- Emoji validation and storage
- Emoji persistence through updates
- YAML validation edge cases

Run tests with: `go test ./internal/yamlutil/...`