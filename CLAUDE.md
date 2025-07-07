Update the CLAUDE.md after every meaningful change with concise information. Make use of CLAUDE.mds in every folder. Make sure that the information is where it needs to be, as far down in the folders as possible. this keeps the main CLAUDE.md lean and structured.

# Project Overview

OnTree is a Docker container management application with a web interface for managing containerized applications.

## Recent Features

### Real-Time Operation Logging (2025-07-07)
The application now includes comprehensive logging for all Docker operations:

- **Log Viewer UI**: Displays real-time logs on the app detail page
- **Operation Tracking**: Shows what's happening during container operations
- **Debug Information**: Includes equivalent Docker commands and API calls
- **Auto-scroll**: Logs automatically scroll to show latest entries
- **Persistent Storage**: Logs are stored in database for debugging

When you click "Create & Start" or any container operation:
1. The operation logs panel appears automatically
2. You can see detailed step-by-step progress
3. Any errors are clearly highlighted in red
4. The equivalent Docker commands are shown for transparency

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

See subdirectory CLAUDE.md files for component-specific documentation.