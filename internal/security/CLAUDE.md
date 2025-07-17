# Security Validation Module

This package provides security validation for docker-compose.yml files before they are executed by OnTree.

## Overview

The security validator enforces strict rules to prevent potentially dangerous container configurations:

1. **Privileged Mode**: Containers cannot run with `privileged: true`
2. **Dangerous Capabilities**: Certain Docker capabilities are blacklisted (e.g., SYS_ADMIN, NET_ADMIN)
3. **Bind Mount Restrictions**: Host path bind mounts are only allowed within `/opt/ontree/apps/mount/{appName}/{serviceName}/`

## Usage

```go
validator := security.NewValidator("my-app")
err := validator.ValidateCompose(yamlContent)
if err != nil {
    // Handle validation error
}
```

## Security Rules

### Privileged Mode
- Any service with `privileged: true` will be rejected
- This prevents containers from having full host access

### Dangerous Capabilities
The following capabilities are not allowed:
- SYS_ADMIN
- NET_ADMIN
- SYS_MODULE
- SYS_RAWIO
- SYS_PTRACE
- SYS_BOOT
- MAC_ADMIN
- MAC_OVERRIDE
- DAC_READ_SEARCH
- SETFCAP

### Bind Mount Restrictions
- All bind mounts must be within `/opt/ontree/apps/mount/`
- Must follow the pattern: `/opt/ontree/apps/mount/{appName}/{serviceName}/`
- Named volumes are allowed without restrictions
- Relative paths are not allowed

## Implementation Details

The validator:
1. Parses the docker-compose.yml using yaml.v3
2. Iterates through each service
3. Checks each security rule in order
4. Returns immediately on the first violation found
5. Provides detailed error messages indicating which service and rule failed

## Testing

Comprehensive unit tests cover:
- Valid configurations
- Invalid YAML
- Privileged mode detection
- Capability validation (with various formats)
- Bind mount path validation
- Complex scenarios with multiple services