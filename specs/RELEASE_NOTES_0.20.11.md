# skillshare v0.20.11 Release Notes

## TL;DR

1. **Grouped tracked repository recovery is fixed** — `skillshare install` now rehydrates tracked repos installed with `--track --into <group>` at their original grouped path.

## Bug fix: grouped tracked repositories rehydrate correctly

v0.20.10 added missing tracked repository detection and the `skillshare install` recovery path for fresh clones. v0.20.11 fixes the grouped tracked repo case: if a repo was installed with `--track --into <group>`, rehydrate now restores the clone under the original group instead of applying the group twice.

```bash
skillshare install https://github.com/team/skills.git --track --into team

# On another machine or after removing the local clone:
skillshare install
skillshare sync
```

This keeps the cross-machine recovery flow working for both top-level tracked repos and grouped tracked repos.
