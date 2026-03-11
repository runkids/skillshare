# GitLab Hosts Config Override E2E Runbook

## Scope

Verify `gitlab_hosts` config field for custom GitLab domains:
- Config loads with valid `gitlab_hosts` entries
- Invalid entries (scheme, path, port, empty) are rejected at load time
- `ParseSourceWithOptions` treats custom hosts as GitLab (full-path repo)
- Project mode config supports `gitlab_hosts`
- No regression in existing install/config workflows

## Environment

- Devcontainer with rebuilt binary
- No network required — unit tests + config validation only

## Steps

### Step 1: Config unit tests — valid gitlab_hosts roundtrip

```bash
cd /workspace
go test ./internal/config/ -run TestLoad_GitLabHosts_Valid -v -count=1
```

**Expected:**
- exit_code: 0
- PASS
- TestLoad_GitLabHosts_Valid
- Not FAIL

### Step 2: Config unit tests — invalid entries rejected

```bash
cd /workspace
go test ./internal/config/ -run TestLoad_GitLabHosts_InvalidEntries -v -count=1
```

**Expected:**
- exit_code: 0
- PASS
- scheme
- slash
- port
- empty
- Not FAIL

### Step 3: Config unit tests — omitted when empty

```bash
cd /workspace
go test ./internal/config/ -run TestLoad_GitLabHosts_OmittedWhenEmpty -v -count=1
```

**Expected:**
- exit_code: 0
- PASS
- Not FAIL

### Step 4: Project config loads gitlab_hosts

```bash
cd /workspace
go test ./internal/config/ -run TestLoadProject_GitLabHosts -v -count=1
```

**Expected:**
- exit_code: 0
- PASS
- TestLoadProject_GitLabHosts
- Not FAIL

### Step 5: ParseSourceWithOptions — custom host treated as GitLab

```bash
cd /workspace
go test ./internal/install/ -run TestParseSourceWithOptions_GitLabHosts -v -count=1
```

**Expected:**
- exit_code: 0
- PASS
- custom_host_treated_as_GitLab
- same_URL_without_opts_uses_2-segment_split
- case-insensitive_host_match
- .git_suffix_still_wins_over_host_heuristic
- /-/_marker_still_wins_over_host_heuristic
- built-in_gitlab.com_detection_unchanged
- Not FAIL

### Step 6: Append gitlab_hosts to config and verify CLI loads it

```bash
cat >> ~/.config/skillshare/config.yaml <<'EOF'
gitlab_hosts:
  - git.company.com
  - code.internal.io
EOF
ss status
```

**Expected:**
- exit_code: 0
- Source

### Step 7: CLI rejects invalid gitlab_hosts entry

```bash
cat > /tmp/bad-config.yaml <<'EOF'
source: ~/.config/skillshare/skills
targets:
  claude:
    path: ~/.claude/skills
gitlab_hosts:
  - https://git.company.com
EOF
SKILLSHARE_CONFIG=/tmp/bad-config.yaml ss status 2>&1 || true
```

**Expected:**
- must be a hostname, not a URL

### Step 8: Project mode loads gitlab_hosts

```bash
mkdir -p /tmp/test-proj/.skillshare
cat > /tmp/test-proj/.skillshare/config.yaml <<'EOF'
targets:
  - claude
gitlab_hosts:
  - git.corp.example
EOF
cd /tmp/test-proj && ss status -p
```

**Expected:**
- exit_code: 0
- Source
- Not error

### Step 9: Regression — existing GitLab subgroup tests pass

```bash
cd /workspace
go test ./internal/install/ -run TestParseSource_GitLabSubgroups -v -count=1
```

**Expected:**
- exit_code: 0
- PASS
- Not FAIL

### Step 10: Regression — full install + config + server packages pass

```bash
cd /workspace
go test ./internal/config/ ./internal/install/ ./internal/server/ -count=1
```

**Expected:**
- exit_code: 0
- regex: ok\s+skillshare/internal/config
- regex: ok\s+skillshare/internal/install
- regex: ok\s+skillshare/internal/server
- Not FAIL

## Pass Criteria

- Steps 1–5: All new gitlab_hosts unit tests PASS
- Steps 6–8: CLI correctly loads/rejects gitlab_hosts in config
- Steps 9–10: Full regression PASS with 0 failures
