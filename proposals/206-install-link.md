# Feature Proposal: Install Local Path as Symlink (`--link`)

## Problem

`skillshare install <local-path>` copies the skill directory into the skillshare source tree. This works for standalone skills but breaks the expected workflow when a skill is **owned and updated by another application** that ships it as part of its own bundle.

**Concrete example — Surge for macOS:**

Surge (a macOS proxy app) ships an authoritative skill at:

```
/Applications/Surge.app/Contents/Resources/Skills/surge
```

After `skillshare install /Applications/Surge.app/Contents/Resources/Skills/surge`, skillshare writes a snapshot copy to `~/.config/skillshare/skills/surge/`. When Surge updates (e.g. new commands added to `command-reference.md`), the copy silently falls behind. There is no `skillshare update` path for this skill because the source is not a git repository. The only fix is to re-run `skillshare install --force`, which most users will not remember to do.

The current workaround — manually deleting the copy and symlinking the source directory — bypasses skillshare's metadata and requires editing the source directory directly, which CONTRIBUTING.md discourages.

## Proposed Solution

Add a `--link` flag to `skillshare install` for local path sources:

```bash
skillshare install --link /Applications/Surge.app/Contents/Resources/Skills/surge
```

Instead of copying the directory, skillshare places a **symlink** in the source tree:

```
~/.config/skillshare/skills/surge  →  /Applications/Surge.app/Contents/Resources/Skills/surge
```

The skill content always reflects the live path. No re-install is needed after app updates.

### CLI surface

```
skillshare install --link <local-path>
skillshare install -L <local-path>     # short form
```

`--link` is rejected for non-local sources (git URLs, GitHub shorthands) with a clear error message.

### Metadata

The install metadata record (used by `skillshare list`, `check`, `update`, `uninstall`) should store `link: true` alongside the existing `source:` path, so skillshare can distinguish linked skills from copied ones.

Proposed `~/.config/skillshare/config.yaml` representation (for persisted installs, if skillshare persists link metadata):

```yaml
# conceptual — exact schema TBD by maintainer
skills:
  surge:
    source: /Applications/Surge.app/Contents/Resources/Skills/surge
    link: true
    installed: 2026-06-07
```

### Behavior of adjacent commands

| Command | Linked skill behavior |
|---|---|
| `skillshare list` | Show `→ /path/to/target` instead of `(local)` to make the live link visible |
| `skillshare check` | Report "externally managed — link is live" when the target path exists |
| `skillshare update` | Skip with "externally managed" message (the link is always current) |
| `skillshare uninstall` | Remove the symlink; do **not** touch the target directory |
| `skillshare doctor` | Warn when a linked target no longer exists (broken symlink) |

### Sync behavior

`skillshare sync` should follow the symlink when propagating to targets, not copy the symlink itself. This matches how the current flow works today when users place symlinks manually in the source directory.

## Alternatives Considered

**Re-run `install --force` after each app update.** Fragile and easy to forget. Breaks the "install once" contract and produces silent drift.

**Manual symlink in source dir.** Works today but bypasses skillshare's tracking entirely: no metadata, no `list` indicator, no `doctor` check for broken links. The feature request is to give this pattern first-class support.

**`mode: link` in config.yaml.** Could complement or replace the CLI flag for users who want persistent config-driven link behavior. A config key would be more consistent with how `mode: copy` / `mode: merge` already work at the target level. This is worth discussing — the CLI flag is a simpler starting point.

**Watch-based auto-sync.** Heavier than needed. A symlink at install time is sufficient and adds zero runtime overhead.

## Scope

- [ ] Small (1-3 files, < 200 lines)
- [x] Medium (3-10 files, 200-500 lines)
- [ ] Large (10+ files, 500+ lines)

Expected touch points:

- `internal/install/source.go` — detect `--link` flag, validate local-path-only constraint
- `internal/install/install_apply.go` — branch on `link` flag: `os.Symlink` instead of `copyDir`
- `internal/install/metadata.go` — persist `link: true` in install record
- `cmd/skillshare/install.go` — wire `--link` / `-L` flag
- `cmd/skillshare/list.go` — display linked skills with `→ path` indicator
- `cmd/skillshare/update.go` — skip update for linked skills with a clear message
- `cmd/skillshare/doctor.go` — detect broken symlinks in source tree

Tests: unit tests for symlink creation path and broken-link detection; integration test for install → sync → uninstall round-trip on a linked skill.

## Open Questions

1. **Config persistence** — Should `--link` be a CLI-only flag per invocation, or should it be persisted in `config.yaml` (e.g. as `mode: link` at the skill level) so that `skillshare install` (no args, from config) recreates the symlink? A config-level key would make the intent declarative and reproducible across machines.

2. **Cross-device / Windows** — On Windows, directory symlinks require elevated permissions or Developer Mode. Should skillshare fall back to a junction point, emit a clear error, or document the limitation? (macOS and Linux have no restrictions.)

3. **`skillshare collect`** — If a user runs `collect` while a linked skill is installed, should skillshare copy the current contents into the source tree and break the link, or skip collecting the linked skill? Skipping with a warning seems safer.

4. **Name override** — Should `--link` support `--name <name>` the same way `install` does? This seems necessary for usability (app bundles may use names that differ from the desired skill name in skillshare).
