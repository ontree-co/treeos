const { test, expect } = require('@playwright/test');

test.describe('Setup Page Regression Tests', () => {
  test.beforeEach(async ({ page }) => {
    // Set a longer timeout for navigation
    page.setDefaultTimeout(30000);
  });

  test('setup page should render all form fields correctly', async ({ page }) => {
    // Navigate directly to setup page
    await page.goto('/setup');
    
    // Wait for page to be fully loaded
    await page.waitForLoadState('networkidle');
    
    // Verify we're on the setup page
    await expect(page).toHaveURL(/\/setup/);
    
    // Check that the main heading is visible
    const heading = page.locator('h2');
    await expect(heading).toBeVisible();
    await expect(heading).toContainText('Welcome to OnTree');
    
    // Verify all form fields are present and visible
    const formFields = {
      'username': 'input[name="username"]',
      'password': 'input[name="password"]',
      'password2': 'input[name="password2"]',
      'node_name': 'input[name="node_name"]',
      'node_description': 'textarea[name="node_description"]'
    };
    
    for (const [fieldName, selector] of Object.entries(formFields)) {
      const field = page.locator(selector);
      await expect(field).toBeVisible({ timeout: 5000 });
      await expect(field).toBeEnabled();
      
      // Log for debugging
      console.log(`✓ Field ${fieldName} is visible and enabled`);
    }
    
    // Verify submit button is present
    const submitButton = page.locator('button[type="submit"]');
    await expect(submitButton).toBeVisible();
    await expect(submitButton).toContainText('Complete Setup');
    
    // Verify form structure
    const form = page.locator('form[action="/setup"]');
    await expect(form).toBeVisible();
    
    // Take a screenshot for visual regression
    await page.screenshot({ 
      path: 'test-results/setup-page-regression.png',
      fullPage: true 
    });
  });

  test('setup page should handle missing template data gracefully', async ({ page }) => {
    // This test verifies the page doesn't break even with missing data
    await page.goto('/setup');
    
    // Check that no error messages are shown initially
    const alerts = page.locator('.alert-danger');
    await expect(alerts).toHaveCount(0);
    
    // Verify page structure is intact
    const cardBody = page.locator('.card-body');
    await expect(cardBody).toBeVisible();
    
    // Check that form sections are present
    await expect(page.locator('text=Admin Account')).toBeVisible();
    await expect(page.locator('text=Node Configuration')).toBeVisible();
  });

  test('setup form validation should work correctly', async ({ page }) => {
    await page.goto('/setup');
    
    // Try to submit empty form
    await page.click('button[type="submit"]');
    
    // Browser validation should prevent submission
    // Check that we're still on setup page
    await expect(page).toHaveURL(/\/setup/);
    
    // Fill in partial data
    await page.fill('input[name="username"]', 'testuser');
    await page.fill('input[name="password"]', 'short'); // Too short
    
    // Password should have minlength validation
    const passwordInput = page.locator('input[name="password"]');
    const minLength = await passwordInput.getAttribute('minlength');
    expect(minLength).toBe('8');
  });

  test('setup page should have proper responsive design', async ({ page }) => {
    await page.goto('/setup');
    
    // Test mobile viewport
    await page.setViewportSize({ width: 375, height: 667 });
    await page.waitForTimeout(500); // Wait for responsive adjustments
    
    // Form should still be visible and usable
    const form = page.locator('form[action="/setup"]');
    await expect(form).toBeVisible();
    
    // All fields should still be accessible
    await expect(page.locator('input[name="username"]')).toBeVisible();
    
    // Test tablet viewport
    await page.setViewportSize({ width: 768, height: 1024 });
    await page.waitForTimeout(500);
    
    // Verify layout adjusts properly
    const container = page.locator('.container');
    await expect(container).toBeVisible();
  });
});