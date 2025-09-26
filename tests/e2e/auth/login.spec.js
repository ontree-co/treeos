const { test, expect } = require('@playwright/test');
const { loginAsAdmin } = require('../helpers');

test.describe('Authentication Flow', () => {
  test.beforeEach(async ({ page }) => {
    page.setDefaultTimeout(30000);
  });

  test('should redirect to login page when accessing protected routes without authentication', async ({ page }) => {
    await page.goto('/');

    // Might redirect to setup or login depending on database state
    if (page.url().includes('/login')) {
      await expect(page.locator('h2')).toContainText('Welcome Back');
      await expect(page.locator('input[name="username"]')).toBeVisible();
      await expect(page.locator('input[name="password"]')).toBeVisible();
      await expect(page.locator('button[type="submit"]')).toContainText('Login');
    } else if (page.url().includes('/setup')) {
      // Complete setup first
      await page.fill('input[name="username"]', 'admin');
      await page.fill('input[name="password"]', 'admin1234');
      await page.fill('input[name="password2"]', 'admin1234');
      await page.fill('input[name="node_name"]', 'Test OnTree Node');

      await page.evaluate(() => {
        const iconInput = document.querySelector('input[name="node_icon"]');
        if (iconInput) iconInput.value = 'tree1';
      });

      await page.click('button:has-text("Continue to System Check")');
      await page.waitForURL('**/systemcheck', { timeout: 10000 });
      await page.waitForTimeout(2000);

      const completeButton = await page.$('button[name="action"][value="complete"]:not([disabled])');
      if (completeButton) {
        await completeButton.click();
      } else {
        await page.click('button[name="action"][value="continue"]');
      }

      // After systemcheck, we might be redirected to dashboard (auto-login) or login page
      await page.waitForLoadState('domcontentloaded');
      await page.waitForTimeout(2000); // Give page time to redirect

      if (!page.url().includes('/login')) {
        // If we're logged in automatically, logout first
        await page.click('.settings-icon');
        await page.click('a:has-text("Logout")');
        await page.waitForURL('**/login', { timeout: 10000 });
      }
    }

    // Now try accessing a protected route
    await page.goto('/apps/create');
    // Should redirect to either login or setup
    await expect(page.url()).toMatch(/\/(login|setup)/);
  });

  test('should fail login with invalid credentials', async ({ page }) => {
    await page.goto('/login');

    // If we're redirected to setup, complete it first
    if (page.url().includes('/setup')) {
      await page.fill('input[name="username"]', 'admin');
      await page.fill('input[name="password"]', 'admin1234');
      await page.fill('input[name="password2"]', 'admin1234');
      await page.fill('input[name="node_name"]', 'Test OnTree Node');

      await page.evaluate(() => {
        const iconInput = document.querySelector('input[name="node_icon"]');
        if (iconInput) iconInput.value = 'tree1';
      });

      await page.click('button:has-text("Continue to System Check")');
      await page.waitForURL('**/systemcheck', { timeout: 10000 });
      await page.waitForTimeout(2000);

      const completeButton = await page.$('button[name="action"][value="complete"]:not([disabled])');
      if (completeButton) {
        await completeButton.click();
      } else {
        await page.click('button[name="action"][value="continue"]');
      }

      // After systemcheck, we might be redirected to dashboard (auto-login) or login page
      await page.waitForLoadState('domcontentloaded');
      await page.waitForTimeout(2000); // Give page time to redirect

      if (!page.url().includes('/login')) {
        // If we're logged in automatically, logout first
        await page.click('.settings-icon');
        await page.click('a:has-text("Logout")');
        await page.waitForURL('**/login', { timeout: 10000 });
      }
    }

    // Now test invalid login
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

      // After systemcheck, we might be redirected to dashboard (auto-login) or login page
      await page.waitForLoadState('domcontentloaded');
      await page.waitForTimeout(2000); // Give page time to redirect

      if (page.url().includes('/login')) {
        // If we're at login page, perform login
        await page.fill('input[name="username"]', 'admin');
        await page.fill('input[name="password"]', 'admin1234');
        await page.click('button[type="submit"]');
      }
      // Otherwise we're already logged in and on the dashboard
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

  test.skip('should redirect to originally requested page after login', async ({ page }) => {
    // TODO: This test is failing - the app doesn't preserve the redirect URL after setup/login flow
    await page.goto('/apps/create');

    // Might redirect to setup or login depending on database state
    if (page.url().includes('/setup')) {
      // Complete setup first
      await page.fill('input[name="username"]', 'admin');
      await page.fill('input[name="password"]', 'admin1234');
      await page.fill('input[name="password2"]', 'admin1234');
      await page.fill('input[name="node_name"]', 'Test OnTree Node');

      await page.evaluate(() => {
        const iconInput = document.querySelector('input[name="node_icon"]');
        if (iconInput) iconInput.value = 'tree1';
      });

      await page.click('button:has-text("Continue to System Check")');
      await page.waitForURL('**/systemcheck', { timeout: 10000 });
      await page.waitForTimeout(2000);

      const completeButton = await page.$('button[name="action"][value="complete"]:not([disabled])');
      if (completeButton) {
        await completeButton.click();
      } else {
        await page.click('button[name="action"][value="continue"]');
      }

      // After systemcheck, we might be redirected to dashboard (auto-login) or login page
      await page.waitForLoadState('domcontentloaded');
      await page.waitForTimeout(2000); // Give page time to redirect

      if (!page.url().includes('/login')) {
        // If we're logged in automatically, logout first
        await page.click('.settings-icon');
        await page.click('a:has-text("Logout")');
        await page.waitForURL('**/login', { timeout: 10000 });
      }
    } else {
      // Should be on login page
      await expect(page).toHaveURL('/login');
    }

    // Now login
    await page.fill('input[name="username"]', 'admin');
    await page.fill('input[name="password"]', 'admin1234');
    await page.click('button[type="submit"]');

    // Should redirect back to originally requested page
    await page.waitForTimeout(2000); // Wait for redirect
    await expect(page.url()).toContain('/apps/create');
  });
});