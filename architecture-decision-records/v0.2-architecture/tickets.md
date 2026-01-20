Of course. Refactoring a "big blob" monolith requires a methodical, step-by-step approach where each step results in a stable, testable state. Large, deliberate tickets are perfect for this.

Here is a list of tickets designed to guide you through the refactoring of your existing application into the new architecture. Each ticket represents a significant, logical chunk of work.

---

### **Project: Refactor OnTree to a Decoupled Core/CLI/Web Architecture**

**Objective:** Transform the current monolithic OnTree application into a clean, testable, and scalable architecture with a shared core engine, a command-line interface, and a web API, all within a single binary.

---

## Ticket 1: Establish the New Project Structure and CLI Foundation**

**Description:**
This is the foundational step. We will create the new project layout without moving any of the old business logic yet. The goal is to set up the skeleton of the application, including the single-binary CLI/server dispatcher.

**Acceptance Criteria:**
1.  **Create New Directory Structure:** Set up the new project layout (`/cmd`, `/internal/ontree`, `/web`).
2.  **Introduce Cobra:** Add Cobra as a dependency. Create a `main.go` that initializes a root Cobra command (`ontree`). CLI is the primary interface for dev/test automation.
3.  **Implement CLI Dispatching:**
    *   Create a placeholder `serve` command in `cmd/serve.go` (`ontree serve`) that prints "Starting web server..." and exits.
    *   Create a placeholder `app` command in `cmd/app.go` (`ontree app install|start|stop|list|health`) that prints placeholders.
    *   Create a placeholder `model` command in `cmd/model.go` (`ontree model install|list|health`) that prints placeholders.
4.  **Initial Logging Setup:**
    *   Implement basic `slog` configuration in `main.go`.
    *   Add global flags `--log-level` and `--log-format` to the root command. The selected logger should be functional in the placeholder commands.
5.  **Build Verification:** The project must compile into a single `ontree` binary that responds correctly to `ontree --help`, `ontree serve`, and `ontree app start`.
6.  **CI/Build Pipeline:** The existing CI build process is updated to build this new `main.go` entrypoint. Existing unit tests might be temporarily disabled or run separately if they don't fit the new structure yet.

---

## Ticket 2: Extract Core Logic into the `internal/ontree` Engine (Synchronous First)**

**Description:**
This is the most critical and largest part of the refactoring. We will identify all the business logic related to Podman, Tailscale, and app management, and move it from the old handlers into the new `internal/ontree` package. **For this first pass, we will keep the logic synchronous.**

**Acceptance Criteria:**
1.  **Define Core Types:** Create `internal/ontree/types.go`. Define the core structs (`App`, `Model`, `Config`, `Status`, etc.) that represent your domain.
2.  **Create the Manager:** Create `internal/ontree/manager.go` with a `Manager` struct that accepts a `*slog.Logger`.
3.  **Migrate Logic:**
    *   Identify the functions that handle Docker/Compose interaction. Move them into `internal/ontree/docker.go` as un-exported helper functions.
    *   Identify Ollama logic and move it to `internal/ontree/ollama.go`.
    *   Identify Tailscale logic and move it to `internal/ontree/tailscale.go`.
    *   Create synchronous public methods on the `Manager` struct (e.g., `StartApp(...) (*Status, error)`, `StopApp(...) error`, `InstallModel(...) error`, `HealthApp(...) error`, `HealthModel(...) error`).
4.  **Dependency Injection:** Ensure all migrated logic uses the logger injected into the `Manager`, removing any direct logging initializations.
5.  **Unit Test the Engine:**
    *   Write **new unit tests** specifically for the `ontree.Manager` and its methods. Mocking external commands (`exec.Command`) will be necessary here. Test the orchestration logic (e.g., "if Tailscale is enabled, the Tailscale function is called").
    *   All new code in `internal/ontree` must be covered by unit tests.

---

## Ticket 3: Wire the CLI and Web Server to the New Synchronous Engine**

**Description:**
With the core logic extracted, we will now connect the CLI and web server entrypoints to it. The application should be fully functional again at this stage, albeit with synchronous blocking behavior.

**Acceptance Criteria:**
1.  **Update CLI Commands (`cmd/`):**
    *   In `cmd/app.go`, replace the placeholder logic.
    *   Instantiate the `ontree.Manager`.
    *   Call the synchronous manager methods (e.g., `manager.StartApp(...)`, `manager.InstallModel(...)`, `manager.HealthApp(...)`, `manager.HealthModel(...)`).
    *   Print the result or error to the console.
    *   CLI output becomes the contract for automation.
2.  **Update Web Handlers (`web/`):**
    *   In `web/handlers.go`, replace the old logic in your HTTP handlers.
    *   Instantiate the `ontree.Manager`.
    *   Call the synchronous manager methods.
    *   Wrap the call in a goroutine (as the current app likely does) to avoid blocking the main server thread.
    *   Return the final JSON response or error.
    *   Web does not need to expose all CLI functionality initially.
3.  **Functional Parity:** The application (both CLI and Web UI) should now have the same functionality as the old "big blob" version. A user should not be able to tell the difference, except that they are now running `ontree serve`.

---

## Ticket 4: Refactor the Core Engine to be Asynchronous with Progress Streaming**

**Description:**
Now that the synchronous flow is stable and tested, we will convert the core engine to support asynchronous operations and real-time progress reporting via channels.

**Acceptance Criteria:**
1.  **Introduce `ProgressEvent`:** Define the `ProgressEvent` struct in `internal/ontree/types.go`.
2.  **Convert Manager Methods:**
    *   Change the signature of `manager.StartApp` from `(...) (*Status, error)` to `(ctx context.Context, ...) <-chan ProgressEvent`.
    *   Rewrite the method implementation to use a goroutine and a channel.
    *   All long-running `exec.Command` calls must be converted to `exec.CommandContext` and passed the context for cancellation.
3.  **Stream Output:** Modify the command execution logic to capture `stdout`/`stderr` and stream it line-by-line as `ProgressEvent`s on the channel.
4.  **Update Engine Unit Tests:** The unit tests for the `Manager` must be updated. Instead of checking a return value, they will now read from the returned channel and assert that the correct sequence of events is received.

---

## Ticket 5: Adapt Consumers (CLI & Web) to the Asynchronous Engine**

**Description:**
With the engine now streaming progress, we need to update the CLI and Web consumers to handle these streams, providing a real-time user experience.

**Acceptance Criteria:**
1.  **Update CLI for Streaming:**
    *   Modify the `cmd/app.go` command.
    *   It will now call the new async `manager.StartApp`.
    *   It will range over the returned channel and print each event's message directly to the console.
2.  **Implement SSE Handler:**
    *   In `web/handlers.go`, the handler for starting an app is completely rewritten.
    *   It must open an SSE stream.
    *   It calls the async `manager.StartApp`, passing the request's context.
    *   It ranges over the returned channel, marshals each event to JSON, and sends it as an SSE message.
    *   If SSE cannot meet a requirement, add WebSocket as an alternative.
3.  **Frontend Integration:** The frontend UI must be updated to use the new WebSocket endpoint and display the stream of progress events.

---

## Ticket 6: Code Cleanup and Test Migration**

**Description:**
The core refactoring is complete. This ticket is for cleaning up technical debt, ensuring code quality, and re-integrating the old test suites.

**Acceptance Criteria:**
1.  **Delete Old Code:** Remove all the old, now-unreferenced "big blob" code.
2.  **Fix Linter Issues:** Run the linter across the entire new codebase and fix all reported issues to ensure consistent code style and quality.
3.  **Migrate and Fix Old Unit Tests:**
    *   Review the original unit test suite.
    *   Migrate any still-relevant tests that were not covered by the new tests written in Ticket 2. Many might be obsolete, but some may test edge cases that were missed.
    *   Adapt them to the new `Manager` structure or delete them if they are redundant.

---

## Ticket 7: Re-enable and Adapt End-to-End (E2E) Tests**

**Description:**
The final validation step. We must ensure that our high-level, end-to-end tests pass against the new architecture for both the CLI and the web API.

**Acceptance Criteria:**
1.  **Adapt E2E Test Suite for CLI:**
    *   Modify the E2E test runner to execute commands against the single `ontree` binary (e.g., `ontree app start ...`).
    *   The tests need to be adapted to parse the streaming text output from the CLI, not just a final state.
2.  **Adapt E2E Test Suite for Web API:**
    *   Modify the E2E tests that target the web API. They must now start the server using `ontree serve`.
    *   Tests for long-running operations need to be rewritten to use a WebSocket client to connect and verify the stream of JSON events.
3.  **Full Test Pass:** The entire E2E test suite must be passing, confirming that the refactored application meets all original functional requirements from an external user's perspective.
4.  **CI Integration:** All tests (unit, linting, E2E) are running successfully in the main CI pipeline.

---

## Ticket 8: Add CLI-First Initial Setup**

**Description:**
Expose initial system setup (admin user + node name) via the CLI so automated provisioning can complete without the web UI.

**Acceptance Criteria:**
1.  **Core API:** Add a core method to create the initial admin user and mark setup complete.
2.  **CLI Command:** Add `ontree setup init --username --password [--node-name] [--node-icon]`.
3.  **CLI Status:** Add `ontree setup status` for automation checks.
4.  **Test Coverage:** Unit tests for setup flow and validation.

---

## Ticket 9: Add CLI-First App Health Checks**

**Description:**
Introduce CLI commands that verify app health using container status and optional HTTP probes. This becomes the basis for automated E2E verification.

**Acceptance Criteria:**
1.  **Health API:** Implement `manager.HealthApp(...)` in the core engine.
2.  **CLI Command:** Add `ontree app health <app>` that returns non-zero on failure.
3.  **Test Coverage:** Add unit tests around health evaluation and failure modes.

---

## Ticket 10: Add CLI-First Model Management**

**Description:**
Expose Ollama model install/list/health via the CLI and core engine.

**Acceptance Criteria:**
1.  **Core API:** Implement `InstallModel`, `ListModels`, `HealthModel` in the core engine.
2.  **CLI Command:** Add `ontree model install|list|health` commands.
3.  **Test Coverage:** Add unit tests for model orchestration and validation logic.

---

## Ticket 11: E2E Harness for Bare-Machine Verification**

**Description:**
Define and implement an E2E harness that provisions a machine via the existing setup script, installs TreeOS, installs Ollama + a small model, installs an app that depends on Ollama, and verifies health.

**Acceptance Criteria:**
1.  **Harness Script/Runner:** Deterministic CLI runner with clear setup/teardown and retry strategy.
2.  **Flow:** `setup -> ontree setup init -> ontree app install ollama-cpu -> ontree model install gemma3:270m -> ontree app install openwebui -> ontree app start openwebui -> ontree app health openwebui`.
3.  **OpenWebUI Verification:** After HTTP 200 readiness, perform a real chat request that exercises Ollama and confirms conversation context is stored. Initial user setup is seeded via env/config (`TREEOS_OPENWEBUI_ADMIN_EMAIL`, `TREEOS_OPENWEBUI_ADMIN_PASSWORD`, `TREEOS_OPENWEBUI_ADMIN_NAME`).
4.  **Scripted Runner:** Use `tests/integration/openwebui_e2e.sh` to authenticate (`/api/v1/auths/signin`), call `/api/chat/completions`, and persist/fetch a chat via `/api/v1/chats/new` and `/api/v1/chats/{id}`.
5.  **Documentation:** Clear instructions for running on a fresh Ubuntu machine.

---

## Ticket 12: CLI Contract Tests (Fast Feedback)**

**Description:**
Add a fast-running test layer that validates CLI behavior, JSONL output, error codes, and edge cases without requiring full E2E provisioning.

**Acceptance Criteria:**
1.  **CLI JSONL Tests:** Verify `--json` output shape and exit codes for `app` and `model` commands.
2.  **Edge Cases:** Invalid inputs, missing args, and health timeouts are covered.
3.  **Test Speed:** Runs quickly and deterministically; uses mocks/stubs for external commands.

---

## Ticket 13: Web Happy-Path Tests (Playwright)**

**Description:**
Add Playwright tests for the web UI focused on happy-path flows only.

**Acceptance Criteria:**
1.  **Minimal Coverage:** Install/start app and view status in UI.
2.  **Reliability:** Tests avoid flaky edge-case behaviors; CLI covers edge cases.
