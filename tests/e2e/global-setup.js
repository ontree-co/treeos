const { execSync } = require('child_process');
const fs = require('fs');
const path = require('path');

// Environment variable support
const TEST_BASE_URL = process.env.TEST_BASE_URL || 'http://localhost:3001';
const TEST_PORT = process.env.TEST_PORT || '3001';
const TEST_DB_PATH = process.env.TEST_DB_PATH || path.join(__dirname, '..', '..', 'ontree.db');
const TEST_ADMIN_USER = process.env.TEST_ADMIN_USER || 'admin';
const TEST_ADMIN_PASSWORD = process.env.TEST_ADMIN_PASSWORD || 'admin1234';
const TEST_NODE_NAME = process.env.TEST_NODE_NAME || 'Test OnTree Node';
const TEST_NODE_DESCRIPTION = process.env.TEST_NODE_DESCRIPTION || 'This is a test node for e2e testing';
const MAX_HEALTH_CHECK_RETRIES = 30;

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
      // Wait before retrying (exponential backoff with max 2s)
      const waitTime = Math.min(1000 * Math.pow(1.5, i), 2000);
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
  execSync(`LISTEN_ADDR=:${TEST_PORT} nohup ${serverPath} > server.log 2>&1 &`, { 
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
      const setupData = `username=${TEST_ADMIN_USER}&password=${TEST_ADMIN_PASSWORD}&password2=${TEST_ADMIN_PASSWORD}&node_name=${encodedNodeName}&node_description=${encodedNodeDesc}`;
      
      execSync(`curl -s -X POST -d '${setupData}' -H 'Content-Type: application/x-www-form-urlencoded' ${TEST_BASE_URL}/setup`, { stdio: 'ignore' });
      console.log('✅ Initial setup completed');
      
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
  
  // Enhanced Docker cleanup
  console.log('🐳 Performing enhanced Docker cleanup...');
  
  // Clean up containers
  try {
    const containers = execSync('docker ps -a --filter "name=ontree-test-" --format "{{.Names}}"', { encoding: 'utf8' }).trim();
    if (containers) {
      const containerList = containers.split('\n').filter(c => c);
      console.log(`  - Found ${containerList.length} test containers to clean up`);
      
      // Force stop and remove containers
      containerList.forEach(container => {
        try {
          execSync(`docker stop ${container} 2>/dev/null || true`, { stdio: 'ignore' });
          execSync(`docker rm -f ${container} 2>/dev/null || true`, { stdio: 'ignore' });
          console.log(`    ✓ Removed container: ${container}`);
        } catch (err) {
          console.error(`    ✗ Failed to remove container: ${container}`);
        }
      });
    }
  } catch (err) {
    console.warn('⚠️  Docker container cleanup warning:', err.message);
  }
  
  // Clean up networks
  try {
    const networks = execSync('docker network ls --filter "name=ontree-test-" --format "{{.Name}}"', { encoding: 'utf8' }).trim();
    if (networks) {
      const networkList = networks.split('\n').filter(n => n);
      console.log(`  - Found ${networkList.length} test networks to clean up`);
      
      networkList.forEach(network => {
        try {
          execSync(`docker network rm ${network} 2>/dev/null || true`, { stdio: 'ignore' });
          console.log(`    ✓ Removed network: ${network}`);
        } catch (err) {
          console.error(`    ✗ Failed to remove network: ${network}`);
        }
      });
    }
  } catch (err) {
    console.warn('⚠️  Docker network cleanup warning:', err.message);
  }
  
  // Clean up volumes
  try {
    const volumes = execSync('docker volume ls --filter "name=ontree-test-" --format "{{.Name}}"', { encoding: 'utf8' }).trim();
    if (volumes) {
      const volumeList = volumes.split('\n').filter(v => v);
      console.log(`  - Found ${volumeList.length} test volumes to clean up`);
      
      volumeList.forEach(volume => {
        try {
          execSync(`docker volume rm ${volume} 2>/dev/null || true`, { stdio: 'ignore' });
          console.log(`    ✓ Removed volume: ${volume}`);
        } catch (err) {
          console.error(`    ✗ Failed to remove volume: ${volume}`);
        }
      });
    }
  } catch (err) {
    console.warn('⚠️  Docker volume cleanup warning:', err.message);
  }
  
  console.log('✅ Global setup complete!');
  console.log('═══════════════════════════════════════\n');
};