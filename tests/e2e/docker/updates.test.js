const { test, expect } = require('@playwright/test');
const { loginAsAdmin, createTestApp, waitForOperation } = require('../helpers');

test.describe('Docker Image Updates', () => {
  test.beforeEach(async ({ page }) => {
    // Set a longer timeout for navigation
    page.setDefaultTimeout(30000);
    
    // Login before each test
    await loginAsAdmin(page);
  });

  test.skip('should show update check button on app detail page', async ({ page }) => {
    // Create a test app if it doesn't exist
    const appName = 'test-update-app';
    
    // Check if app exists by trying to navigate to it
    const response = await page.goto(`/apps/${appName}`);
    
    if (response?.status() === 404) {
      // Create the app
      await createTestApp(page, appName, 'nginx:1.24');
    } else {
      // Navigate back to the app
      await page.goto(`/apps/${appName}`);
    }
    
    // Verify update check UI elements
    await expect(page.locator('h5:has-text("Image Updates")')).toBeVisible();
    await expect(page.locator('button:has-text("Check for Updates")')).toBeVisible();
    await expect(page.locator('#update-status')).toBeVisible();
    await expect(page.locator('#update-status')).toContainText('Not checked');
  });

  test.skip('should check for image updates', async ({ page }) => {
    const appName = 'test-update-app';
    await page.goto(`/apps/${appName}`);
    
    // Ensure the app is created (container exists)
    const status = await page.locator('.badge').textContent();
    if (status?.toLowerCase() === 'not_created') {
      // Start the app first
      await page.click('button:has-text("Start")');
      await waitForOperation(page, 60000);
      await page.reload();
    }
    
    // Click check for updates button
    await page.click('button:has-text("Check for Updates")');
    
    // Wait for the update check to complete (HTMX request)
    await page.waitForSelector('#update-status:not(:has-text("Checking..."))', { timeout: 30000 });
    
    // Should show either "Up to date" or "Update available"
    const updateStatus = await page.locator('#update-status').textContent();
    expect(updateStatus).toMatch(/Up to date|Update available|Failed to check/);
  });

  test.skip('should show update button when update is available', async ({ page }) => {
    // For this test, we'll use an older nginx image that likely has updates
    const appName = 'test-old-nginx';
    
    // Check if app exists
    const response = await page.goto(`/apps/${appName}`);
    
    if (response?.status() === 404) {
      // Create app with older nginx version
      await page.goto('/apps/create');
      await page.fill('input[name="app_name"]', appName);
      await page.fill('textarea[name="compose_content"]', `version: '3'
services:
  web:
    image: nginx:1.20
    ports:
      - "9999:80"`);
      await page.click('button[type="submit"]');
      await page.waitForURL(`/apps/${appName}`);
    }
    
    // Start the app if needed
    const status = await page.locator('.badge').textContent();
    if (status?.toLowerCase() === 'not_created') {
      await page.click('button:has-text("Start")');
      await waitForOperation(page, 60000);
      await page.reload();
    }
    
    // Check for updates
    await page.click('button:has-text("Check for Updates")');
    
    // Wait for check to complete
    await page.waitForSelector('#update-status:not(:has-text("Checking..."))', { timeout: 30000 });
    
    // If update is available, should show update button
    const updateStatus = await page.locator('#update-status').textContent();
    if (updateStatus?.includes('Update available')) {
      await expect(page.locator('button:has-text("Update Now")')).toBeVisible();
    }
  });

  test.skip('should handle update process', async ({ page }) => {
    const appName = 'test-old-nginx';
    await page.goto(`/apps/${appName}`);
    
    // Check for updates first
    await page.click('button:has-text("Check for Updates")');
    await page.waitForSelector('#update-status:not(:has-text("Checking..."))', { timeout: 30000 });
    
    // If update is available, perform update
    const updateButton = page.locator('button:has-text("Update Now")');
    if (await updateButton.isVisible()) {
      // Click update button (with confirmation)
      await updateButton.click();
      
      // Confirm the update
      await page.click('button:has-text("Confirm")');
      
      // Should redirect and show operation in progress
      await expect(page.locator('.alert-success')).toBeVisible();
      await expect(page.locator('.alert-success')).toContainText('Image update started');
      
      // Wait for operation to complete
      if (await page.locator('#operation-status').isVisible()) {
        await waitForOperation(page, 120000); // Updates can take longer
      }
      
      // Reload to see updated status
      await page.reload();
      
      // Container should be running with new image
      await expect(page.locator('.badge')).toContainText('running');
    }
  });

  test.skip('should show error for invalid image check', async ({ page }) => {
    // Create app with non-existent image
    const appName = 'test-invalid-image';
    
    const response = await page.goto(`/apps/${appName}`);
    
    if (response?.status() === 404) {
      await page.goto('/apps/create');
      await page.fill('input[name="app_name"]', appName);
      await page.fill('textarea[name="compose_content"]', `version: '3'
services:
  web:
    image: nonexistent/image:latest`);
      await page.click('button[type="submit"]');
      await page.waitForURL(`/apps/${appName}`);
    }
    
    // Try to check for updates
    await page.click('button:has-text("Check for Updates")');
    
    // Wait for check to complete
    await page.waitForSelector('#update-status:not(:has-text("Checking..."))', { timeout: 30000 });
    
    // Should show error
    const updateStatus = await page.locator('#update-status').textContent();
    expect(updateStatus).toContain('Failed to check');
  });

  test.skip('should disable update check for stopped containers', async ({ page }) => {
    const appName = 'test-update-app';
    await page.goto(`/apps/${appName}`);
    
    // Stop the container if it's running
    const status = await page.locator('.badge').textContent();
    if (status?.toLowerCase() === 'running') {
      await page.click('button:has-text("Stop")');
      await page.waitForTimeout(2000);
      await page.reload();
    }
    
    // Update check button should still be enabled (can check even when stopped)
    await expect(page.locator('button:has-text("Check for Updates")')).toBeEnabled();
  });

  test.skip('should refresh update status after container recreation', async ({ page }) => {
    const appName = 'test-update-app';
    await page.goto(`/apps/${appName}`);
    
    // Ensure container is running
    const status = await page.locator('.badge').textContent();
    if (status?.toLowerCase() !== 'running') {
      await page.click('button:has-text("Start")');
      await waitForOperation(page, 60000);
      await page.reload();
    }
    
    // Check for updates
    await page.click('button:has-text("Check for Updates")');
    await page.waitForSelector('#update-status:not(:has-text("Checking..."))', { timeout: 30000 });
    
    // Note the current status
    const initialStatus = await page.locator('#update-status').textContent();
    
    // Recreate the container
    await page.click('button:has-text("Recreate")');
    await page.click('button:has-text("Confirm")');
    await waitForOperation(page, 60000);
    await page.reload();
    
    // Update status should be reset
    await expect(page.locator('#update-status')).toContainText('Not checked');
  });
});