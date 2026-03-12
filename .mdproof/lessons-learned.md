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

### [critical] Shell variables do NOT persist across code blocks

- **Context**: symlinked_dir_sync_runbook defined `REAL_SOURCE`, `CLAUDE_TARGET`, `REAL_TARGET` in Step 0 and Step 6 code blocks, then referenced them in Steps 6–19 (14 code blocks total)
- **Discovery**: mdproof executes each fenced code block as an isolated `bash -c` invocation. Shell variables, `cd` state, and environment variables set in one block are completely gone in the next block — even within the same step (sub-commands). This caused 9/20 step failures where paths like `$REAL_TARGET/alpha/SKILL.md` expanded to `/alpha/SKILL.md`
- **Fix**: Re-define all needed variables at the top of EVERY code block that uses them. No shortcuts — `export` doesn't help, sourcing a file isn't built-in
- **Impact**: This is the #1 source of false failures in multi-step runbooks. Steps may also "pass for wrong reasons" (e.g., `test -d ""` → false → prints "REMOVED OK" matching assertion)
- **Runbooks affected**: symlinked_dir_sync_runbook.md (14 code blocks fixed)

### [gotcha] Full-directory mdproof runs cause inter-runbook state leakage

- **Context**: Running `mdproof --report json /path/to/tests/` executes all runbooks sequentially in the same environment (same ssenv). Earlier runbooks install skills, modify config, fill trash — this state persists for later runbooks
- **Discovery**: In a full run of 22 runbooks, the first clean run had 2 failures. But the second run (same ssenv, re-running mdproof) had 10 failures — all due to accumulated state from the first run (1257 trash items, stale registry entries, leftover extras). Even within a single full run, alphabetically-later runbooks can fail because of state left by earlier ones
- **Fix**: (1) Each runbook should clean up its own footprint in Step 1 (rm -rf /tmp/ paths, clear trash, reset config sections). (2) For authoritative results, run each runbook in its own fresh ssenv. (3) Full-directory runs are useful as smoke tests but failures should be re-verified in isolation before treating them as real bugs
- **Runbooks affected**: extras_refactor_json (file_count mismatch from extras_commands leftovers), gitlab_hosts_config (trash from previous runbooks broke go test), registry_yaml_split (/tmp/ state from prior runs)
