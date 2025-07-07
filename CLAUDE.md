Update the CLAUDE.md after every meaningful change with concise information. Make use of CLAUDE.mds in every folder. Make sure that the information is where it needs to be, as far down in the folders as possible. this keeps the main CLAUDE.md lean and structured.

# Project Overview

OnTree is a Docker container management application with a web interface for managing containerized applications.

## Recent Features

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