
# onTree Go Rewrite: Development Tickets

This document breaks down the development of the onTree Go rewrite into smaller, achievable tickets. Each ticket represents a meaningful, testable unit of work.

**IMPORTANT:** A new Git repository should be initialized for this project. After each ticket is successfully completed and verified, all changes should be committed to the repository. This ensures a clean, traceable history of the development process.

**Testing:** Where applicable, each feature should be accompanied by tests. The structure and style of the tests in the original Python project should be used as a reference. New tests should be created in a corresponding `_test.go` file and should be runnable with the standard `go test ./...` command.

---

## Ticket 1: Project Scaffolding and CLI Argument Parsing

**Description:**
Initialize a new Git repository. Initialize the Go module and create the basic directory structure. Set up the `main` function to parse command-line arguments. It should differentiate between starting the server (default) and running the `setup-dirs` command.

**Verification:**
1.  A `.git` directory exists.
2.  Run `go run ./cmd/ontree-server`. It should print a message like "Starting server...".
3.  Run `go run ./cmd/ontree-server setup-dirs`. It should print a message like "Running directory setup...".
4.  Commit the changes with the message "feat: Initial project scaffolding and CLI parsing".

---

## Ticket 2: Directory Setup Command (`setup-dirs`)

**Description:**
Implement the `setup-dirs` command. On Linux, it should create `/opt/ontree/apps`, set permissions to `0775`, and set ownership to a `ontreenode` user/group if it exists. It must detect if it's running with `sudo` and fail if not. On macOS, it should simply create a local `./apps` directory.

**Verification:**
1.  On a Linux machine, run `go run ./cmd/ontree-server setup-dirs`. It should fail with a message requiring sudo.
2.  Run `sudo go run ./cmd/ontree-server setup-dirs`. It should create the `/opt/ontree/apps` directory with the correct permissions.
3.  On macOS, run `go run ./cmd/ontree-server setup-dirs`. It should create the `./apps` directory locally.
4.  Commit the changes with the message "feat: Implement setup-dirs command".

---

## Ticket 3: Configuration Management

**Description:**
Implement a configuration package (`internal/config`) that loads settings from a `config.toml` file and allows overrides from environment variables. It should provide a platform-aware default for `AppsDir` (`/opt/ontree/apps` on Linux, `./apps` on macOS).

**Verification:**
1.  Run the application on Linux. The default `AppsDir` should be `/opt/ontree/apps`.
2.  Run on macOS. The default `AppsDir` should be `./apps`.
3.  Set `ONTREE_APPS_DIR=/tmp/myapps` as an environment variable and run the app. The `AppsDir` should now be `/tmp/myapps`.
4.  Commit the changes with the message "feat: Add configuration management".

---

## Ticket 4: Database Setup and Models

**Description:**
Set up the SQLite database connection (`internal/database`). Define the Go structs for all required models (`User`, `SystemSetup`, `SystemVitalLog`, `DockerOperation`). Create an initialization function that connects to the database file and creates the necessary tables if they don't exist.

**Verification:**
1.  Run the application.
2.  A `ontree.db` file (or the configured name) should be created in the project root.
3.  Using a SQLite browser or CLI, inspect the database file.
4.  Verify that the `users`, `system_setup`, `system_vital_logs`, and `docker_operations` tables have been created with the correct columns.
5.  Commit the changes with the message "feat: Implement database setup and models".

---

## Ticket 5: Base Template and Static File Serving

**Description:**
Convert the main `base.html` layout from the Django project to a Go template. Implement the logic to serve static files (CSS, JS, fonts) from the `/static` directory. Create a basic dashboard handler that renders the `index.html` template, which should extend the `base.html` layout.

**Verification:**
1.  Run the application and navigate to the root URL.
2.  The basic page layout (header, footer, sidebar) should render correctly, styled with the project's CSS.
3.  Use the browser's developer tools to confirm that the CSS and JS files are loaded successfully (HTTP 200).
4.  Commit the changes with the message "feat: Add base template and static file serving".

---

## Ticket 6: Setup Wizard and User Authentication

**Description:**
Implement the setup wizard and authentication flow. This includes:
- A middleware that checks if any users exist. If not, it redirects to `/setup`.
- The `/setup` page with a form to create the first admin user.
- The `/login` page and the logic to handle form submission, hash passwords, and manage user sessions (e.g., using secure cookies).
- A middleware to protect all other routes, redirecting unauthenticated users to `/login`.

**Verification:**
1.  Delete the database file and run the application.
2.  Navigating to `/` should redirect to `/setup`.
3.  Complete the setup form. It should create a user and redirect to the dashboard.
4.  Log out. You should be redirected to the `/login` page.
5.  Log back in with the credentials you just created. You should be granted access to the dashboard.
6.  Commit the changes with the message "feat: Implement setup wizard and authentication".

---

## Ticket 7: System Vitals Service & API

**Description:**
Create a service (`internal/system`) that uses the `gopsutil` library to collect CPU usage, memory usage, and disk usage. Create an API endpoint (`/api/system-vitals`) that returns this data as an HTML partial.

**Verification:**
1.  Run the application and log in.
2.  Navigate to `/api/system-vitals` in the browser.
3.  The page should display an HTML snippet containing the current system vitals (e.g., "CPU: 15%, Mem: 45%, Disk: 60%").
4.  Commit the changes with the message "feat: Add system vitals service and API".

---

## Ticket 8: Asynchronous Vitals on Dashboard (HTMX)

**Description:**
Integrate the system vitals into the main dashboard. The `index.html` page should initially show a loading state for the vitals card. Use HTMX to call the `/api/system-vitals` endpoint to load the vitals asynchronously and then refresh them every 30 seconds.

**Verification:**
1.  Log in and navigate to the dashboard.
2.  The system vitals card should initially show a "Loading..." message and then populate with the data.
3.  Monitor the network requests in browser dev tools. A request to `/api/system-vitals` should be made on page load and then automatically every 30 seconds.
4.  Commit the changes with the message "feat: Implement async vitals with HTMX".

---

## Ticket 9: Docker App Discovery and Listing with Prefixed Names

**Description:**
Implement the Docker service (`internal/docker`) to scan the `ONTREE_APPS_DIR`. This service should find all subdirectories containing a `docker-compose.yml` file, parse the file, and get the status of the corresponding container. **All container names must be prefixed with `ontree-`** (e.g., an app named `nginx-test` should have a container named `ontree-nginx-test`). The results should be displayed in a list on the dashboard.

**Verification:**
1.  Run `setup-dirs` to create the apps directory.
2.  Create a few sample applications in the `ONTREE_APPS_DIR`.
3.  Run the application and navigate to the dashboard.
4.  The "Applications" list should show the sample apps, along with their status.
5.  Use `docker ps -a` to confirm that any created containers have the `ontree-` prefix.
6.  Commit the changes with the message "feat: Implement Docker app discovery with prefixed names".

---

## Ticket 10: Application Detail Page with Prefixed Names

**Description:**
Create a dynamic route `/apps/{app_name}` that displays a detail page for a specific application. This page should show the app's configuration, container status, ports, volumes, and the content of its `docker-compose.yml` file. All Docker interactions must use the `ontree-` prefixed container name.

**Verification:**
1.  From the dashboard, click on one of the applications from the list.
2.  Verify that the browser navigates to the correct URL (e.g., `/apps/nginx-test`).
3.  The page should display the detailed information for the selected app, correctly fetching the status of the `ontree-nginx-test` container.
4.  Commit the changes with the message "feat: Create application detail page".

---

## Ticket 11: Container Management with Prefixed Names (Start/Stop/Recreate)

**Description:**
Implement the backend logic and the frontend controls to start, stop, and recreate containers. All Docker commands must target the `ontree-` prefixed container name.

**Verification:**
1.  Navigate to the detail page of a "not_created" app (e.g., `my-app`).
2.  Click the "Start" button. A container named `ontree-my-app` should be created and started. The status on the page should update to "running".
3.  Click the "Stop" button. The status should update to "exited".
4.  Use `docker ps -a` to confirm the state changes for the `ontree-my-app` container.
5.  Commit the changes with the message "feat: Implement container management actions".

---

## Ticket 12: Background Operations for Container Creation

**Description:**
Refactor the "Start" and "Recreate" actions to run in the background using goroutines. Track the operation's state (pending, in_progress, completed, failed) in the `docker_operations` table. Create an API endpoint (`/api/docker/operations/{id}`) to poll for status updates.

**Verification:**
1.  Start an application that has a large image that needs to be pulled.
2.  The UI should immediately become responsive, showing a progress indicator.
3.  Verify that the UI polls the operations endpoint until the status is "completed" or "failed".
4.  Check the `docker_operations` table in the database to see a record of the operation.
5.  Commit the changes with the message "feat: Add background processing for Docker operations".

---

## Ticket 13: Application Creation from Scratch

**Description:**
Implement the "Create Application" feature. This includes the form at `/apps/create` and the backend logic to create a new app directory, save the provided `docker-compose.yml`, and redirect to the new app's detail page.

**Verification:**
1.  Navigate to `/apps/create`.
2.  Fill in an app name and a valid `docker-compose.yml` content.
3.  Submit the form.
4.  Verify you are redirected to the new app's detail page.
5.  Check the filesystem to ensure the new application directory and `docker-compose.yml` file were created correctly in the `ONTREE_APPS_DIR`.
6.  Commit the changes with the message "feat: Implement application creation from scratch".

---

## Ticket 14: Application Creation from Templates

**Description:**
Implement the application templates feature. This involves:
- A service to read template metadata from a JSON file.
- A page at `/templates` to display available templates.
- A form to create an app from a template, which pre-fills the configuration.

**Verification:**
1.  Navigate to `/templates`.
2.  A list of predefined applications should be visible.
3.  Click "Create" on a template.
4.  Enter a name for the new app and submit.
5.  The application should be created, and you should be redirected to its detail page. The configuration should match the template.
6.  Commit the changes with the message "feat: Implement application creation from templates".

---

## Ticket 15: Replicate the Pattern Library

**Description:**
Create the `/patterns` section of the site. Convert the Django templates from the original `pattern_library` app into Go templates. This section is read-only and does not require any complex backend logic.

**Verification:**
1.  Navigate to `/patterns`.
2.  The page should render, displaying all the UI components (buttons, forms, cards, etc.) exactly as they appear in the original project.
3.  Commit the changes with the message "feat: Replicate pattern library".

---

## Ticket 16: Honeycomb and OpenTelemetry Integration

**Description:**
Integrate OpenTelemetry for distributed tracing. The implementation should support exporting traces to Honeycomb in production and Jaeger for local development, configured via environment variables. Add custom spans for key operations, especially Docker-related actions.

**Verification:**
1.  Run the application with Jaeger configured locally.
2.  Perform several actions: log in, view the dashboard, start a container.
3.  Open the Jaeger UI.
4.  Verify that traces for the HTTP requests and custom spans (e.g., `docker.start_app`) are present and correctly nested.
5.  Commit the changes with the message "feat: Add Honeycomb and OpenTelemetry integration".

---

## Ticket 17: PostHog Analytics Integration

**Description:**
Implement client-side analytics using PostHog. The PostHog JavaScript snippet should be included in the base template and configured with an API key from the environment. Custom events should be tracked for important user actions like "login", "start_app", and "create_app_from_template".

**Verification:**
1.  Run the application with a test PostHog API key.
2.  Using the browser's developer console, verify that the PostHog script is loaded.
3.  Log in and start an application.
4.  In the PostHog events explorer, verify that the corresponding events (`user_logged_in`, `app_started`) have been captured with the correct properties.
5.  Commit the changes with the message "feat: Add PostHog analytics integration".

---

## Ticket 18: Manual Docker Image Update Feature

**Description:**
Implement the manual image update feature on the application detail page. This includes:
- A "Check for Updates" button that compares the local image digest with the remote registry.
- An API endpoint to perform this check.
- UI updates via HTMX to show "Update available" or "Up to date".
- An "Update Now" button that appears when an update is found, which triggers a background operation to pull the new image and recreate the container.

**Verification:**
1.  Run a container with an older image tag (e.g., `nginx:1.20`).
2.  Navigate to the app detail page and click "Check for Updates". The UI should change to show "Update available".
3.  Click "Update Now". The application should pull the newer image and recreate the container in the background, showing progress.
4.  After the update, inspect the container to confirm it is running the newer image version.
5.  Commit the changes with the message "feat: Add manual Docker image update feature".

---

## Ticket 19: Comprehensive End-to-End Testing with Playwright

**Description:**
After all features have been implemented, perform a comprehensive walkthrough of the application's main use cases and record Playwright tests. This ticket should capture automated tests for all critical user journeys, ensuring the application works correctly from end to end.

**Main Use Cases to Test:**
1. **Initial Setup Flow:**
   - Navigate to the application
   - Complete the setup wizard
   - Create the first admin user
   - Verify redirect to dashboard

2. **Authentication Flow:**
   - Log out from the application
   - Log in with valid credentials
   - Attempt login with invalid credentials
   - Verify session persistence

3. **Dashboard and System Vitals:**
   - View the main dashboard
   - Verify system vitals load and refresh
   - Check that all UI elements render correctly

4. **Application Management:**
   - Create a new application from scratch
   - Start, stop, and recreate containers
   - View application details and logs
   - Delete an application

5. **Template System:**
   - Browse available templates
   - Create an application from a template
   - Verify the application is configured correctly

6. **Docker Image Updates:**
   - Check for image updates
   - Update an application's Docker image
   - Verify the container is recreated with the new image

**Verification:**
1.  Use Playwright MCP to walk through each use case manually.
2.  While performing the walkthrough, record Playwright tests that capture:
    - User interactions (clicks, form submissions)
    - Expected page navigation
    - Presence of key UI elements
    - Correct application state changes
3.  Ensure tests can run independently and handle setup/teardown.
4.  Run the complete test suite to verify all tests pass.
5.  Tests should be organized in a `tests/e2e/` directory.
6.  Commit the changes with the message "test: Add comprehensive end-to-end Playwright tests".
