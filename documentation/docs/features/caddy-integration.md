---
sidebar_position: 4
---

# Caddy Integration

OnTree seamlessly integrates with Caddy to provide automatic HTTPS certificates and easy domain management for your applications. This allows you to expose your apps securely at custom domains like `https://app.yourdomain.com`.

## How It Works

OnTree uses Caddy's Admin API to dynamically configure reverse proxy routes:

1. **You request** a subdomain for your app
2. **OnTree configures** Caddy with the routing rules
3. **Caddy obtains** HTTPS certificates automatically
4. **Your app becomes** accessible at the custom domain

## Setting Up Integration

### Prerequisites

Before using domain features:

1. **Caddy installed** with admin API enabled
2. **Domain configured** (public or Tailscale)
3. **DNS pointed** to your server
4. **Ports 80 & 443** accessible

### Quick Setup

1. **Install Caddy** (see [Domain Setup](/docs/getting-started/domain-setup))
2. **Configure domains** in OnTree:
   ```bash
   PUBLIC_BASE_DOMAIN=example.com treeos
   ```
3. **Verify integration** - Check app detail pages for Domain & Access section

## Using Domain Management

### Exposing an App

1. **Navigate to your app** in OnTree
2. **Find Domain & Access** section
3. **Enter subdomain** (e.g., `chat`, `wiki`, `cloud`)
4. **Click "Expose App"**

OnTree will:
- Configure Caddy reverse proxy
- Set up HTTPS automatically
- Make app available immediately

### Subdomain Guidelines

Choose subdomains wisely:

- **Descriptive**: `photos` not `app1`
- **Short**: `wiki` not `my-personal-wikipedia`
- **Valid**: Lowercase letters, numbers, hyphens
- **Unique**: Each subdomain used once

### Managing Exposed Apps

For exposed apps, you can:

- **View URLs** - See all access methods
- **Check status** - Verify subdomain availability
- **Unexpose** - Remove public access
- **Change subdomain** - Unexpose and re-expose

## Domain Types

### Public Domains

For internet-accessible applications:

```toml
public_base_domain = "example.com"
```

Features:
- Accessible from anywhere
- Automatic Let's Encrypt certificates
- Perfect for public services

### Tailscale Domains

For private, secure access:

```toml
tailscale_base_domain = "machine.tail-scale.ts.net"
```

Features:
- Only accessible within Tailnet
- Built-in authentication
- Ideal for internal tools

### Using Both

Configure both for flexibility:

```toml
public_base_domain = "example.com"
tailscale_base_domain = "machine.tail-scale.ts.net"
```

Choose per-app whether to use public or private domain.

## Advanced Configuration

### Custom Headers

Add security headers via Caddy:

```caddy
app.example.com {
    reverse_proxy localhost:3000 {
        header_up X-Real-IP {remote_host}
        header_up X-Forwarded-Proto {scheme}
    }
    
    header {
        Strict-Transport-Security "max-age=31536000"
        X-Content-Type-Options "nosniff"
        X-Frame-Options "DENY"
    }
}
```

### Path-Based Routing

Expose apps at paths instead of subdomains:

```
example.com/app1 -> localhost:3001
example.com/app2 -> localhost:3002
```

(Configuration coming soon)

### Wildcard Certificates

Caddy automatically handles wildcard certificates:

- Single certificate for `*.example.com`
- Covers all subdomains
- Automatic renewal

## Security Features

### Automatic HTTPS

All exposed apps get:
- **SSL/TLS encryption** - Data in transit protected
- **HTTP → HTTPS redirect** - Force secure connections
- **Modern cipher suites** - Strong encryption
- **Certificate renewal** - Automatic before expiry

### Access Control

Implement access restrictions:

#### Basic Authentication
```caddy
app.example.com {
    basicauth {
        admin $2a$14$Zkx19XV...
    }
    reverse_proxy localhost:3000
}
```

#### IP Whitelisting
```caddy
app.example.com {
    @allowed {
        remote_ip 192.168.1.0/24 10.0.0.0/8
    }
    handle @allowed {
        reverse_proxy localhost:3000
    }
    respond "Access denied" 403
}
```

### Rate Limiting

Protect against abuse:

```caddy
app.example.com {
    rate_limit {
        zone dynamic 100r/m
    }
    reverse_proxy localhost:3000
}
```

## Troubleshooting

### Common Issues

#### "Caddy is not available"

1. **Check Caddy is running**:
   ```bash
   systemctl status caddy
   ```

2. **Verify admin API**:
   ```bash
   curl http://localhost:2019/config/
   ```

3. **Check firewall** allows localhost:2019

#### Certificate Errors

1. **Check DNS propagation**:
   ```bash
   dig subdomain.example.com
   ```

2. **Verify ports 80/443** are open:
   ```bash
   sudo ufw status
   ```

3. **Check Caddy logs**:
   ```bash
   journalctl -u caddy -f
   ```

#### Subdomain Not Working

1. **Verify DNS wildcard** record exists
2. **Check subdomain** doesn't conflict
3. **Test direct access** to app port
4. **Review OnTree logs** for errors

### Debug Mode

Enable detailed logging:

```toml
# OnTree config
debug = true
caddy_debug = true
```

Check logs for:
- Caddy API requests
- Response details
- Configuration sent

## Best Practices

### Domain Organization

Structure your subdomains:

- **By function**: `api`, `admin`, `public`
- **By environment**: `dev-`, `staging-`, `prod-`
- **By project**: `project1-`, `project2-`

### Security Hardening

1. **Use strong subdomains** - Avoid common names
2. **Enable monitoring** - Watch for unusual traffic
3. **Regular updates** - Keep Caddy current
4. **Backup configurations** - Save Caddy config

### Performance

Optimize for speed:

1. **Enable HTTP/2** - Caddy does this automatically
2. **Configure compression** - Reduce bandwidth
3. **Set cache headers** - Improve load times
4. **Use CDN** - For static assets

## Integration Examples

### WordPress with Custom Domain

1. Deploy WordPress from template
2. Expose at `blog.example.com`
3. Update WordPress settings:
   - WordPress Address: `https://blog.example.com`
   - Site Address: `https://blog.example.com`

### Nextcloud with Tailscale

1. Deploy Nextcloud
2. Expose on Tailscale domain
3. Access securely from anywhere via Tailnet
4. No public internet exposure

### Development Environment

1. Deploy code-server
2. Expose at `dev.example.com`
3. Add basic auth for security
4. Access VS Code from any browser

## Future Enhancements

Planned Caddy integration features:

- **Load balancing** - Multiple container backends
- **Health checks** - Automatic failover
- **WebSocket support** - For real-time apps
- **Custom middleware** - Extended functionality
- **Multi-domain** - Different domains per app
- **Certificate management UI** - View/manage certs

The Caddy integration transforms OnTree from a local container manager into a full-featured hosting platform with enterprise-grade security and convenience.