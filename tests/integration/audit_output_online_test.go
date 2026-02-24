//go:build online

package integration

import (
	"strings"
	"testing"

	"skillshare/internal/testutil"
)

// TestInstall_BatchAuditOutput_Antigravity validates that install --all produces
// rich audit output: blocked/failed section, severity breakdown, hints.
func TestInstall_BatchAuditOutput_Antigravity(t *testing.T) {
	sb := testutil.NewSandbox(t)
	defer sb.Cleanup()

	projectRoot := sb.SetupProjectDir("claude")
	result := sb.RunCLIInDir(projectRoot, "install", "sickn33/antigravity-awesome-skills/skills", "--all", "-p")

	// Batch install exits 0 when some skills succeed (blocked count is a warning)
	result.AssertSuccess(t)

	// Blocked section
	result.AssertAnyOutputContains(t, "Blocked / Failed")
	result.AssertAnyOutputContains(t, "blocked by security audit")
	result.AssertAnyOutputContains(t, "CRITICAL")

	// Severity breakdown
	result.AssertAnyOutputContains(t, "finding(s) across")
	result.AssertAnyOutputContains(t, "HIGH")

	// Hint for more details
	result.AssertAnyOutputContains(t, "--audit-verbose")

	// Install count
	result.AssertAnyOutputContains(t, "Installed")

	// Next steps
	result.AssertAnyOutputContains(t, "Next Steps")
}

// TestUpdateAll_AuditOutputParity_Antigravity verifies that update --all produces
// audit output with similar richness to install --all.
func TestUpdateAll_AuditOutputParity_Antigravity(t *testing.T) {
	sb := testutil.NewSandbox(t)
	defer sb.Cleanup()

	projectRoot := sb.SetupProjectDir("claude")

	// Step 1: install (use --force to bypass blocked skills so we have something to update)
	installResult := sb.RunCLIInDir(projectRoot, "install", "sickn33/antigravity-awesome-skills/skills", "--all", "--force", "-p")
	installResult.AssertSuccess(t)

	// Step 2: update --all (nothing changed, should be clean)
	updateResult := sb.RunCLIInDir(projectRoot, "update", "--all", "-p")
	updateResult.AssertSuccess(t)

	// Audit section present
	updateResult.AssertAnyOutputContains(t, "Audit")

	// Has audit results (CLEAN or findings)
	combined := updateResult.Stdout + updateResult.Stderr
	if !(strings.Contains(combined, "CLEAN") || strings.Contains(combined, "finding(s)")) {
		t.Errorf("expected audit results (CLEAN or findings), got:\nstdout: %s\nstderr: %s",
			updateResult.Stdout, updateResult.Stderr)
	}

	// Batch summary line
	updateResult.AssertAnyOutputContains(t, "skipped")

	// No blocked skills (nothing changed upstream)
	updateResult.AssertOutputNotContains(t, "Blocked / Failed")
	updateResult.AssertOutputNotContains(t, "Blocked / Rolled Back")
}
