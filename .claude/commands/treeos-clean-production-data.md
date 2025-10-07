---
description: Clean TreeOS production data (preserves shared folders)
argument-hint:
---

# Clean TreeOS Production Data (Preserves Shared)

## ⚠️ CONFIRMATION REQUIRED

This command will **permanently delete** TreeOS production data while **preserving shared folders**:

### Will be REMOVED:
- `/opt/ontree/apps/` directory (application configurations)
- `/opt/ontree/logs/` directory (log files)
- `/opt/ontree/ontree.db` and related SQLite files (database)
- `/opt/ontree/treeos` binary
- All Docker containers starting with `ontree-`

### Will be PRESERVED:
- `/opt/ontree/shared/` directory (shared data)
- `/opt/ontree/shared/ollama/` directory (Ollama models)

**To confirm deletion, please type "yes" when prompted in Claude Code.**

If the user confirms with "yes", proceed with cleanup. Otherwise, abort the operation.

## Cleanup Process

### Step 1: Get User Confirmation
Ask the user to confirm by typing "yes" to proceed with deletion.

### Step 2: Check Sudo Access and Guide User (only if confirmed)
If the user types "yes":

1. First check if we can execute with sudo:
!sudo -n true 2>/dev/null && echo "SUDO_AVAILABLE" || echo "SUDO_REQUIRED"

2. If sudo is not available (which is typical in Claude Code), inform the user:
   - This script requires sudo privileges to modify `/opt/ontree/`
   - Claude Code cannot provide sudo passwords for security reasons
   - The user needs to run this command manually on their server

3. Provide the command for the user to run:
```bash
sudo ./.claude/commands/treeos-clean-production-noconfirm.sh
```

4. Offer to review the script for safety:
   - Ask if they'd like you to review the cleanup script first
   - If yes, read and analyze `./.claude/commands/treeos-clean-production-noconfirm.sh`
   - Check for any potential issues or mistakes

### Step 3: Report Results
If sudo was available and cleanup succeeded:
!echo "Production data has been cleaned. Shared data and Ollama models have been preserved."

If sudo is required (typical case):
!echo "Please run the following command manually on your server with sudo access:"
!echo "sudo ./.claude/commands/treeos-clean-production-noconfirm.sh"
!echo "Would you like me to review the script first to ensure it's safe to run?"

If the user doesn't confirm, respond:
!echo "Cleanup cancelled. No data was deleted."

## Post-Cleanup

After cleanup:
- Shared data and Ollama models remain intact
- To reinstall TreeOS, run the setup script
- Use `/treeos-clean-production-data-deep` for complete removal including shared data

**IMPORTANT REMINDERS**:
- ALWAYS get explicit "yes" confirmation from the user before executing cleanup
- Use the non-interactive script `.claude/commands/treeos-clean-production-noconfirm.sh` after confirmation
- Script requires sudo privileges to modify /opt/ontree
- This script PRESERVES shared folders - use deep cleanup for complete removal
- DO NOT use `rm -rf` commands manually