---
description: Quick check if code will pass CI
---

# Quick CI Check

Running all CI checks locally to ensure your code will pass:

## 1. Check Git Status
!git status

## 2. Run Linter
!make lint 2>&1 | tail -10

## 3. Run Tests
!make test 2>&1 | grep -E "(FAIL|PASS|ok|^\\?)" | tail -20

## 4. Summary

Based on the above results:
- If linting passes ✅
- If all tests pass ✅
- Then CI should pass! 🎉

If any issues were found, use `/fix-ci` to automatically fix them.