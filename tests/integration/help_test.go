//go:build !online

package integration

import (
	"testing"

	"skillshare/internal/testutil"
)

func TestHelp_ShowsUsage(t *testing.T) {
	sb := testutil.NewSandbox(t)
	defer sb.Cleanup()

	result := sb.RunCLI("help")
	result.AssertSuccess(t)
	result.AssertOutputContains(t, "skillshare")
	result.AssertOutputContains(t, "CORE COMMANDS")
	result.AssertOutputContains(t, "UTILITIES")
}

func TestHelp_ShortFlag(t *testing.T) {
	sb := testutil.NewSandbox(t)
	defer sb.Cleanup()

	result := sb.RunCLI("-h")
	result.AssertSuccess(t)
	result.AssertOutputContains(t, "CORE COMMANDS")
}

func TestHelp_LongFlag(t *testing.T) {
	sb := testutil.NewSandbox(t)
	defer sb.Cleanup()

	result := sb.RunCLI("--help")
	result.AssertSuccess(t)
	result.AssertOutputContains(t, "CORE COMMANDS")
}

func TestNoArgs_ShowsUsage(t *testing.T) {
	sb := testutil.NewSandbox(t)
	defer sb.Cleanup()

	result := sb.RunCLI()
	result.AssertFailure(t)
	result.AssertOutputContains(t, "CORE COMMANDS")
}

func TestUnknownCommand_ShowsError(t *testing.T) {
	sb := testutil.NewSandbox(t)
	defer sb.Cleanup()

	result := sb.RunCLI("unknowncommand")
	result.AssertFailure(t)
	result.AssertAnyOutputContains(t, "Unknown command")
}
