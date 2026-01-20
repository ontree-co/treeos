Of course. Here is a comprehensive summary document outlining the complete architecture we've designed. This document serves as a blueprint for building your `OnTree` application.

---

### **OnTree Application: Architectural Design Document**

#### 1. Executive Summary

This document outlines the architecture for the **OnTree** application, a tool for managing the lifecycle of containerized applications and local LLM models. The proposed architecture is a unified Go application that serves both as a command-line interface (CLI) for power-users and agents, and as a backend for a web UI.

The core design principle is **separation of concerns**, achieved by isolating the application's business logic into a central, reusable "engine" package. This engine is then consumed by two thin wrappers: one for the CLI and one for the web server. This approach maximizes code reuse, testability, and performance while delivering a consistent user experience across both interfaces.

#### 2. Core Architectural Principles

*   **Single Binary Deployment:** The entire application—CLI and web server—is compiled into a single executable file (`ontree`). This simplifies deployment, eliminates versioning conflicts between components, and provides a "Swiss Army knife" experience for users.
*   **Separation of Logic and Presentation:** The complex business logic (interacting with Docker/Compose, Tailscale, Ollama, etc.) is completely decoupled from the presentation layers (the CLI's console output and the web server's HTTP/SSE handling).
*   **Asynchronous Operations:** Long-running tasks, such as pulling container images, are handled asynchronously using Go channels to provide real-time progress feedback to the user, whether on the command line or in the web UI.
*   **Centralized Configuration and Control:** The application's root (`main.go`) is the single point of control for configuring cross-cutting concerns like logging and telemetry.
*   **Dependency Injection:** Components receive their dependencies (like loggers or configuration) during initialization, making them highly testable and decoupled from global state.

#### 3. Project Structure

The project will be organized into a clean, scalable structure that reflects the separation of concerns.

```
/ontree/
├── go.mod
├── main.go               # Main entrypoint. Initializes logging, CLI commands, and starts the app.

├── cmd/                  # Defines the CLI commands using the Cobra library (CLI-first).
│   ├── root.go           # The root 'ontree' command, configures logging via flags.
│   ├── serve.go          # The 'ontree serve' subcommand; starts the web server.
│   ├── app.go            # Defines 'ontree app <install|start|stop|list|health>' subcommands.
│   └── model.go          # Defines 'ontree model <install|list|health>' subcommands.

├── web/                  # Web-specific components.
│   ├── handlers.go       # HTTP handlers for REST API and SSE endpoints.
│   └── server.go         # Function to set up routes and start the HTTP server.

└── internal/
    └── ontree/           # THE CORE ENGINE: All business logic lives here.
        ├── manager.go    # Defines the 'Manager' struct and high-level orchestration methods (Start, Stop).
        ├── types.go      # Core domain types (App, Model, Config, ProgressEvent).
        ├── docker.go     # Low-level functions for interacting with Docker/Compose.
        ├── ollama.go     # Low-level functions for interacting with Ollama models.
        └── tailscale.go  # Low-level functions for interacting with Tailscale.
```

#### 4. Component Deep Dive

##### 4.1. The Single Binary Entrypoint (`main.go`)

*   **Role:** The "main" function acts as a router. It uses the [Cobra](https://github.com/spf13/cobra) library to parse command-line arguments. The CLI is the primary interface for development and test automation.
*   **Functionality:**
    *   If the command is `ontree serve`, it will initialize and run the web server.
    *   If the command is `ontree app start ...`, it will execute the corresponding CLI logic.
    *   **Owns Logging/Telemetry Configuration:** It reads flags (e.g., `--log-level=debug`, `--log-format=json`) to configure the global `slog.Logger` and OpenTelemetry providers. This configuration is done once at startup.

##### 4.2. The Core Engine (`internal/ontree/`)

This is the heart of the application. It is a pure Go library with no knowledge of HTTP or command-line flags.

*   **`manager.go`:**
    *   Defines a `Manager` struct which holds dependencies like the logger (`*slog.Logger`).
    *   Exposes high-level, asynchronous methods like `StartApp(ctx context.Context, ...) <-chan ProgressEvent` and `InstallModel(ctx context.Context, ...) <-chan ProgressEvent`. These methods orchestrate the calls to the low-level `docker.go`, `ollama.go`, and `tailscale.go` modules.
    *   It immediately returns a read-only channel (`<-chan ProgressEvent`).
    *   It performs its work in a background goroutine, sending `ProgressEvent` updates (logs, errors, success) to the channel.
    *   Crucially, it accepts a `context.Context` to enable cancellation of long-running operations (e.g., if a web user closes their browser).

*   **`types.go`:**
    *   Defines the `ProgressEvent` struct, which is the universal communication object for asynchronous operations. Transport is presentation-layer specific: CLI prints lines; web uses SSE (preferred for HTMX) or WebSocket if needed.
    ```go
    type ProgressEvent struct {
        Type    ProgressEventType `json:"type"` // e.g., "log", "error", "success"
        Message string            `json:"message"`
    }
    ```

*   **`docker.go` / `ollama.go` / `tailscale.go`:**
    *   Contain the low-level, implementation-specific details.
    *   Functions here are responsible for safely constructing and executing external commands (`exec.CommandContext`) and parsing their output.

##### 4.3. The CLI Consumer (`cmd/`)

*   **Role:** Provides a thin wrapper around the Core Engine for command-line users. The CLI is the primary driver for development flows and automated E2E testing.
*   **Functionality:**
    *   Parses command-line arguments and flags.
    *   Retrieves the logger instance initialized in `main.go`.
    *   Creates an instance of `ontree.Manager`, injecting the logger.
    *   Calls the appropriate manager method (e.g., `manager.StartApp(...)`, `manager.InstallModel(...)`).
    *   It then simply loops over the returned `ProgressEvent` channel and prints messages to `stdout` or `stderr`.

##### 4.4. The Web Consumer (`web/`)

*   **Role:** Provides a thin wrapper around the Core Engine for web UI users. Web features can lag behind the CLI where needed; the CLI remains the contract for automation.
*   **Functionality:**
    *   The `serve` command initializes an HTTP server with defined routes.
    *   For long-running actions (like starting an app), the HTTP handler will use **SSE** (preferred for HTMX). WebSocket is optional if SSE does not meet a requirement.
    *   It retrieves the logger and creates an `ontree.Manager`.
    *   It calls the *exact same* manager method (`manager.Start(...)`), passing the `http.Request.Context()`.
    *   It loops over the returned `ProgressEvent` channel, marshals each event to JSON, and sends it over the WebSocket to the frontend client.

#### 5. Logging and Telemetry Strategy

*   **Ownership:** Logging and telemetry are configured **once** in `main.go` and **injected** into all other components. The Core Engine and other packages are consumers, not configurators.
*   **Logging:** Use the standard library's `slog` package for structured logging. The format (text vs. JSON) and level are controlled by command-line flags, allowing for easy switching between development and production environments.
*   **Telemetry:** Follows the same dependency injection pattern using OpenTelemetry. The `main.go` function initializes providers and exporters. The `ontree.Manager` receives a `Tracer` and `Meter` to create spans for operations and record metrics.

#### 6. Architectural Flow: Starting an App

1.  **User Action:**
    *   **CLI:** User runs `ontree app start my-app --log-level=debug`.
    *   **Web:** User clicks a "Start" button in the UI, which opens an SSE connection to `/api/apps/start/my-app`.

2.  **Initialization (`main.go`):**
    *   The `PersistentPreRun` in Cobra parses the `--log-level` flag and configures a global `DEBUG` logger.

3.  **Entrypoint (`cmd/` or `web/`):**
    *   The corresponding command or HTTP handler is invoked.
    *   It retrieves the configured logger.
    *   It creates an `ontree.Manager`, passing the logger to it: `manager := ontree.NewManager(logger)`.

4.  **Core Logic Invocation:**
    *   The entrypoint calls `progressChan := manager.Start(ctx, "my-app", config)`.
    *   The `Start` function returns *immediately*, providing the `progressChan`.

5.  **Asynchronous Execution (`internal/ontree/`):**
    *   A goroutine inside `Start` begins its work.
    *   It sends an event: `progressChan <- ProgressEvent{Type: "log", Message: "Pulling image..."}`.
    *   It executes `docker compose pull...`, streaming `stdout` line-by-line and sending each line as a `log` event over the channel.
    *   If an error occurs, it sends an `error` event.
    *   When finished, it sends a `success` event and closes the channel.

6.  **Feedback to User:**
    *   **CLI:** The `for event := range progressChan` loop prints each `event.Message` to the console as it arrives.
    *   **Web:** The SSE handler's `for` loop receives each event, converts it to JSON, and streams it to the frontend, which dynamically updates the UI.

#### 7. Model Management Flow (Ollama)

*   **Goal:** Treat local models as first-class resources that can be installed, listed, and verified from the CLI and reused by apps.
*   **Core API:** `InstallModel(ctx, name, options) <-chan ProgressEvent`, `ListModels() ([]Model, error)`, `HealthModel(ctx, name) error`.
*   **Implementation:** `ollama.go` uses `exec.CommandContext` to run `ollama pull`, parses progress, and validates availability via `ollama list` or an HTTP health probe.

#### 8. Health Checks and Verification

*   **Goal:** Provide CLI-first health checks to enable automated E2E verification on a fresh machine.
*   **Core API:** `HealthApp(ctx, name) error`, `HealthModel(ctx, name) error`.
*   **Signals:** container status, compose service state, optional HTTP endpoint checks, and model availability checks.
*   **Web UI:** Health checks can be wired later; the CLI is the canonical source.

#### 9. E2E Harness (Bare Machine)

*   **Assumption:** Provisioning is handled by the existing setup script in `skills/treeos-setup-production-cloud-cpu/run.sh`.
*   **E2E flow:** install TreeOS, run initial setup, install Ollama (`ollama-cpu`), pull a small model (`gemma3:270m`), install an app that depends on Ollama (OpenWebUI), start it, then verify health checks including an HTTP 200 probe and a real LLM interaction (send prompt, receive response, verify conversation context is stored). Initial OpenWebUI user setup may be seeded via env/config for the first pass.
*   **OpenWebUI bootstrap (env):** `TREEOS_OPENWEBUI_ADMIN_EMAIL`, `TREEOS_OPENWEBUI_ADMIN_PASSWORD`, `TREEOS_OPENWEBUI_ADMIN_NAME` are injected into the OpenWebUI `.env` as `WEBUI_ADMIN_EMAIL`, `WEBUI_ADMIN_PASSWORD`, `WEBUI_ADMIN_NAME` during `ontree app install openwebui`.
*   **OpenWebUI API flow:** Authenticate via `POST /api/v1/auths/signin`, then call `POST /api/chat/completions` (with Bearer token) and persist a chat via `POST /api/v1/chats/new`, verify via `GET /api/v1/chats/{id}`.
*   **E2E runner:** `tests/integration/openwebui_e2e.sh` executes the HTTP API verification using the seeded admin account.
*   **Test harness:** deterministic CLI scripts (or Go-based runner) that invoke `ontree` commands, assert outputs, and clean up afterward.

#### 10. CLI Contract (Automation-First)

*   **Goal:** The CLI is the primary contract for automated testing and advanced workflows. Web UI can lag behind.
*   **Output Modes:**
    *   **Default:** human-readable streaming output.
    *   **`--json`:** machine-readable JSON events per line (JSONL). Use for E2E and integration tests.
*   **Exit Codes:** `0` success, `1` runtime error, `2` invalid usage.
*   **Command Surface (initial):**
    *   `ontree setup init --username <name> --password <pass> [--node-name <name>] [--node-icon <icon>] [--json]`
    *   `ontree setup status [--json]`
    *   `ontree app list [--json]`
    *   `ontree app install <app> [--version <v>] [--env <path>] [--json]`
    *   `ontree app start <app> [--json]`
    *   `ontree app stop <app> [--json]`
    *   `ontree app health <app> [--http <url>] [--timeout 180s] [--interval 3s] [--json]`
    *   `ontree model list [--json]`
    *   `ontree model install <model> [--json]`
    *   `ontree model health <model> [--timeout 180s] [--interval 3s] [--json]`
*   **Health Semantics:**
    *   App health validates container/compose state.
    *   If `--http` is provided, it additionally polls HTTP until 2xx/3xx or timeout.
    *   Health commands must be suitable for waiting until readiness, not just immediate status.
*   **Event JSONL Format:**
```json
{"type":"log","message":"Pulling image: ollama:latest"}
{"type":"progress","message":"Download 45%","percent":45}
{"type":"success","message":"Started app: my-app"}
{"type":"error","message":"compose up failed","code":"compose_error"}
```

This architecture provides a robust, testable, and maintainable foundation for building the OnTree application, ensuring consistency and high performance for both CLI and web users.
