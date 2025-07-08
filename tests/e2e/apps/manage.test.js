const { test, expect } = require('@playwright/test');
const { loginAsAdmin, createTestApp, waitForOperation, isContainerRunning } = require('../helpers');

test.describe('Application Management', () => {
  test.beforeEach(async ({ page }) => {
    // Set a longer timeout for navigation
    page.setDefaultTimeout(30000);
    
    // Login before each test
    await loginAsAdmin(page);
  });

  test.skip('should create a new application', async ({ page }) => {
    // Navigate to create app page
    await page.goto('/apps/create');
    
    // Verify we're on the create page
    await expect(page.locator('h1')).toContainText('Create New Application');
    
    // Fill in the form
    const appName = 'test-nginx';
    await page.fill('input[name="app_name"]', appName);
    
    const composeContent = `version: '3'
services:
  ${appName}:
    image: nginx:alpine
    container_name: ontree-${appName}
    ports:
      - "8888:80"`;
    
    await page.fill('textarea[name="compose_content"]', composeContent);
    
    // Submit the form
    await page.click('button[type="submit"]');
    
    // Should redirect to app detail page
    await page.waitForURL(`/apps/${appName}`);
    
    // Verify success message
    await expect(page.locator('.alert-success')).toBeVisible();
    await expect(page.locator('.alert-success')).toContainText(`Application '${appName}' has been created successfully`);
    
    // Verify app details are shown
    await expect(page.locator('h2')).toContainText(appName);
    await expect(page.locator('.badge')).toContainText('not_created');
  });

  test.skip('should validate app creation form', async ({ page }) => {
    await page.goto('/apps/create');
    
    // Try to submit empty form
    await page.click('button[type="submit"]');
    
    // Should show validation errors
    await expect(page.locator('.alert-danger')).toBeVisible();
    await expect(page.locator('.alert-danger')).toContainText('App name is required');
    
    // Fill app name but leave compose empty
    await page.fill('input[name="app_name"]', 'test-app');
    await page.click('button[type="submit"]');
    
    await expect(page.locator('.alert-danger')).toContainText('Docker compose content cannot be empty');
    
    // Test invalid app name
    await page.fill('input[name="app_name"]', 'test app with spaces');
    await page.fill('textarea[name="compose_content"]', 'some content');
    await page.click('button[type="submit"]');
    
    await expect(page.locator('.alert-danger')).toContainText('Invalid app name');
  });

  test.skip('should start an application', async ({ page }) => {
    const appName = 'test-nginx';
    
    // Navigate to app detail page
    await page.goto(`/apps/${appName}`);
    
    // Click start button
    await page.click('button:has-text("Start")');
    
    // Should show operation in progress
    await expect(page.locator('.alert-info')).toBeVisible();
    await expect(page.locator('.alert-info')).toContainText('Starting application...');
    
    // Wait for operation to complete
    await waitForOperation(page, 60000);
    
    // Reload to see updated status
    await page.reload();
    
    // Should show running status
    await expect(page.locator('.badge')).toContainText('running');
    
    // Start button should be disabled, stop button should be enabled
    await expect(page.locator('button:has-text("Start")')).toBeDisabled();
    await expect(page.locator('button:has-text("Stop")')).toBeEnabled();
  });

  test.skip('should stop a running application', async ({ page }) => {
    const appName = 'test-nginx';
    
    // Navigate to app detail page
    await page.goto(`/apps/${appName}`);
    
    // Ensure app is running
    const status = await page.locator('.badge').textContent();
    if (status?.toLowerCase() !== 'running') {
      // Start it first
      await page.click('button:has-text("Start")');
      await waitForOperation(page, 60000);
      await page.reload();
    }
    
    // Click stop button
    await page.click('button:has-text("Stop")');
    
    // Should show success message
    await expect(page.locator('.alert-success')).toBeVisible();
    await expect(page.locator('.alert-success')).toContainText('Application stopped successfully');
    
    // Should show exited status
    await expect(page.locator('.badge')).toContainText('exited');
    
    // Stop button should be disabled, start button should be enabled
    await expect(page.locator('button:has-text("Stop")')).toBeDisabled();
    await expect(page.locator('button:has-text("Start")')).toBeEnabled();
  });

  test.skip('should recreate an application', async ({ page }) => {
    const appName = 'test-nginx';
    
    // Navigate to app detail page
    await page.goto(`/apps/${appName}`);
    
    // Click recreate button (with confirmation)
    await page.click('button:has-text("Recreate")');
    
    // Confirm the action
    await page.click('button:has-text("Confirm")');
    
    // Should show operation in progress
    await expect(page.locator('.alert-info')).toBeVisible();
    await expect(page.locator('.alert-info')).toContainText('Recreating application...');
    
    // Wait for operation to complete
    await waitForOperation(page, 60000);
    
    // Reload to see updated status
    await page.reload();
    
    // Should be running after recreate
    await expect(page.locator('.badge')).toContainText('running');
  });

  test.skip('should delete an application container', async ({ page }) => {
    const appName = 'test-nginx';
    
    // Navigate to app detail page
    await page.goto(`/apps/${appName}`);
    
    // First stop the container if it's running
    const status = await page.locator('.badge').textContent();
    if (status?.toLowerCase() === 'running') {
      await page.click('button:has-text("Stop")');
      await page.waitForTimeout(2000);
    }
    
    // Click delete button (with confirmation)
    await page.click('button:has-text("Delete Container")');
    
    // Confirm the action
    await page.click('button:has-text("Confirm")');
    
    // Should show success message
    await expect(page.locator('.alert-success')).toBeVisible();
    await expect(page.locator('.alert-success')).toContainText('Container deleted successfully');
    
    // Should show not_created status
    await expect(page.locator('.badge')).toContainText('not_created');
  });

  test.skip('should list applications on dashboard', async ({ page }) => {
    // Navigate to dashboard
    await page.goto('/');
    
    // Should show applications section
    await expect(page.locator('h2:has-text("Applications")')).toBeVisible();
    
    // Should show the test app we created
    await expect(page.locator('.application-item:has-text("test-nginx")')).toBeVisible();
    
    // Click on the app to navigate to detail page
    await page.click('.application-item:has-text("test-nginx")');
    
    // Should navigate to app detail page
    await expect(page).toHaveURL('/apps/test-nginx');
  });

  test.skip('should show docker-compose.yml content', async ({ page }) => {
    const appName = 'test-nginx';
    
    // Navigate to app detail page
    await page.goto(`/apps/${appName}`);
    
    // Should show compose file content
    await expect(page.locator('pre.compose-content')).toBeVisible();
    
    // Verify content includes expected elements
    const composeContent = await page.locator('pre.compose-content').textContent();
    expect(composeContent).toContain('version:');
    expect(composeContent).toContain('services:');
    expect(composeContent).toContain(appName);
    expect(composeContent).toContain('nginx:alpine');
  });
});