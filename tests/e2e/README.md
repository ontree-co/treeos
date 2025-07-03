# OnTree Node E2E Tests

This directory contains end-to-end tests for the OnTree Node application using Playwright.

## Prerequisites

- Node.js and npm installed
- OnTree server running on port 8085
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
└── helpers.js           # Common test utilities
```

## Notes

- Tests run with a fresh database (cleaned in global-setup.js)
- The server is automatically restarted before tests
- Test containers are prefixed with `ontree-test-`
- All test data is cleaned up after tests complete