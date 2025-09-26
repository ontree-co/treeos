# VS Code Playwright Integration

## Overview
Playwright tests are fully integrated with VS Code's testing panel for easy execution and debugging.

## Configuration
The tests use a special VS Code configuration (`playwright.config.vscode.js`) that ensures:
- Tests always run in headless mode (no display required)
- Proper browser launch arguments for containerized/headless environments
- Disabled GPU and shared memory optimizations

## Running Tests from VS Code

### Using the Testing Panel
1. Open VS Code's Testing panel (beaker icon in the sidebar)
2. Tests will automatically appear under "e2e"
3. Click the play button next to any test to run it
4. Use the debug button to debug with breakpoints

### Using Command Palette
- `Ctrl+Shift+P` → "Test: Run All Tests"
- `Ctrl+Shift+P` → "Test: Debug All Tests"

## Running Tests from Terminal

```bash
# Run all tests
npm test

# Run with VS Code configuration (always headless)
npm run test:vscode

# Run specific test suites
npm run test:auth      # Authentication tests
npm run test:dashboard # Dashboard tests
npm run test:apps      # Application management tests
npm run test:docker    # Docker-related tests

# Run tests with UI (requires display)
npm run test:ui

# Debug mode (requires display)
npm run test:debug
```

## Environment Variables
The following environment variables are automatically set:
- `HEADED=false` - Forces headless mode
- `DISPLAY=""` - Prevents display attachment attempts

## Troubleshooting

### XServer/Display Errors
If you see XServer or display-related errors, the VS Code configuration should resolve them. The tests are configured to run in headless mode by default.

### Test Discovery Issues
If tests don't appear in the testing panel:
1. Ensure Playwright extension is installed
2. Reload VS Code window (`Ctrl+Shift+P` → "Developer: Reload Window")
3. Check that `playwright.config.vscode.js` exists

### Performance
- Tests run faster in headless mode
- Browser reuse is enabled for faster sequential test execution
- Trace collection is enabled for debugging failures