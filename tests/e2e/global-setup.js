const { execSync } = require('child_process');
const fs = require('fs');
const path = require('path');

module.exports = async () => {
  console.log('Running global setup...');
  
  // Clean up any existing test database
  const dbPath = path.join(__dirname, '..', '..', 'ontree.db');
  if (fs.existsSync(dbPath)) {
    console.log('Removing existing database...');
    fs.unlinkSync(dbPath);
  }
  
  // Kill and restart the server to pick up the clean database
  try {
    execSync('pkill ontree-server', { stdio: 'ignore' });
    console.log('Stopped existing server');
  } catch (err) {
    // Server might not be running
  }
  
  // Give it a moment to shut down
  execSync('sleep 1');
  
  // Start the server
  const serverPath = path.join(__dirname, '..', '..', 'ontree-server');
  execSync(`LISTEN_ADDR=:8085 nohup ${serverPath} > server.log 2>&1 &`, { 
    cwd: path.join(__dirname, '..', '..'),
    shell: true 
  });
  console.log('Started server on port 8085');
  
  // Wait for server to be ready
  execSync('sleep 3');
  
  // Check that server is actually running
  try {
    execSync('curl -s http://localhost:8085/ > /dev/null', { stdio: 'ignore' });
    console.log('Server is responding on port 8085');
  } catch (err) {
    console.error('Server not responding on port 8085');
  }
  
  // Clean up any existing test apps
  const appsPath = path.join(__dirname, '..', '..', 'apps');
  const testApps = ['test-nginx', 'test-postgres', 'template-test'];
  
  testApps.forEach(appName => {
    const appPath = path.join(appsPath, appName);
    if (fs.existsSync(appPath)) {
      console.log(`Cleaning up test app: ${appName}`);
      fs.rmSync(appPath, { recursive: true, force: true });
    }
  });
  
  // Clean up any Docker containers from previous tests
  try {
    const containers = execSync('docker ps -a --filter "name=ontree-test-" --format "{{.Names}}"', { encoding: 'utf8' }).trim();
    if (containers) {
      console.log('Cleaning up test containers...');
      containers.split('\n').forEach(container => {
        try {
          execSync(`docker stop ${container} 2>/dev/null || true`);
          execSync(`docker rm ${container} 2>/dev/null || true`);
        } catch (err) {
          // Ignore errors
        }
      });
    }
  } catch (err) {
    // Docker might not be available, ignore
  }
  
  console.log('Global setup complete.');
};