const { chromium } = require('playwright');

(async () => {
  // Launch browser
  const browser = await chromium.launch({
    headless: true
  });

  try {
    const context = await browser.newContext();
    const page = await context.newPage();

    // Listen for console messages
    const consoleMessages = [];
    page.on('console', msg => {
      consoleMessages.push({
        type: msg.type(),
        text: msg.text()
      });
    });

    // Listen for page errors
    const pageErrors = [];
    page.on('pageerror', error => {
      pageErrors.push(error.toString());
    });

    // Navigate to setup page - using port 3001 as that's where the server is actually running
    console.log('Navigating to http://localhost:3001/setup');
    const response = await page.goto('http://localhost:3001/setup', {
      waitUntil: 'networkidle',
      timeout: 30000
    });

    console.log('Response status:', response.status());
    console.log('Response URL:', response.url());

    // Wait a bit for any dynamic content
    await page.waitForTimeout(2000);

    // Take screenshot
    const screenshotPath = '/opt/ontree/treeos/setup-page-screenshot.png';
    await page.screenshot({ 
      path: screenshotPath,
      fullPage: true 
    });
    console.log('Screenshot saved to:', screenshotPath);

    // Get page content
    const pageContent = await page.content();
    console.log('\n--- Page Title ---');
    console.log(await page.title());

    // Check for any visible error messages
    const errorElements = await page.$$eval('[class*="error"], [class*="Error"], [id*="error"], [id*="Error"]', 
      elements => elements.map(el => el.textContent.trim()).filter(text => text.length > 0)
    );
    
    if (errorElements.length > 0) {
      console.log('\n--- Error Elements Found ---');
      errorElements.forEach(error => console.log(error));
    }

    // Log console messages
    if (consoleMessages.length > 0) {
      console.log('\n--- Console Messages ---');
      consoleMessages.forEach(msg => {
        console.log(`[${msg.type}] ${msg.text}`);
      });
    }

    // Log page errors
    if (pageErrors.length > 0) {
      console.log('\n--- Page Errors ---');
      pageErrors.forEach(error => console.log(error));
    }

    // Save page source
    const sourceFilePath = '/opt/ontree/treeos/setup-page-source.html';
    require('fs').writeFileSync(sourceFilePath, pageContent);
    console.log('\nPage source saved to:', sourceFilePath);

    // Check for common setup elements
    const hasForm = await page.$('form') !== null;
    const hasInputs = await page.$$('input').then(inputs => inputs.length);
    const hasButtons = await page.$$('button').then(buttons => buttons.length);
    
    console.log('\n--- Page Elements ---');
    console.log('Has form:', hasForm);
    console.log('Number of inputs:', hasInputs);
    console.log('Number of buttons:', hasButtons);
    
    // Look for the main content container
    const mainContent = await page.$('.container.mt-4');
    if (mainContent) {
      const innerHtml = await mainContent.innerHTML();
      console.log('\n--- Main Container Inner HTML ---');
      console.log(innerHtml.trim());
    }
    
    // Check for any text in the body
    const bodyText = await page.textContent('body');
    console.log('\n--- Body Text Content ---');
    console.log(bodyText.replace(/\s+/g, ' ').trim());

  } catch (error) {
    console.error('Error during page check:', error);
  } finally {
    await browser.close();
  }
})();