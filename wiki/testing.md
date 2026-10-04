# Build, Test, and E2E

Use when building, running the skillshare CLI, executing Go or frontend tests, reproducing bugs, using the devcontainer/ssenv, or creating and running E2E runbooks.

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

## Parallel Tasks: One Worktree and Container Each

Several sessions may share this checkout and the devcontainer, and either can change under you: another task commits, cleans the tree, or recreates the container. When a task gets its own branch or pull request, set this up before the first edit, not after:

```sh
git worktree add -b runkids/<topic> ../skillshare-<topic> origin/main
```

Make every later edit, test, and commit in that worktree. Do not edit the shared checkout and copy files over. The devcontainer mounts only the main checkout, and changing its mounts affects the other sessions, so give the worktree a throwaway container from the devcontainer image instead. Keep the compose file in the session scratchpad:

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
      - /workspace/ui/node_modules
      - /workspace/website/node_modules
      # The worktree's .git file points at the main checkout's .git by absolute path.
      - <abs-path>/skillshare/.git:<abs-path>/skillshare/.git
      - skillshare_devcontainer_go-mod-cache:/go/pkg/mod
volumes:
  skillshare_devcontainer_go-mod-cache: { external: true }
```

Start it with `docker compose -f <file> up -d`. Run commands there with `docker exec <container> bash -c '...'`, not `bash -lc`: its home is empty, so a login shell resets `PATH` and loses Go. Inside it, run `git config --global --add safe.directory /workspace`, disable the credential helper, run `pnpm install --frozen-lockfile` in `ui/` and `website/`, then use the commands below. It publishes no ports, so it never clashes with the shared dev servers.

When the task is finished (pushed, or abandoned), remove both so they stop using disk:

```sh
docker compose -f <file> down -v              # the container and its node_modules volumes
git worktree remove ../skillshare-<topic>     # only when its status is clean
```

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

## Stateful CLI Isolation

Use a fresh `ssenv` for tests that modify configuration or state:

```sh
ENV_NAME="task-<descriptive-id>"
docker exec "$CONTAINER" ssenv create "$ENV_NAME" --init
docker exec "$CONTAINER" ssenv enter "$ENV_NAME" -- ss status --json
```

Rules:

- Create a fresh environment for every E2E run; never reuse stale state.
- `ssenv` isolates only `HOME`. `/tmp` and other system paths are shared, so runbooks must use unique paths or clean an exact target first.
- Use `bash -c` for multi-command sequences and `cd /workspace` before Go commands.
- `--init` already performs global initialization and creates the default `rules` extra; a runbook must not assume an empty environment.
- `--seed` fills the environment for `init` testing: claude (2 skills), codex (1 skill), cursor, and a local repo at `$HOME/remote/skills.git` whose `pdf-tools` clashes with claude's.
- Report the environment after execution. Delete or preserve it for debugging according to user direction; never discard requested evidence silently.

## E2E Runbooks

Runbooks live in `ai_docs/tests/*_runbook.md` and are executed by mdproof. Before creating or changing one:

1. Read `ai_docs/tests/mdproof.json` when present and `.mdproof/lessons-learned.md`.
2. Verify every command and flag against source or `--help` inside the container.
3. Use YAML-free Markdown with Scope, Environment, Steps, and Pass Criteria.
4. Give every step a `bash` block and a machine-checkable `Expected` section.
5. Prefer `--json` or `--format json` with `jq:` assertions over unstable human-readable text.

Execute a runbook with:

```sh
docker exec "$CONTAINER" /workspace/.devcontainer/ensure-mdproof.sh
docker exec "$CONTAINER" env SKILLSHARE_DEV_ALLOW_WORKSPACE_PROJECT=1 \
  ssenv enter "$ENV_NAME" -- \
  mdproof --report json /workspace/ai_docs/tests/<runbook>.md
```

Do not abort the whole runbook after the first failed step. Preserve every step result, then classify the failure:

- **Runbook bug:** stale flag, path, or assertion.
- **Product bug:** CLI behavior contradicts source or acceptance criteria.
- **Environment issue:** container, network, credential, or shared-path state.

## Runbook Quality Checklist

- Verify every flag against `cmd/skillshare/` or `--help`.
- Verify project-init and global-init flag sets separately.
- `registry.yaml` appears only after install/reconcile; installed resources do not belong in `config.yaml`.
- Project state uses `.skillshare/` and global state uses the configuration directory.
- Audit customization uses rule IDs, not pattern-group names.
- Expected results use actual substrings, exit codes, regular expressions, `jq:`, or snapshots rather than prose wishes.
- Prefer `jq:` for JSON assertions; `log --json` emits JSONL, not an array.
- Never append YAML with a non-idempotent bare `cat >>`; prefer the CLI or full replacement.
- Writing through a symlink changes its target. Remove the exact symlink first or use another filename when a local file is required.
- Clean exact `/tmp` targets at the beginning of a step.
- Never assume a repository name equals an installed skill name; verify with `ss list --json`.

## Windows Verification

Linux tests cannot show Windows link behavior (junctions, file symlinks, Developer Mode). For changes to links, sync modes, or paths on Windows, run `ai_docs/tests/windows_file_links_runbook.md` on a real Windows machine; its script is `scripts/windows/e2e-file-links.ps1`. With a local UTM guest, `scripts/windows/utm.sh` handles probe, pinned builds, push, and running scripts as the desktop user (full or basic token); the `skillshare-windows-utm` skill walks through automated and hands-on runs. For child processes, console windows, or runs without a console (scheduled tasks), run `ai_docs/tests/windows_console_windows_runbook.md` (`scripts/windows/e2e-console-windows.ps1`).

- Cross-compile in the devcontainer (`GOOS=windows`, `GOARCH` matching the guest). The host drives a UTM guest with `utmctl` (`file push`, `exec`, `file pull`); this is the one allowed host-side execution path, and it never runs the product on macOS.
- `utmctl exec` is asynchronous and runs as SYSTEM. Scripts write a report ending in `DONE`, and the host polls it with `file pull`, which exits 0 even when the file is missing.
- SYSTEM and administrators can create file symlinks. Run as the desktop user through a scheduled task, and use `runas /trustlevel:0x20000` to test without symlink rights.
- Isolate under `C:\Users\Public\sstest\` by overriding `USERPROFILE`, `HOME`, `APPDATA`, `LOCALAPPDATA`, and `TEMP`; confirm the config path before any mutating command.
- Inspect results with `fsutil reparsepoint query` (`0xa0000003` junction, `0xa000000c` symlink) and by reading the file.
- Clean up with `rmdir /s /q`, which does not follow junctions, and unregister the scheduled tasks.

Report which token (full or basic-user) and architecture each result came from.

## Frontend and Website Verification

Start the dashboard through the devcontainer `ui` command. Run website package commands according to `website/AGENTS.md`. Visual changes require inspected screenshots in addition to builds and tests. Load `frontend` for design-specific checks.

For full-screen TUI changes, screenshot every TUI inside the devcontainer with `scripts/screenshots/record-tui.sh [name...]`. It records the binary in `BIN` (default `/tmp/tuibin`) and writes PNGs to `OUT` (default `/tmp/tui-shots`). Each screen runs in a fresh ssenv that `scripts/screenshots/tui-fixture.sh` fills with real commands. The first run installs vhs and its tools through `.devcontainer/ensure-recording-tools.sh`, which takes a few minutes; later runs skip it.

## Reporting

Report the exact commands, pass/fail status, failure location, and limitations. Starting a command is not proof of success. Never disable tests, hooks, audits, or trust prompts to obtain a pass.
