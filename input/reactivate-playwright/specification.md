# Epic: Reactivate and Enhance End-to-End Testing with Playwright

## 1. Introduction

This document outlines the specification for reactivating and enhancing the end-to-end (E2E) testing suite for the onTree Node project using Playwright. The primary goal is to create a comprehensive, reliable, and maintainable test suite that covers the main features of the application, ensuring its stability and preventing regressions.

This epic is divided into two main phases:

1.  **Documentation:** Review and enhance the existing "Main Features" section in the Docusaurus documentation that describes the core functionalities of the application.
2.  **E2E Testing:** Enhance the existing Playwright test suite based on the documentation and sound testing principles.

## 2. Phase 1: Documentation

### 2.1. Review "Main Features" Section

The existing "Main Features" section in the Docusaurus documentation, located at `/documentation/docs/main-features`, will be reviewed and enhanced as needed. This section already contains descriptions of the application's core user-facing functionalities.

### 2.2. Documentation Content

The documentation covers:

*   **Authentication:** Registering the initial admin user, logging in, and logging out.
*   **Dashboard & System Vitals:** Viewing the main dashboard and inspecting system vitals including Docker container statistics.
*   **Application Management (Simple):** The full lifecycle of a single-service application (Nginx), including creation, verification, start/stop, and deletion.
*   **Application Management (Complex):** The full lifecycle of a multi-service application (OpenWebUI with Ollama).
*   **Naming Convention:** The implemented `ontree-{appName}-{serviceName}-{index}` naming scheme for Docker resources.

## 3. Phase 2: E2E Testing with Playwright

### 3.1. Test Strategy and Principles

The existing E2E test suite in `/tests/e2e/` will be enhanced following these principles to ensure it is robust, reliable, and maintainable.

#### 3.1.1. Data Seeding

The existing global setup process will be reviewed and enhanced to prepare the test environment before any tests are executed.

*   **Global Setup (`tests/e2e/global-setup.js`):** The existing script already:
    *   Ensures the application's SQLite database is in a clean, known state
    *   Seeds the database with a standard 'admin' user account (username: `admin`, password: `admin1234`)
    *   Cleans up Docker containers with the `ontree-test-` prefix
    *   Enhancements will include better server health checking and environment variable support
*   **Global Teardown (`tests/e2e/global-teardown.js`):** The existing teardown script will be enhanced for more thorough cleanup

#### 3.1.2. Test Isolation

Each test must be atomic and self-contained. Tests must not depend on the state or artifacts created by other tests.

*   **Self-Contained Tests:** Tests will be refactored to ensure complete lifecycle coverage within single test blocks
*   **No Order Dependency:** Tests must be able to run in any order without affecting the outcome of other tests
*   **Leverage Existing Helpers:** Use the helper functions in `tests/e2e/helpers.js` for common operations

#### 3.1.3. Cleanup

Robust cleanup mechanisms are essential to maintain a clean test environment and prevent cascading failures.

*   **`beforeEach` Hook:** Used for setup actions like logging in the pre-seeded admin user
*   **`afterEach` Hook:** Critical for cleanup - ensures complete removal of test-created resources
*   **Docker Cleanup:** All test containers will use the `ontree-test-` prefix for easy identification and cleanup

### 3.2. Test Scenarios

The Playwright tests will be consolidated and enhanced to cover the following scenarios:

*   **Authentication (`tests/e2e/auth/login.spec.js`):**
    *   Successfully log in with the pre-seeded admin credentials
    *   Handle failed login attempts
    *   Verify session persistence
    *   Successfully log out
*   **Dashboard (`tests/e2e/dashboard/vitals.spec.js`):**
    *   Verify that the dashboard loads correctly
    *   Verify system vitals (CPU, Memory, Disk) are displayed
    *   Verify Docker container statistics
    *   Test real-time updates via WebSocket
*   **Simple App Lifecycle (`tests/e2e/apps/simple-app-lifecycle.spec.js`):**
    *   A single comprehensive test covering: create Nginx app, verify running state, stop, verify stopped, start, verify running, and delete
    *   Verify naming convention: `ontree-{appName}-nginx-1`
*   **Complex App Lifecycle (`tests/e2e/apps/complex-app-lifecycle.spec.js`):**
    *   A single comprehensive test covering: create OpenWebUI app, verify all services running, test inter-service communication, stop, verify, start, verify, and delete
    *   Verify naming convention for multiple services

## 4. Build System Integration

The project uses a Makefile-based build system. All E2E tests will be executed using:

*   **`make test-e2e`:** Runs the complete Playwright E2E test suite
*   **`make test`:** Runs unit and integration tests
*   **`make test-all`:** (To be added) Runs both unit and E2E tests

The E2E tests themselves use npm/Playwright within the `/tests/e2e/` directory, but the main commands follow the project's Makefile convention.

## 5. Execution Environment

The Playwright tests are configured to run against a local development environment at `http://localhost:3001`. The application runs on this port during E2E testing, as configured in the existing test setup.

## 6. Technology Stack Context

*   **Backend:** Go application with SQLite database
*   **Frontend:** Server-side rendered HTML templates
*   **Container Management:** Docker and Docker Compose
*   **Testing:** Playwright for E2E tests, Go's built-in testing for unit tests
*   **Documentation:** Docusaurus (already configured)