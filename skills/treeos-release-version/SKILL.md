---
name: treeos-release-version
description: Interactive TreeOS release helper that shows current version, available bump options, and can create/push a release tag.
---

# TreeOS Release Version

Show current version and available release options, then perform the selected release after confirmation.

## Run

!./skills/treeos-release-version/run.sh

## Notes

- This script is interactive and will ask for confirmation before creating and pushing tags.
- It checks git status and warns if there are uncommitted changes or if you're not on main/master.
