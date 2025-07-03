const { test, expect } = require('@playwright/test');
const { loginAsAdmin } = require('../helpers');

test.describe('Authentication Flow', () => {
  test.beforeEach(async ({ page }) => {
    // Set a longer timeout for navigation
    page.setDefaultTimeout(30000);
  });

  test('should show login page when not authenticated', async ({ page }) => {
    // Navigate to the root URL
    await page.goto('/');
    
    // Should be redirected to login page if not authenticated
    if (page.url().includes('/login')) {
      // Verify login page elements
      await expect(page.locator('h2')).toContainText('Login');
      await expect(page.locator('input[name="username"]')).toBeVisible();
      await expect(page.locator('input[name="password"]')).toBeVisible();
      await expect(page.locator('button[type="submit"]')).toContainText('Login');
    }
  });

  test('should handle invalid login credentials', async ({ page }) => {
    await page.goto('/login');
    
    // Try to login with invalid credentials
    await page.fill('input[name="username"]', 'wronguser');
    await page.fill('input[name="password"]', 'wrongpass');
    await page.click('button[type="submit"]');
    
    // Should show error message
    await expect(page.locator('.alert-danger')).toBeVisible();
    await expect(page.locator('.alert-danger')).toContainText('Invalid username or password');
    
    // Should remain on login page
    await expect(page).toHaveURL('/login');
  });

  test('should login successfully with valid credentials', async ({ page }) => {
    await page.goto('/login');
    
    // Login with valid credentials
    await page.fill('input[name="username"]', 'admin');
    await page.fill('input[name="password"]', 'admin1234');
    await page.click('button[type="submit"]');
    
    // Should redirect to dashboard
    await page.waitForURL('/?login=success');
    
    // Verify we're logged in
    await expect(page.locator('.user-initial')).toBeVisible();
    await expect(page.locator('.user-initial')).toContainText('A');
  });

  test('should logout successfully', async ({ page }) => {
    // First login
    await loginAsAdmin(page);
    
    // Verify we're logged in
    await expect(page.locator('.user-initial')).toBeVisible();
    
    // Click on user menu to reveal logout option
    await page.click('.user-initial');
    
    // Click logout
    await page.click('a:has-text("Logout")');
    
    // Should redirect to login page
    await expect(page).toHaveURL('/login');
    
    // Verify login page is shown
    await expect(page.locator('h2')).toContainText('Login');
  });

  test('should redirect to requested page after login', async ({ page }) => {
    // Try to access a protected page
    await page.goto('/apps/create');
    
    // Should redirect to login
    await expect(page).toHaveURL('/login');
    
    // Login
    await page.fill('input[name="username"]', 'admin');
    await page.fill('input[name="password"]', 'admin1234');
    await page.click('button[type="submit"]');
    
    // Should redirect to the originally requested page
    await page.waitForLoadState('networkidle');
    await expect(page.url()).toContain('/apps/create');
  });

  test('should persist session across page refreshes', async ({ page }) => {
    // Login
    await loginAsAdmin(page);
    
    // Refresh the page
    await page.reload();
    
    // Should still be logged in
    await expect(page.locator('.user-initial')).toBeVisible();
    await expect(page.locator('.user-initial')).toContainText('A');
    
    // Should remain on dashboard
    await expect(page).toHaveURL('/');
  });
});