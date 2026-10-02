# Windows Runbook: Console Windows of Child Processes

Manual runbook (not mdproof): it runs on a real Windows machine, driven from a macOS
host through `scripts/windows/utm.sh`. It checks that skillshare started without a
console (a scheduled task wrapper, `pythonw`) does not open a console window for
every `git` or native CLI it starts (issue #339), and that runs with a console are
unchanged.

## Scope

`scripts/windows/e2e-console-windows.ps1` starts `ss.exe` four ways and runs `pull`,
`sync plugins --no-tui` and `no-such-command` in each. A monitor polls every 5 ms
for descendants of `ss.exe` and new visible top-level windows.

| Mode | How `ss.exe` starts |
|---|---|
| `INHERIT_HIDDEN` | inherits the task's hidden console |
| `CREATE_NO_WINDOW` | windowless console |
| `DETACHED` | no console at all |
| `NEW_CONSOLE` | its own console, as when Task Scheduler runs it directly |

## Environment

- Windows guest in UTM with the desktop user logged in (Interactive tasks only run
  in an interactive session). `git` on `PATH`.
- The script overrides `USERPROFILE`, `HOME`, `APPDATA`, `LOCALAPPDATA` and the XDG
  variables under `-Root`, and reports `realProfileUnchanged`.
- Window ownership depends on the default terminal: with Windows Terminal, a new
  console shows as a `CASCADIA_HOSTING_WINDOW_CLASS` window plus a
  `PseudoConsoleWindow` owned by the console client.

## Steps

1. Probe and build a pinned commit (`amd64` for x64 guests):

   ```bash
   scripts/windows/utm.sh probe
   scripts/windows/utm.sh build <ref> arm64
   ```

2. Create `C:\Users\Public\sstest` (grant the desktop user full access), then push:

   ```bash
   scripts/windows/utm.sh push "$OUT/ss.exe" 'C:\Users\Public\sstest\ss.exe'
   scripts/windows/utm.sh push scripts/windows/e2e-console-windows.ps1 'C:\Users\Public\sstest\e2e-console-windows.ps1'
   ```

3. Run as the desktop user:

   ```bash
   scripts/windows/utm.sh task sstest-console 'C:\Users\Public\sstest\e2e-console-windows.ps1' -- \
     -Exe C:\\Users\\Public\\sstest\\ss.exe -Root C:\\Users\\Public\\sstest\\run -Out C:\\Users\\Public\\sstest\\out-console.txt
   ```

4. Poll `utm.sh pull 'C:\Users\Public\sstest\out-console.txt'` until the last line is
   `DONE`. Print it with `printf '%s\n'`, not zsh `echo`, which eats backslashes.

5. `scripts/windows/utm.sh clean`.

## Pass Criteria

- Baseline (before the fix), `DETACHED` + `pull`: one `conhost.exe` per `git` that
  `ss.exe` starts and a visible window for each.
- Fixed, `DETACHED`: the tree shows `ss.exe` → `ss.exe` (the hidden-console relaunch)
  and `windows: none` for every command; `pull` output appears in the report.
- Fixed, other modes: no `ss.exe` → `ss.exe` relaunch; `INHERIT_HIDDEN` and
  `CREATE_NO_WINDOW` show `windows: none`; `NEW_CONSOLE` shows only the window of
  `ss.exe` itself (child-process changes cannot remove it).
- Every mode: `pull` and `sync plugins --no-tui` exit 0, `no-such-command` exits 1.
- `realProfileUnchanged=True`.
