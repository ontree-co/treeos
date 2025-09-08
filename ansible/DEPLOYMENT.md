# OnTree Deployment Guide

This guide covers deploying OnTree in production environments using various methods.

## Deployment Methods

### Method 1: Ansible Deployment (Recommended)

OnTree includes Ansible playbooks for automated deployment to Ubuntu/Debian servers.

#### Prerequisites
- Ansible 2.9+ installed locally
- Target server running Ubuntu 20.04+ or Debian 10+
- SSH access to target server with sudo privileges
- 1Password CLI (for secret management) or manual secret configuration

#### Quick Start

1. Clone the repository:
   ```bash
   git clone https://github.com/yourusername/treeos.git
   cd treeos/ansible
   ```

2. Update inventory:
   ```bash
   # Edit inventory.ini with your server details
   [ontree]
   your-server.com ansible_user=ubuntu ansible_python_interpreter=/usr/bin/python3
   ```

3. Deploy OnTree:
   ```bash
   # Install Docker on target server (if needed)
   ansible-playbook -i inventory.ini setup-docker-playbook.yaml
   
   # Deploy OnTree application
   ansible-playbook -i inventory.ini ontreenode-enable-production-playbook.yaml \
     -e github_release_version=v0.1.0 \
     -e auth_username=admin \
     -e auth_password=your-secure-password
   ```

4. (Optional) Set up Caddy for HTTPS:
   ```bash
   ansible-playbook -i inventory.ini setup-caddy-playbook.yaml \
     -e domain_name=ontree.yourdomain.com
   ```

### Method 2: Manual Systemd Deployment

#### 1. Create Application User
```bash
sudo useradd -r -s /bin/false ontree
sudo mkdir -p /opt/ontree/ontreenode
sudo chown -R ontree:ontree /opt/ontree
```

#### 2. Download and Install Binary
```bash
# Download latest release
cd /opt/ontree/ontreenode
sudo -u ontree wget https://github.com/yourusername/treeos/releases/download/v0.1.0/treeos-linux-amd64
sudo -u ontree mv treeos-linux-amd64 treeos
sudo -u ontree chmod +x treeos
```

#### 3. Create Systemd Service
```bash
sudo tee /etc/systemd/system/ontreenode.service > /dev/null << 'EOF'
[Unit]
Description=OnTree Node Service
After=network.target docker.service
Requires=docker.service

[Service]
Type=simple
User=ontree
Group=ontree
WorkingDirectory=/opt/ontree/ontreenode
Environment="PORT=8080"
Environment="DATABASE_PATH=/opt/ontree/ontreenode/ontree.db"
Environment="AUTH_USERNAME=admin"
Environment="AUTH_PASSWORD=CHANGE_THIS_PASSWORD"
Environment="SESSION_KEY=GENERATE_RANDOM_KEY_HERE"
ExecStart=/opt/ontree/treeos/treeos
Restart=always
RestartSec=10

# Security hardening
NoNewPrivileges=true
PrivateTmp=true
ProtectSystem=strict
ProtectHome=true
ReadWritePaths=/opt/ontree/ontreenode

[Install]
WantedBy=multi-user.target
EOF
```

#### 4. Configure and Start Service
```bash
# Set secure password and session key
sudo systemctl edit ontreenode
# Add your environment overrides

# Add ontree user to docker group
sudo usermod -aG docker ontree

# Start and enable service
sudo systemctl daemon-reload
sudo systemctl start ontreenode
sudo systemctl enable ontreenode

# Check status
sudo systemctl status ontreenode
sudo journalctl -u ontreenode -f
```

### Method 3: Docker Compose Deployment

Create `docker-compose.yml`:
```yaml
version: '3.8'

services:
  ontree:
    image: yourusername/ontree:latest
    container_name: ontree
    restart: unless-stopped
    ports:
      - "8080:8080"
    volumes:
      - /var/run/docker.sock:/var/run/docker.sock
      - ./data:/data
    environment:
      - AUTH_USERNAME=admin
      - AUTH_PASSWORD=${ONTREE_PASSWORD}
      - SESSION_KEY=${ONTREE_SESSION_KEY}
      - DATABASE_PATH=/data/ontree.db
    user: "1000:1000"  # Adjust to match docker socket permissions
```

Deploy:
```bash
# Create .env file with secrets
echo "ONTREE_PASSWORD=$(openssl rand -base64 32)" >> .env
echo "ONTREE_SESSION_KEY=$(openssl rand -base64 32)" >> .env

# Start OnTree
docker-compose up -d
```

## Production Configuration

### Environment Variables

**Required:**
- `AUTH_USERNAME`: Admin username for web interface
- `AUTH_PASSWORD`: Admin password (use strong password)

**Recommended:**
- `SESSION_KEY`: 32+ character random string for session encryption
- `DATABASE_PATH`: Path to SQLite database file
- `PORT`: HTTP listen port (default: 8080)

**Optional:**
- `OTEL_EXPORTER_OTLP_ENDPOINT`: OpenTelemetry collector endpoint
- `OTEL_SERVICE_NAME`: Service name for telemetry (default: ontree)

### Security Best Practices

1. **Use HTTPS**: Deploy behind a reverse proxy (Nginx, Caddy) with TLS
2. **Strong Passwords**: Use long, random passwords for AUTH_PASSWORD
3. **Firewall**: Restrict access to OnTree port (only expose through reverse proxy)
4. **Updates**: Regularly update OnTree and system packages
5. **Monitoring**: Set up monitoring and alerting

### Reverse Proxy Configuration

#### Nginx
```nginx
server {
    listen 443 ssl http2;
    server_name ontree.yourdomain.com;
    
    ssl_certificate /path/to/cert.pem;
    ssl_certificate_key /path/to/key.pem;
    
    location / {
        proxy_pass http://localhost:8080;
        proxy_set_header Host $host;
        proxy_set_header X-Real-IP $remote_addr;
        proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;
        proxy_set_header X-Forwarded-Proto $scheme;
    }
}
```

#### Caddy
```caddyfile
ontree.yourdomain.com {
    reverse_proxy localhost:8080
}
```

## Database Management

### Backup
```bash
# Manual backup
sudo -u ontree sqlite3 /opt/ontree/ontreenode/ontree.db ".backup /opt/ontree/backup/ontree-$(date +%Y%m%d).db"

# Automated daily backup (crontab)
0 2 * * * sqlite3 /opt/ontree/ontreenode/ontree.db ".backup /opt/ontree/backup/ontree-$(date +\%Y\%m\%d).db"
```

### Restore
```bash
# Stop service
sudo systemctl stop ontreenode

# Restore backup
sudo -u ontree cp /opt/ontree/backup/ontree-20240107.db /opt/ontree/ontreenode/ontree.db

# Start service
sudo systemctl start ontreenode
```

### Migration
When upgrading OnTree, database migrations are handled automatically on startup.

## Monitoring

### Health Check
```bash
# Basic health check
curl -u admin:password http://localhost:8080/health

# With authentication
curl -u $AUTH_USERNAME:$AUTH_PASSWORD http://localhost:8080/api/apps
```

### Logging
```bash
# View logs
sudo journalctl -u ontreenode -f

# Export logs
sudo journalctl -u ontreenode --since "1 hour ago" > ontree-logs.txt
```

### Metrics
OnTree supports OpenTelemetry for metrics and tracing:
```bash
# Configure OTEL exporter
export OTEL_EXPORTER_OTLP_ENDPOINT="http://localhost:4318"
export OTEL_SERVICE_NAME="ontree-production"
```

## Troubleshooting

### Service Won't Start
```bash
# Check logs
sudo journalctl -u ontreenode -n 50

# Common issues:
# - Port already in use: Change PORT environment variable
# - Permission denied: Ensure ontree user is in docker group
# - Database locked: Stop service, remove ontree.db-shm and ontree.db-wal
```

### Docker Connection Issues
```bash
# Verify docker socket permissions
ls -la /var/run/docker.sock

# Add ontree user to docker group
sudo usermod -aG docker ontree
sudo systemctl restart ontreenode
```

### Development Mode
For debugging on production server:
```bash
# Run playbook to stop production service
ansible-playbook -i inventory.ini ontreenode-allow-local-development-playbook.yaml

# SSH to server and run manually
ssh your-server
cd /opt/ontree/ontreenode
sudo -u ontree ./treeos

# Restore production mode
ansible-playbook -i inventory.ini ontreenode-enable-production-playbook.yaml
```

## Upgrading

### Using Ansible
```bash
ansible-playbook -i inventory.ini ontreenode-enable-production-playbook.yaml \
  -e github_release_version=v0.2.0
```

### Manual Upgrade
```bash
# Backup database
sudo -u ontree sqlite3 /opt/ontree/ontreenode/ontree.db ".backup /opt/ontree/backup/ontree-pre-upgrade.db"

# Download new version
cd /opt/ontree/ontreenode
sudo -u ontree wget https://github.com/yourusername/treeos/releases/download/v0.2.0/treeos-linux-amd64 -O treeos.new
sudo -u ontree chmod +x treeos.new

# Replace binary
sudo systemctl stop ontreenode
sudo -u ontree mv treeos treeos.old
sudo -u ontree mv treeos.new treeos

# Start service (migrations run automatically)
sudo systemctl start ontreenode

# Verify upgrade
sudo journalctl -u ontreenode -n 100
```

## Support

For deployment issues:
1. Check the [troubleshooting section](#troubleshooting)
2. Review logs with `journalctl -u ontreenode`
3. Open an issue at [GitHub Issues](https://github.com/yourusername/treeos/issues)