---
sidebar_position: 2
---

# Template Syntax Checking

TreeOS includes a template syntax checker to catch errors before runtime. This prevents broken templates from being deployed and ensures a consistent user experience.

## Overview

The template checker validates:
- All HTML templates against the base template
- Template syntax (proper opening/closing of blocks)
- Component templates for standalone validity

Template checking is enforced in CI to prevent broken templates from being merged.

## Running Template Checks

### Check All Templates

```bash
# Check all templates
make check-templates
```

### Automatic Checking During Build

Template checking runs automatically during the build process:

```bash
# Build (includes template checking)
make build
```

If templates have syntax errors, the build will fail with a detailed error message indicating which template has issues.

## How It Works

The template checker:

1. **Loads the base template** - Parses `templates/base.html` as the foundation
2. **Validates each template** - Checks each template file against the base
3. **Verifies syntax** - Ensures proper Go template syntax (e.g., `{{define}}`, `{{block}}`, `{{end}}`)
4. **Checks components** - Validates standalone component templates

## Common Template Errors

### Unclosed Blocks

```html
<!-- ❌ Bad: Missing {{end}} -->
{{define "content"}}
  <div>Content here</div>
<!-- Missing {{end}} -->

<!-- ✅ Good -->
{{define "content"}}
  <div>Content here</div>
{{end}}
```

### Invalid Template Actions

```html
<!-- ❌ Bad: Invalid action syntax -->
{{define "content"}}
  <div>{{ .InvalidField }</div>
{{end}}

<!-- ✅ Good -->
{{define "content"}}
  <div>{{.Data.Field}}</div>
{{end}}
```

### Missing Base Template Blocks

```html
<!-- ❌ Bad: Referencing undefined block -->
{{define "content"}}
  {{template "nonexistent-block" .}}
{{end}}

<!-- ✅ Good: Only use blocks defined in base.html -->
{{define "content"}}
  {{template "header" .}}
{{end}}
```

## CI Integration

Template checking is part of the CI pipeline:

```yaml
- name: Check template syntax
  run: make check-templates

- name: Prepare embedded assets
  run: make embed-assets
```

The CI workflow will fail if any template has syntax errors, preventing broken code from being merged.

## Template Structure

TreeOS templates follow this structure:

```
templates/
├── base.html           # Base template with common layout
├── pages/              # Full page templates
│   ├── dashboard.html
│   ├── apps.html
│   └── ...
└── components/         # Reusable components
    ├── header.html
    ├── footer.html
    └── ...
```

## Development Workflow

1. **Edit templates** - Make your changes to templates
2. **Check syntax** - Run `make check-templates`
3. **Test locally** - Run the app and verify the changes
4. **Build** - Run `make build` (includes template checking)
5. **Commit** - The pre-commit hook will run template checks

## Troubleshooting

### Template Check Fails

If the template checker fails:

1. **Read the error message** - It will indicate which template has issues
2. **Check the line number** - The error message includes the approximate location
3. **Verify syntax** - Ensure all template blocks are properly closed
4. **Test incrementally** - Comment out sections to isolate the problem

### Pre-commit Hook Blocks Commit

The pre-commit hook runs template checks automatically. If it fails:

```bash
# Fix the template syntax errors
# Then run the check manually
make check-templates

# Once it passes, commit again
git commit -m "Your commit message"
```

Never use `--no-verify` to bypass the pre-commit hook unless absolutely necessary and approved.

## Related

- [Logging](./logging.md) - Development logging system
- [Configuration Reference](../reference/configuration.md) - Configuration options
