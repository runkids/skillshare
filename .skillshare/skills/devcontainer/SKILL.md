---
name: skillshare-devcontainer
description: >-
  Run CLI commands, tests, and debugging inside the skillshare devcontainer.
  Use this skill whenever you need to: execute skillshare CLI commands for
  verification, run Go tests (unit or integration), reproduce bugs, test new
  features, start the web UI, or perform any operation that requires a Linux
  environment. All CLI execution MUST happen inside the devcontainer — never
  run skillshare commands on the host. If you are about to use Bash to run
  `ss`, `skillshare`, `go test`, or `make test`, stop and use this skill
  first to ensure correct container execution.
argument-hint: "[command-to-run | task-description]"
targets: [claude, codex]
---

Execute CLI commands and tests inside the devcontainer. The host machine is macOS but the project binary is Linux — running CLI commands on the host will silently produce wrong results or fail. This skill prevents that mistake.

## When to Use This

- Running `ss` / `skillshare` commands for verification
- Running `go test`, `make test`, `make check`
- Reproducing a bug report
- Testing a feature you just implemented
- Starting the web UI dashboard
- Any command that needs the skillshare binary or Go toolchain

## When NOT to Use This

- Editing source code (do that on host via Read/Edit tools)
- Running `git` commands (git works on host)
- Running `make fmt`, `make lint` (host-safe Go toolchain commands; no container needed)
- E2E test runbooks → use `cli-e2e-test` skill instead (it handles ssenv isolation)

## Architecture: Two Layers of Isolation

```
Host (macOS)
  └─ Devcontainer (Linux, Debian-based)
       ├─ Default HOME: /home/developer (persistent volume)
       ├─ Source: /workspace (bind-mount of repo root)
       └─ ssenv environments: ~/.ss-envs/<name>/ (isolated HOME dirs)
```

**Devcontainer** = Linux environment with Go, git, pnpm, air (hot-reload). Source code is at `/workspace` (bind-mount of the host repo). The `ss` / `skillshare` wrapper auto-builds from source on every invocation — **no manual `make build` needed**. Edit code on the host, then immediately `docker exec` to run it; the change is picked up automatically.

**ssenv** = Isolated HOME directories within the devcontainer. Each env gets its own `~/.config/skillshare/`, `~/.claude/`, etc. Use ssenv when you need a clean state (testing init, install, sync) without polluting the container's default HOME.

## Zero-Rebuild Workflow

Source code is bind-mounted into the container at `/workspace`. The `ss` wrapper runs `go build` transparently on every invocation:

1. Edit files on host (Read/Edit tools)
2. `docker exec $CONTAINER ss <command>` — picks up your changes instantly
3. No `make build`, no restart, no rebuild step

This also applies to `go test` — tests always compile against the latest source. The Web UI backend uses `air` for hot-reload (same zero-rebuild experience).

## Entering the Devcontainer

The quickest way to get an interactive shell inside the devcontainer:

```bash
make devc                              # interactive bash shell
./scripts/devcontainer_shell.sh make test  # run a command directly
```

The script auto-detects whether the container is running and prints setup instructions if it isn't.

### Programmatic access (for `docker exec` workflows)

```bash
CONTAINER=$(docker compose -f .devcontainer/docker-compose.yml ps -q skillshare-devcontainer 2>/dev/null)
```

If `$CONTAINER` is empty, tell the user:
> Devcontainer is not running. Please start it with VS Code "Reopen in Container" or run:
> `docker compose -f .devcontainer/docker-compose.yml up -d`

Then verify the binary:
```bash
docker exec $CONTAINER bash -c \
  '/workspace/.devcontainer/ensure-skillshare-linux-binary.sh && ss version'
```

## Running Commands

### Simple command (uses container's default HOME)

```bash
docker exec $CONTAINER ss <command> [flags]
```

Good for: `ss version`, `ss status`, `ss list`, `ss check`, `ss audit`.

### Command with isolated HOME (clean state)

```bash
ENV_NAME="test-$(date +%s)"
docker exec $CONTAINER ssenv create "$ENV_NAME" --init
docker exec $CONTAINER ssenv enter "$ENV_NAME" -- ss status
# Cleanup when done:
docker exec $CONTAINER ssenv delete "$ENV_NAME" --force
```

Good for: testing `init`, `install`, `sync`, `uninstall` — anything that modifies config/state.

### Multi-command sequence

```bash
docker exec $CONTAINER ssenv enter "$ENV_NAME" -- bash -c '
  ss install runkids/demo-skills --track --force
  ss list
  ss sync
'
```

Always use `bash -c '...'` for multi-command sequences inside `ssenv enter`.

### Go tests

```bash
# All tests (unit + integration)
docker exec $CONTAINER bash -c 'cd /workspace && make test'

# Unit tests only
docker exec $CONTAINER bash -c 'cd /workspace && make test-unit'

# Integration tests only
docker exec $CONTAINER bash -c 'cd /workspace && make test-int'

# Specific test
docker exec $CONTAINER bash -c 'cd /workspace && go test ./tests/integration -run TestInit_Fresh -count=1'

# Specific package
docker exec $CONTAINER bash -c 'cd /workspace && go test ./internal/install/... -count=1'
```

Always `cd /workspace` before Go commands — ssenv changes HOME which can break module resolution.

### Go tests with auth disabled

Some tests (e.g., `TestResolveToken`, `TestAuthEnv`) need auth credentials removed:

```bash
docker exec $CONTAINER bash -c '
  eval "$(credential-helper --eval off)"
  cd /workspace
  go test ./internal/github -run TestResolveToken -count=1
  eval "$(credential-helper --eval on)"
'
```

## Web UI Dashboard

```bash
# Start (global mode)
docker exec $CONTAINER ui

# Start (project mode — uses ~/demo-project)
docker exec $CONTAINER ui -p

# Stop
docker exec $CONTAINER ui stop
```

Dashboard accessible at `http://localhost:5173` (Vite dev server with HMR).
API backend at `http://localhost:19420`.
Logs: `/tmp/api-dev.log`, `/tmp/vite-dev.log`.

## ssenv Quick Reference

| Shortcut | Full form | Purpose |
|----------|-----------|---------|
| `ssnew <name>` | `ssenv create <name>` + enter | Create and enter isolated shell |
| `ssuse <name>` | `ssenv enter <name>` | Enter existing isolated shell |
| `ssrm <name>` | `ssenv delete <name> --force` | Delete environment |
| `ssls` | `ssenv list` | List all environments |
| `ssback` | `ssenv reset` | Leave isolated context |
| `sshelp` | `help` | Show all devcontainer commands |

For automation (non-interactive), prefer `ssenv enter <name> -- <command>` over `ssnew`/`ssuse` (which launch subshells).

## Ports

| Port | Service | Notes |
|------|---------|-------|
| 5173 | Vite dev server | React dashboard with HMR |
| 19420 | Go API backend | `skillshare ui` server |
| 3000 | Docusaurus | `docs` command in devcontainer |

## Common Mistakes to Avoid

1. **Running `ss` on host** — macOS binary won't match Linux container; always `docker exec`
2. **Forgetting `cd /workspace`** — Go tests fail if HOME was changed by ssenv
3. **Using `make test` on host** — builds macOS binary, then tests run against wrong arch
4. **Skipping `--init` on ssenv create** — env won't have config; most commands will fail
5. **Not cleaning up ssenv** — `ssenv delete <name> --force` after done; or ask user
6. **Running from /workspace root without -g** — the `ss` wrapper auto-redirects to `~/demo-project` in project mode; use `-g` for global or set `SKILLSHARE_DEV_ALLOW_WORKSPACE_PROJECT=1`
7. **Running `make build` before testing** — unnecessary; the `ss` wrapper auto-builds from source every time

## Rules

- **All CLI execution inside devcontainer** — no exceptions
- **Use ssenv for stateful tests** — don't pollute default HOME
- **Always verify** — run the command and check output; never assume it worked
- **Clean up** — delete ssenv environments after use (or ask user)
- **Report container ID** — set `$CONTAINER` at the start and reuse throughout
