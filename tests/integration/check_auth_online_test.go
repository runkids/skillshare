//go:build online

package integration

import (
	"os"
	"strings"
	"testing"

	"skillshare/internal/testutil"
)

// TestCheck_Auth_TrackedPrivateRepoWithToken verifies `skillshare check` can
// read update status for a tracked private HTTPS repo with token auth.
func TestCheck_Auth_TrackedPrivateRepoWithToken(t *testing.T) {
	token := os.Getenv("GITHUB_TOKEN")
	if token == "" {
		t.Skip("GITHUB_TOKEN not set, skipping private check auth test")
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
		"check-auth-private",
	)
	if installResult.ExitCode != 0 {
		t.Skip("private test repo not accessible, skipping")
	}

	// Verify generic fallback token also works for check path.
	t.Setenv("GITHUB_TOKEN", "")
	t.Setenv("SKILLSHARE_GIT_TOKEN", token)

	checkResult := sb.RunCLI("check", "_check-auth-private")
	checkResult.AssertSuccess(t)

	output := checkResult.Output()
	if strings.Contains(output, token) {
		t.Error("token should not appear in check output")
	}
}
