const { test, expect } = require('@playwright/test');
const { loginAsAdmin, createTestApp } = require('../helpers');

test.describe('Application Creation', () => {
  test.beforeEach(async ({ page }) => {
    // Set a longer timeout for navigation
    page.setDefaultTimeout(30000);
    
    // Login before each test
    await loginAsAdmin(page);
  });

  test('should show create app form with correct fields', async ({ page }) => {
    await page.goto('/apps/create');
    
    // Verify page title
    await expect(page.locator('h1')).toContainText('Create New Application');
    
    // Verify form fields
    await expect(page.locator('label:has-text("App Name")')).toBeVisible();
    await expect(page.locator('input[name="app_name"]')).toBeVisible();
    await expect(page.locator('label:has-text("docker-compose.yml Content")')).toBeVisible();
    await expect(page.locator('textarea[name="compose_content"]')).toBeVisible();
    
    // Verify submit button
    await expect(page.locator('button[type="submit"]')).toContainText('Create Application');
  });

  test('should validate service name matches app name', async ({ page }) => {
    await page.goto('/apps/create');
    
    // Fill form with mismatched service name
    await page.fill('input[name="app_name"]', 'my-app');
    await page.fill('textarea[name="compose_content"]', `version: '3'
services:
  different-name:
    image: nginx:latest`);
    
    await page.click('button[type="submit"]');
    
    // Should show error
    await expect(page.locator('.alert-danger')).toBeVisible();
    await expect(page.locator('.alert-danger')).toContainText('The service name (\'different-name\') must match the App Name (\'my-app\')');
  });

  test('should validate compose yaml format', async ({ page }) => {
    await page.goto('/apps/create');
    
    // Fill form with invalid YAML
    await page.fill('input[name="app_name"]', 'my-app');
    await page.fill('textarea[name="compose_content"]', 'invalid yaml content');
    
    await page.click('button[type="submit"]');
    
    // Should show error
    await expect(page.locator('.alert-danger')).toBeVisible();
    await expect(page.locator('.alert-danger')).toContainText('Invalid YAML format');
  });

  test('should require services section', async ({ page }) => {
    await page.goto('/apps/create');
    
    // Fill form without services section
    await page.fill('input[name="app_name"]', 'my-app');
    await page.fill('textarea[name="compose_content"]', `version: '3'
networks:
  default:`);
    
    await page.click('button[type="submit"]');
    
    // Should show error
    await expect(page.locator('.alert-danger')).toBeVisible();
    await expect(page.locator('.alert-danger')).toContainText('The YAML must contain a \'services\' section');
  });

  test('should require exactly one service', async ({ page }) => {
    await page.goto('/apps/create');
    
    // Fill form with multiple services
    await page.fill('input[name="app_name"]', 'my-app');
    await page.fill('textarea[name="compose_content"]', `version: '3'
services:
  service1:
    image: nginx
  service2:
    image: redis`);
    
    await page.click('button[type="submit"]');
    
    // Should show error
    await expect(page.locator('.alert-danger')).toBeVisible();
    await expect(page.locator('.alert-danger')).toContainText('Docker compose file must contain exactly one service');
  });

  test('should require image in service definition', async ({ page }) => {
    await page.goto('/apps/create');
    
    // Fill form without image
    await page.fill('input[name="app_name"]', 'my-app');
    await page.fill('textarea[name="compose_content"]', `version: '3'
services:
  my-app:
    ports:
      - "8080:80"`);
    
    await page.click('button[type="submit"]');
    
    // Should show error
    await expect(page.locator('.alert-danger')).toBeVisible();
    await expect(page.locator('.alert-danger')).toContainText('The service \'my-app\' must contain an \'image\' key');
  });

  test('should prevent duplicate app names', async ({ page }) => {
    // Use a unique app name for this test to avoid conflicts
    const uniqueAppName = `test-duplicate-${Date.now()}`;
    
    // First create an app
    await createTestApp(page, uniqueAppName, 'nginx:latest');
    
    // Now try to create another app with the same name
    await page.goto('/apps/create');
    
    // Try to create app with existing name
    await page.fill('input[name="app_name"]', uniqueAppName);
    await page.fill('textarea[name="compose_content"]', `version: '3'
services:
  ${uniqueAppName}:
    image: nginx:latest`);
    
    await page.click('button[type="submit"]');
    
    // Should show error
    await expect(page.locator('.alert-danger')).toBeVisible();
    await expect(page.locator('.alert-danger')).toContainText(`An application named '${uniqueAppName}' already exists`);
  });

  test.skip('should create app with complex configuration', async ({ page }) => {
    await page.goto('/apps/create');
    
    const appName = 'test-postgres';
    
    // Fill form with PostgreSQL configuration
    await page.fill('input[name="app_name"]', appName);
    await page.fill('textarea[name="compose_content"]', `version: '3'
services:
  ${appName}:
    image: postgres:15-alpine
    container_name: ontree-${appName}
    environment:
      POSTGRES_USER: testuser
      POSTGRES_PASSWORD: testpass
      POSTGRES_DB: testdb
    ports:
      - "5433:5432"
    volumes:
      - ./mnt/data:/var/lib/postgresql/data`);
    
    await page.click('button[type="submit"]');
    
    // Should redirect to app detail page
    await page.waitForURL(`/apps/${appName}`);
    
    // Verify success
    await expect(page.locator('.alert-success')).toBeVisible();
    await expect(page.locator('h2')).toContainText(appName);
    
    // Verify compose content is displayed correctly
    const composeContent = await page.locator('pre.compose-content').textContent();
    expect(composeContent).toContain('postgres:15-alpine');
    expect(composeContent).toContain('POSTGRES_USER: testuser');
  });
});