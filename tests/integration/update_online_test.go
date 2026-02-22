//go:build online

package integration

import (
	"testing"

	"skillshare/internal/testutil"
)

// TestUpdate_TrackedRepo installs a tracked repo then runs update.
// A freshly cloned repo should report "up to date".
func TestUpdate_TrackedRepo(t *testing.T) {
	sb := testutil.NewSandbox(t)
	defer sb.Cleanup()

	sb.WriteConfig(`source: ` + sb.SourcePath + `
targets: {}
`)

	// Step 1: install with --track
	// This repository intentionally contains malicious-pattern fixtures in tests/docs.
	// Use --force so this test validates track+update mechanics, not audit blocking policy.
	installResult := sb.RunCLI("install", "runkids/skillshare", "--track", "--name", "test-tracked", "--force")
	installResult.AssertSuccess(t)

	// Step 2: update the tracked repo (should be up to date)
	updateResult := sb.RunCLI("update", "_test-tracked")
	updateResult.AssertSuccess(t)
	updateResult.AssertAnyOutputContains(t, "up to date")
}
