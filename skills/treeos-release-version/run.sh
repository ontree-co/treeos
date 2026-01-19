#!/usr/bin/env bash
# TreeOS Release Version Helper
# Interactive release flow with visible options and confirmation

set -euo pipefail

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m'

print_error() {
  echo -e "${RED}$1${NC}" >&2
}

print_warn() {
  echo -e "${YELLOW}$1${NC}"
}

print_ok() {
  echo -e "${GREEN}$1${NC}"
}

compute_next_version() {
  local release_type="$1"
  local current_version="$2"

  local version="${current_version#v}"
  if [[ ! "$version" =~ ^([0-9]+)\.([0-9]+)\.([0-9]+)(-beta\.([0-9]+))?$ ]]; then
    print_error "Error: Unable to parse version '$current_version'"
    return 1
  fi

  local major="${BASH_REMATCH[1]}"
  local minor="${BASH_REMATCH[2]}"
  local patch="${BASH_REMATCH[3]}"
  local beta_suffix="${BASH_REMATCH[4]}"
  local beta_num="${BASH_REMATCH[5]}"

  local is_beta=false
  if [ -n "$beta_suffix" ]; then
    is_beta=true
  fi

  local next_version=""
  case "$release_type" in
    beta)
      if [ "$is_beta" = true ]; then
        next_version="v${major}.${minor}.${patch}-beta.$((beta_num + 1))"
      else
        print_error "Error: Cannot increment beta on stable version '$current_version'"
        return 1
      fi
      ;;
    stable|release)
      if [ "$is_beta" = true ]; then
        next_version="v${major}.${minor}.${patch}"
      else
        print_error "Error: Already on stable version '$current_version'"
        return 1
      fi
      ;;
    patch-beta)
      next_version="v${major}.${minor}.$((patch + 1))-beta.1"
      ;;
    minor-beta)
      next_version="v${major}.$((minor + 1)).0-beta.1"
      ;;
    major-beta)
      next_version="v$((major + 1)).0.0-beta.1"
      ;;
    patch)
      next_version="v${major}.${minor}.$((patch + 1))"
      ;;
    minor)
      next_version="v${major}.$((minor + 1)).0"
      ;;
    major)
      next_version="v$((major + 1)).0.0"
      ;;
    *)
      print_error "Error: Invalid release type '$release_type'"
      return 1
      ;;
  esac

  echo "$next_version"
}

if ! git rev-parse --git-dir > /dev/null 2>&1; then
  print_error "Not in a git repository"
  exit 1
fi

echo "🔍 Checking git status..."
if ! git diff-index --quiet HEAD -- 2>/dev/null; then
  print_warn "⚠️  Warning: You have uncommitted changes"
  echo "It's recommended to commit all changes before creating a release"
  echo ""
  git status --short
  echo ""
  echo "Do you want to continue anyway? (yes/no)"
  read -r continue_choice
  if [ "$continue_choice" != "yes" ]; then
    echo "Aborted."
    exit 0
  fi
fi

current_branch=$(git branch --show-current)
if [ "$current_branch" != "main" ] && [ "$current_branch" != "master" ]; then
  print_warn "⚠️  Warning: You are on branch '$current_branch', not on main/master"
  echo "Releases are typically created from the main branch"
  echo ""
  echo "Do you want to continue anyway? (yes/no)"
  read -r continue_choice
  if [ "$continue_choice" != "yes" ]; then
    echo "Aborted."
    exit 0
  fi
fi

echo "📥 Fetching latest tags from remote..."
git fetch --tags --quiet

current_version=$(git describe --tags --abbrev=0 2>/dev/null || echo "v0.0.0")

echo ""
echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
echo "📦 TreeOS Release Version"
echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
echo ""
echo "Current version: $current_version"
echo ""
echo "Available release options:"

release_types=(beta stable patch-beta minor-beta major-beta patch minor major)
option_types=()
option_versions=()

for release_type in "${release_types[@]}"; do
  if next_version=$(compute_next_version "$release_type" "$current_version" 2>/dev/null); then
    option_types+=("$release_type")
    option_versions+=("$next_version")
  fi
done

if [ "${#option_types[@]}" -eq 0 ]; then
  print_error "No valid release options found for $current_version"
  exit 1
fi

for i in "${!option_types[@]}"; do
  index=$((i + 1))
  printf "  %d) %-11s -> %s\n" "$index" "${option_types[$i]}" "${option_versions[$i]}"
done

echo ""
echo "Do you want to create a release now? (yes/no)"
read -r release_choice
if [ "$release_choice" != "yes" ]; then
  echo "Release cancelled."
  exit 0
fi

echo ""
echo "Select an option number:"
read -r option_number

if ! [[ "$option_number" =~ ^[0-9]+$ ]]; then
  print_error "Invalid selection"
  exit 1
fi

selected_index=$((option_number - 1))
if [ "$selected_index" -lt 0 ] || [ "$selected_index" -ge "${#option_types[@]}" ]; then
  print_error "Selection out of range"
  exit 1
fi

release_type="${option_types[$selected_index]}"
next_version="${option_versions[$selected_index]}"

echo ""
echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
echo "Release Type:     $release_type"
echo "Current Version:  $current_version"
echo "New Version:      $next_version"
echo ""
echo "This will:"
echo "  1. Create git tag: $next_version"
echo "  2. Push tag to origin"
echo "  3. Trigger GitHub Actions release workflow"
echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
echo ""
echo "Do you want to proceed? (yes/no)"
read -r confirm_choice
if [ "$confirm_choice" != "yes" ]; then
  echo "Release cancelled."
  exit 0
fi

if git rev-parse "$next_version" >/dev/null 2>&1; then
  print_error "Error: Tag $next_version already exists locally"
  exit 1
fi

if git ls-remote --tags origin | grep -q "refs/tags/$next_version"; then
  print_error "Error: Tag $next_version already exists on remote"
  exit 1
fi

commit_hash=$(git rev-parse --short HEAD)
commit_message=$(git log -1 --pretty=%B)

echo ""
echo "🚀 Creating release..."
tag_message="Release $next_version

Release type: $release_type
Based on commit: $commit_hash

Last commit message:
$commit_message"

git tag -a "$next_version" -m "$tag_message"
print_ok "✓ Tag $next_version created locally"

echo "Pushing tag to origin..."
git push origin "$next_version"
print_ok "✓ Tag $next_version pushed to origin"

echo ""
echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
print_ok "✅ Release $next_version created successfully!"
echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
echo ""
echo "Next steps:"
echo "  1. Check GitHub Actions: https://github.com/ontree-co/treeos/actions"
echo "  2. View release: https://github.com/ontree-co/treeos/releases/tag/$next_version"
echo "  3. Edit release notes if needed"

if [ -n "$current_branch" ]; then
  unpushed=$(git rev-list --count origin/"$current_branch".."$current_branch" 2>/dev/null || echo "0")
  if [ "$unpushed" -gt 0 ]; then
    print_warn "Note: You have $unpushed unpushed commit(s) on branch '$current_branch'"
    echo "Consider pushing your branch: git push origin $current_branch"
  fi
fi
