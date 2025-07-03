const { execSync } = require('child_process');

module.exports = async () => {
  console.log('Running global teardown...');
  
  // Clean up any Docker containers created during tests
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
  
  console.log('Global teardown complete.');
};