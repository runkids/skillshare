//go:build !online

package integration

import (
	"os"
	"path/filepath"
	"testing"

	"skillshare/internal/testutil"
)

func TestCompletion_Bash_OutputsScript(t *testing.T) {
	sb := testutil.NewSandbox(t)
	defer sb.Cleanup()

	result := sb.RunCLI("completion", "bash")
	result.AssertSuccess(t)
	result.AssertOutputContains(t, "complete -F _skillshare skillshare")
	for _, cmd := range []string{"sync", "install", "list", "target", "commit", "completion"} {
		result.AssertOutputContains(t, cmd)
	}
}

func TestCompletion_Zsh_OutputsScript(t *testing.T) {
	sb := testutil.NewSandbox(t)
	defer sb.Cleanup()

	result := sb.RunCLI("completion", "zsh")
	result.AssertSuccess(t)
	result.AssertOutputContains(t, "#compdef skillshare")
	result.AssertOutputContains(t, "_skillshare")
}

func TestCompletion_Fish_OutputsScript(t *testing.T) {
	sb := testutil.NewSandbox(t)
	defer sb.Cleanup()

	result := sb.RunCLI("completion", "fish")
	result.AssertSuccess(t)
	result.AssertOutputContains(t, "complete -c skillshare")
	result.AssertOutputContains(t, "__fish_skillshare_no_subcommand")
}

func TestCompletion_PowerShell_OutputsScript(t *testing.T) {
	sb := testutil.NewSandbox(t)
	defer sb.Cleanup()

	result := sb.RunCLI("completion", "powershell")
	result.AssertSuccess(t)
	result.AssertOutputContains(t, "Register-ArgumentCompleter")
}

func TestCompletion_Nushell_OutputsScript(t *testing.T) {
	sb := testutil.NewSandbox(t)
	defer sb.Cleanup()

	result := sb.RunCLI("completion", "nushell")
	result.AssertSuccess(t)
	result.AssertOutputContains(t, "export extern \"skillshare\"")
}

func TestCompletion_UnsupportedShell_Errors(t *testing.T) {
	sb := testutil.NewSandbox(t)
	defer sb.Cleanup()

	result := sb.RunCLI("completion", "tcsh")
	result.AssertFailure(t)
	result.AssertAnyOutputContains(t, "unsupported shell")
}

func TestCompletion_NoArgs_ShowsUsage(t *testing.T) {
	sb := testutil.NewSandbox(t)
	defer sb.Cleanup()

	result := sb.RunCLI("completion")
	result.AssertSuccess(t)
	result.AssertOutputContains(t, "USAGE")
}

func TestCompletion_Install_WritesFile(t *testing.T) {
	sb := testutil.NewSandbox(t)
	defer sb.Cleanup()

	result := sb.RunCLI("completion", "bash", "--install")
	result.AssertSuccess(t)
	result.AssertAnyOutputContains(t, "installed")

	destPath := filepath.Join(sb.Home, ".local", "share", "bash-completion", "completions", "skillshare")
	if _, err := os.Stat(destPath); os.IsNotExist(err) {
		t.Errorf("expected completion script at %s, but file does not exist", destPath)
	}
}
