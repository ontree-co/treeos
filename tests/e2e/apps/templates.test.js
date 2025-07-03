const { test, expect } = require('@playwright/test');
const { loginAsAdmin, waitForOperation } = require('../helpers');

test.describe('Template System', () => {
  test.beforeEach(async ({ page }) => {
    // Set a longer timeout for navigation
    page.setDefaultTimeout(30000);
    
    // Login before each test
    await loginAsAdmin(page);
  });

  test('should display templates page', async ({ page }) => {
    // Navigate to templates page
    await page.goto('/templates');
    
    // Verify page title
    await expect(page.locator('h2')).toContainText('Application Templates');
    
    // Should show available templates
    await expect(page.locator('.template-card')).toHaveCount(1); // At least one template
    
    // Check for Open WebUI template (from templates.json)
    await expect(page.locator('.template-card:has-text("Open WebUI")')).toBeVisible();
  });

  test('should show template details', async ({ page }) => {
    await page.goto('/templates');
    
    // Find Open WebUI template
    const template = page.locator('.template-card:has-text("Open WebUI")');
    
    // Verify template information
    await expect(template.locator('.card-title')).toContainText('Open WebUI');
    await expect(template.locator('.card-text')).toContainText('User-friendly WebUI for ChatGPT');
    
    // Should have Create button
    await expect(template.locator('a:has-text("Create")')).toBeVisible();
  });

  test('should navigate to create from template page', async ({ page }) => {
    await page.goto('/templates');
    
    // Click create button for Open WebUI template
    await page.click('.template-card:has-text("Open WebUI") a:has-text("Create")');
    
    // Should navigate to create from template page
    await expect(page).toHaveURL('/templates/openwebui/create');
    
    // Verify page elements
    await expect(page.locator('h2')).toContainText('Create App from Template');
    await expect(page.locator('h3')).toContainText('Open WebUI');
    await expect(page.locator('p.lead')).toContainText('User-friendly WebUI for ChatGPT');
  });

  test('should show create from template form', async ({ page }) => {
    await page.goto('/templates/openwebui/create');
    
    // Verify form fields
    await expect(page.locator('label:has-text("Application Name")')).toBeVisible();
    await expect(page.locator('input[name="name"]')).toBeVisible();
    
    // Should show docker compose preview
    await expect(page.locator('h4:has-text("Docker Compose Configuration")')).toBeVisible();
    await expect(page.locator('pre')).toBeVisible();
    
    // Verify compose content includes template specifics
    const composeContent = await page.locator('pre').textContent();
    expect(composeContent).toContain('ghcr.io/open-webui/open-webui:main');
    expect(composeContent).toContain('3000:8080');
    
    // Should have auto-start checkbox
    await expect(page.locator('input[name="auto_start"]')).toBeVisible();
    await expect(page.locator('label:has-text("Start application after creation")')).toBeVisible();
    
    // Should have create button
    await expect(page.locator('button[type="submit"]')).toContainText('Create Application');
  });

  test('should validate template app name', async ({ page }) => {
    await page.goto('/templates/openwebui/create');
    
    // Try to submit without name
    await page.click('button[type="submit"]');
    
    // Browser validation should prevent submission
    const nameInput = page.locator('input[name="name"]');
    const validationMessage = await nameInput.evaluate(el => el.validationMessage);
    expect(validationMessage).toBeTruthy();
  });

  test('should create app from template', async ({ page }) => {
    await page.goto('/templates/openwebui/create');
    
    const appName = 'template-test';
    
    // Fill in the form
    await page.fill('input[name="name"]', appName);
    
    // Don't auto-start for this test
    await page.uncheck('input[name="auto_start"]');
    
    // Submit the form
    await page.click('button[type="submit"]');
    
    // Should redirect to app detail page
    await page.waitForURL(`/apps/${appName}`);
    
    // Verify app was created
    await expect(page.locator('h2')).toContainText(appName);
    await expect(page.locator('.badge')).toContainText('not_created');
    
    // Verify compose content was properly templated
    const composeContent = await page.locator('pre.compose-content').textContent();
    expect(composeContent).toContain(`services:`);
    expect(composeContent).toContain(`${appName}:`);
    expect(composeContent).toContain('ghcr.io/open-webui/open-webui:main');
  });

  test('should create and auto-start app from template', async ({ page }) => {
    await page.goto('/templates/openwebui/create');
    
    const appName = 'template-autostart';
    
    // Fill in the form
    await page.fill('input[name="name"]', appName);
    
    // Check auto-start
    await page.check('input[name="auto_start"]');
    
    // Submit the form
    await page.click('button[type="submit"]');
    
    // Should redirect to app detail page
    await page.waitForURL(`/apps/${appName}`);
    
    // Should show operation in progress (due to auto-start)
    await expect(page.locator('#operation-status')).toBeVisible();
    
    // Wait for operation to complete
    await waitForOperation(page, 60000);
    
    // Reload page to see updated status
    await page.reload();
    
    // Should be running
    await expect(page.locator('.badge')).toContainText('running');
    
    // Clean up - stop the container
    await page.click('button:has-text("Stop")');
    await page.waitForTimeout(2000);
  });

  test('should handle invalid template ID', async ({ page }) => {
    // Try to access non-existent template
    const response = await page.goto('/templates/nonexistent/create');
    
    // Should return 404
    expect(response?.status()).toBe(404);
  });

  test('should return to templates list', async ({ page }) => {
    await page.goto('/templates/openwebui/create');
    
    // Click back to templates button/link
    await page.click('a:has-text("Back to Templates")');
    
    // Should return to templates list
    await expect(page).toHaveURL('/templates');
  });
});