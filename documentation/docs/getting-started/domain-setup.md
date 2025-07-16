---
sidebar_position: 2
---

# Domain Setup

OnTree integrates with Caddy to provide automatic HTTPS certificates and easy domain management. This guide will help you configure domains so you can access your apps at `https://app.yourdomain.com`.

## Overview

When properly configured, OnTree allows you to:
- Expose apps at custom subdomains (e.g., `chat.example.com`)
- Get automatic HTTPS certificates from Let's Encrypt
- Support both public domains and Tailscale domains
- Manage everything through the OnTree interface

## Prerequisites

Before setting up domains, you need:

1. **A domain name** - Either:
   - A public domain you own (e.g., `example.com`)
   - A Tailscale domain (e.g., `machine.tail-scale.ts.net`)

2. **DNS control** - Ability to create DNS records

3. **Caddy installed** - With the admin API enabled

## Step 1: Install Caddy

OnTree uses Caddy's admin API to manage reverse proxy configurations dynamically.

### Ubuntu/Debian
```bash
# Add Caddy repository
sudo apt install -y debian-keyring debian-archive-keyring apt-transport-https
curl -1sLf 'https://dl.cloudsmith.io/public/caddy/stable/gpg.key' | sudo gpg --dearmor -o /usr/share/keyrings/caddy-stable-archive-keyring.gpg
curl -1sLf 'https://dl.cloudsmith.io/public/caddy/stable/debian.deb.txt' | sudo tee /etc/apt/sources.list.d/caddy-stable.list

# Install Caddy
sudo apt update
sudo apt install caddy
```

### macOS
```bash
brew install caddy
```

### Docker
```yaml
# docker-compose.yml
version: '3.8'

services:
  caddy:
    image: caddy:2-alpine
    container_name: caddy
    ports:
      - "80:80"
      - "443:443"
      - "2019:2019"  # Admin API
    volumes:
      - ./Caddyfile:/etc/caddy/Caddyfile
      - caddy_data:/data
      - caddy_config:/config
    restart: unless-stopped

volumes:
  caddy_data:
  caddy_config:
```

## Step 2: Configure Caddy

Create a `Caddyfile` with the admin API enabled:

```caddy
{
    admin localhost:2019
}

# Your existing site configurations can go here
# OnTree will add its own configurations via the API
```

Start Caddy:
```bash
# System service
sudo systemctl start caddy

# Or Docker
docker-compose up -d
```

Verify the admin API is accessible:
```bash
curl http://localhost:2019/config/
```

## Step 3: Configure DNS

### For Public Domains

Create a wildcard DNS record pointing to your server:

1. Log in to your DNS provider (Cloudflare, Namecheap, etc.)
2. Create an A record:
   - **Name**: `*` (wildcard)
   - **Value**: Your server's public IP address
   - **TTL**: 300 (5 minutes)

Example:
```
*.example.com    A    203.0.113.10
```

### For Tailscale Domains

If using Tailscale, your domain is automatically configured. Just ensure:
1. Tailscale is installed and connected
2. Your machine has a stable Tailscale hostname

## Step 4: Configure OnTree

Add your domain configuration to OnTree:

### Using Environment Variables
```bash
# For public domain
PUBLIC_BASE_DOMAIN=example.com ontree-server

# For Tailscale domain
TAILSCALE_BASE_DOMAIN=machine.tail-scale.ts.net ontree-server

# Or both
PUBLIC_BASE_DOMAIN=example.com \
TAILSCALE_BASE_DOMAIN=machine.tail-scale.ts.net \
ontree-server
```

### Using config.toml
```toml
# Public domain configuration
public_base_domain = "example.com"

# Tailscale domain configuration
tailscale_base_domain = "machine.tail-scale.ts.net"

# Caddy admin API (if not on default localhost:2019)
caddy_admin_url = "http://localhost:2019"
```

## Step 5: Test Domain Integration

1. **Open OnTree** and navigate to any app
2. **Click on the app** to view details
3. **Look for "Domain & Access"** section
4. If everything is configured correctly, you'll see:
   - An input field for subdomain
   - Your domain suffix displayed (e.g., `.example.com`)

## Using Domain Management

Once configured, exposing an app is simple:

1. **Navigate to your app** in OnTree
2. **Enter a subdomain** (e.g., `chat` for `chat.example.com`)
3. **Click "Expose App"**
4. OnTree will:
   - Configure Caddy with the reverse proxy
   - Request an HTTPS certificate automatically
   - Make your app accessible at the subdomain

### Example: Exposing Open WebUI

1. Deploy Open WebUI from templates
2. In the Domain & Access section, enter `chat` as subdomain
3. Click "Expose App"
4. Access your app at `https://chat.example.com`

## Troubleshooting

### "Caddy is not available"

This means OnTree cannot connect to Caddy's admin API:

```bash
# Check if Caddy is running
systemctl status caddy

# Check if admin API is accessible
curl http://localhost:2019/config/

# Check OnTree logs for specific errors
journalctl -u ontree -f
```

### "No domains configured"

Ensure you've set the domain environment variables or config:

```bash
# Check current configuration
grep -E "domain|DOMAIN" config.toml

# Restart OnTree after configuration changes
systemctl restart ontree
```

### Certificate Errors

Caddy automatically obtains certificates, but issues can occur:

1. **Check DNS propagation**:
   ```bash
   dig chat.example.com
   ```

2. **Check Caddy logs**:
   ```bash
   journalctl -u caddy -f
   ```

3. **Ensure ports 80 and 443 are open**:
   ```bash
   sudo ufw allow 80
   sudo ufw allow 443
   ```

## Advanced Configuration

### Custom Caddy Configuration

You can add custom Caddy directives via the OnTree interface:

1. Go to Settings → Caddy Configuration
2. Add custom global or per-domain settings
3. OnTree will merge these with its automatic configuration

### Multiple Domains

OnTree supports multiple domains:

```toml
# Primary domains
public_base_domain = "example.com"
tailscale_base_domain = "machine.tail-scale.ts.net"

# Additional domains (future feature)
additional_domains = ["apps.company.com", "internal.company.com"]
```

### Internal-Only Access

For Tailscale domains, apps are only accessible within your Tailnet, providing built-in security for internal tools.

## Security Considerations

- **HTTPS Only**: Caddy automatically redirects HTTP to HTTPS
- **Certificate Management**: Certificates are auto-renewed by Caddy
- **Subdomain Isolation**: Each app runs on its own subdomain
- **Tailscale Security**: Tailscale domains include built-in authentication

## Next Steps

With domains configured, you're ready to:
- [Create your first app](/docs/getting-started/first-app) with domain access
- Learn about [app management features](/docs/features/app-management)
- Explore [monitoring capabilities](/docs/features/monitoring)