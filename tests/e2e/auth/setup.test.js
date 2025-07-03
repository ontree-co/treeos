const { test, expect } = require('@playwright/test');

test.describe('Initial Setup Flow', () => {
  test.beforeEach(async ({ page }) => {
    // Set a longer timeout for navigation
    page.setDefaultTimeout(30000);
  });

  test('should complete full setup flow', async ({ page }) => {
    // Navigate to the root URL
    await page.goto('/');
    
    // Wait for redirect and page load
    await page.waitForLoadState('networkidle');
    
    // If we're on the setup page, complete the setup
    if (page.url().includes('/setup')) {
      // Wait for page to be fully loaded
      await page.waitForSelector('h2', { timeout: 10000 });
      
      // Verify we're on setup page
      await expect(page.locator('h2')).toContainText('Welcome to OnTree');
      
      // Fill in the setup form
      await page.fill('input[name="username"]', 'admin');
      await page.fill('input[name="password"]', 'admin1234');
      await page.fill('input[name="password2"]', 'admin1234');
      await page.fill('input[name="node_name"]', 'Test OnTree Node');
      await page.fill('textarea[name="node_description"]', 'This is a test node for e2e testing');
      
      // Submit the form
      await page.click('button[type="submit"]');
      
      // Wait for redirect to dashboard
      await page.waitForURL('/', { timeout: 10000 });
    }
    
    // Verify we're on the dashboard
    await expect(page).toHaveURL('/');
    
    // Verify we're logged in
    await expect(page.locator('.user-initial')).toBeVisible();
    await expect(page.locator('.user-initial')).toContainText('A');
    
    // Verify dashboard elements
    await expect(page.locator('h1')).toContainText('Test OnTree Node');
    await expect(page.locator('.lead')).toContainText('This is a test node for e2e testing');
  });
});