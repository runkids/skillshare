# CLI E2E Runbook: Bundled References

Validate contained reference checks and doctor parity using synthetic paths only.

**Origin**: audit improvement item 13 of #746.

## Scope

- LOW missing links/images and HIGH parent/symlink escapes
- Explicit code-path forms, benign code examples, and preserved source evidence
- Doctor JSON parity and global rule disables
- Nested code paths use the document for `./` and `../`, and the skill root for other relative paths

## Environment

Run inside the devcontainer using mdproof's isolated HOME and TMPDIR.
Never execute fixture content or reference personal files.

## Steps

### 1. Audit required references and benign examples

```bash
set -eu
fixture="$TMPDIR/bundled-references"
mkdir -p "$fixture/references" "$TMPDIR/bundled-external"
printf '# Existing\n' > "$fixture/references/ok.md"
ln -s "$TMPDIR/bundled-external" "$fixture/external.bundle"
cat > "$fixture/SKILL.md" <<'DOC'
---
name: bundled-references
description: Synthetic reference fixture
---
[existing](references/ok.md)
![missing](assets/missing.png)
[escape](../outside.md)
[symlink](external.bundle/missing.md)
Read `./missing.md`.
Example: `/example/config.json` and `ordinary.identifier`.
DOC
ss audit -g "$fixture" --analyzer structure --format json
```

Expected:
- exit_code: 0
- jq: [.results[].findings[] | select(.ruleId == "dangling-link")] | length == 2
- jq: [.results[].findings[] | select(.ruleId == "reference-escapes-root" and .severity == "HIGH")] | length == 2
- jq: [.results[].findings[] | select(.snippet == "![missing](assets/missing.png)" and .line > 0)] | length == 1

### 2. Doctor uses the same checker

```bash
set -eu
config_dir="${XDG_CONFIG_HOME:-$HOME/.config}/skillshare"
if [ ! -f "$config_dir/config.yaml" ]; then
  ss init -g --force --all-targets --no-git --no-skill >/dev/null
fi
fixture="$config_dir/skills/bundled-doctor"
mkdir -p "$fixture"
printf '%s\n' '---' 'name: bundled-doctor' 'description: Synthetic fixture' '---' '![missing](missing.png)' '[escape](../outside.md)' > "$fixture/SKILL.md"
ss doctor -g --json
```

Expected:
- exit_code: 1
- jq: [.checks[] | select(.name == "bundled_references" and .status == "warning" and (.message | contains("dangling-link")))] | length == 1
- jq: [.checks[] | select(.name == "bundled_references" and .status == "error" and (.message | contains("reference-escapes-root")))] | length == 1

### 3. Rule disables apply to audit and doctor

```bash
set -eu
config_dir="${XDG_CONFIG_HOME:-$HOME/.config}/skillshare"
if [ ! -f "$config_dir/config.yaml" ]; then
  ss init -g --force --all-targets --no-git --no-skill >/dev/null
fi
fixture="$config_dir/skills/bundled-controls"
mkdir -p "$fixture"
printf '%s\n' '---' 'name: bundled-controls' 'description: Synthetic fixture' '---' '![missing](missing.png)' '[escape](../outside.md)' > "$fixture/SKILL.md"
cat > "$config_dir/audit-rules.yaml" <<'RULES'
rules:
  - id: dangling-link
    enabled: false
  - id: reference-escapes-root
    enabled: false
RULES
ss audit -g "$fixture" --analyzer structure --format json > "$TMPDIR/bundled-audit.json"
ss doctor -g --json > "$TMPDIR/bundled-doctor.json" || test "$?" -eq 1
jq -n --slurpfile a "$TMPDIR/bundled-audit.json" --slurpfile d "$TMPDIR/bundled-doctor.json" '{audit: $a[0], doctor: $d[0]}'
```

Expected:
- exit_code: 0
- jq: [.audit.results[].findings[]?] | length == 0
- jq: [.doctor.checks[] | select(.name == "bundled_references")] | length == 0

### 4. Nested code paths preserve their explicit base

```bash
set -eu
printf 'rules: []\n' > "${XDG_CONFIG_HOME:-$HOME/.config}/skillshare/audit-rules.yaml"
fixture="$(mktemp -d "$TMPDIR/bundled-nested.XXXXXX")"
outside="$(mktemp -d "$TMPDIR/bundled-nested-outside.XXXXXX")"
mkdir -p "$fixture/references" "$fixture/scripts"
printf '# Existing\n' > "$fixture/references/auth.md"
printf '# Synthetic script; never executed\n' > "$fixture/scripts/query_design.py"
printf '# Fixture\n' > "$fixture/SKILL.md"
ln -s "$outside" "$fixture/external.bundle"
printf '%s\n' '[sibling](auth.md)' \
  'Read `references/auth.md` and `scripts/query_design.py`.' \
  'Also run `../scripts/query_design.py` from here.' 'Sibling `./auth.md`.' \
  'Read `../../outside.md`.' 'Read `path:external.bundle/missing.md`.' \
  '```skillshare-paths' 'references/auth.md' '../scripts/query_design.py' './auth.md' '```' \
  > "$fixture/references/app-flow.md"
ss audit -g "$fixture" --analyzer structure --format json
```

Expected:
- exit_code: 0
- jq: [.results[].findings[] | select(.ruleId == "dangling-link")] | length == 0
- jq: [.results[].findings[] | select(.ruleId == "reference-escapes-root" and .severity == "HIGH")] | length == 2

## Pass Criteria

All four steps pass. All inputs are synthetic; referenced files are never executed.
