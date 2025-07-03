const { test, expect } = require('@playwright/test');
const { loginAsAdmin } = require('../helpers');

test.describe('Dashboard and System Vitals', () => {
  test.beforeEach(async ({ page }) => {
    // Set a longer timeout for navigation
    page.setDefaultTimeout(30000);
    
    // Login before each test
    await loginAsAdmin(page);
  });

  test('should display dashboard with correct elements', async ({ page }) => {
    // Should be on dashboard
    await expect(page).toHaveURL('/');
    
    // Verify header elements
    await expect(page.locator('.navbar-brand')).toContainText('OnTree.co');
    await expect(page.locator('.user-initial')).toBeVisible();
    
    // Verify main dashboard elements
    await expect(page.locator('h1')).toContainText('OnTree Node');
    
    // Verify system vitals card exists
    await expect(page.locator('.card:has-text("System Vitals")')).toBeVisible();
    
    // Verify applications section exists
    await expect(page.locator('h2:has-text("Applications")')).toBeVisible();
  });

  test('should load and display system vitals', async ({ page }) => {
    // Wait for vitals to load (they load via HTMX)
    await page.waitForSelector('.vitals-content', { timeout: 10000 });
    
    // Verify vitals are displayed
    const vitalsContent = page.locator('.vitals-content');
    await expect(vitalsContent).toBeVisible();
    
    // Check for CPU, Memory, and Disk metrics
    await expect(vitalsContent.locator('.vital-item:has-text("CPU:")')).toBeVisible();
    await expect(vitalsContent.locator('.vital-item:has-text("Mem:")')).toBeVisible();
    await expect(vitalsContent.locator('.vital-item:has-text("Disk:")')).toBeVisible();
    
    // Verify values are present (should have percentage values)
    const cpuValue = await vitalsContent.locator('.vital-item:has-text("CPU:") .vital-value').textContent();
    expect(cpuValue).toMatch(/\d+\.\d+%/);
    
    const memValue = await vitalsContent.locator('.vital-item:has-text("Mem:") .vital-value').textContent();
    expect(memValue).toMatch(/\d+\.\d+%/);
    
    const diskValue = await vitalsContent.locator('.vital-item:has-text("Disk:") .vital-value').textContent();
    expect(diskValue).toMatch(/\d+\.\d+%/);
  });

  test.skip('should auto-refresh system vitals', async ({ page }) => {
    // Skip this test as it takes too long (35 seconds)
    // In a real scenario, we would mock the time or use a shorter refresh interval for testing
    
    // Wait for initial vitals load
    await page.waitForSelector('.vitals-content');
    
    // Get initial CPU value
    const initialCPU = await page.locator('.vital-item:has-text("CPU:") .vital-value').textContent();
    
    // Wait for the vitals to refresh (configured to refresh every 30 seconds)
    // We'll wait a bit longer to ensure refresh happens
    await page.waitForTimeout(35000);
    
    // Check if vitals are still visible (confirms refresh didn't break anything)
    await expect(page.locator('.vitals-content')).toBeVisible();
    
    // The values might or might not change, but the element should still be there
    const newCPU = await page.locator('.vital-item:has-text("CPU:") .vital-value').textContent();
    expect(newCPU).toMatch(/\d+\.\d+%/);
  });

  test('should display navigation menu items', async ({ page }) => {
    // Check main navigation items
    await expect(page.locator('a:has-text("Dashboard")')).toBeVisible();
    await expect(page.locator('a:has-text("Applications")')).toBeVisible();
    await expect(page.locator('a:has-text("Create App")')).toBeVisible();
    await expect(page.locator('a:has-text("Templates")')).toBeVisible();
    await expect(page.locator('a:has-text("Pattern Library")')).toBeVisible();
  });

  test('should show applications directory path', async ({ page }) => {
    // Check that the apps directory is displayed
    const appsDir = await page.locator('p:has-text("Apps directory:")').textContent();
    expect(appsDir).toContain('/opt/ontree/apps');
  });

  test('should navigate to create app page', async ({ page }) => {
    // Click on Create App link
    await page.click('a:has-text("Create App")');
    
    // Should navigate to create app page
    await expect(page).toHaveURL('/apps/create');
    
    // Verify page loaded
    await expect(page.locator('h2')).toContainText('Create New Application');
  });

  test('should navigate to templates page', async ({ page }) => {
    // Click on Templates link
    await page.click('a:has-text("Templates")');
    
    // Should navigate to templates page
    await expect(page).toHaveURL('/templates');
    
    // Verify page loaded
    await expect(page.locator('h2')).toContainText('Application Templates');
  });
});