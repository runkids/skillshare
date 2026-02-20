//go:build online

package integration

import (
	"os"
	"path/filepath"
	"testing"

	"skillshare/internal/testutil"
)

// TestInstall_Auth_HTTPSPublicWithToken verifies that setting GITHUB_TOKEN
// does not break HTTPS clones of public repositories. The token auth injection
// should work transparently without interfering with public access.
func TestInstall_Auth_HTTPSPublicWithToken(t *testing.T) {
	sb := testutil.NewSandbox(t)
	defer sb.Cleanup()

	sb.WriteConfig(`source: ` + sb.SourcePath + `
targets: {}
`)

	t.Setenv("GITHUB_TOKEN", "ghp_this_is_a_fake_token_for_testing")

	result := sb.RunCLI("install", "runkids/skillshare/skills/skillshare", "--dry-run")

	result.AssertSuccess(t)
	result.AssertAnyOutputContains(t, "dry-run")
}

// TestInstall_Auth_PrivateRepoWithoutToken verifies the error message when
// attempting to clone a private repo without credentials. The error should
// show actionable options (SSH, token env var, credential helper).
func TestInstall_Auth_PrivateRepoWithoutToken(t *testing.T) {
	sb := testutil.NewSandbox(t)
	defer sb.Cleanup()

	sb.WriteConfig(`source: ` + sb.SourcePath + `
targets: {}
`)

	// Ensure no tokens are set
	t.Setenv("GITHUB_TOKEN", "")
	t.Setenv("GITLAB_TOKEN", "")
	t.Setenv("BITBUCKET_TOKEN", "")
	t.Setenv("SKILLSHARE_GIT_TOKEN", "")
	t.Setenv("GIT_TERMINAL_PROMPT", "0")

	// Use a known-private or non-existent repo to trigger auth error
	result := sb.RunCLI("install", "https://github.com/runkids/skillshare-private-test-do-not-create.git")

	result.AssertFailure(t)
	// Should show helpful auth options, not raw git error
	result.AssertAnyOutputContains(t, "authentication required")
}

// TestInstall_Auth_PrivateRepoWithToken verifies that HTTPS clone of a private
// repo succeeds when the correct token is set. Requires GITHUB_TOKEN env var
// with access to the test repo.
func TestInstall_Auth_PrivateRepoWithToken(t *testing.T) {
	token := os.Getenv("GITHUB_TOKEN")
	if token == "" {
		t.Skip("GITHUB_TOKEN not set, skipping private repo auth test")
	}

	sb := testutil.NewSandbox(t)
	defer sb.Cleanup()

	sb.WriteConfig(`source: ` + sb.SourcePath + `
targets: {}
`)

	// This test requires a private repo accessible with the provided token.
	// The repo runkids/skillshare-private-test must exist and contain a
	// SKILL.md. Skip if the repo doesn't exist or token lacks access.
	result := sb.RunCLI("install", "https://github.com/runkids/skillshare-private-test.git", "--dry-run")

	if result.ExitCode != 0 {
		t.Skip("private test repo not accessible, skipping")
	}
	result.AssertAnyOutputContains(t, "dry-run")

	// Verify no token leaks in output
	output := result.Output()
	if containsSubstring(output, token) {
		t.Error("token should not appear in CLI output")
	}
}

// TestInstall_Auth_TrackedUpdateWithToken verifies that updating a tracked repo
// via HTTPS works when a token is set. The git pull should use the injected auth.
func TestInstall_Auth_TrackedUpdateWithToken(t *testing.T) {
	token := os.Getenv("GITHUB_TOKEN")
	if token == "" {
		t.Skip("GITHUB_TOKEN not set, skipping tracked update auth test")
	}

	sb := testutil.NewSandbox(t)
	defer sb.Cleanup()

	sb.WriteConfig(`source: ` + sb.SourcePath + `
targets: {}
`)

	// Install a public repo as tracked (creates .git, enabling git pull)
	installResult := sb.RunCLI("install", "runkids/skillshare", "--track", "--name", "auth-tracked-test")
	installResult.AssertSuccess(t)

	trackedDir := filepath.Join(sb.SourcePath, "_auth-tracked-test")
	if !sb.FileExists(trackedDir) {
		t.Fatal("tracked repo should be installed")
	}

	// Update should succeed — git pull with token injection
	updateResult := sb.RunCLI("install", "_auth-tracked-test", "--update")
	updateResult.AssertSuccess(t)

	// Verify no token leaks
	output := updateResult.Output()
	if containsSubstring(output, token) {
		t.Error("token should not appear in update output")
	}
}
