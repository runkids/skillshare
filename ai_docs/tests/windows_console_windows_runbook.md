# Windows Runbook: Console Windows and Child Lifetime

Manual runbook (not mdproof): it runs on a real Windows machine, driven from a macOS
host through `scripts/windows/utm.sh`. It checks that skillshare started without a
console (a scheduled task wrapper, `pythonw`) does not open a console window for
every `git` or native CLI it starts (issue #339), that runs with a console are
unchanged, and that terminating skillshare also stops its foreground children while
processes meant to outlive it keep running.

## Scope

`scripts/windows/e2e-console-windows.ps1` starts `ss.exe` four ways and runs `pull`,
`sync plugins --no-tui` and `no-such-command` in each. A monitor polls every 5 ms
for descendants of `ss.exe` and new visible top-level windows.

| Mode | How `ss.exe` starts |
|---|---|
| `INHERIT_HIDDEN` | inherits the caller's console |
| `CREATE_NO_WINDOW` | windowless console |
| `DETACHED` | no console at all |
| `NEW_CONSOLE` | its own console, as when Task Scheduler runs it directly |

`-Cancel` (with `-UiZip`/`-VersionFile`) adds lifetime checks: a `pull` whose `git`
waits on a sleeping ssh command is terminated (`INHERIT_HIDDEN` and `DETACHED`), a
`DETACHED` foreground `ui` is terminated, a `ui start` server must outlive its
launcher, and a foreground `ui` restart through `POST /api/restart` must come back.
`-SkipWindows` drops the window matrix.

The lifetime result depends on the caller's job. Task Scheduler runs a task in a job
that forbids breakaway, so skillshare does not bind its children there (Task
Scheduler's own job ends them when the task is stopped). Run the lifetime checks
both as a task and as SYSTEM through `utm.sh ps`, which runs outside any job.

`internal/childproc` has Windows-only tests; build them with `go test -c` and run the
binary on the guest the same two ways.

## Environment

- Windows guest in UTM with the desktop user logged in (Interactive tasks only run
  in an interactive session). `git` on `PATH`.
- The script overrides `USERPROFILE`, `HOME`, `APPDATA`, `LOCALAPPDATA` and the XDG
  variables under `-Root`, and reports `realProfileUnchanged`. It reports
  `callerJob=job|none` for the lifetime checks.
- Window ownership depends on the default terminal: with Windows Terminal, a new
  console shows as a `CASCADIA_HOSTING_WINDOW_CLASS` window plus a
  `PseudoConsoleWindow` owned by the console client.
- Ports 19451–19453 must be free.

## Steps

1. Probe, then build a pinned commit for the guest's `arch=` (`ARM64` → `arm64`,
   `AMD64` → `amd64`). Export `OUT` so later steps find the build, and build the
   `childproc` test binary from the same commit:

   ```bash
   scripts/windows/utm.sh probe
   export OUT=${TMPDIR:-/tmp}/skillshare-utm
   scripts/windows/utm.sh build <ref> <arm64|amd64>
   docker exec "$CONTAINER" bash -lc 'rm -rf /tmp/cp && mkdir /tmp/cp && git -C /workspace archive <ref> | tar -x -C /tmp/cp &&
     cd /tmp/cp && GOOS=windows GOARCH=<arm64|amd64> go test -c -o /tmp/childproc.test.exe ./internal/childproc'
   docker cp "$CONTAINER":/tmp/childproc.test.exe "$OUT/"
   ```

2. Create `C:\Users\Public\sstest` (grant the desktop user full access), then push
   `ss.exe`, `ss-ui-dist.zip`, `ss-version.txt`, `childproc.test.exe` and the script:

   ```bash
   for f in ss.exe ss-ui-dist.zip ss-version.txt childproc.test.exe; do
     scripts/windows/utm.sh push "$OUT/$f" "C:\\Users\\Public\\sstest\\$f"
   done
   scripts/windows/utm.sh push scripts/windows/e2e-console-windows.ps1 'C:\Users\Public\sstest\e2e-console-windows.ps1'
   ```

3. Run as the desktop user, once with the full token and once with `--basic`:

   ```bash
   scripts/windows/utm.sh task sstest-console 'C:\Users\Public\sstest\e2e-console-windows.ps1' [--basic] -- \
     -Exe C:\\Users\\Public\\sstest\\ss.exe -Root C:\\Users\\Public\\sstest\\run -Out C:\\Users\\Public\\sstest\\out-console.txt \
     -Cancel -UiZip C:\\Users\\Public\\sstest\\ss-ui-dist.zip -VersionFile C:\\Users\\Public\\sstest\\ss-version.txt
   ```

   Poll `utm.sh pull 'C:\Users\Public\sstest\out-console.txt'` until the last line is
   `DONE`. Print it with `printf '%s\n'`, not zsh `echo`, which eats backslashes.
   Remove `C:\Users\Public\sstest\run` between runs.

4. Run the lifetime checks as SYSTEM (no job): write a one-line wrapper that calls the
   script with the same arguments plus `-SkipWindows`, then
   `scripts/windows/utm.sh ps <wrapper.ps1> 600` and pull the report.

5. Run the test binary as SYSTEM and as a task (quote the flags; PowerShell splits
   an unquoted `-test.v`): `& C:\Users\Public\sstest\childproc.test.exe '-test.v'`.

6. `scripts/windows/utm.sh clean`.

## Pass Criteria

Windows (task, full and basic token):

- Baseline (before #341), `DETACHED` + `pull`: one `conhost.exe` per `git` that
  `ss.exe` starts and a visible window for each.
- `DETACHED`: the tree shows `ss.exe` → `ss.exe` (the hidden-console relaunch) and
  `windows: none` for every command; `pull` output appears in the report.
- Other modes: no `ss.exe` → `ss.exe` relaunch; `INHERIT_HIDDEN` and
  `CREATE_NO_WINDOW` show `windows: none`; `NEW_CONSOLE` shows only the window of
  `ss.exe` itself (child-process changes cannot remove it).
- Every mode: `pull` and `sync plugins --no-tui` exit 0, `no-such-command` exits 1.

Lifetime (`-Cancel`):

- `callerJob=none` (SYSTEM), fixed: both `cancel pull` lines show an empty
  `aliveAfterKill=`, including the `powershell.exe` that `sh.exe` starts (the MSYS
  runtime breaks its children away from any job that allows breakaway). Baseline:
  `git` and the ssh command are still alive.
- `callerJob=job` (task): `cancel pull (INHERIT_HIDDEN)` may leave `git` alive, as
  before (no binding inside a job that forbids breakaway); `DETACHED` still stops
  the relaunched `ss.exe`.
- Every run: `cancel foreground ui (DETACHED)` shows `listeningAfterKill=False`;
  both `ui start` lines show `serverListeningAfterLauncherExit=True` and the
  following `ui stop` leaves `stillListening=False`; `ui restart` shows
  `"restarting":true`, `oldServerExited=True` and `listeningAfterRestart=True`.
- `childproc.test.exe`: all tests pass as SYSTEM and skip as a task.
- `realProfileUnchanged=True`.
