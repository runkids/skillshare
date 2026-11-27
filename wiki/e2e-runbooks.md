# E2E Runbooks and Windows Verification

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
  mdproof --report json /workspace/ai_docs/tests/<runbook>.md
```

Run mdproof directly, not inside `ssenv enter`: `ai_docs/tests/mdproof.json` sets `isolation: per-runbook`, so mdproof gives each runbook its own `HOME` and runs the setup hook there. Wrapped in ssenv, the setup initializes one `HOME` and the steps run in another, so every step fails. Use ssenv only to reproduce a failed step by hand.

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

Linux tests cannot show Windows link behavior (junctions, file symlinks, Developer Mode). For changes to links, sync modes, or paths on Windows, run `ai_docs/tests/windows_file_links_runbook.md` on a real Windows machine; its script is `scripts/windows/e2e-file-links.ps1`. For first-level source links (`follow_source_links`), run `ai_docs/tests/windows_follow_source_links_runbook.md` (`scripts/windows/e2e-follow-source-links.ps1`). With a local UTM guest, `scripts/windows/utm.sh` handles probe, pinned builds, push, and running scripts as the desktop user (full or basic token); the `skillshare-windows-utm` skill walks through automated and hands-on runs. For child processes, console windows, or runs without a console (scheduled tasks), run `ai_docs/tests/windows_console_windows_runbook.md` (`scripts/windows/e2e-console-windows.ps1`).

- Cross-compile in the devcontainer (`GOOS=windows`, `GOARCH` matching the guest). The host drives a UTM guest with `utmctl` (`file push`, `exec`, `file pull`); this is the one allowed host-side execution path, and it never runs the product on macOS.
- `utmctl exec` is asynchronous and runs as SYSTEM. Scripts write a report ending in `DONE`, and the host polls it with `file pull`, which exits 0 even when the file is missing.
- SYSTEM and administrators can create file symlinks. Run as the desktop user through a scheduled task, and use `runas /trustlevel:0x20000` to test without symlink rights.
- Isolate under `C:\Users\Public\sstest\` by overriding `USERPROFILE`, `HOME`, `APPDATA`, `LOCALAPPDATA`, and `TEMP`; confirm the config path before any mutating command.
- Inspect results with `fsutil reparsepoint query` (`0xa0000003` junction, `0xa000000c` symlink) and by reading the file.
- Clean up with `rmdir /s /q`, which does not follow junctions, and unregister the scheduled tasks.

Report which token (full or basic-user) and architecture each result came from.
