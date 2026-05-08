//go:build !online

package integration

import (
	"strings"
	"testing"

	"skillshare/internal/testutil"
)

func TestCompletion_Bash_OutputsScript(t *testing.T) {
	sb := testutil.NewSandbox(t)
	defer sb.Cleanup()

	result := sb.RunCLI("completion", "bash")
	if result.ExitCode != 0 {
		t.Fatalf("expected exit 0, got %d: %s", result.ExitCode, result.Stderr)
	}
	if !strings.Contains(result.Stdout, "complete -F _skillshare skillshare") {
		t.Error("bash script missing completion registration")
	}
	for _, cmd := range []string{"sync", "install", "list", "target", "completion"} {
		if !strings.Contains(result.Stdout, cmd) {
			t.Errorf("bash script missing command: %s", cmd)
		}
	}
}

func TestCompletion_Zsh_OutputsScript(t *testing.T) {
	sb := testutil.NewSandbox(t)
	defer sb.Cleanup()

	result := sb.RunCLI("completion", "zsh")
	if result.ExitCode != 0 {
		t.Fatalf("expected exit 0, got %d: %s", result.ExitCode, result.Stderr)
	}
	if !strings.Contains(result.Stdout, "#compdef skillshare") {
		t.Error("zsh script missing #compdef header")
	}
}

func TestCompletion_Fish_OutputsScript(t *testing.T) {
	sb := testutil.NewSandbox(t)
	defer sb.Cleanup()

	result := sb.RunCLI("completion", "fish")
	if result.ExitCode != 0 {
		t.Fatalf("expected exit 0, got %d: %s", result.ExitCode, result.Stderr)
	}
	if !strings.Contains(result.Stdout, "complete -c skillshare") {
		t.Error("fish script missing completion registration")
	}
}

func TestCompletion_PowerShell_OutputsScript(t *testing.T) {
	sb := testutil.NewSandbox(t)
	defer sb.Cleanup()

	result := sb.RunCLI("completion", "powershell")
	if result.ExitCode != 0 {
		t.Fatalf("expected exit 0, got %d: %s", result.ExitCode, result.Stderr)
	}
	if !strings.Contains(result.Stdout, "Register-ArgumentCompleter") {
		t.Error("powershell script missing Register-ArgumentCompleter")
	}
}

func TestCompletion_Nushell_OutputsScript(t *testing.T) {
	sb := testutil.NewSandbox(t)
	defer sb.Cleanup()

	result := sb.RunCLI("completion", "nushell")
	if result.ExitCode != 0 {
		t.Fatalf("expected exit 0, got %d: %s", result.ExitCode, result.Stderr)
	}
	if !strings.Contains(result.Stdout, "export extern \"skillshare\"") {
		t.Error("nushell script missing extern definition")
	}
}

func TestCompletion_UnsupportedShell_Errors(t *testing.T) {
	sb := testutil.NewSandbox(t)
	defer sb.Cleanup()

	result := sb.RunCLI("completion", "tcsh")
	if result.ExitCode == 0 {
		t.Error("expected non-zero exit for unsupported shell")
	}
	if !strings.Contains(result.Output(), "unsupported shell") {
		t.Errorf("expected 'unsupported shell' error, got: %s", result.Output())
	}
}

func TestCompletion_NoArgs_ShowsUsage(t *testing.T) {
	sb := testutil.NewSandbox(t)
	defer sb.Cleanup()

	result := sb.RunCLI("completion")
	if result.ExitCode != 0 {
		t.Fatalf("expected exit 0, got %d", result.ExitCode)
	}
	if !strings.Contains(result.Stdout, "USAGE") {
		t.Error("expected usage text when no args given")
	}
}

func TestCompletion_Install_WritesFile(t *testing.T) {
	sb := testutil.NewSandbox(t)
	defer sb.Cleanup()

	result := sb.RunCLI("completion", "bash", "--install")
	if result.ExitCode != 0 {
		t.Fatalf("expected exit 0, got %d: %s", result.ExitCode, result.Stderr)
	}
	if !strings.Contains(result.Output(), "installed") {
		t.Error("expected 'installed' confirmation message")
	}
}
