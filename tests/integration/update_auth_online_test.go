//go:build online

package integration

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"skillshare/internal/testutil"
)

// TestUpdate_Auth_TrackedPrivateRepoWithToken verifies `skillshare update`
// can pull a tracked private HTTPS repo when token auth is provided via env.
func TestUpdate_Auth_TrackedPrivateRepoWithToken(t *testing.T) {
	token := os.Getenv("GITHUB_TOKEN")
	if token == "" {
		t.Skip("GITHUB_TOKEN not set, skipping private update auth test")
	}

	sb := testutil.NewSandbox(t)
	defer sb.Cleanup()

	sb.WriteConfig(`source: ` + sb.SourcePath + `
targets: {}
`)

	installResult := sb.RunCLI(
		"install",
		"https://github.com/runkids/skillshare-private-test.git",
		"--track",
		"--name",
		"update-auth-private",
	)
	if installResult.ExitCode != 0 {
		t.Skip("private test repo not accessible, skipping")
	}

	trackedDir := filepath.Join(sb.SourcePath, "_update-auth-private")
	if !sb.FileExists(trackedDir) {
		t.Fatal("tracked repo should be installed")
	}

	// Verify generic token fallback also works for update path.
	t.Setenv("GITHUB_TOKEN", "")
	t.Setenv("SKILLSHARE_GIT_TOKEN", token)

	updateResult := sb.RunCLI("update", "_update-auth-private")
	updateResult.AssertSuccess(t)

	output := updateResult.Output()
	if strings.Contains(output, token) {
		t.Error("token should not appear in update output")
	}
}
