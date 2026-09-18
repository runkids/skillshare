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

func TestHelp_PluginCommandsAndSyncBoundary(t *testing.T) {
	sb := testutil.NewSandbox(t)
	defer sb.Cleanup()
	result := sb.RunCLI("--help")
	result.AssertSuccess(t)
	for _, command := range []string{"PLUGIN MANAGEMENT", "plugin list", "plugin discover", "plugin add", "plugin import", "plugin inspect", "plugin sync", "plugin check", "plugin update", "plugin enable", "plugin disable", "plugin remove", "sync plugins"} {
		result.AssertOutputContains(t, command)
	}
	result = sb.RunCLI("sync", "--help")
	result.AssertSuccess(t)
	result.AssertOutputContains(t, "Plugins are not included in --all")
	result.AssertOutputContains(t, "skillshare sync plugins")
}
