---
description: Clean all TreeOS demo mode data
argument-hint:
---

# Clean TreeOS Demo Mode Data

## ⚠️ CONFIRMATION REQUIRED

This command will **permanently delete** all TreeOS demo mode data including:
- `./apps/` directory (application configurations)
- `./shared/` directory (includes Ollama models)
- `./logs/` directory (log files)
- `./ontree.db` and related SQLite files (database)

**To confirm deletion, please type "yes" when prompted in Claude Code.**

If the user confirms with "yes", proceed with cleanup. Otherwise, abort the operation.

## Cleanup Process

### Step 1: Get User Confirmation
Ask the user to confirm by typing "yes" to proceed with deletion.

### Step 2: Execute Cleanup (only if confirmed)
If the user types "yes", execute the non-interactive cleanup script:

!./.claude/commands/clean-demo-noconfirm.sh

### Step 3: Report Results
After successful cleanup:
!echo "Demo mode data has been successfully cleaned. To start fresh, run TreeOS with TREEOS_RUN_MODE=demo"

If the user doesn't confirm, respond:
!echo "Cleanup cancelled. No data was deleted."

**IMPORTANT REMINDERS**:
- ALWAYS get explicit "yes" confirmation from the user before executing cleanup
- Use the non-interactive script `.claude/commands/clean-demo-noconfirm.sh` after confirmation
- DO NOT use `rm -rf` commands manually
- The script handles all necessary cleanup operations safely