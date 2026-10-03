//go:build !online

package integration

import (
	"strings"
	"testing"

	"skillshare/internal/testutil"
)

func TestHubAdd_Basic(t *testing.T) {
	sb := testutil.NewSandbox(t)
	defer sb.Cleanup()

	sb.WriteConfig(`source: ` + sb.SourcePath + "\ntargets: {}\n")

	result := sb.RunCLI("hub", "add", "https://example.com/hub.json", "--label", "test")
	result.AssertSuccess(t)
	result.AssertAnyOutputContains(t, "Added hub")
	result.AssertAnyOutputContains(t, "test")
	// First add should auto-set default
	result.AssertAnyOutputContains(t, "default")
}

func TestHubAdd_DeriveLabel(t *testing.T) {
	sb := testutil.NewSandbox(t)
	defer sb.Cleanup()

	sb.WriteConfig(`source: ` + sb.SourcePath + "\ntargets: {}\n")

	result := sb.RunCLI("hub", "add", "https://internal.corp/skills/team-hub.json")
	result.AssertSuccess(t)
	result.AssertAnyOutputContains(t, "team-hub")
}

func TestHubAdd_Duplicate(t *testing.T) {
	sb := testutil.NewSandbox(t)
	defer sb.Cleanup()

	sb.WriteConfig(`source: ` + sb.SourcePath + "\ntargets: {}\n")

	result := sb.RunCLI("hub", "add", "https://example.com/hub.json", "--label", "test")
	result.AssertSuccess(t)

	result = sb.RunCLI("hub", "add", "https://other.com/hub.json", "--label", "test")
	result.AssertFailure(t)
	result.AssertAnyOutputContains(t, "already exists")
}

func TestHubList_Empty(t *testing.T) {
	sb := testutil.NewSandbox(t)
	defer sb.Cleanup()

	sb.WriteConfig(`source: ` + sb.SourcePath + "\ntargets: {}\n")

	result := sb.RunCLI("hub", "list")
	result.AssertSuccess(t)
	result.AssertAnyOutputContains(t, "No saved hubs")
}

func TestHubList_ShowsHubs(t *testing.T) {
	sb := testutil.NewSandbox(t)
	defer sb.Cleanup()

	sb.WriteConfig(`source: ` + sb.SourcePath + "\ntargets: {}\nhub:\n  default: alpha\n  hubs:\n    - label: alpha\n      url: https://a.com/hub.json\n    - label: beta\n      url: https://b.com/hub.json\n")

	result := sb.RunCLI("hub", "list")
	result.AssertSuccess(t)
	combined := result.Stdout + result.Stderr
	if !strings.Contains(combined, "https://a.com/hub.json · default") {
		t.Errorf("expected alpha marked as default in output, got:\n%s", combined)
	}
	result.AssertAnyOutputContains(t, "alpha")
	result.AssertAnyOutputContains(t, "beta")
}

func TestHubRemove_Basic(t *testing.T) {
	sb := testutil.NewSandbox(t)
	defer sb.Cleanup()

	sb.WriteConfig(`source: ` + sb.SourcePath + "\ntargets: {}\nhub:\n  default: test\n  hubs:\n    - label: test\n      url: https://example.com/hub.json\n")

	result := sb.RunCLI("hub", "remove", "test")
	result.AssertSuccess(t)
	result.AssertAnyOutputContains(t, "Removed")

	// List should be empty now
	result = sb.RunCLI("hub", "list")
	result.AssertSuccess(t)
	result.AssertAnyOutputContains(t, "No saved hubs")
}

func TestHubRemove_NotFound(t *testing.T) {
	sb := testutil.NewSandbox(t)
	defer sb.Cleanup()

	sb.WriteConfig(`source: ` + sb.SourcePath + "\ntargets: {}\n")

	result := sb.RunCLI("hub", "remove", "nonexistent")
	result.AssertFailure(t)
	result.AssertAnyOutputContains(t, "not found")
}

func TestHubDefault_Show(t *testing.T) {
	sb := testutil.NewSandbox(t)
	defer sb.Cleanup()

	sb.WriteConfig(`source: ` + sb.SourcePath + "\ntargets: {}\nhub:\n  default: team\n  hubs:\n    - label: team\n      url: https://example.com/hub.json\n")

	result := sb.RunCLI("hub", "default")
	result.AssertSuccess(t)
	result.AssertAnyOutputContains(t, "team")
	result.AssertAnyOutputContains(t, "https://example.com/hub.json")
}

func TestHubDefault_Set(t *testing.T) {
	sb := testutil.NewSandbox(t)
	defer sb.Cleanup()

	sb.WriteConfig(`source: ` + sb.SourcePath + "\ntargets: {}\nhub:\n  hubs:\n    - label: alpha\n      url: https://a.com/hub.json\n    - label: beta\n      url: https://b.com/hub.json\n")

	result := sb.RunCLI("hub", "default", "beta")
	result.AssertSuccess(t)
	result.AssertAnyOutputContains(t, "beta")

	// Verify it's set
	result = sb.RunCLI("hub", "default")
	result.AssertSuccess(t)
	result.AssertAnyOutputContains(t, "beta")
}

func TestHubDefault_Reset(t *testing.T) {
	sb := testutil.NewSandbox(t)
	defer sb.Cleanup()

	sb.WriteConfig(`source: ` + sb.SourcePath + "\ntargets: {}\nhub:\n  default: team\n  hubs:\n    - label: team\n      url: https://example.com/hub.json\n")

	result := sb.RunCLI("hub", "default", "--reset")
	result.AssertSuccess(t)
	result.AssertAnyOutputContains(t, "cleared")

	// Verify cleared
	result = sb.RunCLI("hub", "default")
	result.AssertSuccess(t)
	result.AssertAnyOutputContains(t, "No default")
}

func TestHubDefault_NotFound(t *testing.T) {
	sb := testutil.NewSandbox(t)
	defer sb.Cleanup()

	sb.WriteConfig(`source: ` + sb.SourcePath + "\ntargets: {}\n")

	result := sb.RunCLI("hub", "default", "nonexistent")
	result.AssertFailure(t)
	result.AssertAnyOutputContains(t, "not found")
}

func TestHub_HelpShowsSubcommands(t *testing.T) {
	sb := testutil.NewSandbox(t)
	defer sb.Cleanup()

	result := sb.RunCLI("hub", "help")
	result.AssertSuccess(t)
	result.AssertAnyOutputContains(t, "add")
	result.AssertAnyOutputContains(t, "list")
	result.AssertAnyOutputContains(t, "remove")
	result.AssertAnyOutputContains(t, "default")
	result.AssertAnyOutputContains(t, "index")
}
