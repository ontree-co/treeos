const { expect } = require('@playwright/test');

/**
 * Generate a unique test identifier
 */
function generateUniqueId() {
  return `${Date.now()}-${Math.random().toString(36).substr(2, 4)}`;
}

/**
 * Make any app name unique for test isolation
 */
function makeAppNameUnique(baseName) {
  return `${baseName}-${generateUniqueId()}`;
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
 * Helper to log in as admin user
 */
async function loginAsAdmin(page, username = 'admin', password = 'admin1234') {
  await page.goto('/login');

  // Check if we were redirected to setup page
  if (page.url().includes('/setup')) {
    // Complete the setup
    await page.fill('input[name="username"]', username);
    await page.fill('input[name="password"]', password);
    await page.fill('input[name="password2"]', password);
    await page.fill('input[name="node_name"]', 'Test OnTree Node');

    // Select a tree icon if present
    const treeIcon = await page.$('input[name="node_icon"]');
    if (treeIcon) {
      await page.evaluate(() => {
        document.querySelector('input[name="node_icon"]').value = 'tree1';
      });
    }

    await page.click('button:has-text("Continue to System Check")');

    // Wait for system check page and continue
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

    // Wait for navigation after systemcheck
    await page.waitForLoadState('domcontentloaded');
    await page.waitForTimeout(1500); // Give page time to redirect

    // Check where we ended up
    if (page.url().includes('/login')) {
      // If we're at login page, perform login
      await page.fill('input[name="username"]', username);
      await page.fill('input[name="password"]', password);
      await page.click('button[type="submit"]');

      // Wait for page to start navigating after submit
      await page.waitForLoadState('domcontentloaded');
    }

    // Now wait for dashboard (either we logged in or setup redirected us there)
    // Increase timeout and make URL check more flexible
    await page.waitForURL(url => {
      const urlStr = typeof url === 'string' ? url : url.toString();
      const urlObj = typeof url === 'string' ? new URL(url) : url;
      return urlStr.includes('localhost:3002') &&
             (urlObj.pathname === '/' || urlObj.pathname === '' || urlStr.includes('/?login=success'));
    }, { timeout: 15000 });
  } else {
    // Normal login flow
    await page.fill('input[name="username"]', username);
    await page.fill('input[name="password"]', password);
    await page.click('button[type="submit"]');

    // Wait for redirect to dashboard - allow for login=success query parameter
    // Increase timeout and make URL check more flexible
    await page.waitForURL(url => {
      const urlStr = typeof url === 'string' ? url : url.toString();
      const urlObj = typeof url === 'string' ? new URL(url) : url;
      return urlStr.includes('localhost:3002') &&
             (urlObj.pathname === '/' || urlObj.pathname === '' || urlStr.includes('/?login=success'));
    }, { timeout: 15000 });
  }
}

/**
 * Helper to create a test application with automatic unique naming
 */
async function createTestApp(page, appName, dockerImage = 'nginx:latest') {
  // Ensure app name is unique by adding timestamp and random suffix
  const uniqueAppName = `${appName}-${Date.now()}-${Math.random().toString(36).substr(2, 4)}`;

  await page.goto('/apps/create');
  await page.fill('input[name="app_name"]', uniqueAppName);

  const composeContent = `version: '3'
services:
  web:
    image: ${dockerImage}
    ports:
      - "8080:80"`;

  await page.fill('textarea[name="compose_content"]', composeContent);
  await page.click('button[type="submit"]');

  // Wait for redirect to app detail page with the unique name
  await page.waitForURL(`/apps/${uniqueAppName}`);

  // Return the unique name so tests can reference it
  return uniqueAppName;
}

/**
 * Helper to wait for Docker operation to complete
 */
async function waitForOperation(page, maxWaitTime = 25000) {
  // Wait for operation status to appear
  await page.waitForSelector('#operation-status', { timeout: 5000 });

  // Poll until operation completes
  const startTime = Date.now();
  while (Date.now() - startTime < maxWaitTime) {
    const statusText = await page.locator('#operation-status').innerText();

    if (statusText.includes('completed successfully') || statusText.includes('failed')) {
      return statusText;
    }

    await page.waitForTimeout(1000);
  }
  
  throw new Error('Operation timed out');
}

/**
 * Helper to check if a container is running
 */
async function isContainerRunning(page, appName) {
  await page.goto(`/apps/${appName}`);
  const statusBadge = await page.locator('.badge').first();
  const statusText = await statusBadge.innerText();
  return statusText.toLowerCase() === 'running';
}


/**
 * Helper to stop a container for testing
 */
async function stopContainer(page, appName, containerName) {
  const response = await page.request.post(`/api/apps/${appName}/stop`, {
    headers: {
      'Content-Type': 'application/json',
    },
  });
  
  if (!response.ok()) {
    throw new Error(`Failed to stop container: ${response.status()}`);
  }
  
  // Wait for container to stop
  await page.waitForTimeout(2000);
}

/**
 * Helper to start a container for testing
 */
async function startContainer(page, appName) {
  const response = await page.request.post(`/api/apps/${appName}/start`, {
    headers: {
      'Content-Type': 'application/json',
    },
  });
  
  if (!response.ok()) {
    throw new Error(`Failed to start container: ${response.status()}`);
  }
  
  // Wait for container to start
  await page.waitForTimeout(3000);
}

module.exports = {
  generateUniqueId,
  makeAppNameUnique,
  expectFlashMessage,
  loginAsAdmin,
  createTestApp,
  waitForOperation,
  isContainerRunning,
  stopContainer,
  startContainer,
};