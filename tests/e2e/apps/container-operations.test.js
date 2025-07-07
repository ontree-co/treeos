const { test, expect } = require('@playwright/test');
const { loginAsAdmin } = require('../helpers');

test.describe('Container Operations UI', () => {
  test.beforeEach(async ({ page }) => {
    // Set a longer timeout for navigation
    page.setDefaultTimeout(30000);
    
    // Login before each test
    await loginAsAdmin(page);
  });

  test('should show "Creating & Starting..." during create operation', async ({ page }) => {
    const appName = 'test-create-ui';
    
    // First create the app
    await page.goto('/apps/create');
    await page.fill('input[name="app_name"]', appName);
    
    const composeContent = `version: '3'
services:
  app:
    image: nginx:alpine
    ports:
      - "9999:80"`;
    
    await page.fill('textarea[name="compose_content"]', composeContent);
    await page.click('button[type="submit"]');
    
    // Should redirect to app detail page
    await page.waitForURL(`/apps/${appName}`);
    
    // Click Create & Start button
    await page.click('button:has-text("Create & Start")');
    
    // Button should change to show spinner and "Creating & Starting..."
    await expect(page.locator('#app-controls button:disabled')).toBeVisible();
    await expect(page.locator('#app-controls button:disabled .spinner-border')).toBeVisible();
    await expect(page.locator('#app-controls button:disabled')).toContainText('Creating & Starting...');
    
    // Operation logs should appear
    await expect(page.locator('#operation-logs-container')).toBeVisible();
    await expect(page.locator('#logs-content')).toBeVisible();
    
    // Wait for operation to complete (with longer timeout for image pull)
    await page.waitForFunction(
      () => {
        const logsContent = document.querySelector('#logs-content')?.innerHTML || '';
        return logsContent.includes('Operation completed successfully') || 
               logsContent.includes('Operation failed');
      },
      { timeout: 120000 }
    );
    
    // Button should reload and show Stop after successful creation
    await expect(page.locator('#app-controls button:has-text("Stop")')).toBeVisible({ timeout: 10000 });
    
    // Clean up
    await page.click('button:has-text("Stop")');
    await page.waitForTimeout(2000);
    await page.click('button:has-text("Delete Container")');
    await page.click('button:has-text("Confirm Delete")');
  });

  test('should disable buttons during operations', async ({ page }) => {
    const appName = 'test-button-disable';
    
    // Create app first
    await page.goto('/apps/create');
    await page.fill('input[name="app_name"]', appName);
    
    const composeContent = `version: '3'
services:
  app:
    image: nginx:alpine`;
    
    await page.fill('textarea[name="compose_content"]', composeContent);
    await page.click('button[type="submit"]');
    await page.waitForURL(`/apps/${appName}`);
    
    // Start operation
    await page.click('button:has-text("Create & Start")');
    
    // All control buttons should be replaced with a single disabled button
    const buttons = await page.locator('#app-controls button').all();
    expect(buttons.length).toBe(1);
    expect(await buttons[0].isDisabled()).toBe(true);
    
    // Try clicking the disabled button - nothing should happen
    await buttons[0].click({ force: true });
    
    // Should still be on the same page
    expect(page.url()).toContain(`/apps/${appName}`);
  });

  test('should handle operation completion correctly', async ({ page }) => {
    const appName = 'test-completion';
    
    // Create app
    await page.goto('/apps/create');
    await page.fill('input[name="app_name"]', appName);
    
    const composeContent = `version: '3'
services:
  app:
    image: hello-world`; // Quick image for fast test
    
    await page.fill('textarea[name="compose_content"]', composeContent);
    await page.click('button[type="submit"]');
    await page.waitForURL(`/apps/${appName}`);
    
    // Start monitoring for the operation-complete event
    const operationCompletePromise = page.evaluate(() => {
      return new Promise(resolve => {
        document.body.addEventListener('operation-complete', () => resolve(true), { once: true });
      });
    });
    
    // Start operation
    await page.click('button:has-text("Create & Start")');
    
    // Wait for operation to complete
    const eventFired = await Promise.race([
      operationCompletePromise,
      page.waitForTimeout(60000).then(() => false)
    ]);
    
    expect(eventFired).toBe(true);
    
    // Controls should be refreshed
    await expect(page.locator('#app-controls button:not(:disabled)')).toBeVisible({ timeout: 5000 });
  });

  test('should show appropriate button text based on container status', async ({ page }) => {
    const appName = 'test-button-states';
    
    // Create app
    await page.goto('/apps/create');
    await page.fill('input[name="app_name"]', appName);
    
    const composeContent = `version: '3'
services:
  app:
    image: nginx:alpine`;
    
    await page.fill('textarea[name="compose_content"]', composeContent);
    await page.click('button[type="submit"]');
    await page.waitForURL(`/apps/${appName}`);
    
    // Initial state: not_created
    await expect(page.locator('.badge')).toContainText('Not Created');
    await expect(page.locator('button:has-text("Create & Start")')).toBeVisible();
    
    // Start container
    await page.click('button:has-text("Create & Start")');
    
    // During operation: should show "Creating & Starting..."
    await expect(page.locator('#app-controls button:disabled')).toContainText('Creating & Starting...');
    
    // Wait for completion
    await page.waitForFunction(
      () => !document.querySelector('#app-controls button:disabled'),
      { timeout: 60000 }
    );
    
    // After creation: should show Stop button
    await expect(page.locator('button:has-text("Stop")')).toBeVisible();
    
    // Stop container
    await page.click('button:has-text("Stop")');
    await page.click('button:has-text("Confirm Stop")');
    await page.waitForTimeout(2000);
    
    // After stop: should show Start button (not Create & Start)
    await expect(page.locator('button:has-text("Start"):not(:has-text("Create"))')).toBeVisible();
  });
});