# GitLab Hosts Config Override E2E Runbook

## Scope

Verify `gitlab_hosts` config field and `SKILLSHARE_GITLAB_HOSTS` env var for custom GitLab domains:
- Config loads with valid `gitlab_hosts` entries
- Invalid entries (scheme, path, port, empty) are rejected at load time
- `ParseSourceWithOptions` treats custom hosts as GitLab (full-path repo)
- Project mode config supports `gitlab_hosts`
- `SKILLSHARE_GITLAB_HOSTS` env var merges with config (deduplicated)
- Env var entries not persisted to config file on save
- Server project mode uses project config exclusively (no global fallback)
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
go test ./internal/config/ -run "TestLoadProject_GitLabHosts$" -v -count=1
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

### Step 6: Env var unit tests — config + env merged, deduped

```bash
cd /workspace
go test ./internal/config/ -run TestLoad_GitLabHosts_EnvVar -v -count=1
```

**Expected:**
- exit_code: 0
- PASS
- TestLoad_GitLabHosts_EnvVar
- Not FAIL

### Step 7: Env var unit tests — env-only (no config file hosts)

```bash
cd /workspace
go test ./internal/config/ -run TestLoad_GitLabHosts_EnvVarOnly -v -count=1
```

**Expected:**
- exit_code: 0
- PASS
- TestLoad_GitLabHosts_EnvVarOnly
- Not FAIL

### Step 8: Env var unit tests — invalid entries silently skipped

```bash
cd /workspace
go test ./internal/config/ -run TestLoad_GitLabHosts_EnvVarSkipsInvalid -v -count=1
```

**Expected:**
- exit_code: 0
- PASS
- TestLoad_GitLabHosts_EnvVarSkipsInvalid
- Not FAIL

### Step 9: Env var unit tests — env-only hosts not persisted on save

```bash
cd /workspace
go test ./internal/config/ -run TestLoad_GitLabHosts_EnvVarNotPersisted -v -count=1
```

**Expected:**
- exit_code: 0
- PASS
- TestLoad_GitLabHosts_EnvVarNotPersisted
- Not FAIL

### Step 10: Project config EffectiveGitLabHosts merges env var

```bash
cd /workspace
go test ./internal/config/ -run TestLoadProject_GitLabHosts_EffectiveGitLabHosts -v -count=1
```

**Expected:**
- exit_code: 0
- PASS
- TestLoadProject_GitLabHosts_EffectiveGitLabHosts
- Not FAIL

### Step 11: Server parseOpts — project mode isolation (no global fallback)

```bash
cd /workspace
go test ./internal/server/ -run TestParseOpts -v -count=1
```

**Expected:**
- exit_code: 0
- PASS
- TestParseOpts_GlobalMode
- TestParseOpts_ProjectMode_UsesProjectConfig
- TestParseOpts_ProjectMode_NoFallbackToGlobal
- TestParseOpts_ProjectMode_MergesEnvVar
- Not FAIL

### Step 12: Append gitlab_hosts to config and verify CLI loads it

```bash
sed -i '/^gitlab_hosts:/,$d' ~/.config/skillshare/config.yaml
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

### Step 13: CLI rejects invalid gitlab_hosts entry

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

### Step 14: CLI loads env var without error

```bash
SKILLSHARE_GITLAB_HOSTS="git.company.com, code.ci.io" ss status
```

**Expected:**
- exit_code: 0
- Source

### Step 15: Env var does not pollute config file

```bash
cp ~/.config/skillshare/config.yaml /tmp/config-before.yaml
SKILLSHARE_GITLAB_HOSTS="env-only-host.io" ss status
diff /tmp/config-before.yaml ~/.config/skillshare/config.yaml
```

**Expected:**
- exit_code: 0
- Not env-only-host

### Step 16: Project mode loads gitlab_hosts

```bash
rm -rf /tmp/test-proj
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

### Step 17: Regression — existing GitLab subgroup tests pass

```bash
cd /workspace
go test ./internal/install/ -run TestParseSource_GitLabSubgroups -v -count=1
```

**Expected:**
- exit_code: 0
- PASS
- Not FAIL

### Step 18: Regression — full install + config + server packages pass

```bash
cd /workspace
rm -rf ~/.local/share/skillshare/trash/* 2>/dev/null || true
go test ./internal/config/ ./internal/install/ ./internal/server/ -count=1
```

**Expected:**
- exit_code: 0
- regex: ok\s+skillshare/internal/config
- regex: ok\s+skillshare/internal/install
- regex: ok\s+skillshare/internal/server
- Not FAIL

## Pass Criteria

- Steps 1–5: Config field + ParseSourceWithOptions unit tests PASS
- Steps 6–10: SKILLSHARE_GITLAB_HOSTS env var unit tests PASS (merge, dedupe, skip invalid, not persisted, project effective)
- Step 11: Server project-mode parseOpts isolation PASS
- Steps 12–16: CLI-level env var + config loading tests PASS
- Steps 17–18: Full regression PASS with 0 failures
