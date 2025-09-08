---
description: Automatically fix CI issues, push, and monitor GitHub Actions
argument-hint: [commit-message]
---

# Make CI Happy 🚀

I'll automatically fix all CI issues and monitor the GitHub Actions workflow until it passes.

## Step 1: Initial Status Check

!echo "🔍 Checking current git status..."
!git status --short
!git fetch origin

## Step 2: Run Local CI Checks

!echo "🧪 Running local linting..."
!make lint 2>&1

If linting fails, I'll fix common issues:
- Unchecked errors → Add proper error handling or exclusions in .golangci.yml
- Deprecated APIs (ioutil → os, types.Container → types.Container with exclusions)
- File permissions (0644 → 0600)
- Unused code removal
- Style issues (nil checks, fmt.Sprintf)

!echo "🧪 Running local tests..."
!make test 2>&1

If tests fail, I'll analyze and fix:
- **Minor fixes** (auto-fixable):
  - Environment dependencies → Use t.TempDir() instead of /opt/ontree/apps
  - Mock/stub issues → Remove unused types
  - Go version compatibility → Use backward-compatible types
  - Add skip conditions for Docker-dependent tests
  
- **Major issues** (need human review):
  - Logic errors in business code
  - Architectural changes needed
  - Security validation failures
  - API contract changes

## Step 3: Verify All Fixes

!echo "✅ Running final verification..."
!make lint && make test

## Step 4: Commit Changes

If everything passes locally, I'll commit the fixes:

!git add -A
!git diff --staged --stat

$ARGUMENTS

Creating commit with message: "${1:-fix: make CI happy - auto-fixed linting and test issues}"

!git commit -m "${1:-fix: make CI happy - auto-fixed linting and test issues

- Fixed linting issues (errcheck, deprecated APIs, permissions)
- Fixed environment-dependent tests
- Ensured Go 1.21/1.22/1.23 compatibility
- All tests and linting pass locally

🤖 Generated with [Claude Code](https://claude.ai/code)

Co-Authored-By: Claude <noreply@anthropic.com>"

## Step 5: Push to Origin

!echo "📤 Pushing to origin..."
!git push origin main

## Step 6: Monitor CI Pipeline

!echo "👀 Monitoring GitHub Actions workflow..."

# Check if GitHub CLI is installed
!which gh > /dev/null 2>&1 || echo "⚠️ GitHub CLI not installed. Install with: sudo apt install gh (Ubuntu) or brew install gh (Mac)"

If GitHub CLI is available, I'll monitor the workflow:

!if command -v gh &> /dev/null; then \
  echo "Getting latest workflow run..."; \
  gh run list --repo stefanmunz/ontree-node --limit 1 --json databaseId,status,conclusion,name,headBranch 2>/dev/null || echo "Note: gh auth login may be required"; \
  echo "Watching workflow (this will update every 10 seconds)..."; \
  timeout 300 gh run watch --repo stefanmunz/ontree-node --interval 10 2>/dev/null || true; \
  echo "Getting final status..."; \
  gh run list --repo stefanmunz/ontree-node --limit 1 --json status,conclusion 2>/dev/null | jq -r '.[0] | "Status: \(.status), Conclusion: \(.conclusion)"' || echo "Check https://github.com/stefanmunz/ontree-node/actions"; \
else \
  echo "📊 Check CI status manually at: https://github.com/stefanmunz/ontree-node/actions"; \
fi

## Step 7: Final Report

Based on the CI results:
- ✅ **SUCCESS**: CI is happy! All checks passed.
- ❌ **FAILURE**: CI still failing. Checking logs for details:

!if command -v gh &> /dev/null; then \
  gh run view --repo stefanmunz/ontree-node --log-failed 2>/dev/null || echo "View logs at: https://github.com/stefanmunz/ontree-node/actions"; \
else \
  echo "View failure details at: https://github.com/stefanmunz/ontree-node/actions"; \
fi

If CI fails after our fixes, it might be due to:
1. **Cache issues** - The CI cache might need clearing
2. **Environment differences** - CI has different dependencies/versions
3. **Permissions** - GitHub Actions might have different file permissions
4. **Network issues** - Temporary GitHub/dependency failures

## Summary of Changes Made

I've automatically:
1. Fixed all linting issues found
2. Fixed all test failures that could be auto-fixed
3. Verified everything passes locally
4. Committed and pushed the changes
5. Monitored the CI pipeline

**Human intervention needed for:**
- Major test logic changes
- Security policy violations
- Architectural refactoring
- API breaking changes