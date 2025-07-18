# Tickets for Reactivating E2E Testing with Playwright

## Ticket 1: Update Unit Tests for Naming Convention
**Description:** Verify and enhance existing unit tests for the app naming convention `ontree-{appName}-{serviceName}-{index}`.

**Acceptance Criteria:**
- [ ] Review existing naming convention implementation in the codebase
- [ ] Add/update unit tests in the appropriate Go test files for container naming
- [ ] Tests verify that container names follow the format `ontree-{appName}-{serviceName}-{index}`
- [ ] Tests verify the same pattern applies to networks and volumes
- [ ] Tests pass when running `make test`

**Technical Details:**
- The naming convention is already implemented and documented in `/documentation/docs/main-features/naming-convention.md`
- Update tests in relevant Go packages (likely in `internal/` directory)
- Test both single-service and multi-service applications
- Ensure error handling for invalid names

---

## Ticket 2: Enhance Playwright Global Setup and Teardown
**Description:** Review and enhance existing global setup and teardown scripts for Playwright tests to ensure clean test environment.

**Acceptance Criteria:**
- [ ] Review existing `tests/e2e/global-setup.js` and enhance it to:
  - Verify the application is running on http://localhost:3001
  - Ensure database is properly cleaned and seeded with test admin user
  - Improve Docker container cleanup using `ontree-test-` prefix
- [ ] Review existing `tests/e2e/global-teardown.js` and enhance cleanup
- [ ] Add environment variable support for test configuration
- [ ] Document any changes in test README

**Technical Details:**
- Current setup already seeds admin user (username: `admin`, password: `admin1234`)
- Use Makefile commands where appropriate
- Ensure compatibility with SQLite database
- Add health check for server readiness

---

## Ticket 3: Consolidate and Enhance Authentication Tests
**Description:** Review existing authentication tests and consolidate them following the isolation principle.

**Acceptance Criteria:**
- [ ] Review existing tests in `tests/e2e/auth/`
- [ ] Consolidate into `tests/e2e/auth/login.spec.js` with tests for:
  - Successful login with valid credentials
  - Failed login with invalid credentials
  - Redirect to login when accessing protected routes
  - Session persistence across page reloads
  - Logout functionality
- [ ] Each test must be fully isolated and not depend on other tests
- [ ] Remove any duplicate or redundant tests

**Technical Details:**
- Use existing helper functions from `tests/e2e/helpers.js`
- Leverage existing Page Object Model if present
- Run tests with `make test-e2e`

---

## Ticket 4: Consolidate Dashboard Tests
**Description:** Review and enhance existing dashboard tests for vitals display and real-time updates.

**Acceptance Criteria:**
- [ ] Review existing tests in `tests/e2e/dashboard/`
- [ ] Create/enhance `tests/e2e/dashboard/vitals.spec.js` with tests for:
  - CPU usage display and updates
  - Memory usage display and updates
  - Disk usage display and updates
  - Docker container statistics
- [ ] Verify real-time updates using WebSocket connections
- [ ] Test data formatting and units display

**Technical Details:**
- Build upon existing dashboard test infrastructure
- Account for server-side rendered templates
- Test responsive design on different viewports

---

## Ticket 5: Create Complete Simple App Lifecycle Test (Nginx)
**Description:** Implement a complete lifecycle test for a simple single-service application using Nginx, consolidating existing app tests.

**Acceptance Criteria:**
- [ ] Review existing tests in `tests/e2e/apps/`
- [ ] Create/enhance `tests/e2e/apps/simple-app-lifecycle.spec.js` with a single comprehensive test that:
  - Creates an Nginx app with name following convention
  - Verifies the app appears in the dashboard
  - Verifies the app is running and healthy
  - Stops the app and verifies it's stopped
  - Starts the app again and verifies it's running
  - Deletes the app and verifies complete cleanup
- [ ] Use proper cleanup in afterEach hook
- [ ] Verify container naming convention: `ontree-{appName}-nginx-1`

**Technical Details:**
- Use existing helper functions for app operations
- Add appropriate wait conditions for container state changes
- Ensure Docker cleanup includes networks and volumes
- Test should be idempotent and handle partial failures

---

## Ticket 6: Create Complete Complex App Lifecycle Test (OpenWebUI)
**Description:** Implement a complete lifecycle test for a multi-service application using OpenWebUI with Ollama.

**Acceptance Criteria:**
- [ ] Create `tests/e2e/apps/complex-app-lifecycle.spec.js` with a single comprehensive test that:
  - Creates OpenWebUI app using existing template system
  - Verifies all services (OpenWebUI + Ollama) are running
  - Tests inter-service communication
  - Verifies the app is accessible via its assigned URL
  - Stops all services and verifies complete stop
  - Starts all services again and verifies restoration
  - Deletes the app and all its resources
- [ ] Ensure all containers follow naming convention
- [ ] Implement robust cleanup including volumes and networks

**Technical Details:**
- Expected service names: `ontree-openwebui-web-1`, `ontree-openwebui-ollama-1`
- Wait for Ollama model to be ready before testing
- Verify network connectivity between services
- Use Docker compose for multi-service management

---

## Ticket 7: Enhance E2E Test Runner and CI Integration
**Description:** Enhance the existing E2E test runner in the Makefile and ensure smooth CI/CD integration.

**Acceptance Criteria:**
- [ ] Review and enhance the existing `make test-e2e` command
- [ ] Ensure the command:
  - Builds the application if needed
  - Starts the server on port 3001
  - Runs all E2E tests with proper configuration
  - Generates test reports
- [ ] Add `make test-all` command that runs both unit and E2E tests
- [ ] Update CI/CD configuration if needed
- [ ] Document test commands in README

**Technical Details:**
- The E2E tests use npm/Playwright within `/tests/e2e/` directory
- Main commands should use Makefile for consistency
- Ensure proper exit codes for CI/CD
- Consider adding flags for headed mode and specific test selection