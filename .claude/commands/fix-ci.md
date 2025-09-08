---
description: Fix all CI issues (linting, tests, formatting)
argument-hint: [optional-commit-message]
---

# Fix CI Issues

First, let me check the current status and identify any CI problems:

!git status
!git diff --cached

Now I'll systematically fix all CI issues:

## 1. Run Linting and Fix Issues

First, run the linter to identify issues:
!make lint

If there are linting issues, I'll fix them:
- Check for unchecked errors and add proper error handling or exclusions
- Fix deprecated API usage (e.g., ioutil → os, types.Container → proper types)
- Fix security issues (file permissions should be 0600 not 0644)
- Remove unused code
- Fix code style issues (unnecessary nil checks, fmt.Sprintf usage, etc.)

## 2. Run Tests and Fix Failures

Run all tests to identify failures:
!make test

If there are test failures, I'll fix them:
- Check if tests depend on environment-specific paths (use temp directories instead)
- Ensure backward compatibility for different Go versions (1.21, 1.22, 1.23)
- Fix any mock/stub issues
- Update test assertions if API changes were made

## 3. Verify Template Syntax

Check that all templates are valid:
!make check-templates

## 4. Final Verification

Run all checks together:
!make lint && make test

## 5. Commit and Push (if requested)

$ARGUMENTS

If a commit message was provided, I'll commit and push the fixes:
- Stage all changes
- Commit with descriptive message about CI fixes
- Push to origin

Remember: ALWAYS run `make lint` and `make test` before pushing to avoid CI failures!