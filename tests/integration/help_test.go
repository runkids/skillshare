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
	result.AssertOutputContains(t, "Skills and agents")
	result.AssertOutputContains(t, "Common options")
}

func TestHelp_ShortFlag(t *testing.T) {
	sb := testutil.NewSandbox(t)
	defer sb.Cleanup()

	result := sb.RunCLI("-h")
	result.AssertSuccess(t)
	result.AssertOutputContains(t, "Skills and agents")
}

func TestHelp_LongFlag(t *testing.T) {
	sb := testutil.NewSandbox(t)
	defer sb.Cleanup()

	result := sb.RunCLI("--help")
	result.AssertSuccess(t)
	result.AssertOutputContains(t, "Skills and agents")
}

func TestNoArgs_ShowsUsage(t *testing.T) {
	sb := testutil.NewSandbox(t)
	defer sb.Cleanup()

	result := sb.RunCLI()
	result.AssertFailure(t)
	result.AssertOutputContains(t, "Skills and agents")
}

func TestUnknownCommand_ShowsError(t *testing.T) {
	sb := testutil.NewSandbox(t)
	defer sb.Cleanup()

	result := sb.RunCLI("unknowncommand")
	result.AssertFailure(t)
	result.AssertAnyOutputContains(t, "Unknown command")
	result.AssertOutputNotContains(t, "Skills and agents")
}

func TestUnknownCommand_SuggestsClosest(t *testing.T) {
	sb := testutil.NewSandbox(t)
	defer sb.Cleanup()

	result := sb.RunCLI("instal")
	result.AssertFailure(t)
	result.AssertAnyOutputContains(t, `Did you mean "install"?`)
}

func TestLoneOption_ShowsUsage(t *testing.T) {
	sb := testutil.NewSandbox(t)
	defer sb.Cleanup()

	result := sb.RunCLI("--no-tui")
	result.AssertFailure(t)
	result.AssertOutputNotContains(t, "Unknown command")
	result.AssertOutputContains(t, "Skills and agents")
}

func TestHelp_PluginCommandsAndSyncBoundary(t *testing.T) {
	sb := testutil.NewSandbox(t)
	defer sb.Cleanup()
	result := sb.RunCLI("plugin", "--help")
	result.AssertSuccess(t)
	for _, command := range []string{"list", "discover", "add", "import", "inspect", "sync", "check", "update", "enable", "disable", "remove", "skillshare sync plugins"} {
		result.AssertOutputContains(t, command)
	}
	result = sb.RunCLI("sync", "--help")
	result.AssertSuccess(t)
	result.AssertOutputContains(t, "Plugins are not included in --all")
	result.AssertOutputContains(t, "skillshare sync plugins")
}
