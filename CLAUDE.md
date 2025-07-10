Update the CLAUDE.md after every meaningful change with concise information. Make use of CLAUDE.mds in every folder. Make sure that the information is where it needs to be, as far down in the folders as possible. this keeps the main CLAUDE.md lean and structured.

# Project Overview

OnTree is a Docker container management application with a web interface for managing containerized applications.

## Recent Features

### Data Migration Script (2025-07-10 - Ticket 2)

Added migration command to move app metadata from database to docker-compose.yml files:
- Run with: `./ontree-server migrate-to-compose`
- Creates timestamped backups of all compose files before modification
- Migrates subdomain, host_port, and is_exposed fields to x-ontree section
- Provides detailed logging and error handling
- See `internal/migration/CLAUDE.md` for implementation details

### YAML Utilities Package (2025-07-10)

Added yamlutil package for managing docker-compose.yml metadata:
- Preserves comments and formatting when updating YAML files
- Manages OnTree metadata in `x-ontree` extension field
- See `internal/yamlutil/CLAUDE.md` for implementation details

### Caddy Integration UI (2025-07-10)

Added Domain & Access section to app detail page for exposing apps via Caddy reverse proxy:

1. **UI States**: The Domain & Access section displays differently based on prerequisites:
   - **Caddy not available**: Shows disabled form with warning message
   - **No domains configured**: Shows disabled form with info message to configure domains in settings
   - **App not exposed**: Shows form to enter subdomain and expose app
   - **App exposed**: Shows current subdomain, access URLs, and unexpose button

2. **Features**:
   - Subdomain input with validation (lowercase letters, numbers, hyphens only)
   - Shows domain suffix preview (.yourdomain.com)
   - Remembers previously used subdomain
   - Two-step confirmation for unexpose action
   - Check Status button for exposed apps

3. **Integration**: Uses existing backend handlers:
   - POST `/apps/{name}/expose` - Expose app with subdomain
   - POST `/apps/{name}/unexpose` - Remove app from Caddy
   - GET `/api/apps/{name}/status` - Check subdomain availability

The UI gracefully degrades when prerequisites aren't met, always showing users what features are available.

#### Configuration

To enable the Caddy integration feature:

1. **Install and configure Caddy** on the host system with admin API enabled:
   ```
   {
       admin localhost:2019
   }
   ```

2. **Configure domains** via environment variables or config.toml:
   - `PUBLIC_BASE_DOMAIN` - Your public domain (e.g., `homelab.com`)
   - `TAILSCALE_BASE_DOMAIN` - Your Tailscale domain (e.g., `myserver.tailnet.ts.net`)

3. **Ensure OnTree has Docker permissions** to manage containers

When properly configured, apps can be exposed at `https://subdomain.yourdomain.com` with automatic HTTPS certificates managed by Caddy.

### CI Test Fixes (2025-07-08)

Fixed CI test failures for v0.1.1 release:

1. **Template Nil Pointer Errors**: Fixed nil pointer dereference errors in dashboard templates when accessing `.Config.Container.Image` and `.Config.Ports`. Added proper nil checks using Go template's `and` operator.

2. **Port Configuration Mismatch**: Fixed port mismatch in GitHub Actions workflow where the server was started on port 8085 but tests expected port 3001. Updated workflow to use port 3001 consistently.

The fixes ensure that:
- Dashboard templates handle apps without config data gracefully
- E2E tests run successfully in CI environment
- No template rendering errors occur when displaying app information

### Improved Container Operation UI (2025-07-07)
Enhanced the container operation experience with better visual feedback:

- **Dynamic Button States**: During operations, buttons show appropriate text:
  - "Creating & Starting..." when creating a new container
  - "Processing..." for other operations
- **Operation Lock**: All control buttons are disabled during operations to prevent conflicts
- **Auto-refresh**: Controls automatically update when operations complete
- **Seamless Updates**: Uses HTMX to refresh controls without page reload

### Real-Time Operation Logging (2025-07-07)
The application now includes comprehensive logging for all Docker operations:

- **Log Viewer UI**: Displays real-time logs on the app detail page
- **Operation Tracking**: Shows what's happening during container operations
- **Debug Information**: Includes equivalent Docker commands and API calls
- **Auto-scroll**: Logs automatically scroll to show latest entries
- **Persistent Storage**: Logs are stored in database for debugging

When you click "Create & Start" or any container operation:
1. The button changes to show operation status with a spinner
2. The operation logs panel appears automatically
3. You can see detailed step-by-step progress
4. Any errors are clearly highlighted in red
5. The equivalent Docker commands are shown for transparency
6. Controls refresh automatically when the operation completes

### Stale Operation Handling (2025-07-07)
Fixed issue where old pending operations would show spinner indefinitely:
- Operations older than 5 minutes are filtered from UI
- Background cleanup marks stale operations as failed
- See `internal/server/CLAUDE.md` for details

## Important Instruction Reminders

- Do what has been asked; nothing more, nothing less
- NEVER create files unless they're absolutely necessary for achieving your goal
- ALWAYS prefer editing an existing file to creating a new one
- NEVER proactively create documentation files (*.md) or README files unless explicitly requested

## Architecture

- **Backend**: Go with Gorilla/mux
- **Frontend**: HTMX + Bootstrap
- **Database**: SQLite
- **Container Management**: Docker API
- **Background Jobs**: Worker pool pattern
- **Asset Embedding**: Static files and templates are embedded into the binary using Go's embed package

## Build Process

### Asset Embedding (2025-07-07)
The application now embeds all static assets (CSS, fonts) and HTML templates directly into the binary:
- Assets are copied to `internal/embeds/` during build via `make embed-assets`
- The binary is self-contained and can run without the `static/` and `templates/` directories
- This makes deployment simpler and supports the goal of single-binary distribution

See subdirectory CLAUDE.md files for component-specific documentation.