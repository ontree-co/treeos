## Ticket 1: Foundation - Integrate Docker Compose Go SDK

    Description: Integrate the official Docker Compose Go library (github.com/docker/compose/v2) into the project. Create a new service/package (e.g., pkg/compose) to act as a wrapper around the SDK. This initial implementation should be capable of performing a basic up and down operation on a hardcoded docker-compose.yml file in a test directory.

    Goal: Prove that the application can programmatically start and stop a multi-service stack using the Go SDK.

    Acceptance Criteria:

        A new Go module dependency for the Docker Compose SDK is added.

        A new service/wrapper exists for interacting with the SDK.

        A unit/integration test successfully starts a simple two-service (e.g., nginx and redis) stack from a local test file.

        The same test successfully stops and removes the stack.

        Code passes all linting and existing unit tests.

## Ticket 2: Core Data Model and File Structure

    Description: Update the core application data models and create the backend logic for managing the new file-based storage structure. This does not yet involve any Docker operations.

    Goal: Establish the on-disk and in-memory representation of a multi-service "App".

    Acceptance Criteria:

        The App struct is updated to include Services, an aggregate AppStatus, and other relevant fields. The Service struct is created.

        Backend logic is created to provision the directory structure /opt/onTree/apps/{appName}/ and /opt/onTree/apps/mount/{appName}/ when a new app is conceptualized.

        Functions for reading/writing docker-compose.yml and .env files to/from these directories are implemented.

        Code passes all linting and unit tests.

## Ticket 3: Implement CreateApp and UpdateApp API Endpoints

    Description: Create the API endpoints that allow a user to create a new app or update an existing one. These endpoints will accept the docker-compose.yml and .env content and save them to the correct location on the file system defined in Ticket 2.

    Goal: Allow app configurations to be managed via the API.

    Acceptance Criteria:

        A POST /api/apps endpoint is created. It takes an app name and YAML/env content, creates the file structure, and saves the files.

        A PUT /api/apps/{appName} endpoint is created to update the files for an existing app.

        API requests are validated (e.g., app name is valid, YAML content is parseable).

        The endpoints are fully tested via API integration tests.

        Code passes all linting and existing unit tests.

## Ticket 4: Implement the Security Validation Module

    Description: Create a dedicated, standalone Go module/package responsible for validating a docker-compose.yml file's content against the defined security policy. This module will be pure logic and highly testable.

    Goal: Create a reusable security gatekeeper that can be invoked before any start operation.

    Acceptance Criteria:

        The validator correctly identifies and rejects configurations with privileged: true.

        The validator correctly identifies and rejects configurations with bind mounts outside of the /opt/onTree/apps/mount/{appName}/{serviceName}/ structure.

        The validator correctly identifies and rejects configurations with disallowed Docker capabilities.

        Extensive unit tests cover all validation rules, including both valid and invalid configurations.

        Code passes all linting and unit tests.

## Ticket 5: Implement StartApp API Endpoint

    Description: Implement the API endpoint for starting an app. This ticket ties together the SDK, file structure, and security validator.

    Goal: To securely start a multi-service application as a Docker Compose project.

    Acceptance Criteria:

        A POST /api/apps/{appName}/start endpoint is created.

        The endpoint logic first reads the docker-compose.yml from the app's directory.

        It then passes the content to the Security Validation Module (from Ticket 4). The operation is aborted if validation fails.

        If valid, it uses the Docker Compose SDK wrapper (from Ticket 1) to execute an up command, using ontree-{appName} as the project name.

        The operation is tested with valid and invalid (insecure) compose files.

        Code passes all linting and existing unit tests.

## Ticket 6: Implement StopApp and DeleteApp API Endpoints

    Description: Implement the API endpoints for stopping an app (non-destructively) and deleting an app (destructively).

    Goal: Provide full lifecycle control over an app via the API.

    Acceptance Criteria:

        A POST /api/apps/{appName}/stop endpoint is created. It invokes the SDK's down command without the -v flag (preserving named volumes).

        A DELETE /api/apps/{appName} endpoint is created. It invokes the SDK's down command with the -v flag to remove all associated resources, including named volumes. It also removes the app's configuration and mount directories.

        Both endpoints are fully tested via API integration tests.

        Code passes all linting and existing unit tests.

## Ticket 7: Implement App Status API Endpoint

    Description: Create an API endpoint that inspects the state of all containers belonging to a project and returns the aggregated status (running, partial, stopped, error) along with the status of each individual service.

    Goal: Provide the data necessary for the UI to accurately display the state of a multi-service app.

    Acceptance Criteria:

        A GET /api/apps/{appName}/status endpoint is created.

        It uses the Docker Compose SDK's ps command (or equivalent) to list containers for the project ontree-{appName}.

        It correctly calculates the aggregate status based on the state of the services.

        The response payload includes the overall status and a list of services with their individual statuses.

        Unit tests cover all aggregation logic (all running, some running, all stopped, etc.).

        Code passes all linting and existing unit tests.

## Ticket 8: UI - Update Dashboard for Multi-Service Apps

    Description: Modify the main application dashboard/list view to correctly display multi-service apps.

    Goal: Give users an at-a-glance overview of their multi-service apps.

    Acceptance Criteria:

        The dashboard now calls the new status endpoint (GET /api/apps/{appName}/status).

        Each app entry displays an aggregate status badge (e.g., "Partial", "Running").

        Each app entry shows the number of services (e.g., "3 Services").

        The UI remains clean and functional.

        Code passes all linting and existing unit tests.

## Ticket 9: UI - Overhaul App Detail Page

    Description: Completely redesign the app detail page to display the list of services and provide master controls.

    Goal: Allow users to inspect and manage a multi-service app.

    Acceptance Criteria:

        The detail page displays the overall app status at the top.

        A table or list displays all services within the app, showing each service's name, status, and image.

        Primary "Start All" and "Stop All" buttons are present and call the start and stop APIs respectively.

        The "Delete App" button is present (likely in a settings area) and protected by a confirmation modal.

        The page is responsive and user-friendly.

        Code passes all linting and existing unit tests.

## Ticket 10: UI & Backend - Implement Aggregated Log Viewer

    Description: Create the backend endpoint and frontend UI for viewing aggregated logs from all services in an app.

    Goal: Provide a centralized place for users to debug their multi-service applications.

    Acceptance Criteria:

        A backend endpoint (GET /api/apps/{appName}/logs) is created that uses the SDK's logs command to stream logs from all services in the project.

        The UI features a new "Logs" tab or section on the app detail page.

        The log viewer displays a real-time stream of logs, with each line prefixed by its service name (e.g., api-1 | Request received).

        A dropdown filter is implemented in the UI to allow users to view logs for a specific service.

        Code passes all linting and existing unit tests.

## Ticket 11: UI - Implement App Creation and Editor Flow

    Description: Build the UI forms for creating a new app and editing an existing one.

    Goal: Allow users to manage app configurations directly from the UI.

    Acceptance Criteria:

        A "New App" page is created with text areas for the App Name, docker-compose.yml content, and .env content.

        The form correctly calls the POST /api/apps endpoint.

        An "Edit" page is created that prepopulates the form fields by fetching the current configuration and calls the PUT /api/apps/{appName} endpoint on save.

        Error messages from the backend (e.g., invalid YAML) are displayed clearly to the user.

        Code passes all linting and existing unit tests.

## Ticket 12: Migration Tool for Existing Single-Service Apps

    Description: Create a one-off script or an admin-only backend function to migrate legacy single-container apps to the new multi-service format.

    Goal: Ensure backward compatibility and a smooth transition for existing users.

    Acceptance Criteria:

        The tool correctly identifies legacy apps that don't have a corresponding directory in /opt/onTree/apps/.

        For each legacy app, the tool generates a basic docker-compose.yml file based on the existing container's configuration (image, ports, volumes, environment).

        The tool creates the new directory structure and saves the generated file.

        The tool safely renames the existing container and network to match the new ontree-{appName}-{serviceName}-1 convention.

        The migration can be run on a per-app basis or in a batch.

        Code passes all linting and existing unit tests.