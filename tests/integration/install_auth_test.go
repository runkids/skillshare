//go:build !online

package integration

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"skillshare/internal/install"
	"skillshare/internal/testutil"
)

// TestInstall_Auth_TokenEnvDoesNotBreakFileClone verifies that setting token
// environment variables (e.g. GITHUB_TOKEN) does not interfere with file://
// protocol installs. authEnv() returns nil for non-HTTPS URLs.
func TestInstall_Auth_TokenEnvDoesNotBreakFileClone(t *testing.T) {
	sb := testutil.NewSandbox(t)
	defer sb.Cleanup()

	sb.WriteConfig(`source: ` + sb.SourcePath + `
targets: {}
`)

	gitRepoPath := filepath.Join(sb.Root, "auth-test-repo")
	os.MkdirAll(gitRepoPath, 0755)
	sb.WriteFile(filepath.Join(gitRepoPath, "SKILL.md"), "# Auth Test")
	initGitRepo(t, gitRepoPath)

	// Set token env vars — these should not affect file:// clones
	t.Setenv("GITHUB_TOKEN", "ghp_fake_token_12345")
	t.Setenv("GITLAB_TOKEN", "glpat-fake-token")

	result := sb.RunCLI("install", "file://"+gitRepoPath, "--all")

	result.AssertSuccess(t)
	result.AssertAnyOutputContains(t, "Installed")

	installedPath := filepath.Join(sb.SourcePath, "auth-test-repo", "SKILL.md")
	if !sb.FileExists(installedPath) {
		t.Error("skill should be installed despite token env vars being set")
	}
}

// TestInstall_Auth_UpdateGitPullWithTokenEnv verifies the complete update path
// (getRemoteURL → authEnv → gitPull) works when token env vars are set.
// For file:// repos, authEnv returns nil so tokens should not interfere.
func TestInstall_Auth_UpdateGitPullWithTokenEnv(t *testing.T) {
	sb := testutil.NewSandbox(t)
	defer sb.Cleanup()

	sb.WriteConfig(`source: ` + sb.SourcePath + `
targets: {}
`)

	// Create a git repo with initial content
	gitRepoPath := filepath.Join(sb.Root, "tracked-auth-skill")
	os.MkdirAll(gitRepoPath, 0755)
	sb.WriteFile(filepath.Join(gitRepoPath, "SKILL.md"), "# Version 1")
	initGitRepo(t, gitRepoPath)

	// Install as tracked repo using internal API (to get .git preserved)
	source, err := install.ParseSource("file://" + gitRepoPath)
	if err != nil {
		t.Fatalf("failed to parse source: %v", err)
	}
	_, err = install.Install(source, filepath.Join(sb.SourcePath, "tracked-auth-skill"), install.InstallOptions{Force: true})
	if err != nil {
		t.Fatalf("failed to install: %v", err)
	}

	// Update the source repo
	sb.WriteFile(filepath.Join(gitRepoPath, "SKILL.md"), "# Version 2")
	cmd := exec.Command("git", "add", ".")
	cmd.Dir = gitRepoPath
	cmd.Run()
	cmd = exec.Command("git", "commit", "-m", "v2")
	cmd.Dir = gitRepoPath
	cmd.Run()

	// Set token env vars before update
	t.Setenv("GITHUB_TOKEN", "ghp_should_not_break_file_pull")

	// Update via CLI — exercises getRemoteURL + authEnv + gitPull
	updateResult := sb.RunCLI("install", "tracked-auth-skill", "--update")
	updateResult.AssertSuccess(t)

	content := sb.ReadFile(filepath.Join(sb.SourcePath, "tracked-auth-skill", "SKILL.md"))
	if content != "# Version 2" {
		t.Errorf("expected Version 2 after update, got: %s", content)
	}
}
