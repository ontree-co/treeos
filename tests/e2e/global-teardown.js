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
  
  // Enhanced Docker cleanup - containers, networks, and volumes
  console.log('🐳 Performing comprehensive Docker cleanup...');
  
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
    } else {
      console.log('  - No test containers found');
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
    } else {
      console.log('  - No test networks found');
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
    } else {
      console.log('  - No test volumes found');
    }
  } catch (err) {
    console.warn('⚠️  Docker volume cleanup warning:', err.message);
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
      execSync('sleep 2');
      
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