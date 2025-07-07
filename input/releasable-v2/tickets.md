# OnTree V2 Release Tickets - CI/CD Hardening

This file breaks down the work required to harden the OnTree CI/CD pipeline, based on the lessons learned from the V2 specification.

---

## Ticket 1: CI - Separate CI Jobs\*\*

- **Description**: Split the main CI workflow into three distinct jobs: `test`, `lint`, and `build`. This will improve feedback time by running jobs in parallel and make it easier to identify the source of failures.
- **Acceptance Criteria**:
  - The GitHub Actions workflow is updated to have three separate jobs.
  - Each job runs independently and reports its status correctly.
  - The changes are committed to the repository.

## Ticket 2: CI - Go Version Matrix\*\*

- **Description**: Configure the `test` job to run across multiple Go versions (1.21, 1.22, and 1.23) to ensure backward and forward compatibility.
- **Acceptance Criteria**:
  - The `test` job successfully runs and passes for all specified Go versions.
  - The workflow file is updated with the version matrix strategy.
  - The changes are committed to the repository.

## Ticket 3: CI - Implement Go Module Caching\*\*

- **Description**: Add caching for Go modules to the CI workflows to speed up build and test times. Use `cache: true` and set `cache-dependency-path: go.sum`.
- **Acceptance Criteria**:
  - Subsequent CI runs are noticeably faster due to dependency caching.
  - The workflow files are updated to include the caching mechanism.
  - The changes are committed to the repository.

## Ticket 4: CI - Adopt `golangci-lint` GitHub Action\*\*

- **Description**: Replace the manual installation script for `golangci-lint` with the official `golangci/golangci-lint-action`. This ensures a reliable and up-to-date linter is always used.
- **Acceptance Criteria**:
  - The `lint` job uses the official GitHub Action.
  - The linter runs successfully within the new action.
  - The changes are committed to the repository.

## Ticket 5: CI - Simplify Workflow Triggers\*\*

- **Description**: Update the workflow triggers to run only on specific, relevant branches (e.g., `main`, `develop`, `release/*`) instead of all branches (`*`).
- **Acceptance Criteria**:
  - CI workflows are triggered only for pushes and pull requests to the specified branches.
  - The changes are committed to the repository.

## Ticket 6: Build - Enhance Makefile\*\*

- **Description**: Update the `Makefile` to include new targets for `test-race`, `test-coverage`, `install-tools`, `fmt`, and `vet`. Add a `help` target to make the Makefile self-documenting.
- **Acceptance Criteria**:
  - All new Makefile targets (`test-race`, `test-coverage`, `install-tools`, `fmt`, `vet`, `help`) are implemented and function correctly.
  - The changes are committed to the repository.

## Ticket 7: CI - Add Race Detector to Tests\*\*

- **Description**: Update the `test` job in the CI workflow to run the race detector (`go test -race`) to identify potential concurrency issues in the codebase.
- **Acceptance Criteria**:
  - The `test` job executes the race detector.
  - Any race conditions identified are fixed.
  - The changes are committed to the repository.

## Ticket 8: Linting - Enhance `.golangci.yml`\*\*

- **Description**: Update the `.golangci.yml` configuration to enable and configure additional linters, including `gosec`, `gosimple`, `unused`, `errcheck`, and `revive`.
- **Acceptance Criteria**:
  - The `.golangci.yml` file is updated with the new linters and their configurations.
  - The application code passes the new, stricter linting rules.
  - The changes are committed to the repository.

### Phase 3: E2E Testing and Local Verification

## Ticket 9: CI - Restructure E2E Test Workflow\*\*

- **Description**: Refactor the E2E test setup in the CI workflow to use a PostgreSQL service container directly, instead of `docker-compose`. This involves running the application binary directly, adding a database migration step, and using a health check to ensure the app is ready before tests run.
- **Acceptance Criteria**:
  - The E2E tests run successfully in CI using a service container.
  - `docker-compose` is removed from the CI E2E test setup.
  - The changes are committed to the repository.

## Ticket 10: Local - Verify and Fix Unit Tests\*\*

- **Description**: Ensure all unit tests are passing locally. This includes running the race detector (`go test -race`) to catch concurrency issues as highlighted in the V2 spec. Address any failures, such as those related to `http.NoBody` preferences or other test-specific issues.
- **Acceptance Criteria**:
  - The command to run unit tests (e.g., `make test`) completes successfully.
  - The command to run the race detector (e.g., `make test-race`) completes successfully.
  - All failing tests are fixed and committed to the repository.

## Ticket 11: Local - Verify and Fix Linting\*\*

- **Description**: Run the linter locally (e.g., `make lint`) and methodically fix all reported issues. The V2 specification identified several common problems to look for: `gofmt` errors, variable shadowing, missing error checks, and improper use of `log.Fatalf` with `defer`.
- **Acceptance Criteria**:
  - The linting command runs without reporting any errors.
  - All linting issues are resolved in the codebase.
  - The fixes are committed to the repository.

## Ticket 12: Local - Verify and Fix E2E Tests\*\*

- **Description**: Run the Playwright E2E tests locally and resolve any failures one by one. The V2 specification notes potential environment issues, such as `docker-compose` discrepancies or the need for service health checks before tests run.
- **Acceptance Criteria**:
  - The full Playwright test suite runs and passes locally.
  - Any environment or test-related issues are resolved.
  - The fixes are committed to the repository.
