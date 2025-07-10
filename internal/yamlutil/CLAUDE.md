# YAML Utilities Package

This package provides utilities for reading and writing docker-compose.yml files while preserving formatting and comments.

## File Locking (2025-07-10)

Added thread-safe file locking mechanism to prevent race conditions during concurrent writes:

- `getFileLock(path)`: Returns a mutex for the given file path
- All write operations through `WriteComposeWithMetadata()` are automatically protected
- Uses a global map of file-specific mutexes to allow concurrent access to different files
- Lock is held only during the actual file write operation to minimize contention

## Key Features

- **Format Preservation**: Uses gopkg.in/yaml.v3 to maintain comments and formatting when updating YAML files
- **Atomic Writes**: Writes to temporary file first, then renames to prevent corruption
- **OnTree Metadata**: Manages the `x-ontree` extension field in docker-compose.yml files

## Structs

- `OnTreeMetadata`: Contains subdomain, host_port, and is_exposed fields
- `ComposeFile`: Represents docker-compose.yml structure with x-ontree extension

## Main Functions

- `ReadComposeWithMetadata(path)`: Reads compose file preserving structure
- `WriteComposeWithMetadata(path, compose)`: Writes compose file preserving formatting
- `ReadComposeMetadata(appPath)`: Helper to read just the metadata from app directory
- `UpdateComposeMetadata(appPath, metadata)`: Helper to update just the metadata

## Usage Example

```go
// Read metadata from an app
metadata, err := yamlutil.ReadComposeMetadata("/apps/myapp")

// Update metadata
metadata.Subdomain = "newsubdomain"
metadata.IsExposed = true
err = yamlutil.UpdateComposeMetadata("/apps/myapp", metadata)
```

## Implementation Notes

- The package uses yaml.Node internally to preserve document structure
- When x-ontree doesn't exist, it's inserted after the version field
- All file operations are atomic using temp file + rename pattern