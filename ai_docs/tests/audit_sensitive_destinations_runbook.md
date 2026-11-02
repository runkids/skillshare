# CLI E2E Runbook: Sensitive Destinations and Deserialization

Validate static destination/deserializer evidence, language filters and rule applicability. Fixture content is written as text and never executed.

## Scope

- Metadata prose warns at HIGH; executable Markdown requests whose parsed hostname matches a metadata host are CRITICAL. A public URL with a metadata address only in its path or query remains HIGH.
- Extensionless shebang scripts retain CRITICAL metadata findings; public-host twins stay clean.
- Unsafe PHP and Python calls are reported; call-local safe forms stay negative.
- Effective file filters appear in rule listings and invalid globs fail closed.

## Environment

Run inside the devcontainer with a fresh ssenv and the current worktree binary on PATH. All fixtures and configuration live in the isolated HOME. No network request or deserialization is executed.

## Steps

### 1. Create fixture text and scan at the critical threshold

```bash
mkdir -p "$HOME/item10-skill" "$HOME/.config/skillshare"
cat > "$HOME/item10-skill/SKILL.md" <<'FIXTURE'
---
name: item10-fixture
description: Static audit fixture
---
Metadata reference: http://169.254.169.254/latest
Inline guidance: `curl http://169.254.169.254` is a metadata request.
FIXTURE
printf '%s\n' '```sh' 'curl http://169.254.169.254/latest' '```' >> "$HOME/item10-skill/SKILL.md"
cat > "$HOME/item10-skill/public-url.py" <<'FIXTURE'
path_url = "https://ipinfo.io/169.254.169.254/json"
query_url = "https://ipinfo.io/lookup?ip=169.254.169.254"
FIXTURE
cat > "$HOME/item10-skill/unsafe.py" <<'FIXTURE'
yaml.load(a, Loader=yaml.SafeLoader); yaml.load(b)
torch.load(a, weights_only=True); torch.load(b)
FIXTURE
cat > "$HOME/item10-skill/safe.py" <<'FIXTURE'
yaml.safe_load(data)
yaml.load(data, Loader=yaml.SafeLoader)
torch.load(path, weights_only=True)
FIXTURE
cat > "$HOME/item10-skill/unsafe.php" <<'FIXTURE'
<?php unserialize($data);
FIXTURE
skillshare audit "$HOME/item10-skill" -g --format json > "$HOME/item10-scan.json"
status=$?
cat "$HOME/item10-scan.json"
test "$status" -eq 1 || exit 1
skillshare install "$HOME/item10-skill" -g --yes > "$HOME/item10-install-blocked.log" 2>&1
status=$?
test "$status" -eq 1
```

**Expected**

- exit_code: 0
- jq: `.results[0].findings | any(.ruleId == "ssrf-metadata-0" and .severity == "CRITICAL")`
- jq: `.results[0].findings | any(.ruleId == "ssrf-metadata-0" and .severity == "HIGH")`
- jq: `[.results[0].findings[] | select(.ruleId == "ssrf-metadata-0" and .file == "public-url.py" and .severity == "HIGH")] | length == 2`
- jq: `[.results[0].findings[] | select(.ruleId == "ssrf-metadata-0" and .file == "SKILL.md" and .severity == "CRITICAL")] | length == 2`
- jq: `.results[0].findings | any(.ruleId == "unsafe-deserialization-php" and .file == "unsafe.php")`
- jq: `[.results[0].findings[] | select(.pattern == "unsafe-deserialization" and .file == "unsafe.py")] | length == 2`
- jq: `[.results[0].findings[] | select(.pattern == "unsafe-deserialization" and .file == "safe.py")] | length == 0`

### 2. Prose alone warns at the default threshold and blocks at HIGH

```bash
mkdir -p "$HOME/item10-prose"
cat > "$HOME/item10-prose/SKILL.md" <<'FIXTURE'
---
name: item10-prose
description: Static prose fixture
---
Never call the instance metadata service at 169.254.169.254.
FIXTURE
skillshare audit "$HOME/item10-prose" -g --format json > "$HOME/item10-prose.json"
status=$?
test "$status" -eq 0 || exit 1
skillshare audit "$HOME/item10-prose" -g --threshold high --format json > "$HOME/item10-prose-high.json"
status=$?
test "$status" -eq 1 || exit 1
skillshare install "$HOME/item10-prose" -g --yes > "$HOME/item10-install-prose.log" 2>&1
status=$?
test "$status" -eq 0 || exit 1
cat "$HOME/item10-prose.json"
```

**Expected**

- exit_code: 0
- jq: `.results[0].findings | any(.ruleId == "ssrf-metadata-0" and .severity == "HIGH")`
- jq: `[.results[0].findings[] | select(.severity == "CRITICAL")] | length == 0`

### 3. Override applicability and list effective rules

```bash
cat > "$HOME/.config/skillshare/audit-rules.yaml" <<'RULES'
rules:
  - id: unsafe-deserialization-python
    files: ['scripts/*.py']
RULES
skillshare audit rules -g --format json
```

**Expected**

- exit_code: 0
- jq: `.rules | any(.id == "unsafe-deserialization-python" and .files == ["scripts/*.py"] and .source == "global")`

### 4. Reject malformed applicability even on a disabled rule

```bash
cat > "$HOME/.config/skillshare/audit-rules.yaml" <<'RULES'
rules:
  - id: unsafe-deserialization-python
    enabled: false
    files: ['[']
RULES
skillshare audit rules -g --format json 2>&1
```

**Expected**

- exit_code: 2
- invalid files glob

### 5. Reject malformed applicability during an actual scan

```bash
skillshare audit "$HOME/item10-skill" -g --format json 2>&1
```

**Expected**

- exit_code: 2
- invalid files glob

### 6. Restore the isolated rule file

```bash
printf 'rules: []\n' > "$HOME/.config/skillshare/audit-rules.yaml"
skillshare audit rules -g --format json
```

**Expected**

- exit_code: 0
- jq: `.rules | any(.id == "unsafe-deserialization-python" and .files == ["*.py"])`

### 7. Extensionless shebang scripts retain blocking metadata evidence

```bash
mkdir -p "$HOME/item10-script/scripts" "$HOME/item10-public/scripts"
printf -- '---\nname: item10-script\ndescription: Static script fixture\n---\n' > "$HOME/item10-script/SKILL.md"
printf -- '---\nname: item10-public\ndescription: Static public fixture\n---\n' > "$HOME/item10-public/SKILL.md"
cat > "$HOME/item10-script/scripts/creds" <<'FIXTURE'
#!/bin/bash
T=$(curl -X PUT "http://169.254.169.254/latest/api/token" -H "X-aws-ec2-metadata-token-ttl-seconds: 60")
curl --silent -H "X-aws-ec2-metadata-token: $T" http://169.254.169.254/latest/meta-data/iam/security-credentials/
FIXTURE
sed 's/169\.254\.169\.254/example.com/g' "$HOME/item10-script/scripts/creds" > "$HOME/item10-public/scripts/creds"
skillshare audit "$HOME/item10-script" -g --format json > "$HOME/item10-script.json"
status=$?
test "$status" -eq 1 || exit 1
jq -e '[.results[0].findings[] | select(.ruleId == "ssrf-metadata-0" and .severity == "CRITICAL" and .file == "scripts/creds")] | length == 2' "$HOME/item10-script.json" >/dev/null || exit 1
skillshare install "$HOME/item10-script" -g --yes > "$HOME/item10-install-script.log" 2>&1
status=$?
test "$status" -eq 1 || exit 1
test ! -e "$HOME/.config/skillshare/skills/item10-script" || exit 1
skillshare audit "$HOME/item10-public" -g --format json > "$HOME/item10-public.json"
status=$?
test "$status" -eq 0 || exit 1
skillshare install "$HOME/item10-public" -g --yes > "$HOME/item10-install-public.log" 2>&1
status=$?
test "$status" -eq 0 || exit 1
test -f "$HOME/.config/skillshare/skills/item10-public/scripts/creds" || exit 1
cat "$HOME/item10-public.json"
```

**Expected**

- exit_code: 0
- jq: `(.results | length) == 1 and ((.results[0].findings // []) | length) == 0`

## Pass Criteria

All steps pass, findings refer only to fixture text, and safe forms suppress only the matching call.
