//go:build !online

package integration

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"skillshare/internal/testutil"
)

// setupExtrasOneTarget is a helper that creates a sandbox with a minimal global
// config and one extra ("rules") pointing to dir1. It returns the sandbox and
// the path of the first target directory.
func setupExtrasOneTarget(t *testing.T) (*testutil.Sandbox, string) {
	t.Helper()
	sb := testutil.NewSandbox(t)

	claudeTarget := sb.CreateTarget("claude")
	dir1 := filepath.Join(sb.Home, "extra-target-1")
	if err := os.MkdirAll(dir1, 0755); err != nil {
		t.Fatal(err)
	}

	sb.WriteConfig(`source: ` + sb.SourcePath + `
targets:
  claude:
    path: ` + claudeTarget + `
extras:
  - name: rules
    targets:
      - path: ` + dir1 + `
`)
	return sb, dir1
}

// TestExtrasTarget_AddTarget_AddsSecondTarget verifies that --add-target appends
// a new entry to the extra's targets list and persists it in the config.
func TestExtrasTarget_AddTarget_AddsSecondTarget(t *testing.T) {
	sb, dir1 := setupExtrasOneTarget(t)
	defer sb.Cleanup()

	dir2 := filepath.Join(sb.Home, "extra-target-2")
	if err := os.MkdirAll(dir2, 0755); err != nil {
		t.Fatal(err)
	}

	result := sb.RunCLI("extras", "rules", "--add-target", dir2, "--mode", "copy", "-g")
	result.AssertSuccess(t)

	configContent := sb.ReadFile(sb.ConfigPath)
	if !strings.Contains(configContent, dir1) {
		t.Errorf("config should still contain first target %s:\n%s", dir1, configContent)
	}
	if !strings.Contains(configContent, dir2) {
		t.Errorf("config should now contain second target %s:\n%s", dir2, configContent)
	}
}

func TestExtrasTarget_AddTarget_NameMatchesSubcommand(t *testing.T) {
	sb := testutil.NewSandbox(t)
	defer sb.Cleanup()

	dir1 := filepath.Join(sb.Home, "extra-target-1")
	dir2 := filepath.Join(sb.Home, "extra-target-2")
	if err := os.MkdirAll(dir1, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(dir2, 0755); err != nil {
		t.Fatal(err)
	}

	sb.WriteConfig(`source: ` + sb.SourcePath + `
extras:
  - name: list
    targets:
      - path: ` + dir1 + `
`)

	result := sb.RunCLI("extras", "list", "--add-target", dir2, "-g")
	result.AssertSuccess(t)

	configContent := sb.ReadFile(sb.ConfigPath)
	if !strings.Contains(configContent, dir1) {
		t.Errorf("config should still contain first target %s:\n%s", dir1, configContent)
	}
	if !strings.Contains(configContent, dir2) {
		t.Errorf("config should now contain second target %s:\n%s", dir2, configContent)
	}
}

func TestExtrasTarget_AddTarget_TildeDuplicateErrors(t *testing.T) {
	sb, dir1 := setupExtrasOneTarget(t)
	defer sb.Cleanup()

	tildePath := strings.Replace(dir1, sb.Home, "~", 1)
	result := sb.RunCLI("extras", "rules", "--add-target", tildePath, "-g")
	result.AssertFailure(t)
	result.AssertAnyOutputContains(t, "already exists")
}

// TestExtrasTarget_AddTarget_DuplicateErrors verifies that adding a target path
// that already exists on the extra returns a non-zero exit and reports the
// duplicate.
func TestExtrasTarget_AddTarget_DuplicateErrors(t *testing.T) {
	sb, dir1 := setupExtrasOneTarget(t)
	defer sb.Cleanup()

	result := sb.RunCLI("extras", "rules", "--add-target", dir1, "-g")
	result.AssertFailure(t)
	result.AssertAnyOutputContains(t, "already exists")
}

// TestExtrasTarget_RemoveTarget_RemovesOne verifies that --remove-target strips
// the specified entry from the config while preserving the remaining targets.
func TestExtrasTarget_RemoveTarget_RemovesOne(t *testing.T) {
	sb, dir1 := setupExtrasOneTarget(t)
	defer sb.Cleanup()

	dir2 := filepath.Join(sb.Home, "extra-target-2")
	if err := os.MkdirAll(dir2, 0755); err != nil {
		t.Fatal(err)
	}

	// First add a second target so we have two.
	addResult := sb.RunCLI("extras", "rules", "--add-target", dir2, "-g")
	addResult.AssertSuccess(t)

	// Now remove the second target.
	rmResult := sb.RunCLI("extras", "rules", "--remove-target", dir2, "-g")
	rmResult.AssertSuccess(t)

	configContent := sb.ReadFile(sb.ConfigPath)
	if !strings.Contains(configContent, dir1) {
		t.Errorf("config should still contain first target %s:\n%s", dir1, configContent)
	}
	if strings.Contains(configContent, dir2) {
		t.Errorf("config should no longer contain second target %s:\n%s", dir2, configContent)
	}
}

func TestExtrasTarget_RemoveTarget_TildePathMatchesExpandedGlobalTarget(t *testing.T) {
	sb, dir1 := setupExtrasOneTarget(t)
	defer sb.Cleanup()

	dir2 := filepath.Join(sb.Home, "extra-target-2")
	if err := os.MkdirAll(dir2, 0755); err != nil {
		t.Fatal(err)
	}
	addResult := sb.RunCLI("extras", "rules", "--add-target", dir2, "-g")
	addResult.AssertSuccess(t)

	tildePath := strings.Replace(dir1, sb.Home, "~", 1)
	rmResult := sb.RunCLI("extras", "rules", "--remove-target", tildePath, "-g")
	rmResult.AssertSuccess(t)

	configContent := sb.ReadFile(sb.ConfigPath)
	if strings.Contains(configContent, dir1) || strings.Contains(configContent, tildePath) {
		t.Errorf("config should no longer contain removed target %s:\n%s", dir1, configContent)
	}
	if !strings.Contains(configContent, dir2) {
		t.Errorf("config should still contain remaining target %s:\n%s", dir2, configContent)
	}
}

func TestExtrasTarget_RemoveTargetPrune_CopyPreservesLocalFiles(t *testing.T) {
	sb := testutil.NewSandbox(t)
	defer sb.Cleanup()

	srcDir := filepath.Join(sb.Home, ".config", "skillshare", "extras", "rules")
	sb.WriteFile(filepath.Join(srcDir, "managed.md"), "managed\n")

	dir1 := filepath.Join(sb.Home, "extra-target-1")
	dir2 := filepath.Join(sb.Home, "extra-target-2")
	sb.WriteConfig(`source: ` + sb.SourcePath + `
extras:
  - name: rules
    targets:
      - path: ` + dir1 + `
        mode: copy
      - path: ` + dir2 + `
        mode: copy
`)

	syncResult := sb.RunCLI("sync", "extras", "-g", "--force")
	syncResult.AssertSuccess(t)

	managed := filepath.Join(dir1, "managed.md")
	local := filepath.Join(dir1, "local.md")
	if _, err := os.Stat(managed); err != nil {
		t.Fatalf("expected managed file before prune: %v", err)
	}
	sb.WriteFile(local, "local\n")

	rmResult := sb.RunCLI("extras", "rules", "--remove-target", dir1, "--prune", "-g")
	rmResult.AssertSuccess(t)

	if _, err := os.Stat(managed); !os.IsNotExist(err) {
		t.Fatalf("expected managed file to be pruned, stat err = %v", err)
	}
	if _, err := os.Stat(local); err != nil {
		t.Fatalf("local file must be preserved, stat err = %v", err)
	}

	configContent := sb.ReadFile(sb.ConfigPath)
	if strings.Contains(configContent, dir1) {
		t.Errorf("config should no longer contain removed target %s:\n%s", dir1, configContent)
	}
	if !strings.Contains(configContent, dir2) {
		t.Errorf("config should still contain remaining target %s:\n%s", dir2, configContent)
	}
}

func TestExtrasTarget_RemoveTargetPrune_SymlinkTargetRealDirFailsAndKeepsConfig(t *testing.T) {
	sb := testutil.NewSandbox(t)
	defer sb.Cleanup()

	srcDir := filepath.Join(sb.Home, ".config", "skillshare", "extras", "rules")
	sb.WriteFile(filepath.Join(srcDir, "managed.md"), "managed\n")

	dir1 := filepath.Join(sb.Home, "extra-target-1")
	dir2 := filepath.Join(sb.Home, "extra-target-2")
	sb.WriteFile(filepath.Join(dir1, "local.md"), "local\n")
	if err := os.MkdirAll(dir2, 0755); err != nil {
		t.Fatal(err)
	}

	sb.WriteConfig(`source: ` + sb.SourcePath + `
extras:
  - name: rules
    targets:
      - path: ` + dir1 + `
        mode: symlink
      - path: ` + dir2 + `
        mode: copy
`)

	result := sb.RunCLI("extras", "rules", "--remove-target", dir1, "--prune", "-g")
	result.AssertFailure(t)
	result.AssertAnyOutputContains(t, "target is not a symlink")

	if _, err := os.Stat(filepath.Join(dir1, "local.md")); err != nil {
		t.Fatalf("local file must be preserved after failed prune, stat err = %v", err)
	}

	configContent := sb.ReadFile(sb.ConfigPath)
	if !strings.Contains(configContent, dir1) {
		t.Errorf("config should still contain failed target %s:\n%s", dir1, configContent)
	}
	if !strings.Contains(configContent, dir2) {
		t.Errorf("config should still contain remaining target %s:\n%s", dir2, configContent)
	}
}

func TestExtrasTarget_RemoveTargetPrune_ExtensionTargetRemovesGeneratedFiles(t *testing.T) {
	sb := testutil.NewSandbox(t)
	defer sb.Cleanup()

	extDir := filepath.Join(sb.Home, ".config", "skillshare", "extensions", "passthrough")
	if err := os.MkdirAll(extDir, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(extDir, "extension.yaml"), []byte("run: [\"cat\"]\noutput_ext: toml\n"), 0644); err != nil {
		t.Fatal(err)
	}

	srcDir := filepath.Join(sb.Home, ".config", "skillshare", "extras", "commands")
	if err := os.MkdirAll(srcDir, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(srcDir, "hello.md"), []byte("hello\n"), 0644); err != nil {
		t.Fatal(err)
	}

	dir1 := filepath.Join(sb.Home, "extra-target-1")
	dir2 := filepath.Join(sb.Home, "extra-target-2")
	sb.WriteConfig(`source: ` + sb.SourcePath + `
extras:
  - name: commands
    targets:
      - path: ` + dir1 + `
        extension: passthrough
      - path: ` + dir2 + `
        extension: passthrough
`)

	syncResult := sb.RunCLI("sync", "extras", "-g", "--force")
	syncResult.AssertSuccess(t)

	generated := filepath.Join(dir1, "hello.toml")
	if _, err := os.Stat(generated); err != nil {
		t.Fatalf("expected generated file before prune: %v", err)
	}

	rmResult := sb.RunCLI("extras", "commands", "--remove-target", dir1, "--prune", "-g")
	rmResult.AssertSuccess(t)

	if _, err := os.Stat(generated); !os.IsNotExist(err) {
		t.Fatalf("expected generated file to be pruned, stat err = %v", err)
	}

	configContent := sb.ReadFile(sb.ConfigPath)
	if strings.Contains(configContent, dir1) {
		t.Errorf("config should no longer contain removed target %s:\n%s", dir1, configContent)
	}
	if !strings.Contains(configContent, dir2) {
		t.Errorf("config should still contain remaining target %s:\n%s", dir2, configContent)
	}
}

// TestExtrasTarget_RemoveTarget_LastTargetErrors verifies that attempting to
// remove the sole remaining target returns a non-zero exit and reports the
// "last target" constraint.
func TestExtrasTarget_RemoveTarget_LastTargetErrors(t *testing.T) {
	sb, dir1 := setupExtrasOneTarget(t)
	defer sb.Cleanup()

	result := sb.RunCLI("extras", "rules", "--remove-target", dir1, "-g")
	result.AssertFailure(t)
	result.AssertAnyOutputContains(t, "last target")
}

// TestExtrasTarget_ModeSubcommandRemoved verifies that "extras mode" is no
// longer a valid subcommand. When called as "extras mode --mode copy", the word
// "mode" is treated as an extra name; since no extra with that name exists, the
// command must fail.
func TestExtrasTarget_ModeSubcommandRemoved(t *testing.T) {
	sb, _ := setupExtrasOneTarget(t)
	defer sb.Cleanup()

	result := sb.RunCLI("extras", "mode", "--mode", "copy", "-g")
	result.AssertFailure(t)
}
