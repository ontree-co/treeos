# Caddy Integration Tickets

This file outlines the tickets for integrating Caddy as a reverse proxy for the onTree-node application. The tickets should be worked on sequentially.

## Ticket 1: Fix Typo in DeployedApp Model

- **File:** `internal/database/models.go`
- **Task:** In the `DeployedApp` struct, the `UpdatedAt` field has a typo. It is currently `time.time` and should be `time.Time`. Correct this typo.

## Ticket 2: Update Ansible Playbook for Caddy Setup

- **File:** `ansible/setup-caddy-playbook.yaml`
- **Task:** Update the playbook to correctly configure Caddy according to the specification.
  - The playbook should ensure Caddy is installed from the official repository.
  - The playbook must create the `/etc/caddy/Caddyfile` with the following content:
    ```
    {
        # Enable the admin API on localhost only. This is secure and sufficient.
        admin localhost:2019
    }
    ```
  - The playbook should ensure the Caddy service is enabled and running (`sudo systemctl enable --now caddy`).
  - Remove any unnecessary tasks from the playbook, like creating example domains or test HTML files.

## Ticket 3: Add Caddy Health Check to Production Deployment Playbook

- **File:** `ansible/ontreenode-enable-production-playbook.yaml`
- **Task:** Before the "Service Management" phase, add a new task to verify that Caddy is installed and running correctly.
  - This task should make a GET request to Caddy's admin API at `http://localhost:2019/`.
  - The playbook should fail if the request does not return a 200 OK status code, with a clear error message indicating that Caddy is not running or accessible.

## Ticket 4: Implement Caddy Health Check in Manager

- **File(s):** `internal/server/server.go` (or appropriate startup location)
- **Task:** Implement a health check in the onTree-node application that runs on startup.
  - The application must perform a GET request to `http://localhost:2019/`.
  - If the request fails, the application should log a persistent error message: "Cannot connect to Caddy Admin API at localhost:2019. Please ensure Caddy is installed and running."
  - The feature to expose apps should be disabled in the UI if the health check fails.

## Ticket 5: Implement Caddy API Integration for App Deployment

- **File(s):** `internal/server/app_create_handler.go`, `internal/server/handlers.go`, and a new `internal/caddy/caddy.go` service.
- **Task:** Implement the core logic for managing Caddy routes when an application is created, updated, or deleted.
  - Create a new package `internal/caddy` with a client to interact with the Caddy Admin API.
  - This client should have methods to:
    - `AddOrUpdateRoute(routeJSON string)`: Sends a POST request to `http://localhost:2019/config/apps/http/servers/srv0/routes`.
    - `DeleteRoute(routeID string)`: Sends a DELETE request to `http://localhost:2019/id/<routeID>`.
  - When an app is created or exposed, the manager should:
    1.  Construct the Caddy route JSON as specified in the architecture document.
    2.  Call the `AddOrUpdateRoute` method.
  - When an app is stopped or un-exposed, the manager should call the `DeleteRoute` method.

## Ticket 6: Implement Synchronization Logic on Startup

- **File(s):** `internal/server/server.go` (or appropriate startup location)
- **Task:** Implement the synchronization logic that runs when the onTree-node application starts.
  - After the Caddy health check passes, the application should:
    1.  Fetch all `DeployedApp` entities from the database that have `IsExposed = true`.
    2.  For each exposed app, construct the Caddy route JSON.
    3.  Call the `AddOrUpdateRoute` method from the Caddy client to ensure Caddy's configuration is synchronized with the database state.

## Ticket 7: Implement Manual Subdomain Status Check

-   **File(s):**
    -   `templates/dashboard/app_detail.html`: To add the button.
    -   `internal/server/handlers.go` (or a new handler file): To create a new API endpoint for the check.
    -   `internal/server/server.go`: To register the new route.
-   **Task:** Implement a feature to manually check the status of an application's subdomains from the app detail page.
    -   **Frontend:**
        -   In `app_detail.html`, add a "Check Status" button for exposed applications.
        -   When the button is clicked, it should make an API call to a new backend endpoint (e.g., `/api/apps/{id}/status`).
        -   The UI should display the status result to the user (e.g., "OK", "Error: Could not resolve domain", "Error: Received status 502").
    -   **Backend:**
        -   Create a new handler that receives the app ID.
        -   The handler should fetch the `DeployedApp` from the database.
        -   It should then construct the full public and Tailscale URLs (if applicable) from the app's subdomain and the system's base domains.
        -   For each URL, the handler will perform an HTTP GET request.
        -   The handler should return a JSON response indicating the status of each check (e.g., success, failure, HTTP status code, error message).
        -   If both public and Tailscale domains are configured, the check should be performed for both.