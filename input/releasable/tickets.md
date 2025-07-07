# OnTree Release Tickets

This file breaks down the work required to get OnTree to its first releasable state, based on the approved specification. Each ticket represents a self-contained unit of work.

---

##Ticket 1: Build - Create Makefile**
  - **Description**: Create a `Makefile` with targets for `build`, `build-all`, `test`, `test-e2e`, `lint`, and `clean`. This will standardize development and CI workflows.
  - **Acceptance Criteria**: 
    - All Makefile targets execute correctly on a local development machine.
    - All existing tests must pass locally after the changes.
    - The changes must be committed.

##Ticket 2: Build - Implement Asset Embedding**
  - **Description**: Modify the Go application to embed the `static/` and `templates/` directories into the binary using the `embed` package.
  - **Acceptance Criteria**: 
    - The application runs correctly without the `static/` and `templates/` directories being present on the filesystem.
    - All existing tests must pass locally after the changes.
    - The changes must be committed.

##Ticket 3: Build - Implement Database Migrations**
  - **Description**: Integrate the `pressly/goose` library to manage database schema migrations. Create an initial migration that reflects the current database schema.
  - **Acceptance Criteria**: 
    - A `migrations` directory is created with at least one SQL migration file.
    - A command (`goose up` or a Makefile target) successfully applies the schema.
    - All existing tests must pass locally after the changes.
    - The changes must be committed.

##Ticket 4: Build - Configure GoReleaser**
  - **Description**: Create a `.goreleaser.yml` file. Configure it to build binaries for `darwin/arm64` and `linux/amd64`, and to generate a changelog from git commits.
  - **Acceptance Criteria**: 
    - Running `goreleaser release --snapshot --clean` locally produces the expected binaries and a changelog.
    - All existing tests must pass locally after the changes.
    - The changes must be committed.

##Ticket 5: CI/CD - Create Test Workflow**
  - **Description**: Create a GitHub Actions workflow in `.github/workflows/test.yml` that triggers on every push and pull request. It should use the `Makefile` to run linting, unit tests, and build the application.
  - **Acceptance Criteria**: 
    - The workflow passes for any push that meets quality standards.
    - The changes must be committed.

##Ticket 6: CI/CD - Add E2E Tests to Workflow**
  - **Description**: Extend the `test.yml` workflow to run the Playwright E2E tests. This will involve setting up a full environment with Docker Compose.
  - **Acceptance Criteria**: 
    - The E2E test suite runs to completion in the CI pipeline.
    - The changes must be committed.

##Ticket 7: CI/CD - Create Release Workflow**
  - **Description**: Create a GitHub Actions workflow in `.github/workflows/release.yml` that triggers on new version tags (e.g., `v0.1.0`). It should use GoReleaser to build, create a GitHub Release, and upload artifacts.
  - **Acceptance Criteria**: 
    - Pushing a new tag successfully creates a GitHub Release with binaries and a changelog.
    - The changes must be committed.

##Ticket 8: Deployment - Update Ansible Production Playbook**
  - **Description**: Refactor the `ontreenode-enable-production-playbook.yaml` to deploy the new Go binary from a GitHub Release instead of cloning the repository. The playbook should handle downloading the artifact, running migrations, and restarting the service.
  - **Acceptance Criteria**: 
    - The playbook successfully deploys `v0.1.0` of the application to a target server.
    - All existing tests must pass locally after the changes.
    - The changes must be committed.

##Ticket 9: Deployment - Verify Ansible Development Playbook**
  - **Description**: Review and test the `ontreenode-allow-local-development-playbook.yaml` to ensure it correctly stops the production service, allowing for manual debugging on a server.
  - **Acceptance Criteria**: 
    - The playbook successfully stops the `ontreenode` service.
    - The changes must be committed.

##Ticket 10: Documentation - Write Core Docs**
  - **Description**: Create initial versions of `README.md`, `INSTALL.md`, and `DEPLOYMENT.md`.
  - **Acceptance Criteria**: 
    - The documentation provides clear, actionable instructions for getting started, installing, and deploying the application.
    - The changes must be committed.