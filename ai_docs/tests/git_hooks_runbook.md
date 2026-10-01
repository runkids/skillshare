# Git hooks execution proof

The Go/CLI/API suites manage configuration and use read-only `git hook list`;
they never execute hooks. `scripts/hooks/git-e2e.sh` is the sole execution proof.
It requires the `/workspace` devcontainer, Git 2.54+ and a fresh `ssenv` HOME.
Do not run it natively or against a user's normal configuration.

Inside the devcontainer:

```bash
cd /workspace
make build
ssenv create git-hooks-proof
ssenv enter git-hooks-proof -- bash /workspace/scripts/hooks/git-e2e.sh
```

The script isolates global/system Git config, XDG paths and Skillshare source.
It creates local repositories and a bare remote, checks commit/amend/push,
config-before-hookdir order, arguments and full stdin per hook, `--no-verify`,
a linked-worktree commit, and portable wrappers using a stub Roborev. Evidence
is retained under the disposable HOME; inspect the final JSON path on failure.
Delete only that disposable environment after inspection with `ssenv`.

For environments where execution is forbidden, validate shell syntax with
`bash -n scripts/hooks/git-e2e.sh` and report execution as skipped. Registration,
management lifecycle and native parser tests remain required.
