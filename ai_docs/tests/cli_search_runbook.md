# CLI E2E Runbook: Offline Command Discovery

## Scope

- Task words discover top-level commands, not remote skills.
- JSON stays parseable with an invalid configuration and no initialized source.
- No matches succeed; invalid input has a stable usage code and help hint.
- `--` separates task words from flags.

## Environment

Run inside the devcontainer with `bin/skillshare` built from this worktree and `jq`
available. Do not run the CLI on the host. Each step is independent; fixtures live
under the runner's isolated TMPDIR.

Use the repository's mdproof workflow. For this read-only command, avoid the
legacy init/cleanup hooks with:

`mdproof --report json --build true --setup true --step-setup true --step-teardown true --teardown true --isolation per-runbook --workdir '$HOME' /workspace/ai_docs/tests/cli_search_runbook.md`

## Steps

### 1. Discover without usable configuration

```bash
set -eu
fixture=$(mktemp -d "$TMPDIR/cli-discovery.XXXXXX")
mkdir -p "$fixture/home" "$fixture/config"
printf 'invalid: [\n' > "$fixture/config/broken.yaml"
cp "$fixture/config/broken.yaml" "$fixture/before.yaml"
HOME="$fixture/home" XDG_CONFIG_HOME="$fixture/config" \
  XDG_DATA_HOME="$fixture/data" XDG_STATE_HOME="$fixture/state" \
  SKILLSHARE_CONFIG="$fixture/config/broken.yaml" \
  /workspace/bin/skillshare cli search "scan security" --json --limit 1 \
  > "$fixture/result.json" 2> "$fixture/stderr"
cmp "$fixture/before.yaml" "$fixture/config/broken.yaml"
test ! -s "$fixture/stderr"
test ! -e "$fixture/data"
test ! -e "$fixture/state"
cat "$fixture/result.json"
```

Expected:
- exit_code: 0
- jq: .query == "scan security"
- jq: .results | length == 1
- jq: .results[0].command == "skillshare audit"
- jq: (.results[0] | keys) == ["command", "summary", "usage"]

### 2. No matches are not an error

```bash
/workspace/bin/skillshare cli search unmatchedxyz --json
```

Expected:
- exit_code: 0
- jq: .query == "unmatchedxyz" and .results == []

### 3. Invalid input has a machine-readable usage error

```bash
/workspace/bin/skillshare cli search "scan security" --limit 0 --json
```

Expected:
- exit_code: 1
- jq: .error.code == "usage"
- jq: .error.message | length > 0
- jq: .error.hint == "Run 'skillshare cli search --help' for usage and examples."

### 4. Delimited flags are task words

```bash
/workspace/bin/skillshare cli search --json -- --limit
```

Expected:
- exit_code: 0
- jq: .query == "--limit"
- jq: .results | type == "array"

## Pass Criteria

All steps satisfy their exit-code and JSON assertions. Discovery does not repair
or overwrite the malformed configuration, create data/state directories, or emit
non-JSON progress text. Runtime execution must be reported separately from this
runbook's source review.
