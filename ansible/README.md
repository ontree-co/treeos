# Install New Machine

- Put Ubuntu 24.04 LTS on a USB Stick with Balena Etcher
- use root as username next time, first time with ontree created problems with passwordless sudo
- attach to monitor and install on machine with mostly defaults. When it comes to ssh keys, allow only ssh and import files from Github (so far via user stefanmunz, in the future via ontree-co)
- if applicable: in the bios set the machine to maximum performance mode
- if applicable: in the bios set machine to auto start when power connects. So that afer a power failure the machine will come up automatically again
- test ssh login, then move the machine to final destination without monitor.
- if applicable: go to fritzbox and set it so that the machine always gets the same ip

# Install Ansible

- ontree-1 was setup with username ontree, so modifiying sudoers was necessary. Set it in almost the last line or it will not pick it up.
- ontree-2 was also setup with username ontree, also with only one partition because windows crossboot was left activated for benchmarking

```
sudo visudo
ontree ALL=(ALL) NOPASSWD: ALL
```

- add ip to inventory.ini and test the node:

```
ansible myhosts -m ping -i inventory.ini
```

- run docker-playbook.yaml

```
ansible-playbook -i inventory.ini docker-playbook.yaml
```

## Directory Structure

### templates/
This directory contains Jinja2 templates used by the Ansible playbooks.

- `ontreenode.service.j2` - systemd service template for the OnTreeNode application

## 1Password Integration

This playbook uses 1Password CLI to manage SSH deployment keys securely.

**Prerequisites:**
1. Install 1Password CLI: `brew install --cask 1password-cli` (macOS) or see [1Password CLI docs](https://developer.1password.com/docs/cli/get-started/)
2. Sign in to 1Password CLI: `op signin`
3. Create an SSH key and store it in 1Password:
   ```bash
   # Generate SSH key in OpenSSH format (IMPORTANT: use ed25519)
   ssh-keygen -t ed25519 -f ~/ontreenode-deploy-key -N ""
   
   # Store in 1Password (OnTree vault)
   op item create --category="SSH Key" --title="ontreenode-github-deploy-key" --vault="OnTree" \
     "private key=$(cat ~/ontreenode-deploy-key)" \
     "public key=$(cat ~/ontreenode-deploy-key.pub)"
   
   # Clean up local files
   rm ~/ontreenode-deploy-key ~/ontreenode-deploy-key.pub
   ```
4. Add the public key to GitHub as a deploy key

**Important SSH Key Format Requirements:**
- The SSH key MUST be in OpenSSH format (starts with `-----BEGIN OPENSSH PRIVATE KEY-----`)
- Do NOT use PEM format keys (that start with `-----BEGIN PRIVATE KEY-----`)
- Always use `ed25519` key type as it's always in OpenSSH format
- If you have a PEM format key, convert it: `ssh-keygen -p -f key.pem -m OpenSSH -N ""`

**Note:** The playbook retrieves the SSH key using: `op://OnTree/ontreenode-github-deploy-key/private key`

## Playbooks

### ontreenode-playbook.yaml
Deploys the OnTreeNode Django application to target servers. This playbook:
- Creates a dedicated application user
- Installs system dependencies (Python, Git, ACL)
- Retrieves SSH keys from 1Password for private repository access
- Clones the application from GitHub
- Sets up a Python virtual environment
- Installs Python dependencies
- Configures and starts the application as a systemd service

**Usage:**
```bash
# Ensure you're signed in to 1Password CLI first
op signin

# Run the playbook
ansible-playbook -i inventory.ini ontreenode-playbook.yaml
```

## Security Notes

- SSH keys are stored in 1Password, never in the repository
- Use Ansible Vault for other sensitive variables
- Ensure proper file permissions on deployed keys (600)
- 1Password CLI session required for playbook execution
