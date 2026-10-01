---
sidebar_position: 4
---

# Recipe: Git hooks with portable helpers

Manage Git config hooks across machines without embedding a tool's installation
path in every repository. Git 2.54+ is required; `parallel` needs 2.55+.

## Scenario

Roborev remaps reviews after a rewrite and flushes pending batches before a push.
Git appends event arguments, while Roborev's `post-commit` accepts no positional
arguments. Helpers keep that boundary explicit and resolve the executable from
`PATH` after upgrades.

## Solution

Save this Entry as `roborev.yaml`:

```yaml
description: Remap reviews after rewrites and flush batches before push
bindings:
  git:
    commands:
      roborev.post-rewrite:
        events: [post-rewrite]
        command: "{files}/post-rewrite"
      roborev.pre-push:
        events: [pre-push]
        command: "{files}/pre-push"
    files:
      post-rewrite: |
        #!/bin/sh
        PATH="$PATH:/opt/homebrew/bin:/home/linuxbrew/.linuxbrew/bin:/usr/local/bin"
        command -v roborev >/dev/null 2>&1 || exit 0
        exec roborev remap --quiet 2>/dev/null
      pre-push: |
        #!/bin/sh
        PATH="$PATH:/opt/homebrew/bin:/home/linuxbrew/.linuxbrew/bin:/usr/local/bin"
        command -v roborev >/dev/null 2>&1 || exit 0
        roborev post-commit --flush-push 2>/dev/null || true
```

These wrappers match Roborev 0.69 hook behavior. The rewrite helper inherits
Git's stdin directly. The push helper intentionally consumes no Git positional
arguments and does not block a push if flushing fails. Review flags after a
Roborev hook-template change.

```bash
skillshare hooks add roborev --file roborev.yaml -g --dry-run
skillshare hooks add roborev --file roborev.yaml -g
skillshare hooks sync -g --dry-run
skillshare hooks sync -g
```

The preview shows the generated include and executable helpers. Skillshare adds
an owned include to a regular writable global config. If your config is a
symlink, add the exact suggested include lines to the file you manage yourself,
then sync again. Those manual lines stay user-owned. A conditional include keeps
its conditions; Skillshare never broadens it.

Config hooks coexist with existing hooks-directory scripts. Inspect those
registrations and retire duplicate Roborev copies separately before relying on
this setup. This phase does not scan or recognize them automatically. Re-running
`roborev init` may reinstall hooks-directory copies; inspect again afterward.

## Verification

Inside a repository, inspect registration without executing hooks:

```bash
skillshare hooks list -g --json | jq .git
git hook list --show-scope post-rewrite
git hook list --show-scope pre-push
skillshare hooks sync -g --dry-run
```

Expect config-hook support, a present include, both friendly names at global
scope and unchanged outputs. Conditional includes may activate only in matching
repositories; presence alone is not proof. Git below 2.54 reports inactive.
Sync itself never executes hooks. The generated command paths refer to each
machine's own helper directory, and helpers resolve Roborev at runtime, so a
binary-path change alone does not require regenerated hook definitions.

## Variations

Put a custom post-commit cadence in a separate entry with a friendly name such
as `review.batch`, `events: [post-commit]`, and `command: "{files}/post-commit"`.
Keep its logic in the inline helper; ordering is entry name then friendly name.

Use `-p` to publish config hooks for one project, or declare an absolute root in
`hooks.projects` from global config. Linked worktrees share the common Git
config, so declare one root per common directory. No hooks-directory script is
created. The dashboard edits the Git binding as raw YAML.

```bash
skillshare hooks disable roborev --sync -g
skillshare hooks enable roborev --sync -g
skillshare hooks remove roborev --sync -g
skillshare hooks restore BACKUP_ID --dry-run -g
```

Removal deletes owned outputs and includes, preserving manual includes and
unrelated settings. Restore affects native outputs, not the source definition.
Git entries cannot use `--keep-files`. See the
[hooks reference](../../reference/commands/hooks#git-hooks) for collision and
replacement rules. Git import and script-mode bindings are deferred; native
Windows helper execution is unverified.
