const { expect } = require('@playwright/test');
const crypto = require('crypto');

/**
 * Generate a unique test ID for isolation
 */
function generateTestId() {
  return crypto.randomBytes(4).toString('hex');
}

/**
 * Helper to wait for and verify a flash message
 */
async function expectFlashMessage(page, messageType, messageText) {
  const flashSelector = `.alert.alert-${messageType}`;
  await page.waitForSelector(flashSelector, { timeout: 5000 });

  if (messageText) {
    const flashMessage = await page.locator(flashSelector).innerText();
    expect(flashMessage).toContain(messageText);
  }
}

/**
 * Helper to log in with session isolation
 * Each test gets its own unique session to avoid conflicts
 */
async function loginAsTestUser(page, testId = null) {
  // Generate a unique test ID if not provided
  const uniqueId = testId || generateTestId();
  const username = `test_${uniqueId}`;
  const password = 'test1234';

  // Try to login first
  await page.goto('/login');

  // Check if we need to complete setup (first test to run)
  if (page.url().includes('/setup')) {
    // Complete setup with a shared admin account
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

    // Wait for redirect to login
    await page.waitForLoadState('networkidle', { timeout: 15000 });
  }

  // For now, use the admin account but with isolated browser contexts
  // In a real scenario, you'd create separate user accounts
  await page.fill('input[name="username"]', 'admin');
  await page.fill('input[name="password"]', 'admin1234');
  await page.click('button[type="submit"]');

  // Wait for dashboard redirect
  await page.waitForLoadState('networkidle', { timeout: 15000 });
  await page.waitForURL(url => {
    const urlStr = typeof url === 'string' ? url : url.toString();
    const urlObj = typeof url === 'string' ? new URL(url) : url;
    return urlStr.includes('localhost:3002') &&
           (urlObj.pathname === '/' || urlObj.pathname === '' || urlStr.includes('/?login=success'));
  }, { timeout: 20000 });

  return { username, password, testId: uniqueId };
}

/**
 * Helper to create a test application with unique naming
 */
async function createIsolatedTestApp(page, baseName, dockerImage = 'nginx:latest') {
  const testId = generateTestId();
  const appName = `${baseName}-${testId}`;

  await page.goto('/apps/create');
  await page.fill('input[name="app_name"]', appName);

  const composeContent = `version: '3'
services:
  web:
    image: ${dockerImage}
    ports:
      - "8080"`;

  await page.fill('textarea[name="compose"]', composeContent);
  await page.click('button[type="submit"]');

  // Wait for success message
  await page.waitForSelector('.alert-success', { timeout: 10000 });

  return { appName, testId };
}

/**
 * Helper to delete a test application
 */
async function deleteTestApp(page, appName) {
  await page.goto('/');
  const appCard = page.locator(`.app-card:has-text("${appName}")`);

  if (await appCard.count() > 0) {
    await appCard.locator('button:has-text("Delete")').click();
    await page.locator('button:has-text("Confirm")').click();
    await page.waitForSelector('.alert-success', { timeout: 10000 });
  }
}

/**
 * Helper to clean up all test apps created by a specific test
 */
async function cleanupTestApps(page, testId) {
  await page.goto('/');

  // Find all apps with this test ID in their name
  const appCards = page.locator(`.app-card`);
  const count = await appCards.count();

  for (let i = 0; i < count; i++) {
    const card = appCards.nth(i);
    const appName = await card.locator('h3').innerText();

    if (appName.includes(testId)) {
      await deleteTestApp(page, appName);
    }
  }
}

module.exports = {
  generateTestId,
  expectFlashMessage,
  loginAsTestUser,
  createIsolatedTestApp,
  deleteTestApp,
  cleanupTestApps,
  // Keep backward compatibility
  loginAsAdmin: loginAsTestUser
};