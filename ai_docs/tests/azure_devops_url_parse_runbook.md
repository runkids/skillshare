# Azure DevOps URL Parsing E2E Runbook

## Scope

Verify `ParseSource` correctly handles all Azure DevOps URL formats:
modern HTTPS (`dev.azure.com`), legacy HTTPS (`visualstudio.com`),
SSH v3 (`ssh.dev.azure.com`), and `ado:` shorthand. Also verify
`TrackName` and `GitHubOwner` return correct values.

## Environment

- Devcontainer with rebuilt binary (`go build -o bin/skillshare ./cmd/skillshare`)
- No network required — all tests are offline unit tests

## Steps

### Step 1: Run Azure DevOps unit tests

```bash
cd /workspace
go test ./internal/install/ -run TestParseSource_AzureDevOps -v -count=1
```

**Expected**: All 9 subtests PASS:
- modern HTTPS, HTTPS .git, HTTPS+subdir
- legacy visualstudio.com
- SSH v3, SSH v3 .git, SSH v3+subdir
- ado: shorthand, ado:+subdir

### Step 2: Run Azure DevOps TrackName tests

```bash
cd /workspace
go test ./internal/install/ -run TestParseSource_AzureDevOps_TrackName -v -count=1
```

**Expected**: All 4 subtests PASS:
- modern HTTPS → `org-proj-repo`
- legacy visualstudio.com → `myorg-myproj-myrepo`
- SSH v3 → `org-proj-repo`
- ado: shorthand → `org-proj-repo`

### Step 3: Run Azure DevOps GitHubOwner empty tests

```bash
cd /workspace
go test ./internal/install/ -run TestParseSource_AzureDevOps_GitHubOwnerEmpty -v -count=1
```

**Expected**: All 3 subtests PASS — `GitHubOwner()` and `GitHubRepo()` return empty for Azure DevOps URLs.

### Step 4: Regression — all existing source tests still pass

```bash
cd /workspace
go test ./internal/install/ -run TestParseSource -v -count=1
```

**Expected**: All `TestParseSource_*` tests PASS (LocalPath, GitHubShorthand, GitSSH, GitHTTPS, FileURL, Errors, DomainShorthand, GitHubEnterprise, GeminiCLI, GitHubShorthandExpansion, AzureDevOps).

### Step 5: Regression — all install package tests pass

```bash
cd /workspace
go test ./internal/install/ -count=1
```

**Expected**: `ok skillshare/internal/install` with 0 failures.

### Step 6: Dry-run with Azure DevOps URL (CLI integration)

```bash
cd /workspace
bin/skillshare install --dry-run "https://dev.azure.com/testorg/testproj/_git/testrepo" 2>&1
```

**Expected**: Output contains `Source  https://dev.azure.com/testorg/testproj/_git/testrepo` — confirms CLI dispatches Azure URL to correct parser (clone fails as expected, no real repo).

### Step 7: Dry-run with ado: shorthand (CLI integration)

```bash
cd /workspace
bin/skillshare install --dry-run "ado:testorg/testproj/testrepo" 2>&1
```

**Expected**: Output contains `Source  https://dev.azure.com/testorg/testproj/_git/testrepo` — confirms `ado:` shorthand expansion works end-to-end.

## Pass Criteria

- Steps 1–5: All Go unit tests PASS with 0 failures
- Steps 6–7: CLI output shows correctly parsed Azure DevOps URL
