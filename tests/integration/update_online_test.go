//go:build online

package integration

import (
	"testing"

	"skillshare/internal/testutil"
)

// TestUpdate_TrackedRepo installs a tracked repo then runs update.
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

	// Step 2: update the tracked repo. A push to main can land during the
	// clone, so the update may pull new commits instead of being up to date;
	// both must succeed. --force for the same reason as the install.
	updateResult := sb.RunCLI("update", "_test-tracked", "--force")
	updateResult.AssertSuccess(t)
	updateResult.AssertAnyOutputContains(t, "Update    _test-tracked")
}
