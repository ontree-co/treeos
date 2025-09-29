const { execSync } = require('child_process');
const fs = require('fs');
const path = require('path');

// Environment variable support
const TEST_BASE_URL = process.env.TEST_BASE_URL || process.env.BASE_URL || 'http://localhost:3002';
const TEST_PORT = process.env.TEST_PORT || '3002';
const TEST_DB_PATH = process.env.TEST_DB_PATH || path.join(__dirname, '..', '..', 'ontree.db');
const TEST_ADMIN_USER = process.env.TEST_ADMIN_USER || 'admin';
const TEST_ADMIN_PASSWORD = process.env.TEST_ADMIN_PASSWORD || 'admin1234';
const TEST_NODE_NAME = process.env.TEST_NODE_NAME || 'Test OnTree Node';
const TEST_NODE_DESCRIPTION = process.env.TEST_NODE_DESCRIPTION || 'This is a test node for e2e testing';
const MAX_HEALTH_CHECK_RETRIES = 20;  // Reduced from 30 for faster failure detection

// Helper function for health check with retries
async function waitForServerHealth(url, maxRetries = MAX_HEALTH_CHECK_RETRIES) {
  console.log(`Waiting for server health check at ${url}...`);
  
  for (let i = 0; i < maxRetries; i++) {
    try {
      execSync(`curl -s -f ${url}/ > /dev/null`, { stdio: 'ignore' });
      console.log(`✅ Server is healthy and responding at ${url}`);
      return true;
    } catch (err) {
      if (i === maxRetries - 1) {
        console.error(`❌ Server health check failed after ${maxRetries} attempts`);
        return false;
      }
      // Wait before retrying (exponential backoff with max 1s for faster startup)
      const waitTime = Math.min(500 * Math.pow(1.5, i), 1000);
      execSync(`sleep ${waitTime / 1000}`);
    }
  }
}

module.exports = async () => {
  console.log('🚀 Running global setup...');
  console.log(`📍 Test configuration:
  - Base URL: ${TEST_BASE_URL}
  - Port: ${TEST_PORT}
  - DB Path: ${TEST_DB_PATH}
  - Admin User: ${TEST_ADMIN_USER}`);
  
  // Clean up any existing test database
  if (fs.existsSync(TEST_DB_PATH)) {
    console.log('🗑️  Removing existing database...');
    fs.unlinkSync(TEST_DB_PATH);
  }
  
  // Kill and restart the server to pick up the clean database
  try {
    execSync('pkill treeos', { stdio: 'ignore' });
    console.log('⏹️  Stopped existing server');
  } catch (err) {
    // Server might not be running
  }
  
  // Give it a moment to shut down
  execSync('sleep 1');
  
  // Start the server
  const serverPath = path.join(__dirname, '..', '..', 'build', 'treeos');
  if (!fs.existsSync(serverPath)) {
    throw new Error(`Server binary not found at ${serverPath}. Please run 'make build' first.`);
  }
  execSync(`nohup ${serverPath} --demo -p ${TEST_PORT} > server.log 2>&1 &`, {
    cwd: path.join(__dirname, '..', '..'),
    shell: true
  });
  console.log(`🟢 Started server on port ${TEST_PORT}`);
  
  // Wait for server to be healthy
  const isHealthy = await waitForServerHealth(TEST_BASE_URL);
  if (!isHealthy) {
    throw new Error('Server failed to start or is not healthy');
  }
  
  // Check if setup is needed and complete it
  try {
    const response = execSync(`curl -s -o /dev/null -w "%{http_code}" ${TEST_BASE_URL}/setup`, { encoding: 'utf8' }).trim();
    if (response === '200') {
      console.log('📝 Setup page is accessible, completing initial setup...');
      
      // Complete setup using curl with environment variables
      const encodedNodeName = encodeURIComponent(TEST_NODE_NAME).replace(/%20/g, '+');
      const encodedNodeDesc = encodeURIComponent(TEST_NODE_DESCRIPTION).replace(/%20/g, '+');
      const setupData = `username=${TEST_ADMIN_USER}&password=${TEST_ADMIN_PASSWORD}&password2=${TEST_ADMIN_PASSWORD}&node_name=${encodedNodeName}&node_description=${encodedNodeDesc}&node_icon=tree1`;

      const setupResponse = execSync(`curl -s -X POST -d '${setupData}' -H 'Content-Type: application/x-www-form-urlencoded' -w "\\n%{http_code}" ${TEST_BASE_URL}/setup`, { encoding: 'utf8' });
      const lines = setupResponse.trim().split('\n');
      const statusCode = lines[lines.length - 1];

      if (statusCode === '302' || statusCode === '303') {
        console.log('✅ Initial setup completed successfully (redirect received)');
      } else if (statusCode === '200') {
        // Check if we got an error message in the response
        if (setupResponse.includes('alert-danger') || setupResponse.includes('error')) {
          console.error('❌ Setup failed - error in response');
          throw new Error('Setup failed with validation errors');
        }
        console.log('✅ Initial setup completed');
      } else {
        console.error(`❌ Unexpected status code from setup: ${statusCode}`);
        throw new Error(`Setup failed with status ${statusCode}`);
      }
      
      // Give it a moment to process
      execSync('sleep 1');
    } else {
      console.log('ℹ️  Setup already completed');
    }
  } catch (err) {
    console.error('❌ Error checking/completing setup:', err.message);
  }
  
  // Clean up any existing test apps
  const appsPath = path.join(__dirname, '..', '..', 'apps');
  const testApps = ['test-nginx', 'test-postgres', 'template-test'];
  
  console.log('🧹 Cleaning up test applications...');
  testApps.forEach(appName => {
    const appPath = path.join(appsPath, appName);
    if (fs.existsSync(appPath)) {
      console.log(`  - Removing test app: ${appName}`);
      fs.rmSync(appPath, { recursive: true, force: true });
    }
  });
  
  // Optimized Docker cleanup - run in parallel for speed
  console.log('🐳 Performing Docker cleanup...');

  try {
    // Use a single command to clean up all test containers, networks, and volumes
    // This is much faster than iterating through each one
    execSync(`
      # Stop and remove all test containers in one go
      docker ps -aq --filter "name=ontree-test-" | xargs -r docker rm -f 2>/dev/null || true;
      # Remove all test networks
      docker network ls -q --filter "name=ontree-test-" | xargs -r docker network rm 2>/dev/null || true;
      # Remove all test volumes
      docker volume ls -q --filter "name=ontree-test-" | xargs -r docker volume rm 2>/dev/null || true;
    `, { stdio: 'ignore', shell: '/bin/bash' });
    console.log('  ✓ Docker cleanup completed');
  } catch (err) {
    console.warn('⚠️  Docker cleanup warning:', err.message);
  }
  
  console.log('✅ Global setup complete!');
  console.log('═══════════════════════════════════════\n');
};