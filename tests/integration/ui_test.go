//go:build !online

package integration

import (
	"path/filepath"
	"testing"

	"skillshare/internal/testutil"
)

func TestUI_NoConfig_ReturnsInitError(t *testing.T) {
	sb := testutil.NewSandbox(t)
	defer sb.Cleanup()

	result := sb.RunCLI("ui", "--no-open")

	result.AssertFailure(t)
	result.AssertAnyOutputContains(t, "run 'skillshare init' first")
}

func TestUI_SourceMissing_ReturnsInitError(t *testing.T) {
	sb := testutil.NewSandbox(t)
	defer sb.Cleanup()

	missingSource := filepath.Join(sb.Root, "missing-source")
	sb.WriteConfig(`source: ` + missingSource + `
targets: {}
`)

	result := sb.RunCLI("ui", "--no-open")

	result.AssertFailure(t)
	result.AssertAnyOutputContains(t, "run 'skillshare init' first")
}
