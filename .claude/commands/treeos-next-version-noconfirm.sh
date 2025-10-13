#!/bin/bash
# TreeOS Next Version Calculator
# Calculates the next version based on release type
# Usage: ./treeos-next-version-noconfirm.sh <beta|stable|patch-beta|minor-beta|major-beta|patch|minor|major>

set -e

# Get release type from argument
RELEASE_TYPE="${1:-}"

if [ -z "$RELEASE_TYPE" ]; then
    echo "Error: Release type required" >&2
    exit 1
fi

# Get the latest tag
CURRENT_VERSION=$(git describe --tags --abbrev=0 2>/dev/null || echo "v0.0.0")

# Remove 'v' prefix for processing
VERSION="${CURRENT_VERSION#v}"

# Parse version components
# Handle both v1.0.0 and v1.0.0-beta.01 formats
if [[ "$VERSION" =~ ^([0-9]+)\.([0-9]+)\.([0-9]+)(-beta\.([0-9]+))?$ ]]; then
    MAJOR="${BASH_REMATCH[1]}"
    MINOR="${BASH_REMATCH[2]}"
    PATCH="${BASH_REMATCH[3]}"
    BETA_SUFFIX="${BASH_REMATCH[4]}"
    BETA_NUM="${BASH_REMATCH[5]}"
else
    echo "Error: Unable to parse version '$CURRENT_VERSION'" >&2
    exit 1
fi

# Determine if we're currently on a beta version
IS_BETA=false
if [ -n "$BETA_SUFFIX" ]; then
    IS_BETA=true
fi

# Calculate next version based on release type
case "$RELEASE_TYPE" in
    beta)
        if [ "$IS_BETA" = true ]; then
            # Increment beta number (zero-padded to 2 digits)
            NEXT_BETA=$((10#$BETA_NUM + 1))
            NEXT_VERSION="v${MAJOR}.${MINOR}.${PATCH}-beta.$(printf '%02d' $NEXT_BETA)"
        else
            echo "Error: Cannot increment beta on stable version '$CURRENT_VERSION'" >&2
            echo "Use 'patch-beta', 'minor-beta', or 'major-beta' to start a new beta series" >&2
            exit 1
        fi
        ;;

    stable|release)
        if [ "$IS_BETA" = true ]; then
            # Remove beta suffix to create stable release
            NEXT_VERSION="v${MAJOR}.${MINOR}.${PATCH}"
        else
            echo "Error: Already on stable version '$CURRENT_VERSION'" >&2
            echo "Use 'patch', 'minor', or 'major' for a new stable release" >&2
            echo "Or use 'patch-beta', 'minor-beta', or 'major-beta' to start a beta series" >&2
            exit 1
        fi
        ;;

    patch-beta)
        if [ "$IS_BETA" = true ]; then
            # Skip current beta, go to next patch beta
            NEXT_PATCH=$((PATCH + 1))
            NEXT_VERSION="v${MAJOR}.${MINOR}.${NEXT_PATCH}-beta.01"
        else
            # Start beta for next patch
            NEXT_PATCH=$((PATCH + 1))
            NEXT_VERSION="v${MAJOR}.${MINOR}.${NEXT_PATCH}-beta.01"
        fi
        ;;

    minor-beta)
        if [ "$IS_BETA" = true ]; then
            # Skip current beta, go to next minor beta
            NEXT_MINOR=$((MINOR + 1))
            NEXT_VERSION="v${MAJOR}.${NEXT_MINOR}.0-beta.01"
        else
            # Start beta for next minor
            NEXT_MINOR=$((MINOR + 1))
            NEXT_VERSION="v${MAJOR}.${NEXT_MINOR}.0-beta.01"
        fi
        ;;

    major-beta)
        if [ "$IS_BETA" = true ]; then
            # Skip current beta, go to next major beta
            NEXT_MAJOR=$((MAJOR + 1))
            NEXT_VERSION="v${NEXT_MAJOR}.0.0-beta.01"
        else
            # Start beta for next major
            NEXT_MAJOR=$((MAJOR + 1))
            NEXT_VERSION="v${NEXT_MAJOR}.0.0-beta.01"
        fi
        ;;

    patch)
        if [ "$IS_BETA" = true ]; then
            # Skip current beta, go directly to next patch stable
            NEXT_PATCH=$((PATCH + 1))
            NEXT_VERSION="v${MAJOR}.${MINOR}.${NEXT_PATCH}"
        else
            # Direct patch release
            NEXT_PATCH=$((PATCH + 1))
            NEXT_VERSION="v${MAJOR}.${MINOR}.${NEXT_PATCH}"
        fi
        ;;

    minor)
        if [ "$IS_BETA" = true ]; then
            # Skip current beta, go directly to next minor stable
            NEXT_MINOR=$((MINOR + 1))
            NEXT_VERSION="v${MAJOR}.${NEXT_MINOR}.0"
        else
            # Direct minor release
            NEXT_MINOR=$((MINOR + 1))
            NEXT_VERSION="v${MAJOR}.${NEXT_MINOR}.0"
        fi
        ;;

    major)
        if [ "$IS_BETA" = true ]; then
            # Skip current beta, go directly to next major stable
            NEXT_MAJOR=$((MAJOR + 1))
            NEXT_VERSION="v${NEXT_MAJOR}.0.0"
        else
            # Direct major release
            NEXT_MAJOR=$((MAJOR + 1))
            NEXT_VERSION="v${NEXT_MAJOR}.0.0"
        fi
        ;;

    *)
        echo "Error: Unknown release type '$RELEASE_TYPE'" >&2
        echo "Valid types: beta, stable, patch-beta, minor-beta, major-beta, patch, minor, major" >&2
        exit 1
        ;;
esac

# Output the next version
echo "$NEXT_VERSION"
