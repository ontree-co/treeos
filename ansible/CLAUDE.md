# Claude Session Learnings - OnTreeNode Ansible Integration

## Project Overview
This directory contains Ansible playbooks for deploying and managing OnTree applications and infrastructure. These playbooks were previously in a separate repository (OnTreeAnsible) but have been integrated into the main OnTreeNode repository for unified management. The main focus is on the OnTreeNode Django application deployment, along with supporting services like Caddy, Docker, OpenWebUI, and Readeck.

## Repository Structure Update (2025-06-23)
The ansible playbooks are now located in the `ansible/` subdirectory of the main OnTreeNode repository:
- Previous location: Separate OnTreeAnsible repository
- Current location: `/ansible/` directory within OnTreeNode repository
- All playbook paths remain relative to the ansible directory

## Current Playbook Structure

### OnTreeNode Playbooks
1. **ontreenode-enable-production-playbook.yaml**
   - Full deployment playbook for OnTreeNode Django application
   - Creates dedicated user, installs dependencies, clones from GitHub
   - Sets up Python virtual environment and systemd service
   - Targets hosts in `ontreenodes` group from inventory
   - Usage: `ansible-playbook -i inventory.ini ontreenode-enable-production-playbook.yaml`

2. **ontreenode-allow-local-development-playbook.yaml**
   - Stops the OnTreeNode service on remote servers
   - Allows local development without port conflicts
   - Usage: `ansible-playbook -i inventory.ini ontreenode-allow-local-development-playbook.yaml`

### Setup Playbooks
- **setup-caddy-playbook.yaml** - Installs and configures Caddy web server
- **setup-docker-playbook.yaml** - Installs Docker and Docker Compose

### Installation Playbooks
- **install-openwebui-playbook.yaml** - Installs OpenWebUI application
- **install-readeck-playbook.yaml** - Installs Readeck application
- **install-readeck-treefile.yaml** - Related Readeck configuration

## Inventory Management
- **inventory.ini** - Contains host definitions
- Current structure uses group `[ontreenodes]` (note: no hyphen in group name)
- Example targeting specific hosts: `--limit 93.201.63.187`

## Key Implementation Details

### 1. Git Repository Updates
The deployment playbook now ensures it always pulls the latest code:
- Checks if repository exists
- If exists: `git fetch`, `git reset --hard origin/main`, `git clean -fd`
- If not: clones fresh
- This guarantees the latest main branch code is deployed

### 2. Service Management Flow
- Development mode: Stops service on remote server
- Production mode: Full deployment + service restart
- Service runs on port 3000 via systemd

### 3. SSH Key Management
- Uses 1Password CLI for secure key storage
- Key stored as: `op://OnTree/ontreenode-github-deploy-key/private key`
- Must be in OpenSSH format (not PEM)
- Never commit SSH keys to repository

### 4. Required System Packages
- `acl` - For proper ansible become_user permissions
- `python3.12` - Specific Python version
- `git`, `python3-pip`, `python3-venv` - Development tools

## Best Practices Discovered

1. **Playbook Naming**: Use clear action prefixes (setup-, install-, enable-, allow-)
2. **Host Targeting**: Use inventory groups and --limit for specific hosts
3. **Service Control**: Always stop service before code updates
4. **Git Operations**: Force reset to ensure clean state
5. **Security**: Use 1Password for secrets, never commit keys

## Common Commands

```bash
# Full deployment to all nodes
ansible-playbook -i inventory.ini ontreenode-enable-production-playbook.yaml

# Deploy to specific host
ansible-playbook -i inventory.ini ontreenode-enable-production-playbook.yaml --limit 93.201.63.187

# Stop service for development
ansible-playbook -i inventory.ini ontreenode-allow-local-development-playbook.yaml

# Local playbooks (if you need sudo password)
ansible-playbook ontreenode-enable-production-playbook.yaml -K
```

## Troubleshooting

### Service Not Starting
- Check service status: `sudo systemctl status ontreenode`
- View service logs: `sudo journalctl -u ontreenode -n 50`
- Check if service is enabled: `sudo systemctl is-enabled ontreenode`
- Follow logs in real-time: `sudo journalctl -u ontreenode -f`
- Verify service file: `sudo cat /etc/systemd/system/ontreenode.service`

### "Gathering Facts" Hangs Forever
- Ensure the playbook's `hosts:` matches your inventory group
- Check SSH connectivity to remote hosts
- Verify inventory group names (no special characters)

### Invalid Characters in Group Names Warning
- Check inventory.ini for group naming
- Use `ontreenodes` not `ontree-nodes`

### Port Already in Use
- Run the allow-local-development playbook first
- Check for other processes: `sudo lsof -i :3000`

### Git Pull Not Getting Latest
- The playbook now does hard reset to origin/main
- Ensures all local changes are discarded

### SSH Key Issues
- Ensure both private and public keys are stored in 1Password
- Check key permissions: private key should be 600, public key 644
- Verify keys match: `ssh-keygen -y -f ~/.ssh/id_ed25519`

## Architecture Notes

- **Application User**: `ontreenode_user` (dedicated non-root user)
- **Application Path**: `/opt/ontreenode`
- **Service Name**: `ontreenode.service`
- **Python Environment**: Virtual environment at `/opt/ontreenode/venv`
- **Entry Point**: `/opt/ontreenode/venv/bin/python /opt/ontreenode/manage.py runprod`

## Recent Updates (2025-06-20)

### OnTreeNode Script Consolidation
The OnTreeNode repository has consolidated all shell scripts into manage.py (as of 2025-06-20):

1. **Removed Scripts**: `run_dev.sh`, `run_prod.sh`, `run_tests.sh`, `scripts/setup_ontree_dirs.sh` (no longer exist in the repository)
2. **Django Management Commands**:
   - `python manage.py rundev` - Start development server
   - `python manage.py runprod` - Start production server with Gunicorn
   - `sudo python manage.py setup_dirs` - Create /opt/ontree/apps directory (required)
   - `python manage.py test_all` - Run test suite

3. **Ansible Updates**:
   - Updated systemd service to use `python manage.py runprod` instead of `run_prod.sh`
   - Added task to run `setup_dirs` during deployment (creates `/opt/ontree/apps`)
   - Service now uses Python virtual environment directly

4. **New Dependencies**:
   - `gunicorn>=21.2.0` - Production WSGI server
   - `whitenoise>=6.5.0` - Static file serving

## Next Steps and Improvements
- Consider adding health check endpoints
- Add rollback capability for failed deployments
- Implement blue-green deployment strategy
- Add monitoring and alerting integration
- Consider using Ansible Vault for additional secrets
- Add support for ONTREE_APPS_DIR environment variable in systemd service