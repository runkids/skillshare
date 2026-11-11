# Security Boundaries

## Scope

Defensive regression verification for credential origin authorization, repository `.agents/skills` fallback discovery, and discovered-agent destination writes. Tests use dummy tokens, local HTTP test servers, and temporary filesystem/Git fixtures; do not supply real credentials or probe external hosts. This runbook does not install remote resources.

## Environment

Run each step inside the isolated Linux devcontainer with the checkout at `/workspace`, Go dependencies already cached, and Git available. Use the repository's mdproof runner for this file only; do not run on the host. `GOPROXY=off GOSUMDB=off` prevents dependency downloads. Every step is independent. These commands are verification instructions, not a record of an executed result.

## Steps

### 1. Credential authorization and browser remote-read origins

Provider-looking names and nondefault ports must not receive provider tokens. Explicit configured Gitea/CNB hosts and the generic HTTPS fallback remain supported, including grouped skill updates and tracked agent updates. GitHub clients must not forward authorization across origins, and cross-site browser searches and previews must be rejected before fetching.

```bash
set -eu
cd /workspace
export GOPROXY=off GOSUMDB=off
unset GITHUB_TOKEN GH_TOKEN GITLAB_TOKEN BITBUCKET_TOKEN BITBUCKET_USERNAME AZURE_DEVOPS_TOKEN GITEA_TOKEN CNB_TOKEN SKILLSHARE_GIT_TOKEN
go test -count=1 -v ./internal/install -run '^(TestProviderCredentialsRejectUntrustedOrigins|TestForgeRequestsScopeProviderCredentials|TestForgeFallbackPreservesCredentialOrigin)$'
go test -count=1 -v ./internal/github -run '^(TestNewRequestScopesGitHubCredentials|TestClientDoesNotForwardCredentialsAcrossOrigins)$'
go test -count=1 -v ./internal/search -run '^(TestFetchPreviewDoesNotAuthorizeProviderLookalikes|TestBuildHubRequest_GitHubHeaders)$'
go test -count=1 -v ./internal/server -run '^(TestCrossSitePreviewIsRefused|TestCrossSiteHubSearchIsRefused)$'
go test -count=1 -v ./internal/install ./cmd/skillshare -run '^(TestUpdateSkillsFromRepo_ConfiguredHosts|TestBatchUpdateTrackedAgents_ConfiguredHosts)$'
```

**Expected**

- exit_code: 0
- --- PASS: TestProviderCredentialsRejectUntrustedOrigins
- --- PASS: TestForgeRequestsScopeProviderCredentials
- --- PASS: TestForgeFallbackPreservesCredentialOrigin
- --- PASS: TestNewRequestScopesGitHubCredentials
- --- PASS: TestClientDoesNotForwardCredentialsAcrossOrigins
- --- PASS: TestFetchPreviewDoesNotAuthorizeProviderLookalikes
- --- PASS: TestBuildHubRequest_GitHubHeaders
- --- PASS: TestCrossSitePreviewIsRefused
- --- PASS: TestCrossSiteHubSearchIsRefused
- --- PASS: TestUpdateSkillsFromRepo_ConfiguredHosts
- --- PASS: TestBatchUpdateTrackedAgents_ConfiguredHosts

### 2. Real-directory fallback discovery

Linked `.agents` or `skills` components must not expose aliases of excluded skills, even when source links are followed. Real fallback directories beneath an intentionally followed repository root must still work.

```bash
set -eu
cd /workspace
export GOPROXY=off GOSUMDB=off
go test -count=1 -v ./internal/sourcewalk -run '^TestAgentsSkillsDirRequiresRealDirectories$'
go test -count=1 -v ./internal/skillserve -run '^(TestBuild_RefusesAgentsFallbackAliasesOfExcludedSkills|TestBuild_ServesRealAgentsFallbackThroughFollowedRepoRoot)$'
```

**Expected**

- exit_code: 0
- --- PASS: TestAgentsSkillsDirRequiresRealDirectories
- --- PASS: TestBuild_RefusesAgentsFallbackAliasesOfExcludedSkills
- --- PASS: TestBuild_ServesRealAgentsFallbackThroughFollowedRepoRoot

### 3. Rooted agent publication and audit preservation

Fresh, forced, and update writes must refuse intermediate and final destination links, including dangling links. An audit block must preserve installed contents. Normal installs, updates, metadata, dry runs, and supported followed roots must continue working.

```bash
set -eu
cd /workspace
export GOPROXY=off GOSUMDB=off
go test -count=1 -v ./internal/install -run '^(TestInstallAgentFromDiscovery_TrackedDestinationEscape|TestDiscoveredAgent_RefusesDestinationLinks|TestDiscoveredAgent_BlockedAuditPreservesInstalledContent|TestDiscoveredAgent_InstallUpdateMetadataAndDryRun|TestDiscoveredAgent_FollowPolicyAndLinkedSourceRoot)$'
```

**Expected**

- exit_code: 0
- --- PASS: TestInstallAgentFromDiscovery_TrackedDestinationEscape
- --- PASS: TestDiscoveredAgent_RefusesDestinationLinks
- --- PASS: TestDiscoveredAgent_BlockedAuditPreservesInstalledContent
- --- PASS: TestDiscoveredAgent_InstallUpdateMetadataAndDryRun
- --- PASS: TestDiscoveredAgent_FollowPolicyAndLinkedSourceRoot

## Pass Criteria

Every named test runs and passes; skipped or missing tests are not evidence. The fixtures verify that untrusted origins receive no provider authorization, linked fallback components are not discovered, destination links cannot redirect writes, and blocked agent audits preserve previous bytes. Tests own and clean their temporary fixtures. Windows junction and native file-link behavior additionally requires the existing Windows link runbooks on Windows; Linux results do not establish Windows verification.
