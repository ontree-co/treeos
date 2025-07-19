# OnTree Node E2E Tests

This directory contains end-to-end tests for the OnTree Node application using Playwright.

## Prerequisites

- Node.js and npm installed
- OnTree server running on port 3001
- Docker installed and running (for app management tests)

## Installation

```bash
npm install
npx playwright install chromium
```

## Running Tests

### Run all tests
```bash
npm test
```

### Run specific test suites
```bash
npm run test:setup     # Initial setup tests
npm run test:auth      # Authentication tests  
npm run test:dashboard # Dashboard and vitals tests
npm run test:apps      # App management tests
npm run test:docker    # Docker update tests
```

### Run tests in UI mode
```bash
npm run test:ui
```

### Run tests in headed mode
```bash
npm run test:headed
```

## Test Structure

```
tests/e2e/
├── auth/
│   ├── setup.test.js    # Initial setup flow
│   └── login.test.js    # Login/logout functionality
├── dashboard/
│   └── vitals.test.js   # Dashboard and system vitals
├── apps/
│   ├── create.test.js   # App creation validation
│   ├── manage.test.js   # Start/stop/recreate/delete
│   └── templates.test.js # Template system
├── docker/
│   └── updates.test.js  # Docker image updates
├── global-setup.js      # Enhanced test environment setup
├── global-teardown.js   # Enhanced test environment cleanup
└── helpers.js           # Common test utilities
```

## Environment Variables

The test suite supports the following environment variables for configuration:

### Setup Configuration
- `TEST_BASE_URL` - Base URL for the test server (default: `http://localhost:3001`)
- `TEST_PORT` - Port for the test server (default: `3001`)
- `TEST_DB_PATH` - Path to the test database file (default: `./ontree.db`)
- `TEST_ADMIN_USER` - Admin username for tests (default: `admin`)
- `TEST_ADMIN_PASSWORD` - Admin password for tests (default: `admin1234`)
- `TEST_NODE_NAME` - Name for the test node (default: `Test OnTree Node`)
- `TEST_NODE_DESCRIPTION` - Description for the test node (default: `This is a test node for e2e testing`)

### Teardown Configuration
- `KEEP_SERVER_RUNNING` - Set to `true` to keep the server running after tests (default: `false`)
- `KEEP_TEST_DATA` - Set to `true` to preserve test data after tests (default: `false`)

### Example Usage
```bash
# Run tests with custom configuration
TEST_PORT=3002 TEST_ADMIN_USER=testadmin npm test

# Run tests and keep server running for debugging
KEEP_SERVER_RUNNING=true npm test

# Run tests and preserve test data for inspection
KEEP_TEST_DATA=true npm test
```

## Enhanced Features

### Global Setup (`global-setup.js`)
- **Health Check with Retries**: Implements exponential backoff retry mechanism for server health checks
- **Environment Variable Support**: Full configuration through environment variables
- **Enhanced Docker Cleanup**: Comprehensive cleanup of containers, networks, and volumes
- **Better Logging**: Clear, emoji-enhanced logging for better visibility
- **Error Handling**: Improved error handling and recovery

### Global Teardown (`global-teardown.js`)
- **Comprehensive Cleanup**: Removes containers, networks, volumes, and test app directories
- **Configurable Behavior**: Options to keep server running or preserve test data
- **Graceful Shutdown**: Attempts graceful server shutdown before force killing
- **Temporary File Cleanup**: Removes log files and other temporary artifacts

## Notes

- Tests run with a fresh database (cleaned in global-setup.js)
- The server is automatically restarted before tests
- Test containers are prefixed with `ontree-test-`
- All test data is cleaned up after tests complete (unless `KEEP_TEST_DATA=true`)
- The setup process seeds an admin user with configurable credentials
- Docker resources (containers, networks, volumes) are thoroughly cleaned up