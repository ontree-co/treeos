---
sidebar_position: 6
---

# Logging

TreeOS provides a simple logging system optimized for development and production environments.

## Development Mode

In development, TreeOS writes all logs to a single file for easy debugging:

```bash
# Enable debug mode
export DEBUG=true
# Or add to .env file: DEBUG=true

# Start the server
go run cmd/treeos/main.go

# Logs are written to ./logs/treeos.log
tail -f ./logs/treeos.log
```

### Features

- **Unified logging** - Server and browser logs in one file
- **Chronological order** - All events in sequence
- **Browser error capture** - JavaScript errors automatically sent to server
- **Console override** - `console.log/error/warn` forwarded to server

### Log Format

```
2025/09/15 09:28:16 [INFO] [SERVER] Starting application...
2025/09/15 09:28:18 [ERROR] [BROWSER] TypeError: Cannot read property...
2025/09/15 09:28:19 [INFO] [SERVER] GET /api/apps 200 15ms
```

## Production Mode

In production, TreeOS only logs to stdout/stderr:

- No file logging (reduces I/O overhead)
- Logs captured by systemd/Docker
- PostHog handles error tracking
- No browser log forwarding

## API Endpoints

### Send Browser Logs
```javascript
fetch('/api/log', {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({
        level: 'error',
        message: 'Something went wrong',
        details: { component: 'MyComponent' }
    })
});
```

### Query Logs (Development Only)
```bash
# Get recent logs
curl http://localhost:8080/api/logs?limit=100

# Filter by source
curl http://localhost:8080/api/logs?source=browser
```

## Configuration

### Environment Variables
```bash
DEBUG=true  # Enable file logging and debug logs
```

### Browser Settings
```javascript
// Enable logging in production (not recommended)
localStorage.setItem('enableLogging', 'true');

// Enable debug mode
localStorage.setItem('debug', 'true');
```

## Best Practices

1. **Use structured logging** in Go:
   ```go
   log.Printf("[Component] Action failed: %v", err)
   ```

2. **Add context** to browser logs:
   ```javascript
   console.error('API failed', { endpoint: '/api/data', status: 500 });
   ```

3. **Never log sensitive data** (passwords, tokens, personal info)

4. **Keep logs concise** and actionable

## Troubleshooting

### Logs not appearing?
- Check `DEBUG=true` is set
- Verify `./logs/` directory exists
- Ensure server has write permissions

### Browser logs not forwarding?
- Check browser console for `/api/log` errors
- Verify you're on localhost (production disabled by default)