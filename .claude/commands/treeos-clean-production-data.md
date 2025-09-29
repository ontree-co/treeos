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

### Will be PRESERVED:
- `/opt/ontree/shared/` directory (shared data)
- `/opt/ontree/shared/ollama/` directory (Ollama models)

**To confirm deletion, please type "yes" when prompted in Claude Code.**

If the user confirms with "yes", proceed with cleanup. Otherwise, abort the operation.

## Cleanup Process

### Step 1: Get User Confirmation
Ask the user to confirm by typing "yes" to proceed with deletion.

### Step 2: Execute Cleanup (only if confirmed)
If the user types "yes", execute the non-interactive cleanup script with sudo:

!sudo ./.claude/commands/clean-production-noconfirm.sh

### Step 3: Report Results
After successful cleanup:
!echo "Production data has been cleaned. Shared data and Ollama models have been preserved."

If the user doesn't confirm, respond:
!echo "Cleanup cancelled. No data was deleted."

## Post-Cleanup

After cleanup:
- Shared data and Ollama models remain intact
- To reinstall TreeOS, run the setup script
- Use `/treeos-clean-production-data-deep` for complete removal including shared data

**IMPORTANT REMINDERS**:
- ALWAYS get explicit "yes" confirmation from the user before executing cleanup
- Use the non-interactive script `.claude/commands/clean-production-noconfirm.sh` after confirmation
- Script requires sudo privileges to modify /opt/ontree
- This script PRESERVES shared folders - use deep cleanup for complete removal
- DO NOT use `rm -rf` commands manually