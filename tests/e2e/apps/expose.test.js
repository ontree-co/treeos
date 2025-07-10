const { test, expect } = require('@playwright/test');
const { loginAsAdmin, createTestApp, waitForOperation } = require('../helpers');

test.describe('App Creation and Exposure', () => {
  test.beforeEach(async ({ page }) => {
    // Set a longer timeout for navigation
    page.setDefaultTimeout(30000);
    
    // Login before each test
    await loginAsAdmin(page);
  });

  test('should create app and expose with subdomain', async ({ page }) => {
    // Step 1: Create a test app using the standard method
    const appName = `expose-test-${Date.now()}`;
    await page.goto('/apps/create');
    
    // Fill in the form
    await page.fill('input[name="app_name"]', appName);
    await page.fill('textarea[name="compose_content"]', `version: '3'
services:
  ${appName}:
    image: nginx:alpine
    container_name: ontree-${appName}
    ports:
      - "8080:80"`);
    
    // Submit the form
    await page.click('button[type="submit"]');
    
    // Wait for redirect to app detail page
    await page.waitForURL(`/apps/${appName}`);
    await expect(page.locator('h1')).toContainText(appName);
    
    // Step 2: Start the container
    // Check if "Create & Start" button exists, otherwise use "Start" button
    const createStartButton = page.locator('button:has-text("Create & Start")');
    const startButton = page.locator('button:has-text("Start")').first();
    
    if (await createStartButton.isVisible()) {
      await createStartButton.click();
    } else if (await startButton.isVisible()) {
      await startButton.click();
    }
    
    // Wait for operation to complete
    await waitForOperation(page, 60000);
    
    // Reload to get latest status
    await page.reload();
    
    // Step 3: Check if exposure functionality is available
    // Look for the Domain & Access section
    const domainSection = page.locator('div.card:has(h5:has-text("Domain & Access"))');
    await expect(domainSection).toBeVisible();
    
    // Check if domains are configured
    const noDomains = await page.locator('text=No domains configured').isVisible();
    const caddyNotAvailable = await page.locator('text=Caddy is not available').isVisible();
    
    if (noDomains || caddyNotAvailable) {
      console.log('Exposure functionality not available in test environment');
      // Clean up - stop the container
      await page.click('button:has-text("Stop")');
      await waitForOperation(page, 30000);
      return; // Skip the rest of the test
    }
    
    // Fill in subdomain
    const subdomain = `test-${Date.now()}`;
    await domainSection.locator('input[name="subdomain"]').fill(subdomain);
    
    // Click expose button
    await domainSection.locator('button:has-text("Expose App")').click();
    
    // Wait for operation to complete
    await expect(page.locator('.alert-success')).toBeVisible({ timeout: 10000 });
    await expect(page.locator('.alert-success')).toContainText('exposed successfully');
    
    // Step 6: Verify app is exposed
    // The section should now show the exposed state
    await page.reload();
    
    const exposedSection = page.locator('div.card:has(h5:has-text("Domain & Access"))');
    await expect(exposedSection).toBeVisible();
    
    // Should show current subdomain
    await expect(exposedSection).toContainText(`Subdomain: ${subdomain}`);
    
    // Should show access URLs
    await expect(exposedSection).toContainText('Access URLs:');
    
    // Should have unexpose button
    await expect(exposedSection.locator('button:has-text("Unexpose App")')).toBeVisible();
    
    // Step 7: Check status button functionality
    const checkStatusButton = exposedSection.locator('button:has-text("Check Status")');
    if (await checkStatusButton.isVisible()) {
      await checkStatusButton.click();
      // Wait for status check to complete
      await page.waitForTimeout(2000);
    }
    
    // Clean up: Unexpose the app
    await exposedSection.locator('button:has-text("Unexpose App")').click();
    
    // Confirm unexpose
    const confirmButton = page.locator('button:has-text("Yes, Unexpose")');
    if (await confirmButton.isVisible({ timeout: 2000 })) {
      await confirmButton.click();
    }
    
    // Wait for unexpose to complete
    await expect(page.locator('.alert-success')).toBeVisible({ timeout: 10000 });
    await expect(page.locator('.alert-success')).toContainText('unexposed successfully');
    
    // Stop the container
    await page.click('button:has-text("Stop")');
    await waitForOperation(page, 30000);
  });

  test.skip('should validate subdomain format', async ({ page }) => {
    // Skip this test in CI as it requires domain configuration
    // This test would validate subdomain input patterns when domains are configured
  });

  test.skip('should remember previously used subdomain', async ({ page }) => {
    // Skip this test in CI as it requires domain configuration
    // This test would verify that the subdomain input remembers previously used values
  });
});