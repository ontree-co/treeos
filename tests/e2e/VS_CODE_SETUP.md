# Running E2E Tests in VS Code

## Setup

1. **Install the Playwright Extension**:
   - Open VS Code
   - Go to Extensions (Ctrl+Shift+X)
   - Search for "Playwright Test for VSCode"
   - Install the extension by Microsoft

2. **Install Playwright Dependencies**:
   ```bash
   cd tests/e2e
   npm install
   npx playwright install
   ```

## Running Tests from VS Code

### Method 1: Using the Testing Tab (Recommended)
1. Open the Testing tab in VS Code (flask icon in the activity bar)
2. You should see:
   - **Go Tests** (80+ unit tests)
   - **Playwright Tests** (65+ e2e tests) - if the extension is installed
3. Click the play button next to any test to run it
4. Click the debug button to debug with breakpoints

### Method 2: Using Launch Configurations
1. Go to Run and Debug (Ctrl+Shift+D)
2. Select from the dropdown:
   - **"Debug Playwright Test"** - Run tests with Playwright Inspector
   - **"Run E2E Tests (with server)"** - Run all tests with server startup

### Method 3: Using Tasks
1. Open Command Palette (Ctrl+Shift+P)
2. Type "Tasks: Run Task"
3. Select:
   - **"Run Playwright Tests"** - Run all e2e tests
   - **"test-e2e"** - Run via Make (includes server setup)

### Method 4: Using npm Scripts
In the terminal:
```bash
cd tests/e2e

# Run all tests
npm test

# Run with UI mode (interactive)
npm run test:ui

# Run in headed mode (see browser)
npm run test:headed

# Debug mode (with Playwright Inspector)
npm run test:debug

# Run specific test suites
npm run test:auth      # Auth tests only
npm run test:dashboard # Dashboard tests only
npm run test:apps      # App management tests only
```

## Test File Locations
- `tests/e2e/auth/` - Authentication tests
- `tests/e2e/apps/` - Application lifecycle tests
- `tests/e2e/dashboard/` - Dashboard and vitals tests

## Troubleshooting

### Tests Not Showing in Testing Tab
1. Ensure Playwright extension is installed
2. Reload VS Code window (Ctrl+Shift+P → "Developer: Reload Window")
3. Check that `playwright.config.js` exists in `tests/e2e/`

### Server Connection Issues
- The tests expect the server on port 3001
- Tests will automatically start a server if needed
- To use an existing server, start it first:
  ```bash
  make run
  ```

### Debugging Tips
1. Use `test.only()` to run a single test
2. Add `await page.pause()` to stop execution and inspect
3. Use the Playwright Inspector with debug mode
4. Check test artifacts in `tests/e2e/test-results/`

## VS Code Settings
The following settings have been configured:
- `playwright.reuseBrowser`: Reuses browser between test runs (faster)
- `playwright.showTrace`: Shows trace viewer on failure
- `testing.automaticallyOpenPeekView`: Opens test results inline

## Running Specific Tests
To run a specific test file from VS Code:
1. Open the test file
2. Click the green play button in the gutter next to the test
3. Or right-click and select "Run Test" or "Debug Test"