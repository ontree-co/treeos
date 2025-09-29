const { execSync } = require('child_process');
const fs = require('fs');
const path = require('path');

// Environment variable support (matching global-setup.js)
const TEST_PORT = process.env.TEST_PORT || '3002';
const KEEP_SERVER_RUNNING = process.env.KEEP_SERVER_RUNNING === 'true';
const KEEP_TEST_DATA = process.env.KEEP_TEST_DATA === 'true';

module.exports = async () => {
  console.log('\n═══════════════════════════════════════');
  console.log('🧹 Running global teardown...');
  
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
  
  // Clean up test application directories
  if (!KEEP_TEST_DATA) {
    console.log('📁 Cleaning up test application directories...');
    const appsPath = path.join(__dirname, '..', '..', 'apps');
    const testAppPatterns = ['test-', 'e2e-', 'ontree-test-'];
    
    try {
      if (fs.existsSync(appsPath)) {
        const appDirs = fs.readdirSync(appsPath);
        appDirs.forEach(appDir => {
          if (testAppPatterns.some(pattern => appDir.startsWith(pattern))) {
            const appPath = path.join(appsPath, appDir);
            try {
              fs.rmSync(appPath, { recursive: true, force: true });
              console.log(`  ✓ Removed test app directory: ${appDir}`);
            } catch (err) {
              // Try with sudo if permission denied
              if (err.code === 'EACCES' || err.code === 'EPERM') {
                try {
                  execSync(`sudo rm -rf ${appPath} 2>/dev/null || true`, { stdio: 'ignore' });
                  console.log(`  ✓ Removed test app directory with elevated permissions: ${appDir}`);
                } catch (sudoErr) {
                  console.warn(`  ⚠️  Could not remove test app directory: ${appDir} (permission denied)`);
                }
              } else {
                console.error(`  ✗ Failed to remove test app directory: ${appDir} - ${err.message}`);
              }
            }
          }
        });
      }
    } catch (err) {
      console.warn('⚠️  Test app directory cleanup warning:', err.message);
    }
  } else {
    console.log('ℹ️  Keeping test data (KEEP_TEST_DATA=true)');
  }
  
  // Stop the test server if not explicitly asked to keep it running
  if (!KEEP_SERVER_RUNNING) {
    console.log(`⏹️  Stopping test server on port ${TEST_PORT}...`);
    try {
      // Try to stop the server gracefully
      execSync('pkill -TERM treeos 2>/dev/null || true', { stdio: 'ignore' });
      
      // Give it a moment to shut down gracefully
      execSync('sleep 1');
      
      // Force kill if still running
      execSync('pkill -KILL treeos 2>/dev/null || true', { stdio: 'ignore' });
      console.log('  ✓ Test server stopped');
    } catch (err) {
      console.warn('  ⚠️  Server may not have been running');
    }
  } else {
    console.log('ℹ️  Keeping server running (KEEP_SERVER_RUNNING=true)');
  }
  
  // Clean up any temporary files
  console.log('🗑️  Cleaning up temporary files...');
  const tempFiles = ['server.log', 'test-results.json', 'coverage.out'];
  const projectRoot = path.join(__dirname, '..', '..');
  
  tempFiles.forEach(file => {
    const filePath = path.join(projectRoot, file);
    if (fs.existsSync(filePath)) {
      try {
        fs.unlinkSync(filePath);
        console.log(`  ✓ Removed temporary file: ${file}`);
      } catch (err) {
        console.error(`  ✗ Failed to remove temporary file: ${file}`);
      }
    }
  });
  
  console.log('✅ Global teardown complete!');
  console.log('═══════════════════════════════════════\n');
};