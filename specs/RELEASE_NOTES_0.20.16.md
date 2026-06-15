# skillshare v0.20.16 Release Notes

## TL;DR

1. **Repository subdir installs are safer** — Skillshare now rejects repository subdirectories that try to escape the repo with traversal segments, backslashes, absolute paths, control characters, or encoded traversal.
2. **Blob-style `SKILL.md` URLs are checked before trimming** — GitHub, GitLab, and Bitbucket skill-file URLs now keep suspicious path segments visible to validation instead of silently cleaning them.
3. **Metadata stays readable after saves** — `.metadata.json` is written with repository-friendly permissions so Git and other tools can read it after install or update operations.

## Bug fix: repository subdir installs reject traversal paths

Source parsing now enforces that repository subdirectories stay inside the repository. Inputs that contain traversal segments or encoded traversal are rejected before install and download flows use them:

```bash
skillshare install github.com/owner/repo/../../etc/passwd
# rejected: unsafe repository subdir

skillshare install github.com/owner/repo/skills/frontend
# still accepted
```

This also covers source formats beyond GitHub shorthand, including SSH URLs, `file://` repos, Azure DevOps sources, generic HTTPS Git URLs, and blob-style `SKILL.md` paths from GitHub, GitLab, and Bitbucket.

Refs: #224.

## Bug fix: metadata files stay readable

Skillshare now writes `.metadata.json` with `0644` permissions when saving metadata. This keeps metadata readable by Git and other tooling after install or update operations replace the file atomically.
