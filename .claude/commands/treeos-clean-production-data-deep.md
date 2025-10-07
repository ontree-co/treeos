---
description: Deep clean ALL TreeOS production data (including shared folders)
argument-hint:
---

# Deep Clean ALL TreeOS Production Data

## ⚠️ CRITICAL CONFIRMATION REQUIRED

This command will **PERMANENTLY DELETE** ALL TreeOS production data including:

### TreeOS Files and Data:
- `/opt/ontree/apps/` directory (application configurations)
- `/opt/ontree/shared/` directory (ALL shared data)
- `/opt/ontree/shared/ollama/` directory (ALL Ollama models)
- `/opt/ontree/logs/` directory (log files)
- `/opt/ontree/ontree.db` and related SQLite files (database)
- `/opt/ontree/treeos` binary

### Docker Containers and Images:
- **ALL containers starting with `ontree-`** will be stopped and removed
- **ALL associated container images** will be deleted
- **Container images will need to be re-downloaded** for testing scenarios
- Docker image cache will be pruned

**This is a COMPLETE removal of TreeOS and will require full reinstallation!**

**To confirm this destructive operation, please type "yes" when prompted in Claude Code.**

If the user confirms with "yes", proceed with deep cleanup. Otherwise, abort the operation.

## Cleanup Process

### Step 1: Get User Confirmation
Ask the user to confirm by typing "yes" to proceed with COMPLETE deletion.

### Step 2: Provide Cleanup Command (only if confirmed)
If the user types "yes", provide the cleanup command for the user to run manually:

Tell the user to run this command in their terminal:
```bash
sudo ./.claude/commands/treeos-clean-production-deep-noconfirm.sh
```

**IMPORTANT**: DO NOT immediately check for results or assume the script has run. The script requires sudo and must be run manually by the user.

### Step 3: Wait for User Confirmation
After providing the command, ask the user to:
1. Run the command in their terminal
2. Once complete, either:
   - Type "done" to confirm they've run it
   - Or paste the output from the script

**DO NOT** check `/opt/ontree` or Docker containers immediately - the folder might already be missing from previous operations, which doesn't mean the script has run.

### Step 4: Verify Results (only after user confirms)
Only after the user confirms they've run the script (by typing "done" or pasting output):
- Check if `/opt/ontree` exists
- Check for remaining Docker containers/images
- Report the final status

If the user doesn't confirm with "yes" in Step 1:
!echo "Deep cleanup cancelled. No data was deleted."

## Reinstallation

After deep cleanup, TreeOS must be completely reinstalled:
1. Download the latest release
2. Run the setup script
3. All container images will be re-downloaded on first use

**CRITICAL REMINDERS**:
- ALWAYS get explicit "yes" confirmation from the user before executing deep cleanup
- Use the non-interactive script `.claude/commands/treeos-clean-production-deep-noconfirm.sh` after confirmation
- Script requires sudo privileges
- This removes EVERYTHING - complete data loss
- Container images will be re-downloaded (important for testing download scenarios)