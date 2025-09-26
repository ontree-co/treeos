const { test, expect } = require('@playwright/test');
const { loginAsAdmin } = require('../helpers');

test.describe('Authentication Flow', () => {
  test.beforeEach(async ({ page }) => {
    page.setDefaultTimeout(30000);
  });

  test('should redirect to login page when accessing protected routes without authentication', async ({ page }) => {
    await page.goto('/');
    
    if (page.url().includes('/login')) {
      await expect(page.locator('h2')).toContainText('Welcome Back');
      await expect(page.locator('input[name="username"]')).toBeVisible();
      await expect(page.locator('input[name="password"]')).toBeVisible();
      await expect(page.locator('button[type="submit"]')).toContainText('Login');
    }
    
    await page.goto('/apps/create');
    await expect(page).toHaveURL('/login');
  });

  test('should fail login with invalid credentials', async ({ page }) => {
    await page.goto('/login');
    
    await page.fill('input[name="username"]', 'wronguser');
    await page.fill('input[name="password"]', 'wrongpass');
    await page.click('button[type="submit"]');
    
    await expect(page.locator('.alert-danger')).toBeVisible();
    await expect(page.locator('.alert-danger')).toContainText('Invalid username or password');
    
    await expect(page).toHaveURL('/login');
  });

  test('should successfully login with valid credentials', async ({ page }) => {
    await page.goto('/login');

    // Check if we were redirected to setup page
    if (page.url().includes('/setup')) {
      // Complete the setup first
      await page.fill('input[name="username"]', 'admin');
      await page.fill('input[name="password"]', 'admin1234');
      await page.fill('input[name="password2"]', 'admin1234');
      await page.fill('input[name="node_name"]', 'Test OnTree Node');

      // Set tree icon
      await page.evaluate(() => {
        const iconInput = document.querySelector('input[name="node_icon"]');
        if (iconInput) iconInput.value = 'tree1';
      });

      await page.click('button:has-text("Continue to System Check")');
      await page.waitForURL('**/systemcheck', { timeout: 10000 });

      // Wait for system check to complete and buttons to be available
      await page.waitForTimeout(2000);

      // Submit the form with the appropriate action
      // Try to click Complete Setup if available and enabled
      const completeButton = await page.$('button[name="action"][value="complete"]:not([disabled])');
      if (completeButton) {
        await completeButton.click();
      } else {
        // Otherwise click Continue Without Fixing Everything
        await page.click('button[name="action"][value="continue"]');
      }
    } else {
      // Normal login flow
      await page.fill('input[name="username"]', 'admin');
      await page.fill('input[name="password"]', 'admin1234');
      await page.click('button[type="submit"]');
    }

    await page.waitForURL(url => {
      return url.pathname === '/' || url.toString().includes('/?login=success');
    }, { timeout: 10000 });

    await expect(page.locator('.settings-icon')).toBeVisible();
  });

  test('should persist session across page reloads', async ({ page }) => {
    await loginAsAdmin(page);
    
    await page.reload();
    
    await expect(page.locator('.settings-icon')).toBeVisible();
    
    await expect(page).toHaveURL(url => {
      return url.pathname === '/' || url.includes('/?login=success');
    });
  });

  test('should successfully logout', async ({ page }) => {
    await loginAsAdmin(page);
    
    await expect(page.locator('.settings-icon')).toBeVisible();
    
    await page.click('.settings-icon');
    
    await page.click('a:has-text("Logout")');
    
    await expect(page).toHaveURL('/login');
    
    await expect(page.locator('h2')).toContainText('Welcome Back');
  });

  test('should redirect to originally requested page after login', async ({ page }) => {
    await page.goto('/apps/create');
    
    await expect(page).toHaveURL('/login');
    
    await page.fill('input[name="username"]', 'admin');
    await page.fill('input[name="password"]', 'admin1234');
    await page.click('button[type="submit"]');
    
    await page.waitForLoadState('networkidle');
    await expect(page.url()).toContain('/apps/create');
  });
});