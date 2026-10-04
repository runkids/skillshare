# Windows Runbook: .skillfollow Junctions and Directory Symlinks

Manual runbook (not mdproof): it runs on a real Windows machine, driven from a macOS
host through UTM with `scripts/windows/utm.sh`. It mirrors the Linux
`skillfollow_runbook.md` scenarios with Windows link kinds: followed first-level
entries created as directory junctions (every token) and as directory symlinks (only
with the symlink right). Script: `scripts/windows/e2e-skillfollow.ps1`. Proposal:
`proposals/274-skillfollow.md` §3 and §6. Reference: `website/docs/reference/skillfollow.md`.

## Scope

For each link kind the token can create, in a fresh isolated home under `-Root\<kind>`:

1. An undeclared first-level link is invisible to `list --json` and `status --json`.
2. Declaring `group` and `_team` in `.skillfollow` (written with CRLF line endings)
   makes their skills visible; `_team`'s `.skillignore` and the nested
   `group\sub\_nested` repository's `.skillignore` both apply.
3. `status --json` `source.skillfollow` counts, entry states, and `_team`'s
   `resolved_target`.
4. `sync` creates four skill junctions (`0xa0000003`) whose stored target is the
   logical source path (`...\src\skills\group\alpha`), never the external tree, and
   `SKILL.md` reads through them.
5. A declared entry `_off` whose link dangles is `missing`: `sync`, `sync --json`,
   `status --json`, and `diff` report the prune pause, and a stale target junction
   into `_off` is kept.
6. Restoring `_off` resumes prune: the stale junction is removed and the restored
   skill is linked.
7. `update _team --force --dry-run` is refused with exit 1 and HEAD does not move.
8. Removing `_team` from `.skillfollow` prunes its managed link; the external repo stays.

`-Extended` adds: `doctor --json` skillfollow and undeclared checks, plain `list`
showing the resolved path, a second `sync` with `updated=0` and the junction not
recreated, `status --json` counting the links as `merged`, a `.skillfollow.local`
union (duplicate name collapsed), and `invalid-target` for a regular file and for a
link to a file.

Not covered: project mode (relative links), the Developer Mode relative-symlink branch
of `createLink`, a linked source root or target parent, the dashboard, and source Git
staging.

## Environment

- Windows guest in UTM with the guest agent; `utm.sh probe` must show a non-empty
  `desktopUser=` (the scheduled task only runs in an interactive session). Use the
  probe's `arch=` as the build arch.
- Two tokens, both as the desktop user through `utm.sh task`:
  - full (`sstest-full`): an administrator's token; `SeCreateSymbolicLinkPrivilege`
    present, directory symlinks allowed, so both kinds run;
  - basic (`sstest-basic`, `runas /trustlevel:0x20000`): no symlink right, so the
    symlink kind prints `SKIP` with the reason.
- Developer Mode (`devMode=` in the probe and the report header): with it on, the
  basic token may create symlinks too. State it in the report.
- Git for Windows must be on `PATH`; the script initializes the fixture repositories.
- Everything lives under `C:\Users\Public\sstest\`. The script overrides `USERPROFILE`,
  `HOME`, `APPDATA`, `LOCALAPPDATA`, and `TEMP`, and stops a kind unless
  `status --json` reports the source written to the isolated config.

## Steps

1. Preflight and build a pinned commit:

   ```bash
   scripts/windows/utm.sh probe
   OUT=<dir> scripts/windows/utm.sh build <commit> arm64   # or amd64
   ```

2. Create the guest folder and push the binary and script:

   ```bash
   scripts/windows/utm.sh ps /dev/stdin <<'EOF'
   New-Item -ItemType Directory -Force C:\Users\Public\sstest | Out-Null
   icacls C:\Users\Public\sstest /grant '<user>:(OI)(CI)F' | Out-Null
   EOF
   scripts/windows/utm.sh push <dir>/ss.exe 'C:\Users\Public\sstest\ss.exe'
   scripts/windows/utm.sh push scripts/windows/e2e-skillfollow.ps1 'C:\Users\Public\sstest\e2e-skillfollow.ps1'
   ```

3. Run once per token:

   ```bash
   scripts/windows/utm.sh task sstest-full 'C:\Users\Public\sstest\e2e-skillfollow.ps1' -- \
     -Exe 'C:\Users\Public\sstest\ss.exe' -Root 'C:\Users\Public\sstest\sf-full' -Out 'C:\Users\Public\sstest\sf-full.txt' -Extended
   scripts/windows/utm.sh task sstest-basic 'C:\Users\Public\sstest\e2e-skillfollow.ps1' --basic -- \
     -Exe 'C:\Users\Public\sstest\ss.exe' -Root 'C:\Users\Public\sstest\sf-basic' -Out 'C:\Users\Public\sstest\sf-basic.txt' -Extended
   ```

4. Poll each report until its last line is `DONE` (`pull` exits 0 even when the file
   is missing), then save it:

   ```bash
   scripts/windows/utm.sh pull 'C:\Users\Public\sstest\sf-full.txt'
   scripts/windows/utm.sh pull 'C:\Users\Public\sstest\sf-basic.txt'
   ```

5. `scripts/windows/utm.sh clean`.

## Pass Criteria

- Each report ends with `SUMMARY pass=N fail=0 skip=K` and `DONE`.
- Full token: `directory symlink probe: CREATED`, both `[junction]` and `[symlink]`
  checks run, no `SKIP`.
- Basic token: `SeCreateSymbolicLinkPrivilege: absent`, every `[junction]` check
  passes, and the only `SKIP` is `[symlink] all directory-symlink scenarios`.
- In both kinds the sync-created target links are junctions (`0xa0000003`) whose
  `Target` is under `\src\skills\`, not `\ext\`.
- Report the commit, arch, token, and Developer Mode state with every result; a
  skipped kind is not a pass.
