# Feature Proposal: Visible `skillshare/` Project Directory

## Problem

Project mode is identified solely by `.skillshare/config.yaml`, and the project's operational state is anchored to that same hidden directory.

`sources` (proposal [153](153-custom-project-sources.md), shipped in v0.19.15) solved this for *content*. The motivation accepted there was that some repositories want project skills to be "reviewed, discovered, and maintained alongside normal contributor documentation instead of being tied to a tool-specific hidden directory".

For repositories where skills are first-class reviewable content rather than tool state, the same argument applies to the marker and to `config.yaml` itself. Those repositories currently end up with skills in a visible directory *plus* a hidden `.skillshare/` beside it whose only job is to hold `config.yaml` and operational state. `config.yaml` is arguably the file reviewers most need to see — it lists sync targets and the audit threshold — and it is the one file that cannot be moved.

Affected: repositories that commit their skills and treat them as contributor-facing documentation; the same audience `sources` was built for.

There is currently no flag, environment variable, or config key that relocates the marker, and a config key could not work anyway — the marker has to be found before any config is parsed.

## Proposed Solution

Recognize a visible `skillshare/` directory as an alternative project marker, resolved in a fixed order:

1. `.skillshare/config.yaml`
2. `skillshare/config.yaml`

First hit wins, so `.skillshare/` remains the default and takes precedence when both exist. Adding a visible directory can never silently move an existing project, and no migration is required.

A repository opting in looks like:

```text
my-repo/
├── skillshare/
│   ├── config.yaml
│   └── skills/
└── src/
```

### What changes

- **New leaf package** (e.g. `internal/projectdir`) exposing the recognized names, the resolution order, and helpers to find the active directory. It needs to import nothing else internal, because `internal/audit` cannot import `internal/config` (`config` → `install` → `audit` would cycle).
- **Detection**: `projectConfigExists` in `cmd/skillshare/mode.go` and `config.ProjectConfigPath` resolve through the package instead of joining a literal.
- **Operational state follows the active marker**: `internal/trash`, `internal/backup`, `internal/oplog` (`logs/`), and `audit-rules.yaml` in `internal/audit`, so config and state always sit together.
- **Gitignore management**: `ProjectGitignoreTarget` and `ReconcileProjectSkills` use the resolved directory.
- **`init -p --visible`** creates the visible layout. Default `init -p` behaviour and output are unchanged. `config.Save` gains a variant that writes into an explicit directory, since during init the directory is chosen rather than discovered.
- **Source defaults**: `EffectiveSkillsSource` / `EffectiveAgentsSource` / `EffectiveExtrasSource` fall back to the active marker. `sources` still wins, and still resolves from the project root rather than from the marker.

### Two details worth flagging

**The visible name collides with the global config directory.** `BaseDir()` is `<config-home>/skillshare`, so a directory literally named `skillshare` already means "global config". Two places infer scope from a config path by directory name — `oplog.LogDir` / `ensureProjectLogGitignore` and `isProjectLogConfig` in `cmd/skillshare/log.go`. Both must exclude the global location by comparing the full path against `BaseDir()`, not by name. Missing this silently routes global operation logs into the global config directory. The existing `TestLogDir_Global` passes a `/home/user/.config/skillshare/...` fixture without declaring a config home, so it needs `XDG_CONFIG_HOME` set to stay meaningful.

**An unrelated directory named `skillshare/` must not be adopted.** The partial-init repair path (a shared skills repo cloned after `--config local` gitignored the config) treats an existing project directory without a `config.yaml` as a project to repair. For the hidden name that is unambiguous; for the visible name the directory should additionally have to contain `skills/` or `agents/` before it is claimed.

`ProjectTargetDotDirs` is deliberately left alone. It documents itself as a set of *hidden* directory names to skip during discovery, and adding a non-hidden `skillshare` would skip legitimate content in any scanned repository.

No new runtime dependencies.

## Alternatives Considered

**Environment variable or config key naming the marker directory.** A config key cannot be read before the config is located. An environment variable makes project identity depend on shell state, so a repository could not describe its own layout to a collaborator or to CI.

**Upward search from the current directory.** Useful on its own, but a separate behaviour change with a different blast radius, and it does not address visibility. Out of scope here.

**Extending `sources` with a marker key.** Same bootstrapping problem as any config key.

**Symlink `skillshare` → `.skillshare`.** Works on some platforms, does not survive every checkout, and reintroduces exactly the hidden indirection the change is meant to remove.

**Do nothing and rely on `sources`.** This is the status quo. It relocates content but still forces a hidden directory to exist for `config.yaml` and state, which is the specific complaint.

## Scope

Estimate the scope of changes:

- [ ] Small (1-3 files, < 200 lines)
- [ ] Medium (3-10 files, 200-500 lines)
- [x] Large (10+ files, 500+ lines)

Large by file count, though each edit is small and mechanical. A working implementation touches 14 non-test files plus one new package, and 6 test files, at roughly 300 non-test lines added.

Expected areas:

- New project-directory resolution package and tests
- Project config path, source defaults, and gitignore target
- Trash, backup, oplog, audit rule paths
- `init -p` flag and project detection in `cmd/skillshare`
- One server handler that derives the extras path

## Open Questions

- **Directory name.** `skillshare/` reads naturally but collides with the global config directory name, as described above. If that collision is unwelcome, is there a preferred visible name?
- **Should `--visible` be the flag spelling?** `--dir <name>` would generalize, but that invites arbitrary names and a much larger detection surface. This proposal deliberately keeps exactly two recognized names.
- **`skillshare init -p --visible` only.** Should there be a supported way to convert an existing project, or is "move the directory yourself, it will be picked up" sufficient? With the marker recognized, a plain `mv` is already enough.
- **Related behaviour, not proposed here:** `status`, `sync`, `install`, `list`, `update`, and `target` call `performProjectInit` when no project config is found, while `check` and `audit` return an error. Before the marker is recognized, renaming `.skillshare/` to `skillshare/` therefore re-inits silently, writes a fresh empty config, orphans the existing skills, and exits `0`. Recognizing the marker removes that particular trap, and the auto-init path is intentional for the shared-repo flow, so this proposal does not change it — but the inconsistency may be worth a separate look.
