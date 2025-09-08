# Claude Code Custom Commands

This directory contains custom slash commands for Claude Code to help with CI/CD and development workflows.

## Available Commands

### `/make-ci-happy` 🚀
**Purpose**: One-command solution to fix all CI issues and get a green build

**What it does**:
1. Runs linting and tests locally
2. Automatically fixes common issues
3. Commits and pushes if everything passes
4. Monitors the GitHub Actions workflow
5. Reports success or failure with actionable feedback

**Usage**: 
- `/make-ci-happy` - Uses default commit message
- `/make-ci-happy "fix: specific issue"` - Uses custom commit message

**Auto-fixes**:
- Linting errors (errcheck, deprecated APIs, file permissions)
- Test environment dependencies (temp dirs instead of system paths)
- Go version compatibility issues
- Unused code and mock types

### `/fix-ci`
**Purpose**: Fix CI issues without pushing

**Usage**: `/fix-ci` or `/fix-ci "commit message"`

### `/check-ci`
**Purpose**: Quick local CI status check

**Usage**: `/check-ci`

### `/fix-tests`
**Purpose**: Focus specifically on fixing test failures

**Usage**: `/fix-tests` or `/fix-tests TestName`

## Requirements

- Go 1.21+ installed
- Make installed
- Git configured with push access
- (Optional) GitHub CLI (`gh`) for workflow monitoring

## Installation

These commands are automatically available in Claude Code when working in this repository. No installation needed!

## Contributing

To add a new command:
1. Create a new `.md` file in `.claude/commands/`
2. The filename becomes the command name (without .md)
3. Add frontmatter for description and argument hints
4. Use `!` prefix for bash commands
5. Use `$ARGUMENTS` or `$1`, `$2` for arguments

Example:
```markdown
---
description: Your command description
argument-hint: [optional-args]
---

# Command content here
!echo "Bash command"
$ARGUMENTS
```