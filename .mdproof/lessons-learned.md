# mdproof Lessons Learned

### [assertion] Use jq: for JSON output, not python3 pipe

- **Context**: extras_refactor_json_runbook had 7 steps using `python3 -c` to parse JSON and print key=value pairs for substring assertions
- **Discovery**: mdproof's native `jq:` assertion is cleaner — one-liner per check, no script maintenance, and jq-exit-code failure is automatic
- **Fix**: Replace `cmd --json | python3 -c "import json; ..."` with bare `cmd --json` + `jq:` assertions
- **Runbooks affected**: extras_refactor_json_runbook.md (steps 3, 4, 7, 10, 11, 17, 18)

### [gotcha] cat >> is not idempotent across re-runs

- **Context**: Several runbooks used `cat >> config.yaml` to append YAML sections (extras, gitlab_hosts)
- **Discovery**: If a runbook is re-run in the same ssenv (or /tmp persists), `cat >>` appends duplicate YAML keys, causing parse errors
- **Fix**: Prepend `sed -i '/^section:/,$d'` before `cat >>` to strip existing section first. Or use CLI commands (`ss extras init`, `ss extras remove --force`) which handle duplicates gracefully
- **Runbooks affected**: extras_refactor_json_runbook.md, sync_extras_runbook.md, gitlab_hosts_config_runbook.md

### [gotcha] ssenv only isolates $HOME — /tmp/ persists across environments

- **Context**: Steps creating files in /tmp/ (e.g., /tmp/test-project, /tmp/extras-proj) left artifacts that broke re-runs
- **Discovery**: `ssenv` sets an isolated `$HOME` but shares `/tmp/`, `/var/`, and other system paths with the host container
- **Fix**: Add `rm -rf /tmp/<path>` at the start of steps that create /tmp/ directories. Or use mdproof.json `step_setup` for common cleanup patterns
- **Runbooks affected**: extras_refactor_json_runbook.md, extras_commands_runbook.md, gitlab_hosts_config_runbook.md

### [gotcha] ssenv --init creates default extras — runbooks must clean up

- **Context**: Runbook Step 1 assumed an empty environment (`ss extras list --json` → `[]`), but `ssenv create --init` pre-creates a `rules` extra
- **Discovery**: `--init` runs `ss init` which creates default extras (rules). Subsequent `extras init rules` fails with "already exists", and target dirs (e.g., `~/.claude/rules/`) already exist before any runbook step syncs
- **Fix**: Add cleanup at the start of runbooks that assume no pre-existing extras: `ss extras remove rules --force -g 2>/dev/null || true` + `rm -rf ~/.claude/rules 2>/dev/null || true`
- **Runbooks affected**: extras_commands_runbook.md (step 1), sync_extras_runbook.md (step 1)

### [gotcha] echo > symlink writes through to source

- **Context**: Step tried `echo "content" > target/file.md` to create a "local file" after sync, but the file was a symlink
- **Discovery**: Shell `echo > symlink` writes to the symlink's target, not creating a new file. This means the "local file" ends up in source, and `collect` sees nothing to collect
- **Fix**: Create files with different names than synced files, or `rm` the symlink first then create the file
- **Runbooks affected**: extras_commands_runbook.md (step 18 collect), extras_refactor_json_runbook.md
