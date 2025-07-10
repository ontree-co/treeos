# Tickets: Remove deployed_apps Model

This file breaks down the work outlined in the `specification.md` into a series of sequential tickets that can be executed by an agent.

---

### Ticket 1: Implement YAML Helper Functions

**Description:** Create the YAML utility functions required to safely read and write to `docker-compose.yml` files while preserving formatting and comments. This is the foundational step for all subsequent work.

**Tasks:**
1.  Create a new Go package or file for YAML utilities (e.g., `internal/yamlutil/yamlutil.go`).
2.  Implement `ReadComposeWithMetadata(path string) (*ComposeFile, error)`.
3.  Implement `WriteComposeWithMetadata(path string, compose *ComposeFile) error)`.
4.  Define the `ComposeFile` and `OnTreeMetadata` structs as specified.
5.  Ensure the implementation uses a library that preserves comments and formatting (e.g., `gopkg.in/yaml.v3`).

---

### Ticket 2: Create Data Migration Script

**Description:** Develop a one-time script to migrate existing app metadata from the `deployed_apps` database table into the `x-ontree` section of the corresponding `docker-compose.yml` files.

**Tasks:**
1.  Create a new command or script for the migration.
2.  The script should read all entries from the `deployed_apps` table.
3.  For each entry, use the YAML helpers from Ticket 1 to update its `docker-compose.yml` with the `subdomain`, `host_port`, and `is_exposed` metadata.
4.  Implement backup and logging mechanisms as described in the specification.

---

### Ticket 3: Update Handlers to Use Compose Files

**Description:** Refactor all HTTP handlers related to app management to use the `docker-compose.yml` file as the single source of truth, removing all dependencies on the `deployed_apps` table.

**Tasks:**
1.  **Read Operations:** Modify `handleAppDetail` to read metadata from `x-ontree`.
2.  **Write Operations:** Modify `handleAppExpose`, `handleAppUnexpose`, and the new app creation handlers to write metadata to `x-ontree` instead of performing database operations.
3.  Ensure file-locking is implemented to prevent race conditions during write operations.

---

### Ticket 4: Update Background Worker

**Description:** Modify the background worker's operations (`processExposeOperation`, `processUnexposeOperation`) to read metadata from the `docker-compose.yml` file instead of the database.

**Tasks:**
1.  Refactor `processExposeOperation` to fetch app details from the compose file.
2.  Refactor `processUnexposeOperation` to fetch app details from the compose file.
3.  Update the logic to modify `is_exposed` in the compose file upon successful Caddy configuration.

---

### Ticket 5: Database Cleanup

**Description:** After confirming the migration is successful and all code has been updated, completely remove the `deployed_apps` table and its related code.

**Tasks:**
1.  Create a new database migration file to `DROP TABLE deployed_apps;`.
2.  Remove the `DeployedApp` model from `internal/database/models.go`.
3.  Remove the table creation logic from `internal/database/database.go`.
4.  Remove any remaining code that references the old model or table.

---

### Ticket 6: Verify Tests and CI

**Description:** Add and update tests to ensure the new implementation is correct and robust. Verify that all local and CI checks pass.

**Tasks:**
1.  Write unit tests for the new YAML helper functions.
2.  Update existing unit tests for handlers and workers to reflect the new data flow.
3.  Run `go test ./...` to execute all unit tests.
4.  Run the linter (`golangci-lint run`) to check for style issues.
5.  Ensure the `test.yml` GitHub Actions workflow passes successfully.

---

### Ticket 7: End-to-End (E2E) Validation with Playwright

**Description:** Create a new Playwright E2E test to validate the entire user flow of creating an app and exposing it with a subdomain.

**Tasks:**
1.  Create a new test file in `tests/e2e/apps/`.
2.  The test should programmatically navigate the UI to the "Create App" page.
3.  Fill out the form to create a new application from a template.
4.  Navigate to the app detail page for the newly created app.
5.  Use the UI to expose the app with a specific subdomain.
6.  Verify that the operation completes successfully and the app becomes accessible at the expected subdomain.
