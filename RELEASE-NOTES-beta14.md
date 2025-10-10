# TreeOS v1.0.0-beta.14 Release Notes

## 🎉 New Features

### New Application Templates
- **Miniflux RSS Reader**: A minimalist and privacy-focused RSS feed reader with automatic tracking removal, full-text extraction, and keyboard navigation support. Includes PostgreSQL database and comprehensive configuration options.
- **Readeck**: A simple web application to save and organize articles, web pages, and documents for later reading with full-text search and offline reading capabilities.

### Developer Experience Improvements
- **ASDF Tool Version Management**: Migrated to `.tool-versions` for consistent tool versions across all development environments
- **Development Setup Script**: Added automated setup script with lowest-friction approach for new developers
- **Enhanced Documentation**: Added comprehensive guide for app backup and restoration

## 🐛 Bug Fixes

### Container Operations
- **Removed 5-minute timeout**: Fixed issue where container start operations would timeout after 5 minutes, now containers can take as long as needed to start

### CI/CD Pipeline
- **Fixed Node.js version management**: Updated GitHub Actions to use `.tool-versions` instead of deprecated `.node-version` files
- **E2E test fixes**: Added embed-assets step to CI workflow to ensure templates are available during testing
- **Linting configuration**: Updated to golangci-lint v2.5.0 with proper v2 configuration format

### UI Improvements
- **Monitoring cards**: Added "Last 24 hours" label to monitoring cards in dashboard for clarity

## 🔧 Technical Improvements

### Build System
- **Makefile updates**: Updated to use golangci-lint v2.5.0 from asdf for consistent linting
- **Version management**: Consolidated tool versions in single `.tool-versions` file (Go 1.24.4, golangci-lint 2.5.0, Node.js 22.11.0)

### Documentation
- **Nzyme investigation**: Preserved comprehensive Docker investigation for nzyme network defense system in future/ folder for reference
- **Version mismatch protocol**: Added guidelines for handling tool version mismatches in CLAUDE.md

## 📝 Configuration Changes

### Miniflux Template
- Requires explicit `.env` configuration for all passwords (no hidden defaults)
- Includes comprehensive `.env.example` file with all configuration options
- Supports OAuth2, polling frequency, worker pool size, and timezone configuration

### Readeck Template
- Simple single-container setup with SQLite database
- Automatic volume management for data persistence
- Support for custom domains and ports

## 🚀 Migration Notes

### For Developers
1. Remove any local `.node-version` files
2. Install asdf and required plugins:
   ```bash
   asdf plugin add golang
   asdf plugin add golangci-lint
   asdf plugin add nodejs
   ```
3. Run `asdf install` in the project root to get correct tool versions

### For Users
- No migration required for existing installations
- New Miniflux installations require creating `.env` file from `.env.example` template

## 📊 Statistics
- 15+ commits since beta.13
- 2 new application templates added
- 3 major CI/CD issues resolved
- Improved developer onboarding experience

## 🙏 Acknowledgments
Thanks to all contributors and users who reported issues and provided feedback for this release!

---
*For detailed commit history, see: [v1.0.0-beta.13...v1.0.0-beta.14](https://github.com/stefanmunz/treeos/compare/v1.0.0-beta.13...v1.0.0-beta.14)*