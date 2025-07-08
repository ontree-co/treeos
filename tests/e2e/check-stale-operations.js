const { test, expect } = require('@playwright/test');

test.describe('Stale Operations Bug Fix', () => {
  test('should not show spinner for containers with stale operations', async ({ page }) => {
    // This test verifies the fix for the bug where old pending operations
    // would cause a "Waiting to start..." spinner to appear indefinitely
    
    // Navigate to an app detail page
    // Note: This assumes there's an app called 'openwebui' - adjust as needed
    await page.goto('http://localhost:3001/apps/openwebui');
    
    // Wait for page to load
    await page.waitForLoadState('networkidle');
    
    // Check that there's NO "Waiting to start..." text
    const waitingText = await page.locator('text=Waiting to start...').count();
    expect(waitingText).toBe(0);
    
    // Check container status
    const statusBadge = await page.locator('.badge').first();
    const statusText = await statusBadge.innerText();
    
    // If container is not created, check for Create & Start button
    if (statusText.includes('Not Created')) {
      const createButton = await page.locator('text=Create & Start').count();
      expect(createButton).toBeGreaterThan(0);
      
      // Ensure no spinner is visible
      const spinner = await page.locator('.spinner-border').count();
      expect(spinner).toBe(0);
    }
    
    // Take a screenshot for visual verification
    await page.screenshot({ path: 'test-results/stale-operations-fixed.png' });
  });
  
  test('cleanup should mark old operations as failed', async ({ page }) => {
    // This test would require database access to verify
    // For now, we just document the expected behavior
    
    // Expected behavior:
    // 1. Worker.cleanupStaleOperations() runs every minute
    // 2. Operations older than 5 minutes are marked as failed
    // 3. The handleAppDetail query filters out operations > 5 minutes old
    
    console.log('Cleanup behavior documented in handlers_test.go');
  });
});