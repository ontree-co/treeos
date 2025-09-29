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

### Podman Containers and Images:
- **ALL containers starting with `ontree-`** will be stopped and removed
- **ALL associated container images** will be deleted
- **Container images will need to be re-downloaded** for testing scenarios
- Podman build cache and dangling images will be pruned

**This is a COMPLETE removal of TreeOS and will require full reinstallation!**

**To confirm this destructive operation, please type "yes" when prompted in Claude Code.**

If the user confirms with "yes", proceed with deep cleanup. Otherwise, abort the operation.

## Cleanup Process

### Step 1: Get User Confirmation
Ask the user to confirm by typing "yes" to proceed with COMPLETE deletion.

### Step 2: Execute Deep Cleanup (only if confirmed)
If the user types "yes", execute the deep cleanup script with sudo:

!sudo ./.claude/commands/clean-production-deep-noconfirm.sh

### Step 3: Report Results
After successful cleanup:
!echo "TreeOS has been completely removed from the system, including all containers, images, shared data and Ollama models."

If the user doesn't confirm, respond:
!echo "Deep cleanup cancelled. No data was deleted."

## Reinstallation

After deep cleanup, TreeOS must be completely reinstalled:
1. Download the latest release
2. Run the setup script
3. All container images will be re-downloaded on first use

**CRITICAL REMINDERS**:
- ALWAYS get explicit "yes" confirmation from the user before executing deep cleanup
- Use the non-interactive script `.claude/commands/clean-production-deep-noconfirm.sh` after confirmation
- Script requires sudo privileges
- This removes EVERYTHING - complete data loss
- Container images will be re-downloaded (important for testing download scenarios)