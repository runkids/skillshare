# GitLab Subgroup URL Parsing E2E Runbook

## Scope

Verify `ParseSource` correctly handles GitLab nested subgroup URLs (Issue #72):
deep paths like `group/subgroup/project` are treated as full repo paths,
not as `group/subgroup` with subdir `project`. Also verify `.git` suffix
as explicit repo/subdir boundary, `/-/tree` web URLs with subgroups,
and `TrackName` output for multi-segment paths.

## Environment

- Devcontainer with rebuilt binary
- No network required — all tests are offline unit tests

## Steps

### Step 1: Run GitLab subgroup unit tests

```bash
cd /workspace
go test ./internal/install/ -run TestParseSource_GitLabSubgroups -v -count=1
```

**Expected:**
- exit_code: 0
- PASS
- gitlab_two-level_subgroup
- onprem_gitlab_deep_subgroup
- gitlab_subgroup_with_.git_subdir_boundary
- gitlab_subgroup_shorthand
- gitlab_subgroup_web_URL_with_-/tree
- Not FAIL

### Step 2: Run updated domain shorthand tests (gitlab multi-segment path)

```bash
cd /workspace
go test ./internal/install/ -run TestParseSource_DomainShorthand -v -count=1
```

**Expected:**
- exit_code: 0
- PASS
- gitlab_multi-segment_path
- gitlab_with_.git_subdir_boundary
- Not FAIL

### Step 3: Run GHE subdir tests (updated behavior)

```bash
cd /workspace
go test ./internal/install/ -run TestParseSource_GitHubEnterprise -v -count=1
```

**Expected:**
- exit_code: 0
- PASS
- GHE_Server_multi-segment_path
- GHE_Server_with_.git_subdir_boundary
- GHE_Cloud_multi-segment_path
- GHE_Cloud_with_.git_subdir_boundary
- Not FAIL

### Step 4: Run TrackName tests (subgroup paths)

```bash
cd /workspace
go test ./internal/install/ -run TestSource_TrackName -v -count=1
```

**Expected:**
- exit_code: 0
- PASS
- gitlab_subgroup_HTTPS
- gitlab_deep_subgroup_shorthand
- Not FAIL

### Step 5: Regression — Bitbucket web URL tests still pass

```bash
cd /workspace
go test ./internal/install/ -run TestParseSource_GitHTTPS -v -count=1
```

**Expected:**
- exit_code: 0
- PASS
- bitbucket_web_URL_with_src/main
- bitbucket_web_URL_nested_subdir
- gitlab_web_URL_with_-/tree/main
- gitlab_web_URL_with_-/blob/main
- Not FAIL

### Step 6: Regression — all source parsing tests pass

```bash
cd /workspace
go test ./internal/install/ -run TestParseSource -v -count=1
```

**Expected:**
- exit_code: 0
- PASS
- regex: ok\s+skillshare/internal/install
- Not FAIL

### Step 7: Regression — full install package tests pass

```bash
cd /workspace
go test ./internal/install/ -count=1
```

**Expected:**
- exit_code: 0
- regex: ok\s+skillshare/internal/install
- Not FAIL

## Pass Criteria

- Steps 1–5: All new and updated GitLab subgroup tests PASS
- Steps 6–7: Full regression suite PASS with 0 failures
