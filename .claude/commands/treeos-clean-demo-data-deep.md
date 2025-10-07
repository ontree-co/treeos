---
description: Deep clean ALL TreeOS demo mode data (including Docker containers)
argument-hint:
---

# Deep Clean ALL TreeOS Demo Mode Data

## ⚠️ CRITICAL CONFIRMATION REQUIRED

This command will **PERMANENTLY DELETE** ALL TreeOS demo mode data including:

### Demo Mode Files and Data:
- `./apps/` directory (application configurations)
- `./shared/` directory (includes Ollama models)
- `./logs/` directory (log files)
- `./ontree.db` and related SQLite files (database)

### Docker Containers and Images:
- **ALL containers starting with `ontree-`** will be stopped and removed
- **ALL associated container images** will be deleted
- **Container images will need to be re-downloaded** for testing scenarios
- Docker image cache will be pruned

**This is a COMPLETE removal of demo mode data and containers!**

**To confirm this destructive operation, please type "yes" when prompted in Claude Code.**

If the user confirms with "yes", proceed with deep cleanup. Otherwise, abort the operation.

## Cleanup Process

### Step 1: Get User Confirmation
Ask the user to confirm by typing "yes" to proceed with COMPLETE deletion.

### Step 2: Execute Deep Cleanup (only if confirmed)
If the user types "yes", execute the deep cleanup script:

!./.claude/commands/treeos-clean-demo-deep-noconfirm.sh

### Step 3: Report Results
After successful cleanup:
!echo "Demo mode deep cleanup complete! All data, containers, and images have been removed."

If the user doesn't confirm, respond:
!echo "Deep cleanup cancelled. No data was deleted."

## Post-Cleanup

After deep cleanup:
- Run TreeOS with `TREEOS_RUN_MODE=demo` to start fresh
- All container images will be re-downloaded on first use

**CRITICAL REMINDERS**:
- ALWAYS get explicit "yes" confirmation from the user before executing deep cleanup
- Use the non-interactive script `.claude/commands/treeos-clean-demo-deep-noconfirm.sh` after confirmation
- This removes EVERYTHING - complete data loss for demo mode
- Container images will be re-downloaded (important for testing download scenarios)
- DO NOT use `rm -rf` commands manually