#!/usr/bin/env bash
set -euo pipefail

root_dir="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
skills_dir="$root_dir/skills"
codex_dir="${CODEX_HOME:-$HOME/.codex}/skills"

if [ ! -d "$skills_dir" ]; then
  echo "No skills directory found at $skills_dir" >&2
  exit 1
fi

mkdir -p "$codex_dir"

for skill_path in "$skills_dir"/*; do
  [ -d "$skill_path" ] || continue
  if [ ! -f "$skill_path/SKILL.md" ]; then
    echo "Skipping $(basename "$skill_path"): missing SKILL.md" >&2
    continue
  fi

  skill_name="$(basename "$skill_path")"
  link_path="$codex_dir/$skill_name"

  rm -f "$link_path"
  ln -s "$skill_path" "$link_path"
  echo "Linked $skill_name -> $skill_path"
done
