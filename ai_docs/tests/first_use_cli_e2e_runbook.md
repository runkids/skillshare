# CLI E2E Runbook: First-Time `ss` User

This is a human-readable execution flow (not a `.sh` script) to simulate a first-time `ss` user end-to-end.

## Scope

- Global mode first setup
- `init` creates config successfully
- First skill sync to targets
- Per-target mode tuning (`target --mode`)
- Basic health check (`doctor`)

## Environment

Run inside devcontainer by default.

## Optional: use `ssenv` for isolated HOME switching

Create and switch into a named isolated environment:

```bash
ssenv create first-use-demo
eval "$(ssenv --eval use first-use-demo)"
ssenv status
```

Or use no-`eval` shortcuts:

```bash
ssnew first-use-demo       # create + enter isolated shell
ssuse first-use-demo       # enter existing isolated shell
ssback                     # leave isolated context helper
```

Reset back to your normal shell environment:

```bash
eval "$(ssenv --eval reset)"
```

Delete isolated environment:

```bash
ssenv delete first-use-demo --force
```

## Step 0: Verify command entrypoint

```bash
echo "HOME=$HOME"
which ss
which skillshare
ss version
```

Expected:

- `ss version` runs successfully
- No `Exec format error`

If you see `Cannot run macOS (Mach-O) executable in Docker`, run:

```bash
/workspace/.devcontainer/ensure-skillshare-linux-binary.sh
```

## Step 1: Create isolated first-use HOME

```bash
export E2E_HOME="/tmp/ss-e2e-first-$(date +%s)"
rm -rf "$E2E_HOME"
mkdir -p "$E2E_HOME"
echo "E2E_HOME=$E2E_HOME"
```

## Step 2: First init (non-interactive)

```bash
HOME="$E2E_HOME" ss init --no-copy --targets claude,cursor --mode merge --no-git --no-skill
```

Expected:

- Output contains `Initialized successfully`
- Output contains `Config: $E2E_HOME/.config/skillshare/config.yaml`

## Step 3: Verify config was created

```bash
test -f "$E2E_HOME/.config/skillshare/config.yaml" && echo "config_created=yes"
cat "$E2E_HOME/.config/skillshare/config.yaml"
```

Expected:

- `config_created=yes`
- YAML includes:
  - `source: $E2E_HOME/.config/skillshare/skills`
  - `mode: merge`
  - `targets:` with `claude` and `cursor`

## Step 4: Add one demo skill to source

```bash
mkdir -p "$E2E_HOME/.config/skillshare/skills/hello-world"
cat > "$E2E_HOME/.config/skillshare/skills/hello-world/SKILL.md" <<'EOF'
# hello-world

This is an E2E demo skill.
EOF
```

## Step 5: First sync

```bash
HOME="$E2E_HOME" ss sync
```

Expected:

- Sync output includes both targets:
  - `claude: ...`
  - `cursor: ...`

## Step 6: Verify skill reached both targets

```bash
test -f "$E2E_HOME/.claude/skills/hello-world/SKILL.md" && echo "claude_ok=yes"
test -f "$E2E_HOME/.cursor/skills/hello-world/SKILL.md" && echo "cursor_ok=yes"
```

Expected:

- `claude_ok=yes`
- `cursor_ok=yes`

## Step 7: Simulate per-target compatibility tuning

Change only `cursor` to `copy` mode (leave global/default as-is).

```bash
HOME="$E2E_HOME" ss target cursor --mode copy
HOME="$E2E_HOME" ss sync
grep -n "mode: copy" "$E2E_HOME/.config/skillshare/config.yaml"
```

Expected:

- Command succeeds
- Config now contains `mode: copy` under cursor target override

## Step 8: Run doctor

```bash
HOME="$E2E_HOME" ss doctor
```

Expected:

- Doctor completes without fatal errors

## Pass/Fail Criteria

Pass when all are true:

- `init` creates `config.yaml`
- first `sync` delivers `hello-world` to both targets
- `target cursor --mode copy` is persisted and re-sync succeeds
- `doctor` succeeds

Fail if any step errors or expected files are missing.
