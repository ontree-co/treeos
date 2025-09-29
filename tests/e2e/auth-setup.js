const { test as setup } = require('@playwright/test');
const path = require('path');

const authFile = path.join(__dirname, '.auth', 'user.json');

setup('authenticate', async ({ page }) => {
  // Perform authentication steps
  await page.goto('/login');

  // Check if we need setup
  if (page.url().includes('/setup')) {
    // Complete the setup
    await page.fill('input[name="username"]', 'admin');
    await page.fill('input[name="password"]', 'admin1234');
    await page.fill('input[name="password2"]', 'admin1234');
    await page.fill('input[name="node_name"]', 'Test OnTree Node');

    // Select a tree icon if present
    const treeIcon = await page.$('input[name="node_icon"]');
    if (treeIcon) {
      await page.evaluate(() => {
        document.querySelector('input[name="node_icon"]').value = 'tree1';
      });
    }

    await page.click('button:has-text("Continue to System Check")');
    await page.waitForURL('**/systemcheck', { timeout: 10000 });
    await page.waitForTimeout(2000);

    // Complete system check
    const completeButton = await page.$('button[name="action"][value="complete"]:not([disabled])');
    if (completeButton) {
      await completeButton.click();
    } else {
      await page.click('button[name="action"][value="continue"]');
    }

    await page.waitForLoadState('networkidle', { timeout: 15000 });
  }

  // Login
  await page.fill('input[name="username"]', 'admin');
  await page.fill('input[name="password"]', 'admin1234');
  await page.click('button[type="submit"]');

  // Wait for navigation
  await page.waitForLoadState('networkidle', { timeout: 15000 });

  // Save signed-in state
  await page.context().storageState({ path: authFile });
});