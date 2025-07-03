# Software Specification: onTree Node (Go Rewrite)

## 1. Introduction

This document outlines the software specification for rewriting the onTree Node project from Python/Django to the Go programming language. The goal is to replicate the existing functionality and user experience as closely as possible, using standard Go libraries and best practices.

## 2. Core Features

The Go implementation will include the following core features:

*   **Web-based Dashboard:** A web interface for system monitoring and application management.
*   **System Vitals Monitoring:** Display CPU, memory, and disk usage.
*   **Docker Application Management:**
    *   Deploy applications from `docker-compose.yml` files.
    *   Start, stop, restart, and delete Docker containers.
    *   View container status and logs.
    *   Manage application configurations.
*   **Application Templates:** A system for creating new applications from predefined templates.
*   **Setup Wizard:** An initial setup process for new installations.
*   **User Authentication:** A login system to protect the dashboard.
*   **Pattern Library:** A separate section to display UI components.
*   **CLI for Directory Setup:** A command-line interface to set up the necessary directories with correct permissions.

## 3. Architecture

The application will be a monolithic Go application with an embedded web server. It will follow a standard Go project layout.

### 3.1. Project Structure

```
/ontree-go
|-- /cmd
|   |-- /ontree-server
|       |-- main.go
|-- /internal
|   |-- /server
|   |   |-- server.go
|   |   |-- routes.go
|   |   |-- handlers.go
|   |   |-- middleware.go
|   |-- /system
|   |   |-- vitals.go
|   |-- /docker
|   |   |-- client.go
|   |   |-- apps.go
|   |   |-- templates.go
|   |-- /database
|   |   |-- database.go
|   |   |-- models.go
|   |-- /config
|       |-- config.go
|-- /templates
|   |-- /layouts
|   |   |-- base.html
|   |-- /dashboard
|   |   |-- index.html
|   |   |-- ... (other dashboard templates)
|   |-- /pattern_library
|   |   |-- ... (pattern library templates)
|-- /static
|   |-- /css
|   |-- /js
|   |-- /fonts
|-- go.mod
|-- go.sum
```

### 3.2. Key Components

*   **`main.go`:** The main entry point of the application. It will handle command-line arguments to either start the web server or run the directory setup command.
*   **`server.go`:** The web server implementation, using the standard `net/http` package.
*   **`routes.go`:** Defines the application's URL routes and maps them to handler functions.
*   **`handlers.go`:** Contains the HTTP handler functions for each route.
*   **`middleware.go`:** Implements middleware for authentication, logging, and other cross-cutting concerns.
*   **`vitals.go`:** Contains the logic for collecting system vitals (CPU, memory, disk) using a library like `gopsutil`.
*   **`docker/` package:** Encapsulates all Docker-related functionality, using the official Docker Go SDK.
*   **`database/` package:** Manages the database connection and data access, using the `database/sql` package and a lightweight ORM or direct SQL queries. The database will be SQLite to match the original project.
*   **`config/` package:** Handles application configuration, reading from environment variables and a configuration file.

## 4. Frontend

The frontend will be rendered on the server-side using Go's `html/template` package. It will replicate the existing HTML structure, CSS, and JavaScript.

*   **HTML Templates:** The existing Django templates will be converted to Go templates. This includes the base layout, partials, and page-specific templates.
*   **CSS and JavaScript:** The existing static assets (CSS, JS, fonts) will be used as-is.
*   **HTMX:** The project will continue to use HTMX for asynchronous updates, such as the system vitals and Docker operation progress. Go handlers will be created to serve the HTML partials required by HTMX.

## 5. Database

The application will use a SQLite database, consistent with the original project. The database schema will be replicated to store:

*   **`users`:** User accounts for authentication.
*   **`system_vitals_logs`:** Time-series data for system vitals.
*   **`system_setup`:** A singleton table to track the setup status.
*   **`docker_operations`:** A log of background Docker operations.

## 6. Docker Integration

The Go application will interact with the Docker daemon using the official Docker Go SDK. The `internal/docker` package will provide the following functionality:

*   **`scan_apps()`:** Scans a directory for `docker-compose.yml` files to discover applications.
*   **`get_app_details()`:** Retrieves the configuration and status of a specific application.
*   **`start_app()`, `stop_app()`, `recreate_app()`:** Manages the lifecycle of application containers.
*   **`pull_image_with_progress()`:** Pulls Docker images and provides real-time progress updates, which will be streamed to the frontend via HTMX.

## 7. Background Operations

Similar to the original project, long-running Docker operations (e.g., pulling images) will be executed in the background to avoid blocking the UI. This will be achieved using Go's built-in concurrency features (goroutines and channels).

*   A background worker pool will be implemented to manage these operations.
*   The status of each operation will be tracked in the `docker_operations` database table.
*   The frontend will use HTMX to poll for progress updates.

## 8. Configuration

The application will be configured through a combination of a configuration file (e.g., `config.toml`) and environment variables. Key configuration options will include:

*   `ONTREE_APPS_DIR`: The directory where applications are stored. Defaults to `/opt/ontree/apps` on Linux and `./apps` on macOS.
*   `DATABASE_PATH`: The path to the SQLite database file.
*   `LISTEN_ADDR`: The address and port for the web server.

## 9. Directory Setup Command

A command-line interface will be provided to set up the application directories. This is analogous to the `setup_dirs` management command in the original project.

*   **Command:** `ontree-server setup-dirs`
*   **Functionality:**
    *   On Linux, this command will require `sudo`.
    *   It will create the `/opt/ontree/apps` directory.
    *   It will set the appropriate ownership and permissions on the directory to allow the `ontree` user/group to write to it.
    *   On macOS, it will simply create a local `./apps` directory without requiring `sudo`.

## 10. API Endpoints

The following is a preliminary list of API endpoints (routes) that will be implemented:

| Method | Path                                      | Description                                      |
|--------|-------------------------------------------|--------------------------------------------------|
| GET    | /                                         | Main dashboard                                   |
| GET    | /login                                    | Login page                                       |
| POST   | /login                                    | Handle login form submission                     |
| GET    | /logout                                   | Log the user out                                 |
| GET    | /setup                                    | Setup wizard page                                |
| POST   | /setup                                    | Handle setup form submission                     |
| GET    | /apps/{app_name}                          | Application detail page                          |
| POST   | /apps/{app_name}/start                    | Start an application                             |
| POST   | /apps/{app_name}/stop                     | Stop an application                              |
| POST   | /apps/{app_name}/recreate                 | Recreate an application                          |
| POST   | /apps/{app_name}/delete                   | Delete an application container                  |
| GET    | /apps/create                              | Create new application page                      |
| POST   | /apps/create                              | Handle create application form submission        |
| GET    | /apps/{app_name}/edit                     | Edit application configuration page              |
| POST   | /apps/{app_name}/edit                     | Handle edit application form submission          |
| GET    | /templates                                | Application templates page                       |
| GET    | /templates/{template_id}/create           | Create application from template page            |
| POST   | /templates/{template_id}/create           | Handle create from template form submission      |
| GET    | /api/system-vitals                        | Get system vitals (for HTMX)                     |
| GET    | /api/docker/operations/{operation_id}     | Get Docker operation status (for HTMX)           |
| GET    | /patterns                                 | Pattern library                                  |

## 11. Dependencies

The following Go libraries will be used:

*   **`net/http`:** For the web server.
*   **`html/template`:** For server-side HTML rendering.
*   **`database/sql`:** For database interaction.
*   **`mattn/go-sqlite3`:** SQLite driver.
*   **`docker/docker`:** Official Docker Go SDK.
*   **`shirou/gopsutil`:** For collecting system vitals.
*   **`gorilla/mux` or `chi`:** (Optional) For more advanced routing.
*   **`bcrypt`:** For password hashing.

## 12. Development Approach

### 12.1. Overall Goal

The primary objective of this project is to perform a **1:1 rewrite** of the original Python/Django onTree application into the Go programming language.

- **Feature Parity:** The Go version should replicate the functionality of the original application as closely as possible.
- **No Unsolicited Enhancements:** Do not add new features or make significant architectural changes unless they are explicitly outlined in a ticket in the `tickets.md` file.

### 12.2. Source of Truth

If any requirement in this specification or any task in `tickets.md` is ambiguous or lacks detail, the **original Python codebase in the parent directory is the definitive source of truth.** The Go implementation should mimic the behavior and logic observed in the Python code.

### 12.3. Development Workflow

The development process should strictly follow the tickets outlined in `specifications/tickets.md`.

1. **Sequential Order:** Complete the tickets in the specified order.
2. **Verification:** After implementing a ticket, perform all verification steps listed for that ticket.
3. **Commit After Each Ticket:** Once a ticket's implementation is complete and verified, commit the changes to the Git repository with the suggested commit message. This creates a clean, step-by-step history of the project's development.

## 13. Testing Philosophy

A robust test suite is a critical requirement for this project.

### 13.1. Unit Testing

- **Reference Existing Tests:** The tests in the original Python project (`/dashboard/tests/`) should be used as a reference for the structure, scope, and style of the new tests.
- **Create New Tests:** For any new functionality or refactored logic, create corresponding tests in a `_test.go` file within the same package.
- **Test Command:** All tests should be runnable using the standard `go test ./...` command from the project root.

### 13.2. Verification During Development

During the implementation of tickets, the coding agent should:
- Use Playwright MCP for **manual walkthroughs** to verify that features work correctly
- Focus on quick functionality verification rather than writing extensive tests
- Avoid writing Playwright tests after each ticket implementation

### 13.3. Comprehensive End-to-End Testing

After all features are implemented (see Ticket 19), the agent should:
- Perform a complete walkthrough of all major user journeys
- Record comprehensive Playwright tests while verifying functionality
- Ensure all critical paths are covered with automated tests

### 13.4. Testing Strategy Details

The testing strategy is divided into two distinct phases:

1. **Development Phase (Tickets 1-18):** Quick manual verification using Playwright MCP
2. **Testing Phase (Ticket 19):** Comprehensive automated test creation

#### During Development Phase

**When to Use Playwright MCP:**
- Manual walkthroughs to verify features work as expected
- Quick functionality checks without writing test code
- Visual verification of UI elements and layouts
- Interaction testing to ensure buttons, forms, and navigation work

**What NOT to Do:**
- Do NOT write Playwright test files after each ticket
- Do NOT spend time creating test assertions and expectations
- Do NOT build a test suite incrementally
- Do NOT focus on test coverage during implementation

#### During Testing Phase (Ticket 19)

**Test Organization Structure:**
```
tests/e2e/
├── auth/
│   ├── setup.test.js      # Initial setup wizard tests
│   └── login.test.js      # Authentication flow tests
├── dashboard/
│   └── vitals.test.js     # System vitals monitoring tests
├── apps/
│   ├── create.test.js     # App creation tests
│   ├── manage.test.js     # Start/stop/recreate tests
│   └── templates.test.js  # Template system tests
└── docker/
    └── updates.test.js    # Image update tests
```

**Benefits of This Approach:**
1. Faster Development: No time wasted on premature test creation
2. Better Test Quality: Tests written with full application context
3. Comprehensive Coverage: All features tested together as a system
4. Maintainable Tests: Tests reflect the final implementation, not intermediate states

This specification provides a comprehensive overview of the onTree Node rewrite in Go. The focus is on maintaining feature parity with the original Django application while leveraging the strengths of the Go language for performance and concurrency.