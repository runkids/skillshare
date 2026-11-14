# Build and Test

Use when building, running the skillshare CLI, executing Go or frontend tests, reproducing bugs, or using the devcontainer/ssenv. E2E runbooks and Windows verification are in `e2e-runbooks`.

## Execution Boundary

The host is macOS; the project verification environment is the Linux devcontainer. Run these inside the devcontainer:

- `ss` and `skillshare` commands;
- `go build`, `go test`, `make test`, and `make check`;
- dashboard and website package scripts;
- bug reproductions and E2E runbooks.

The host may be used for source edits, `git status/diff/log`, read-only searches, `python3 scripts/ai-context.py check`, and documentation checks that do not execute the product binary.

## Devcontainer

```sh
make devc          # start, initialize, and enter
make devc-up       # start without opening a shell
make devc-status
make devc-down
```

For programmatic access, resolve the container first:

```sh
CONTAINER=$(docker compose -f .devcontainer/docker-compose.yml ps -q skillshare-devcontainer)
```

If it is empty, stop and ask the user to run `make devc-up` before executing product commands. The source tree is bind-mounted at `/workspace`. The `ss` wrapper automatically builds current source, so ordinary CLI verification does not need a separate `make build`.

## Parallel Tasks: Shared Container, Isolated Worktrees and State

Use the existing devcontainer by default. `ssenv` gives each task its own HOME and XDG directories; it does not isolate source code, processes, ports, or `/tmp`. A new test environment alone does not need another container.

For a separate branch or pull request, create a worktree before the first edit:

```sh
git worktree add -b runkids/<topic> ../skillshare-<topic> origin/main
```

Keep all edits, tests, and commits in that worktree. Never switch the shared checkout's branch or copy task changes into it to run tests.

### Share the Linux Toolchain

Before using a worktree in the shared container, verify that it is mounted and that its `.git` reference resolves there. The current compose configuration mounts only the main checkout at `/workspace`; sibling and Orca worktrees are not automatically available. Arrange worktree mounts and the main repository's Git metadata at a coordinated container restart. Do not recreate an active shared container or change its mounts underneath other sessions. If the task cannot wait for that setup, use a temporary container as described below.

For an already mounted worktree, create a fresh environment and explicitly select its wrapper and build output:

```sh
ENV_NAME="task-<unique-run-id>"
WORKTREE="/worktrees/<topic>" # Actual path already mounted inside the container.
docker exec "$CONTAINER" ssenv create "$ENV_NAME"
docker exec "$CONTAINER" ssenv enter "$ENV_NAME" -- \
  env -u SKILLSHARE_CONFIG SKILLSHARE_DEV_WORKSPACE_ROOT="$WORKTREE" \
  SKILLSHARE_DEV_USE_BINARY=0 bash -c '
    export SKILLSHARE_DEV_TMP_BINARY="$HOME/skillshare-dev"
    export TMPDIR="$HOME/tmp"
    mkdir -p "$TMPDIR"
    bash "$SKILLSHARE_DEV_WORKSPACE_ROOT/.devcontainer/bin/skillshare" version
  '
```

Use the same environment and exports for subsequent task commands. Invoke the selected worktree's `skillshare` wrapper explicitly: the `ss` shortcut points at `/workspace`. Run initialization through that wrapper when the test needs it; creating the environment without `--init` avoids initializing through the main checkout's binary. Do not run concurrent builds into the same environment's binary path; allocate one environment per concurrent run.

For Go tests, change to the mounted worktree inside the command before running the narrowest required check. Shared Go module and build caches can remain shared. Use task-specific temporary paths; `TMPDIR` does not isolate code that hardcodes `/tmp`. Avoid changing shared Git configuration or restarting shared services. Frontend tasks additionally need Linux `node_modules` for their own worktree and distinct ports/output paths; never reuse another branch's writable dependency directory or host macOS bindings.

### Temporary Containers When Needed

Use a separate container when the worktree is not mounted yet, a test changes system dependencies, or services and fixed paths cannot safely coexist. Reuse the existing devcontainer image and Go cache volumes. Keep task compose files in a repository scratch directory, and install only dependencies required by that task: Go-only work does not need UI or website packages.

```yaml
name: skillshare_wt_<topic>
services:
  dev:
    image: skillshare_devcontainer-skillshare-devcontainer:latest
    working_dir: /workspace
    command: sleep infinity
    environment: { HOME: /home/developer, GOCACHE: /go/build-cache }
    volumes:
      - <abs-path>/skillshare-<topic>:/workspace
      # The worktree's .git file references this absolute host path.
      - <abs-path>/skillshare/.git:<abs-path>/skillshare/.git
      - skillshare_devcontainer_go-mod-cache:/go/pkg/mod
      - skillshare_devcontainer_go-build-cache:/go/build-cache
      # Add Linux node_modules volumes only for frontend work:
      # - /workspace/ui/node_modules
      # - /workspace/website/node_modules
volumes:
  skillshare_devcontainer_go-mod-cache: { external: true }
  skillshare_devcontainer_go-build-cache: { external: true }
```

Verify the image and external volumes exist before starting with `docker compose -f <file> up -d`. Use `bash -c`, not `bash -lc`: an empty home can make a login shell lose Go from PATH. Configure Git safe-directory and credential isolation only in the task's own HOME. Publish ports only when needed, choosing unused host ports.

At completion, identify the exact task-owned environment, container, volumes, and clean worktree. Obtain authorization for deletion unless task cleanup was already explicitly authorized; preserve requested debugging evidence. Remove temporary containers with `docker compose -f <file> down -v` so their anonymous dependency volumes do not accumulate. External shared caches must remain. Do not use broad system or volume pruning as task cleanup, and do not remove another task's running container.

Check for leftovers on the host when the disk runs low, since the host Docker volumes are not the only source. Use `/usr/bin/du`, because `du` may be aliased to `dust`.

- `.git/objects/pack/tmp_pack_*` comes from interrupted fetch or push operations and once reached 32 GB. When `pgrep -fl "git (fetch|push|gc|repack)"` shows nothing running, delete only that glob, then run `git fsck --connectivity-only`.
- The ignored repository-root directories `.verify-home`, `.pnpm-store`, `.cache`, and `tmp/` hold verification homes, package stores, and built binaries. Remove only the ones the current task created, after confirming with the user.
- Report sizes before deleting, and never prune Docker volumes broadly; `skillshare_devcontainer_go-build-cache` is shared.

## Narrow Verification First

Change to `/workspace` inside the container:

```sh
docker exec "$CONTAINER" bash -lc 'cd /workspace && go test ./internal/<package>/... -count=1'
docker exec "$CONTAINER" bash -lc 'cd /workspace && go test ./tests/integration -run TestName -count=1'
docker exec "$CONTAINER" bash -lc 'cd /workspace && make test-unit'
docker exec "$CONTAINER" bash -lc 'cd /workspace && make test-int'
docker exec "$CONTAINER" bash -lc 'cd /workspace && make check'
```

Start with the specific package or test that proves the change. Broaden only when new risk, failure evidence, or a release gate requires it. Base retries on new evidence and distinguish pre-existing failures from regressions.

Real-shell completion tests (`TestCompletion_{Zsh,Fish}_Completes*`) skip when zsh or fish is missing, as in the devcontainer. To run them, start a throwaway container from the devcontainer image with the checkout at `/workspace` and the Go cache volumes, install `zsh fish` with apt-get there only, then `make build` and `go test ./tests/integration -run 'Completion' -count=1`.

## Test Value

- Name the concrete failure a new test catches before adding it. Prefer the smallest existing check that already proves the change; test count is not a completion criterion.
- Test a rule once, in the package that owns it (e.g. `internal/uninstall`, `RemoveByNames`); put a fix's regression test there, not in every entry point.
- Handler and CLI tests cover translation only: status/error code and response fields per outcome, flag wiring, exit code, mode selection.
- `tests/integration` is for cross-package flows and real-binary behavior. Assert `--json` or files over wording.
- For copy, translations and straightforward visual changes, reuse locale/placeholder checks, builds and rendered inspection. Do not create a new test solely because a file changed.
- Add regression coverage when there is meaningful behavior to protect, especially data loss, permissions, input validation or state transitions. Do not weaken existing validation to reduce cost, and respect explicit user instructions about adding tests.

## Stateful CLI Isolation

Use a fresh `ssenv` for tests that modify configuration or state:

```sh
ENV_NAME="task-<descriptive-id>"
docker exec "$CONTAINER" ssenv create "$ENV_NAME" --init
docker exec "$CONTAINER" ssenv enter "$ENV_NAME" -- ss status --json
```

Rules:

- Create a fresh environment for every E2E run; never reuse stale state.
- `ssenv` isolates HOME and XDG directories. `/tmp`, source trees, binary outputs, and ports remain shared; follow the parallel-task isolation rules above.
- Prefer `bash -c` for isolated multi-command sequences so login profiles do not reset the task environment. Before Go commands, change to the selected source tree (`/workspace` only for the main checkout or a task container mounted there).
- `--init` already performs global initialization and creates the default `rules` extra; a runbook must not assume an empty environment.
- `--seed` fills the environment for `init` testing: claude (2 skills), codex (1 skill), cursor, and a local repo at `$HOME/remote/skills.git` whose `pdf-tools` clashes with claude's.
- Report the environment after execution. Delete or preserve it for debugging according to user direction; never discard requested evidence silently.

## E2E Runbooks and Windows

Load `e2e-runbooks` to write or run `ai_docs/tests/*_runbook.md` runbooks or to verify on a Windows guest.

For changes to what `mcp serve` exposes through the Skills extension (SEP-2640), also run `bash scripts/mcp/skills-conformance.sh <absolute-report-dir>` inside the devcontainer. It fetches the official conformance suite at a pinned commit (needs network and `npm ci`), serves a synthetic source, and fails unless every check is `SUCCESS`. It is not part of `make check` or CI.

## Frontend and Website Verification

Start the dashboard through the devcontainer `ui` command. Run website package commands according to `website/AGENTS.md`. Visual changes require inspected screenshots in addition to builds and tests. Load `frontend` for design-specific checks.

For full-screen TUI changes, screenshot every TUI inside the devcontainer with `scripts/screenshots/record-tui.sh [name...]`. It records the binary in `BIN` (default `/tmp/tuibin`) and writes PNGs to `OUT` (default `/tmp/tui-shots`). Each screen runs in a fresh ssenv that `scripts/screenshots/tui-fixture.sh` fills with real commands. The first run installs vhs and its tools through `.devcontainer/ensure-recording-tools.sh`, which takes a few minutes; later runs skip it.

## Reporting

Report the exact commands, pass/fail status, failure location, and limitations. Starting a command is not proof of success. Never disable tests, hooks, audits, or trust prompts to obtain a pass.
