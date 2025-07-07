# OnTree Release Specification

## Overview
This specification outlines the requirements and implementation plan for getting OnTree to a production-ready, releasable state. The goal is to create installable releases that support both ARM64 (Apple Silicon Macs) and AMD64 (modern x86_64 CPUs) architectures, with proper CI/CD pipeline and deployment automation.

## Release Goals

### 1. Testing & Quality Assurance
- **Unit Tests**: Achieve >70% code coverage across all packages
- **Integration Tests**: Test Docker API interactions, database operations, and API endpoints
- **E2E Tests**: Ensure existing Playwright tests pass consistently
- **CI Pipeline**: All tests must pass in GitHub Actions before release

### 2. Build & Distribution
- **Multi-Architecture Support**:
  - darwin/arm64 (Apple Silicon Macs)
  - linux/amd64 (Intel/AMD servers)
  - Optional: darwin/amd64, linux/arm64
- **Release Artifacts**:
  - Binary executables for each platform
  - Docker images (multi-arch)
  - Debian/RPM packages (optional)
  - Homebrew formula (optional)

### 3. Deployment Infrastructure
- **Ansible Playbooks**:
  - Update existing playbooks to deploy Go binaries (not Django)
  - Production deployment with systemd service
  - Local development setup
  - Automated updates and rollbacks

## Implementation Plan

### Phase 1: Build Infrastructure (Week 1)

#### 1.1 Create Makefile
```makefile
# Key targets needed:
- build: Compile for current platform
- build-all: Cross-compile for all platforms
- test: Run unit tests
- test-e2e: Run Playwright E2E tests
- lint: Run golangci-lint
- clean: Remove build artifacts
- release: Create release binaries
```

#### 1.2 Version Management
- Implement semantic versioning (e.g., v1.0.0)
- Store version in `version.go` or use build flags
- Git tags for releases

### Phase 2: CI/CD Pipeline (Week 1-2)

#### 2.1 GitHub Actions Workflows

**`.github/workflows/test.yml`** - On every push/PR:
```yaml
- Checkout code
- Setup Go 1.21+
- Run unit tests
- Run linting
- Build binary
- Run E2E tests with Docker
- Upload test coverage
```

**`.github/workflows/release.yml`** - On version tags:
```yaml
- Run all tests
- Build multi-arch binaries
- Create GitHub release
- Upload release artifacts
- Build and push Docker images
- Update release notes
```

#### 2.2 GoReleaser Configuration
- Configure `.goreleaser.yml` for automated releases
- Define build targets for each platform
- Setup changelog generation
- Configure Docker image building

### Phase 3: Testing Improvements (Week 2)

#### 3.1 Unit Tests
Prioritize testing for:
- Database operations (`internal/db/`)
- Docker client operations (`internal/docker/`)
- API handlers (`internal/server/`)
- Worker pool (`internal/worker/`)

#### 3.2 Integration Tests
- Mock Docker API for reliable testing
- Test database migrations
- Test authentication flows
- Test background job processing

#### 3.3 Test Infrastructure
- Setup test database fixtures
- Configure test containers for Docker
- Add test coverage reporting
- Setup parallel test execution

### Phase 4: Ansible Updates (Week 2-3)

#### 4.1 Fix Systemd Service Template
Update `ansible/templates/ontreenode.service.j2`:
```ini
[Unit]
Description=OnTree Node Service
After=network.target

[Service]
Type=simple
User={{ app_user }}
Group={{ app_group }}
WorkingDirectory={{ app_dir }}
ExecStart={{ app_dir }}/ontree-node
Restart=on-failure
RestartSec=10

# Security hardening
PrivateTmp=true
NoNewPrivileges=true
ProtectSystem=strict
ProtectHome=true
ReadWritePaths={{ app_dir }}/data

[Install]
WantedBy=multi-user.target
```

#### 4.2 Production Playbook Updates
- Binary deployment instead of Python/Django
- Database initialization and migrations
- Log rotation setup
- Backup configuration
- Update mechanism

#### 4.3 Local Development Playbook
- Developer environment setup
- Go toolchain installation
- Docker setup
- Test environment configuration

### Phase 5: Release Process (Week 3)

#### 5.1 Release Checklist
1. Update version number
2. Run full test suite
3. Update CHANGELOG.md
4. Create git tag
5. Push tag to trigger release workflow
6. Verify GitHub release artifacts
7. Test installation on target platforms
8. Update documentation

#### 5.2 Installation Methods
- **Direct Download**: From GitHub releases
- **Docker**: `docker pull ontree/ontree-node:latest`
- **Ansible**: Using provided playbooks
- **Package Managers**: Future consideration

### Phase 6: Documentation (Week 3-4)

#### 6.1 Required Documentation
- **README.md**: Project overview, quick start, features
- **INSTALL.md**: Detailed installation instructions
- **DEPLOYMENT.md**: Production deployment guide
- **CONTRIBUTING.md**: Development setup and guidelines
- **API.md**: API documentation
- **CHANGELOG.md**: Release history

#### 6.2 Ansible Documentation
- Prerequisites and requirements
- Variable configuration
- Playbook usage examples
- Troubleshooting guide

## Technical Requirements

### Binary Requirements
- Static linking where possible for portability
- Embedded assets (templates, static files)
- Configuration via environment variables or config file
- Graceful shutdown handling
- Signal handling for reload

### Security Considerations
- No hardcoded credentials
- Secure default configurations
- TLS/HTTPS support (via Caddy)
- Authentication required for all operations
- Rate limiting on API endpoints

### Performance Targets
- Startup time < 2 seconds
- Memory usage < 100MB baseline
- Support 100+ concurrent containers
- Response time < 200ms for UI operations

## Success Criteria

1. **Tests**: All unit, integration, and E2E tests passing in CI
2. **Builds**: Successful cross-platform builds for ARM64 Mac and AMD64 Linux
3. **Installation**: One-command installation via Ansible playbooks
4. **Documentation**: Complete user and deployment documentation
5. **Release**: Automated release process producing downloadable binaries

## Timeline

- **Week 1**: Build infrastructure and CI/CD setup
- **Week 2**: Testing improvements and coverage
- **Week 3**: Ansible updates and release automation
- **Week 4**: Documentation and first release

## Risks & Mitigations

1. **E2E Test Flakiness**: May need to stabilize tests or add retries
2. **Cross-platform Issues**: Test thoroughly on both architectures
3. **Ansible Complexity**: Keep playbooks simple and well-documented
4. **Binary Size**: Monitor and optimize if needed

## Next Steps

1. Review and approve this specification
2. Create GitHub issues for each major task
3. Begin with Makefile and CI/CD setup
4. Iterate based on testing feedback