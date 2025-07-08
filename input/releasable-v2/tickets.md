# OnTree V2 Release Tickets - CI/CD Hardening

This file breaks down the work required to harden the OnTree CI/CD pipeline, based on the lessons learned from the V2 specification.

---

## Ticket 1: Linting - Enhance `.golangci.yml`\*\*

- **Description**: Update the `.golangci.yml` configuration to enable and configure additional linters, including `gosec`, `gosimple`, `unused`, `errcheck`, and `revive`.
- **Acceptance Criteria**:
  - The `.golangci.yml` file is updated with the new linters and their configurations.
  - The application code passes the new, stricter linting rules.
  - The changes are committed to the repository.

### Phase 3: E2E Testing and Local Verification

## Ticket 2: CI - Restructure E2E Test Workflow\*\*

- **Description**: Refactor the E2E test setup in the CI workflow to use a PostgreSQL service container directly, instead of `docker-compose`. This involves running the application binary directly, adding a database migration step, and using a health check to ensure the app is ready before tests run.
- **Acceptance Criteria**:
  - The E2E tests run successfully in CI using a service container.
  - `docker-compose` is removed from the CI E2E test setup.
  - The changes are committed to the repository.

## Ticket 3: Local - Verify and Fix Unit Tests\*\*

- **Description**: Ensure all unit tests are passing locally. This includes running the race detector (`go test -race`) to catch concurrency issues as highlighted in the V2 spec. Address any failures, such as those related to `http.NoBody` preferences or other test-specific issues.
- **Acceptance Criteria**:
  - The command to run unit tests (e.g., `make test`) completes successfully.
  - The command to run the race detector (e.g., `make test-race`) completes successfully.
  - All failing tests are fixed and committed to the repository.

## Ticket 4: Local - Verify and Fix Linting\*\*

- **Description**: Run the linter locally (e.g., `make lint`) and methodically fix all reported issues. The V2 specification identified several common problems to look for: `gofmt` errors, variable shadowing, missing error checks, and improper use of `log.Fatalf` with `defer`.
- **Acceptance Criteria**:
  - The linting command runs without reporting any errors.
  - All linting issues are resolved in the codebase.
  - The fixes are committed to the repository.

## Ticket 5: Local - Verify and Fix E2E Tests\*\*

- **Description**: Run the Playwright E2E tests locally and resolve any failures one by one. The V2 specification notes potential environment issues, such as `docker-compose` discrepancies or the need for service health checks before tests run.
- **Acceptance Criteria**:
  - The full Playwright test suite runs and passes locally.
  - Any environment or test-related issues are resolved.
  - The fixes are committed to the repository.
