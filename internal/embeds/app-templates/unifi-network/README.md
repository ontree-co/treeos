# Unifi Network Application Template for TreeOS

## Overview

This template deploys the Unifi Network Application (formerly Unifi Controller) on TreeOS, providing a comprehensive network management solution for Ubiquiti devices including access points, switches, and security gateways.

## Components

- **Unifi Network Application**: Web-based management interface (LinuxServer.io image v8.4.62)
- **MongoDB 6.0.15**: Database backend for storing configurations and statistics
- **Persistent Volumes**: Separate volumes for MongoDB data and Unifi configuration

## Features

- 🌐 Complete network device management
- 📊 Real-time statistics and monitoring
- 🔒 Guest portal and hotspot management
- 📡 Wireless network configuration
- 🔧 Automatic device adoption and provisioning
- 📈 Historical data and analytics
- 🎯 Deep packet inspection (DPI)
- 🔐 RADIUS server integration

## Quick Start

### 1. Deploy via TreeOS Dashboard

1. Navigate to the TreeOS Apps section
2. Select "Unifi Network Application" from the template library
3. Configure the deployment settings
4. Click "Deploy"

### 2. Manual Deployment

```bash
# Create app directory
mkdir -p /opt/ontree/apps/unifi-network
cd /opt/ontree/apps/unifi-network

# Copy template files
cp /path/to/templates/compose/unifi-network.yml docker-compose.yml
cp /path/to/templates/compose/unifi-network/.env.template .env

# Configure environment
nano .env  # Set MONGO_PASS and adjust other settings

# Deploy
docker-compose up -d
```

## Configuration

### Required Environment Variables

| Variable | Description | Default |
|----------|-------------|---------|
| `MONGO_PASS` | MongoDB password (MUST CHANGE) | None |
| `TZ` | Timezone | UTC |
| `PUID` | Process user ID | 1000 |
| `PGID` | Process group ID | 1000 |

### Port Mappings

| Port | Protocol | Purpose |
|------|----------|---------|
| 8443 | TCP | Web admin (HTTPS) |
| 8080 | TCP | Device communication |
| 3478 | UDP | STUN |
| 10001 | UDP | Device discovery |
| 1900 | UDP | L2 discovery |
| 6789 | TCP | Speed test |
| 5514 | UDP | Syslog |

## Initial Setup

### First Run

1. Access the web interface at `https://your-server:8443`
2. Follow the setup wizard:
   - Create admin account
   - Configure basic settings
   - Set up your first site
3. Adopt your Ubiquiti devices

### Device Adoption

Devices can be adopted using:
- Layer 2 adoption (same network)
- Layer 3 adoption (DNS or DHCP option 43)
- SSH adoption: `set-inform http://your-server:8080/inform`

## Backup and Restore

### Creating Backups

```bash
# Automated backup script
cd /opt/ontree/apps/unifi-network
docker-compose exec app backup
```

### Restoring from Backup

See [backup-data/restore-instructions.md](backup-data/restore-instructions.md) for detailed restoration procedures.

## Migration from Existing Installation

If you have an existing Unifi Controller:

1. Export a backup from your current controller
2. Deploy this template
3. Import the backup through the web interface
4. Update device inform URLs if needed

## Security Considerations

### Best Practices

1. **Change default passwords immediately**
2. **Use HTTPS only** - Disable HTTP in settings
3. **Configure firewall rules** - Limit access to management ports
4. **Enable 2FA** for admin accounts
5. **Regular backups** - Automate daily backups
6. **Keep updated** - Monitor for security updates

### Firewall Rules

```bash
# Essential ports only
ufw allow 8443/tcp  # Admin interface
ufw allow 8080/tcp  # Device inform
ufw allow 3478/udp  # STUN
ufw allow 10001/udp # Discovery
```

## Troubleshooting

### Common Issues

#### MongoDB Connection Failed
```bash
# Check MongoDB status
docker-compose logs mongo
docker exec -it ontree-unifi-network-mongo-1 mongosh --eval "db.adminCommand('ping')"
```

#### Devices Not Adopting
1. Check firewall rules
2. Verify inform URL: `http://your-server:8080/inform`
3. Check device connectivity: `ping your-server`
4. SSH to device and set-inform manually

#### High Memory Usage
Add to `.env`:
```bash
MEM_LIMIT=1024
MEM_STARTUP=1024
```

### Logs

```bash
# Application logs
docker-compose logs -f app

# MongoDB logs
docker-compose logs -f mongo

# All logs
docker-compose logs -f
```

## Performance Tuning

### For Large Deployments (>100 devices)

1. Increase MongoDB memory:
```yaml
mongo:
  command: --wiredTigerCacheSizeGB 2
```

2. Adjust Java heap size in `.env`:
```bash
MEM_LIMIT=4096
MEM_STARTUP=2048
```

3. Use SSD storage for volumes

## Updates

### Updating the Application

```bash
cd /opt/ontree/apps/unifi-network
docker-compose down
docker-compose pull
docker-compose up -d
```

**Note**: Always backup before updating. Major version updates may require migration steps.

## Support and Resources

- **Official Unifi Documentation**: https://help.ui.com/
- **Community Forums**: https://community.ui.com/
- **TreeOS Support**: Check TreeOS documentation
- **LinuxServer.io Docs**: https://docs.linuxserver.io/images/docker-unifi-network-application

## License

This template is provided as-is for use with TreeOS. The Unifi Network Application is proprietary software by Ubiquiti Inc.

## Version History

- **1.0.0** - Initial TreeOS template
  - MongoDB 6.0.15 for stability
  - Unifi Network Application 8.4.62
  - Full backup/restore support
  - TreeOS integration with proper naming scheme