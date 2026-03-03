//go:build !online

package integration

import (
	"os"
	"strings"
	"testing"

	"skillshare/internal/testutil"
)

func TestTUI_ShowDefault(t *testing.T) {
	sb := testutil.NewSandbox(t)
	defer sb.Cleanup()

	sb.WriteConfig("source: " + sb.SourcePath + "\ntargets: {}\n")

	result := sb.RunCLI("tui")
	result.AssertSuccess(t)
	result.AssertAnyOutputContains(t, "on")
	result.AssertAnyOutputContains(t, "default")
}

func TestTUI_Off(t *testing.T) {
	sb := testutil.NewSandbox(t)
	defer sb.Cleanup()

	sb.WriteConfig("source: " + sb.SourcePath + "\ntargets: {}\n")

	result := sb.RunCLI("tui", "off")
	result.AssertSuccess(t)
	result.AssertAnyOutputContains(t, "disabled")

	// Verify config file has tui: false
	data, err := os.ReadFile(sb.ConfigPath)
	if err != nil {
		t.Fatalf("read config: %v", err)
	}
	if !strings.Contains(string(data), "tui: false") {
		t.Errorf("config should contain 'tui: false', got:\n%s", data)
	}
}

func TestTUI_On(t *testing.T) {
	sb := testutil.NewSandbox(t)
	defer sb.Cleanup()

	// Start with tui: false
	sb.WriteConfig("source: " + sb.SourcePath + "\ntargets: {}\ntui: false\n")

	result := sb.RunCLI("tui", "on")
	result.AssertSuccess(t)
	result.AssertAnyOutputContains(t, "enabled")

	// Verify status shows on
	result = sb.RunCLI("tui")
	result.AssertSuccess(t)
	result.AssertAnyOutputContains(t, "on")
}

func TestTUI_InvalidArg(t *testing.T) {
	sb := testutil.NewSandbox(t)
	defer sb.Cleanup()

	sb.WriteConfig("source: " + sb.SourcePath + "\ntargets: {}\n")

	result := sb.RunCLI("tui", "banana")
	result.AssertFailure(t)
	result.AssertAnyOutputContains(t, "unknown argument")
}
