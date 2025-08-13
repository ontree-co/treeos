const { test, expect } = require('@playwright/test');
const { 
  loginAsAdmin, 
  createTestApp, 
  triggerAgentRun,
  getChatMessages,
  waitForNewChatMessage,
  stopContainer,
  startContainer,
  isContainerRunning
} = require('../helpers');

test.describe('AI Agent Chat UI', () => {
  // Test app name with timestamp to avoid conflicts
  const testAppName = `test-agent-${Date.now()}`;
  
  test.beforeEach(async ({ page }) => {
    // Login as admin before each test
    await loginAsAdmin(page);
  });

  test.describe('Scenario 1: All OK', () => {
    test('should post green status message when all services are healthy', async ({ page }) => {
      // Create a test application with a simple nginx container
      await createTestApp(page, testAppName, 'nginx:alpine');
      
      // Wait for container to be fully running
      await page.waitForTimeout(5000);
      
      // Verify the container is running
      const isRunning = await isContainerRunning(page, testAppName);
      expect(isRunning).toBe(true);
      
      // Get initial message count
      const initialMessages = await getChatMessages(page, testAppName);
      const initialCount = initialMessages.length;
      
      // Trigger agent run
      await triggerAgentRun(page);
      
      // Wait for new message to appear
      const newMessage = await waitForNewChatMessage(page, testAppName, initialCount);
      
      // Verify the message indicates OK status
      expect(newMessage.status_level).toBe('OK');
      expect(newMessage.message_summary.toLowerCase()).toContain('nominal');
      
      // Navigate to the app detail page to verify UI
      await page.goto(`/apps/${testAppName}`);
      
      // Check that the chat message is visible in the UI with green styling
      const chatMessage = await page.locator('.chat-message.status-ok').first();
      await expect(chatMessage).toBeVisible();
      
      // Verify the message content
      const messageText = await chatMessage.locator('.chat-message-summary').innerText();
      expect(messageText.toLowerCase()).toContain('nominal');
    });
  });

  test.describe('Scenario 2: Container Down', () => {
    test('should post critical red message when container is stopped', async ({ page }) => {
      // Use existing app or create new one
      const appName = `test-down-${Date.now()}`;
      await createTestApp(page, appName, 'nginx:alpine');
      
      // Wait for container to be running
      await page.waitForTimeout(5000);
      
      // Get initial message count
      const initialMessages = await getChatMessages(page, appName);
      const initialCount = initialMessages.length;
      
      // Stop the container to simulate failure
      await stopContainer(page, appName);
      
      // Trigger agent run
      await triggerAgentRun(page);
      
      // Wait for new message
      const newMessage = await waitForNewChatMessage(page, appName, initialCount, 20000);
      
      // Verify critical status
      expect(newMessage.status_level).toBe('CRITICAL');
      expect(newMessage.message_summary.toLowerCase()).toMatch(/down|critical|offline|stopped/);
      
      // Navigate to app detail page
      await page.goto(`/apps/${appName}`);
      
      // Check for critical message in UI
      const criticalMessage = await page.locator('.chat-message.status-critical').first();
      await expect(criticalMessage).toBeVisible();
      
      // Verify critical badge
      const statusBadge = await criticalMessage.locator('.chat-status-badge.status-critical');
      await expect(statusBadge).toBeVisible();
      
      // Clean up - restart the container
      await startContainer(page, appName);
    });
  });

  test.describe('Scenario 3: History Check', () => {
    test('should preserve and display message history correctly', async ({ page }) => {
      // Use existing app or create new one
      const appName = `test-history-${Date.now()}`;
      await createTestApp(page, appName, 'nginx:alpine');
      
      // Wait for container to be running
      await page.waitForTimeout(5000);
      
      // Trigger first agent run
      await triggerAgentRun(page);
      await page.waitForTimeout(5000);
      
      // Get first batch of messages
      const firstBatch = await getChatMessages(page, appName);
      const firstCount = firstBatch.length;
      expect(firstCount).toBeGreaterThan(0);
      
      // Trigger second agent run
      await triggerAgentRun(page);
      await page.waitForTimeout(5000);
      
      // Get updated messages
      const secondBatch = await getChatMessages(page, appName);
      const secondCount = secondBatch.length;
      
      // Verify new message was added
      expect(secondCount).toBeGreaterThan(firstCount);
      
      // Navigate to app detail page
      await page.goto(`/apps/${appName}`);
      
      // Verify multiple messages are visible
      const chatMessages = await page.locator('.chat-message').all();
      expect(chatMessages.length).toBeGreaterThanOrEqual(2);
      
      // Verify messages are ordered by timestamp (newest first)
      const timestamps = [];
      for (const message of chatMessages) {
        const timestampText = await message.locator('.chat-message-timestamp').innerText();
        timestamps.push(timestampText);
      }
      
      // Check that timestamps are in descending order (newest first)
      for (let i = 0; i < timestamps.length - 1; i++) {
        const current = new Date(timestamps[i]);
        const next = new Date(timestamps[i + 1]);
        expect(current.getTime()).toBeGreaterThanOrEqual(next.getTime());
      }
      
      // Verify that older messages are still present
      const oldestMessage = chatMessages[chatMessages.length - 1];
      await expect(oldestMessage).toBeVisible();
    });
  });

  test.describe('Chat UI Components', () => {
    test('should display correct styling for different status levels', async ({ page }) => {
      // Navigate to an app with existing messages (or create test data)
      const appName = `test-ui-${Date.now()}`;
      await createTestApp(page, appName, 'nginx:alpine');
      
      await page.goto(`/apps/${appName}`);
      
      // Check for chat container
      const chatContainer = await page.locator('#chat-messages-container');
      await expect(chatContainer).toBeVisible();
      
      // Trigger agent run to generate a message
      await triggerAgentRun(page);
      await page.waitForTimeout(5000);
      
      // Reload page to see the message
      await page.reload();
      
      // Check for agent avatar
      const avatar = await page.locator('.chat-message-avatar').first();
      await expect(avatar).toBeVisible();
      
      // Check for timestamp formatting
      const timestamp = await page.locator('.chat-message-timestamp').first();
      await expect(timestamp).toBeVisible();
      const timestampText = await timestamp.innerText();
      expect(timestampText).toMatch(/\d{4}-\d{2}-\d{2}/); // Basic date format check
      
      // Check for message summary
      const summary = await page.locator('.chat-message-summary').first();
      await expect(summary).toBeVisible();
    });
    
    test('should handle empty state gracefully', async ({ page }) => {
      // Create app without triggering agent
      const appName = `test-empty-${Date.now()}`;
      await createTestApp(page, appName, 'nginx:alpine');
      
      await page.goto(`/apps/${appName}`);
      
      // Check for chat container
      const chatContainer = await page.locator('#chat-messages-container');
      await expect(chatContainer).toBeVisible();
      
      // Check for empty state or no messages
      const messages = await getChatMessages(page, appName);
      if (messages.length === 0) {
        // Should show empty state
        const emptyState = await page.locator('.chat-empty-state');
        if (await emptyState.isVisible()) {
          const emptyText = await emptyState.innerText();
          expect(emptyText.toLowerCase()).toContain('no messages');
        }
      }
    });
  });

  test.afterAll(async ({ page }) => {
    // Clean up test apps if needed
    // This could include deleting the test apps created during tests
    console.log('Test suite completed');
  });
});