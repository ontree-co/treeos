const { defineConfig, devices } = require('@playwright/test');

// Force headless mode regardless of any environment variables or CLI args
process.env.HEADED = 'false';
process.env.DISPLAY = '';

/**
 * @see https://playwright.dev/docs/test-configuration
 */
module.exports = defineConfig({
  testDir: './',
  /* Run tests in files in parallel, but tests within a file sequentially */
  fullyParallel: false,
  /* Fail the build on CI if you accidentally left test.only in the source code. */
  forbidOnly: !!process.env.CI,
  /* Retry on CI only */
  retries: process.env.CI ? 2 : 0,
  /* Use multiple workers for faster execution - one per file */
  workers: process.env.CI ? 4 : 6,
  /* Reporter to use. See https://playwright.dev/docs/test-reporters */
  reporter: 'html',
  /* Shared settings for all the projects below. See https://playwright.dev/docs/api/class-testoptions. */
  use: {
    /* Base URL to use in actions like `await page.goto('/')`. */
    baseURL: process.env.BASE_URL || 'http://localhost:3002',

    /* Collect trace when retrying the failed test. See https://playwright.dev/docs/trace-viewer */
    trace: 'on-first-retry',

    /* Screenshot on failure */
    screenshot: 'only-on-failure',

    /* Video on failure */
    video: 'retain-on-failure',

    /* Always run in headless mode - required for environments without display */
    headless: true,
  },

  /* Configure projects for major browsers */
  projects: [
    {
      name: 'chromium',
      use: {
        ...devices['Desktop Chrome'],
        headless: true,  // Force headless for this project
        launchOptions: {
          headless: true,  // Double ensure headless at launch level
          args: ['--headless=new', '--no-sandbox', '--disable-setuid-sandbox', '--disable-gpu', '--disable-dev-shm-usage']
        }
      },
    },
  ],

  /* Run your local dev server before starting the tests */
  // webServer: {
  //   command: 'cd ../.. && go run ./cmd/treeos',
  //   port: 8083,
  //   timeout: 120 * 1000,
  //   reuseExistingServer: !process.env.CI,
  // },

  /* Test timeout */
  timeout: 60 * 1000,

  /* Global setup/teardown - always run to ensure proper test environment */
  globalSetup: require.resolve('./global-setup.js'),
  globalTeardown: require.resolve('./global-teardown.js'),
});