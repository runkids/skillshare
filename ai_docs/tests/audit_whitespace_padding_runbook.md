# CLI E2E Runbook: Whitespace Padding

Verify padding findings, benign Markdown twins, report identity and rule controls.

**Origin**: item 17 of #746 — verifies whitespace-padding detection and rule controls.

## Scope

- Forty blank lines yield one HIGH `whitespace-padding` finding.
- Fenced Markdown examples remain benign.
- Global severity, disabling and re-enabling work through supported CLI commands.
- JSON, Markdown and SARIF reports retain the rule identity.

## Environment

Run in the devcontainer with the skillshare-cli-e2e-test workflow and an isolated
ssenv HOME/TMPDIR. Build `bin/skillshare` first. Fixtures are generated text and
are never executed. No network or existing skills are needed.

Use mdproof with `--build true --setup true --step-setup true
--step-teardown true --teardown true --isolation per-runbook --workdir '$HOME'`
to avoid the directory-wide legacy cleanup hooks. Each step creates its own
fixtures and configuration under TMPDIR.

## Steps

### 1. Compare padding with a benign fence

```bash
set -eu
fixture="$TMPDIR/padding-basic"
mkdir -p "$fixture/skills"
export SKILLSHARE_CONFIG="$fixture/config.yaml"
printf 'source: %s\ntargets: {}\n' "$fixture/skills" > "$SKILLSHARE_CONFIG"
python3 - "$fixture" <<'PY'
from pathlib import Path
import sys
root = Path(sys.argv[1])
text = "heading\n" + "\n" * 40 + "instructions\n"
(root / "padded.md").write_text(text)
(root / "benign.md").write_text("```text\n" + text + "```\n")
PY
ss audit -g "$fixture/padded.md" --analyzer static --format json > "$fixture/padded.json"
ss audit -g "$fixture/benign.md" --analyzer static --format json > "$fixture/benign.json"
ss audit -g "$fixture/padded.md" --format sarif > "$fixture/padded.sarif"
ss audit -g "$fixture/padded.md" --format markdown > "$fixture/padded-report.md"
grep -q whitespace-padding "$fixture/padded-report.md"
jq -n --slurpfile p "$fixture/padded.json" --slurpfile b "$fixture/benign.json" \
  --slurpfile s "$fixture/padded.sarif" '{padded:$p[0], benign:$b[0], sarif:$s[0]}'
```

Expected:
- exit_code: 0
- jq: [.padded.results[].findings[]? | select(.ruleId == "whitespace-padding") | .severity] == ["HIGH"]
- jq: [.benign.results[].findings[]? | select(.ruleId == "whitespace-padding")] | length == 0
- jq: [.sarif.runs[].results[].ruleId] | index("whitespace-padding") != null

### 2. Change severity, disable and re-enable the detector

```bash
set -eu
fixture="$TMPDIR/padding-controls"
mkdir -p "$fixture/skills"
export SKILLSHARE_CONFIG="$fixture/config.yaml"
printf 'source: %s\ntargets: {}\n' "$fixture/skills" > "$SKILLSHARE_CONFIG"
python3 - "$fixture/padded.md" <<'PY'
from pathlib import Path
import sys
Path(sys.argv[1]).write_text("heading\n" + "\n" * 40 + "instructions\n")
PY
ss audit rules severity whitespace-padding low -g > /dev/null
ss audit -g "$fixture/padded.md" --profile strict --format json > "$fixture/low.json"
ss audit rules disable --pattern whitespace-padding -g > /dev/null
ss audit -g "$fixture/padded.md" --format json > "$fixture/disabled.json"
ss audit rules enable --pattern whitespace-padding -g > /dev/null
ss audit -g "$fixture/padded.md" --profile strict --format json > "$fixture/enabled.json"
jq -n --slurpfile l "$fixture/low.json" --slurpfile d "$fixture/disabled.json" \
  --slurpfile e "$fixture/enabled.json" '{low:$l[0], disabled:$d[0], enabled:$e[0]}'
```

Expected:
- exit_code: 0
- jq: [.low.results[].findings[]? | select(.ruleId == "whitespace-padding") | .severity] == ["LOW"]
- jq: [.disabled.results[].findings[]? | select(.ruleId == "whitespace-padding")] | length == 0
- jq: [.enabled.results[].findings[]? | select(.ruleId == "whitespace-padding") | .severity] == ["LOW"]

## Pass Criteria

Both steps pass every assertion. All fixtures and configuration stay in the
runner's temporary environment; the run does not access user data.
