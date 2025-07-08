const { expect } = require('@playwright/test');

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
  await page.fill('input[name="username"]', username);
  await page.fill('input[name="password"]', password);
  await page.click('button[type="submit"]');
  
  // Wait for redirect to dashboard - allow for login=success query parameter
  await page.waitForURL(url => {
    return url.pathname === '/' || url.includes('/?login=success');
  }, { timeout: 10000 });
}

/**
 * Helper to create a test application
 */
async function createTestApp(page, appName, dockerImage = 'nginx:latest') {
  await page.goto('/apps/create');
  await page.fill('input[name="app_name"]', appName);
  
  const composeContent = `version: '3'
services:
  ${appName}:
    image: ${dockerImage}
    container_name: ontree-${appName}
    ports:
      - "8080:80"`;
  
  await page.fill('textarea[name="compose_content"]', composeContent);
  await page.click('button[type="submit"]');
  
  // Wait for redirect to app detail page
  await page.waitForURL(`/apps/${appName}`);
}

/**
 * Helper to wait for Docker operation to complete
 */
async function waitForOperation(page, maxWaitTime = 30000) {
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

module.exports = {
  expectFlashMessage,
  loginAsAdmin,
  createTestApp,
  waitForOperation,
  isContainerRunning,
};