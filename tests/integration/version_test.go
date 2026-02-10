//go:build !online

package integration

import (
	"testing"

	"skillshare/internal/testutil"
)

func TestVersion_ShowsVersion(t *testing.T) {
	sb := testutil.NewSandbox(t)
	defer sb.Cleanup()

	result := sb.RunCLI("version")
	result.AssertSuccess(t)
	result.AssertOutputContains(t, "skillshare")
}

func TestVersion_ShortFlag(t *testing.T) {
	sb := testutil.NewSandbox(t)
	defer sb.Cleanup()

	result := sb.RunCLI("-v")
	result.AssertSuccess(t)
	result.AssertOutputContains(t, "skillshare v")
}

func TestVersion_LongFlag(t *testing.T) {
	sb := testutil.NewSandbox(t)
	defer sb.Cleanup()

	result := sb.RunCLI("--version")
	result.AssertSuccess(t)
	result.AssertOutputContains(t, "skillshare v")
}
