---
description: Create a new TreeOS release with proper semantic versioning
argument-hint: <help|beta|stable|patch-beta|minor-beta|major-beta|patch|minor|major>
---

# TreeOS Release Version Management

## Parse arguments
RELEASE_TYPE="${ARGUMENTS:-help}"

## Handle help request
if [ "$RELEASE_TYPE" = "help" ] || [ -z "$RELEASE_TYPE" ]; then
    echo "📦 TreeOS Release Version Management - Help"
    echo ""
    echo "USAGE:"
    echo "  /treeos-release-version <type>"
    echo ""
    echo "RELEASE TYPES:"
    echo "  help        Show this help message"
    echo ""
    echo "Beta Management:"
    echo "  beta        Increment beta number"
    echo "              v1.0.0-beta.13 → v1.0.0-beta.14"
    echo ""
    echo "  stable      Graduate from beta to stable (alias: release)"
    echo "              v1.0.0-beta.13 → v1.0.0"
    echo ""
    echo "Start New Beta Series:"
    echo "  patch-beta  Start beta for next patch version"
    echo "              v1.0.0 → v1.0.1-beta.01"
    echo "              v1.0.0-beta.13 → v1.0.1-beta.01"
    echo ""
    echo "  minor-beta  Start beta for next minor version"
    echo "              v1.0.0 → v1.1.0-beta.01"
    echo "              v1.0.0-beta.13 → v1.1.0-beta.01"
    echo ""
    echo "  major-beta  Start beta for next major version"
    echo "              v1.0.0 → v2.0.0-beta.01"
    echo "              v1.0.0-beta.13 → v2.0.0-beta.01"
    echo ""
    echo "Direct Releases (skip beta):"
    echo "  patch       Direct patch release"
    echo "              v1.0.0 → v1.0.1"
    echo ""
    echo "  minor       Direct minor release"
    echo "              v1.0.0 → v1.1.0"
    echo ""
    echo "  major       Direct major release"
    echo "              v1.0.0 → v2.0.0"
    echo ""

    # Get current version
    CURRENT_VERSION=$(git describe --tags --abbrev=0 2>/dev/null || echo "No tags found")
    echo "CURRENT VERSION: $CURRENT_VERSION"
    echo ""

    if [ "$CURRENT_VERSION" != "No tags found" ]; then
        echo "EXAMPLES based on current version:"
        echo "  /treeos-release-version beta         # Next beta"
        echo "  /treeos-release-version stable       # Graduate to stable"
        echo "  /treeos-release-version minor-beta   # Start next minor beta series"
    fi
    echo ""
    echo "NOTES:"
    echo "  - Beta versions use two-digit format (v1.0.0-beta.01, v1.0.0-beta.10)"
    echo "  - Beta versions are pre-releases (v1.0.0-beta.01 < v1.0.0)"
    echo "  - Always commit your changes before creating a release"
    echo "  - Tags are immediately pushed to GitHub after confirmation"
    exit 0
fi

## Validate release type
VALID_TYPES="beta stable release patch-beta minor-beta major-beta patch minor major"
if ! echo "$VALID_TYPES" | grep -wq "$RELEASE_TYPE"; then
    echo "❌ Invalid release type: '$RELEASE_TYPE'"
    echo ""
    echo "Valid types are: beta, stable, patch-beta, minor-beta, major-beta, patch, minor, major"
    echo "Use '/treeos-release-version help' for detailed information"
    exit 1
fi

## Convert 'release' alias to 'stable'
if [ "$RELEASE_TYPE" = "release" ]; then
    RELEASE_TYPE="stable"
fi

## Check git status
echo "🔍 Checking git status..."

# Check if we're in a git repository
if ! git rev-parse --git-dir > /dev/null 2>&1; then
    echo "❌ Not in a git repository"
    exit 1
fi

# Check for uncommitted changes
if ! git diff-index --quiet HEAD -- 2>/dev/null; then
    echo "⚠️  Warning: You have uncommitted changes"
    echo "It's recommended to commit all changes before creating a release"
    echo ""
    !git status --short
    echo ""
    echo "Do you want to continue anyway? (yes/no)"
    read -r CONTINUE
    if [ "$CONTINUE" != "yes" ]; then
        echo "Aborted."
        exit 0
    fi
fi

# Check current branch
CURRENT_BRANCH=$(git branch --show-current)
if [ "$CURRENT_BRANCH" != "main" ] && [ "$CURRENT_BRANCH" != "master" ]; then
    echo "⚠️  Warning: You are on branch '$CURRENT_BRANCH', not on main/master"
    echo "Releases are typically created from the main branch"
    echo ""
    echo "Do you want to continue anyway? (yes/no)"
    read -r CONTINUE
    if [ "$CONTINUE" != "yes" ]; then
        echo "Aborted."
        exit 0
    fi
fi

## Fetch latest tags
echo "📥 Fetching latest tags from remote..."
!git fetch --tags --quiet

## Get current version
CURRENT_VERSION=$(git describe --tags --abbrev=0 2>/dev/null || echo "v0.0.0")
echo "Current version: $CURRENT_VERSION"

## Calculate next version
echo "🔢 Calculating next version..."
SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
NEXT_VERSION=$("$SCRIPT_DIR/treeos-next-version-noconfirm.sh" "$RELEASE_TYPE")

if [ $? -ne 0 ] || [ -z "$NEXT_VERSION" ]; then
    echo "❌ Failed to calculate next version"
    echo "Error: $NEXT_VERSION"
    exit 1
fi

## Display release information
echo ""
echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
echo "📦 TreeOS Release Version Management"
echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
echo ""
echo "Release Type:     $RELEASE_TYPE"
echo "Current Version:  $CURRENT_VERSION"
echo "New Version:      $NEXT_VERSION"
echo ""
echo "This will:"
echo "  1. Create git tag: $NEXT_VERSION"
echo "  2. Push tag to origin"
echo "  3. Trigger GitHub Actions release workflow"
echo ""
echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
echo ""
echo "Do you want to proceed? (yes/no)"
read -r CONFIRM

if [ "$CONFIRM" != "yes" ]; then
    echo "❌ Release cancelled"
    exit 0
fi

## Create and push release
echo ""
echo "🚀 Creating release..."
"$SCRIPT_DIR/treeos-release-version-noconfirm.sh" "$RELEASE_TYPE"

if [ $? -eq 0 ]; then
    echo ""
    echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
    echo "✅ Release $NEXT_VERSION created successfully!"
    echo ""
    echo "Next steps:"
    echo "  1. Check GitHub Actions: https://github.com/stefanmunz/treeos/actions"
    echo "  2. View release: https://github.com/stefanmunz/treeos/releases/tag/$NEXT_VERSION"
    echo "  3. Edit release notes if needed"
    echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
else
    echo "❌ Failed to create release"
    exit 1
fi