#!/bin/bash
# TreeOS Release Version Creator
# Creates and pushes a new git tag based on release type
# Usage: ./treeos-release-version-noconfirm.sh <beta|stable|patch-beta|minor-beta|major-beta|patch|minor|major>

set -e

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

# Get release type from argument
RELEASE_TYPE="${1:-}"

if [ -z "$RELEASE_TYPE" ]; then
    echo -e "${RED}Error: Release type required${NC}" >&2
    exit 1
fi

# Convert 'release' alias to 'stable'
if [ "$RELEASE_TYPE" = "release" ]; then
    RELEASE_TYPE="stable"
fi

# Get the script directory
SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"

# Calculate next version
echo "Calculating next version..."
NEXT_VERSION=$("$SCRIPT_DIR/treeos-next-version-noconfirm.sh" "$RELEASE_TYPE")

if [ $? -ne 0 ] || [ -z "$NEXT_VERSION" ]; then
    echo -e "${RED}Failed to calculate next version${NC}" >&2
    echo "$NEXT_VERSION" >&2
    exit 1
fi

# Check if tag already exists locally
if git rev-parse "$NEXT_VERSION" >/dev/null 2>&1; then
    echo -e "${RED}Error: Tag $NEXT_VERSION already exists locally${NC}" >&2
    exit 1
fi

# Check if tag exists on remote
if git ls-remote --tags origin | grep -q "refs/tags/$NEXT_VERSION"; then
    echo -e "${RED}Error: Tag $NEXT_VERSION already exists on remote${NC}" >&2
    exit 1
fi

# Get current commit hash for the tag message
COMMIT_HASH=$(git rev-parse --short HEAD)
COMMIT_MESSAGE=$(git log -1 --pretty=%B)

# Create annotated tag with meaningful message
echo "Creating tag $NEXT_VERSION..."
TAG_MESSAGE="Release $NEXT_VERSION

Release type: $RELEASE_TYPE
Based on commit: $COMMIT_HASH

Last commit message:
$COMMIT_MESSAGE"

git tag -a "$NEXT_VERSION" -m "$TAG_MESSAGE"

if [ $? -ne 0 ]; then
    echo -e "${RED}Failed to create tag${NC}" >&2
    exit 1
fi

echo -e "${GREEN}✓ Tag $NEXT_VERSION created locally${NC}"

# Push the tag to origin
echo "Pushing tag to origin..."
git push origin "$NEXT_VERSION"

if [ $? -ne 0 ]; then
    echo -e "${RED}Failed to push tag to origin${NC}" >&2
    echo "The tag was created locally but not pushed."
    echo "You can try pushing manually with: git push origin $NEXT_VERSION"
    exit 1
fi

echo -e "${GREEN}✓ Tag $NEXT_VERSION pushed to origin${NC}"

# Success message
echo ""
echo -e "${GREEN}━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━${NC}"
echo -e "${GREEN}✅ Release $NEXT_VERSION created successfully!${NC}"
echo -e "${GREEN}━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━${NC}"
echo ""
echo "The GitHub Actions workflow should now be triggered automatically."
echo "Check the progress at: https://github.com/ontree-co/treeos/actions"
echo ""

# Check if we should also push the current branch
CURRENT_BRANCH=$(git branch --show-current)
if [ -n "$CURRENT_BRANCH" ]; then
    # Check if there are unpushed commits
    UNPUSHED=$(git rev-list --count origin/"$CURRENT_BRANCH".."$CURRENT_BRANCH" 2>/dev/null || echo "0")
    if [ "$UNPUSHED" -gt 0 ]; then
        echo -e "${YELLOW}Note: You have $UNPUSHED unpushed commit(s) on branch '$CURRENT_BRANCH'${NC}"
        echo "Consider pushing your branch: git push origin $CURRENT_BRANCH"
    fi
fi