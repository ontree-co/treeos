---
description: Fix failing tests, especially environment-dependent ones
argument-hint: [test-name-pattern]
---

# Fix Failing Tests

$ARGUMENTS

## 1. Identify Failing Tests

Running tests to find failures:
!go test ./... -v 2>&1 | grep -E "FAIL|Error|panic" | head -20

## 2. Common Test Issues to Fix

### Environment Dependencies
- Tests looking for `/opt/ontree/apps` → Use `t.TempDir()` instead
- Tests requiring Docker → Add skip condition: `t.Skip("Docker not available")`
- Tests requiring specific services → Mock or stub them

### Go Version Compatibility
- `container.Summary` not available in Go 1.21/1.22 → Use `types.Container`
- Add linter exclusions for deprecation warnings if needed

### Mock/Stub Issues
- Unused mock types → Remove them
- Interface mismatches → Update mock implementations

## 3. Run Specific Test (if pattern provided)

If you provided a test pattern, I'll run just that test:
!go test ./... -run "$1" -v

## 4. Fix and Verify

After fixing issues, verify all tests pass:
!make test

Common fixes applied:
- Replace system paths with temporary directories
- Add proper error handling in tests
- Fix timing-sensitive tests with proper synchronization
- Update deprecated API usage