//go:build !online

package integration

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"skillshare/internal/testutil"
)

// TestExtras_Init_Global verifies that "extras init" creates the source directory
// and persists the extra entry in the global config.
func TestExtras_Init_Global(t *testing.T) {
	sb := testutil.NewSandbox(t)
	defer sb.Cleanup()

	// Need at least a minimal config so config.Load() succeeds.
	claudeTarget := sb.CreateTarget("claude")
	sb.WriteConfig(`source: ` + sb.SourcePath + `
targets:
  claude:
    path: ` + claudeTarget + `
`)

	rulesTarget := filepath.Join(sb.Home, ".claude", "rules")

	result := sb.RunCLI("extras", "init", "rules", "--target", rulesTarget, "-g")

	result.AssertSuccess(t)
	result.AssertAnyOutputContains(t, "Created extras/rules/")

	// Verify source directory was created under extras/
	sourceDir := filepath.Join(sb.Home, ".config", "skillshare", "extras", "rules")
	if _, err := os.Stat(sourceDir); os.IsNotExist(err) {
		t.Errorf("expected extras source dir %s to exist", sourceDir)
	}

	// Verify config.yaml now contains extras section
	configContent := sb.ReadFile(sb.ConfigPath)
	if !strings.Contains(configContent, "extras:") {
		t.Errorf("expected config to contain 'extras:', got:\n%s", configContent)
	}
	if !strings.Contains(configContent, "rules") {
		t.Errorf("expected config to contain 'rules', got:\n%s", configContent)
	}
}

// TestExtras_Init_InvalidName verifies that names with invalid characters are rejected.
func TestExtras_Init_InvalidName(t *testing.T) {
	sb := testutil.NewSandbox(t)
	defer sb.Cleanup()

	claudeTarget := sb.CreateTarget("claude")
	sb.WriteConfig(`source: ` + sb.SourcePath + `
targets:
  claude:
    path: ` + claudeTarget + `
`)

	result := sb.RunCLI("extras", "init", "../bad", "--target", "/tmp/x", "-g")

	result.AssertFailure(t)
	result.AssertAnyOutputContains(t, "invalid")
}

// TestExtras_Init_ReservedName verifies that reserved names (e.g. "skills") are rejected.
func TestExtras_Init_ReservedName(t *testing.T) {
	sb := testutil.NewSandbox(t)
	defer sb.Cleanup()

	claudeTarget := sb.CreateTarget("claude")
	sb.WriteConfig(`source: ` + sb.SourcePath + `
targets:
  claude:
    path: ` + claudeTarget + `
`)

	result := sb.RunCLI("extras", "init", "skills", "--target", "/tmp/x", "-g")

	result.AssertFailure(t)
	result.AssertAnyOutputContains(t, "reserved")
}

// TestExtras_List_Empty verifies that "extras list" reports nothing when no extras are configured.
func TestExtras_List_Empty(t *testing.T) {
	sb := testutil.NewSandbox(t)
	defer sb.Cleanup()

	claudeTarget := sb.CreateTarget("claude")
	sb.WriteConfig(`source: ` + sb.SourcePath + `
targets:
  claude:
    path: ` + claudeTarget + `
`)

	result := sb.RunCLI("extras", "list", "-g")

	result.AssertSuccess(t)
	result.AssertAnyOutputContains(t, "No extras configured")
}

// TestExtras_List_WithExtras verifies that "extras list" shows extra name and file count.
func TestExtras_List_WithExtras(t *testing.T) {
	sb := testutil.NewSandbox(t)
	defer sb.Cleanup()

	rulesTarget := filepath.Join(sb.Home, ".claude", "rules")
	os.MkdirAll(rulesTarget, 0755)

	// Create extras source directory with files.
	rulesSource := filepath.Join(sb.Home, ".config", "skillshare", "extras", "rules")
	os.MkdirAll(rulesSource, 0755)
	os.WriteFile(filepath.Join(rulesSource, "coding.md"), []byte("# Coding"), 0644)
	os.WriteFile(filepath.Join(rulesSource, "testing.md"), []byte("# Testing"), 0644)

	claudeTarget := sb.CreateTarget("claude")
	sb.WriteConfig(`source: ` + sb.SourcePath + `
targets:
  claude:
    path: ` + claudeTarget + `
extras:
  - name: rules
    targets:
      - path: ` + rulesTarget + `
`)

	result := sb.RunCLI("extras", "list", "-g")

	result.AssertSuccess(t)
	// Header and extra name should be present.
	result.AssertAnyOutputContains(t, "Extras")
	result.AssertAnyOutputContains(t, "rules")
	// File count should be shown in the source line.
	result.AssertAnyOutputContains(t, "2 files")
}

// TestExtras_Remove verifies that "extras remove --force" removes the entry from config
// while leaving source files intact.
func TestExtras_Remove(t *testing.T) {
	sb := testutil.NewSandbox(t)
	defer sb.Cleanup()

	rulesTarget := filepath.Join(sb.Home, ".claude", "rules")
	os.MkdirAll(rulesTarget, 0755)

	// Create extras source with a file.
	rulesSource := filepath.Join(sb.Home, ".config", "skillshare", "extras", "rules")
	os.MkdirAll(rulesSource, 0755)
	os.WriteFile(filepath.Join(rulesSource, "coding.md"), []byte("# Coding"), 0644)

	claudeTarget := sb.CreateTarget("claude")
	sb.WriteConfig(`source: ` + sb.SourcePath + `
targets:
  claude:
    path: ` + claudeTarget + `
extras:
  - name: rules
    targets:
      - path: ` + rulesTarget + `
`)

	result := sb.RunCLI("extras", "remove", "rules", "--force", "-g")

	result.AssertSuccess(t)

	// Config should no longer contain the extras entry.
	configContent := sb.ReadFile(sb.ConfigPath)
	if strings.Contains(configContent, "name: rules") {
		t.Errorf("expected config to no longer contain 'name: rules', got:\n%s", configContent)
	}

	// Source files should still exist.
	sourceFile := filepath.Join(rulesSource, "coding.md")
	if !sb.FileExists(sourceFile) {
		t.Error("source file should be preserved after remove")
	}
}

// TestExtras_SyncExtras_Global verifies that "sync extras" syncs files from the
// extras source directory into the configured target.
func TestExtras_SyncExtras_Global(t *testing.T) {
	sb := testutil.NewSandbox(t)
	defer sb.Cleanup()

	// Create extras source with files.
	rulesSource := filepath.Join(sb.Home, ".config", "skillshare", "extras", "rules")
	os.MkdirAll(rulesSource, 0755)
	os.WriteFile(filepath.Join(rulesSource, "coding.md"), []byte("# Coding"), 0644)

	rulesTarget := filepath.Join(sb.Home, ".claude", "rules")
	os.MkdirAll(rulesTarget, 0755)

	claudeTarget := sb.CreateTarget("claude")
	sb.WriteConfig(`source: ` + sb.SourcePath + `
targets:
  claude:
    path: ` + claudeTarget + `
extras:
  - name: rules
    targets:
      - path: ` + rulesTarget + `
`)

	result := sb.RunCLI("sync", "extras", "-g")

	result.AssertSuccess(t)
	// Header should show "Sync Extras"
	result.AssertAnyOutputContains(t, "Sync Extras")
	// Sync verb or file count should appear
	result.AssertAnyOutputContains(t, "synced")

	// Verify file was symlinked into target.
	codingLink := filepath.Join(rulesTarget, "coding.md")
	if !sb.IsSymlink(codingLink) {
		t.Error("coding.md should be a symlink in target after sync")
	}
}

// TestExtras_Init_Duplicate verifies that initialising an extra with an already-used
// name is rejected.
func TestExtras_Init_Duplicate(t *testing.T) {
	sb := testutil.NewSandbox(t)
	defer sb.Cleanup()

	rulesTarget := filepath.Join(sb.Home, ".claude", "rules")
	os.MkdirAll(rulesTarget, 0755)

	claudeTarget := sb.CreateTarget("claude")
	// Config already has an extra named "rules".
	sb.WriteConfig(`source: ` + sb.SourcePath + `
targets:
  claude:
    path: ` + claudeTarget + `
extras:
  - name: rules
    targets:
      - path: ` + rulesTarget + `
`)

	result := sb.RunCLI("extras", "init", "rules", "--target", rulesTarget, "-g")

	result.AssertFailure(t)
	result.AssertAnyOutputContains(t, "already exists")
}

// TestExtras_Init_Force_Global verifies that "extras init --force" overwrites
// an existing extra in global mode.
func TestExtras_Init_Force_Global(t *testing.T) {
	sb := testutil.NewSandbox(t)
	defer sb.Cleanup()

	oldTarget := filepath.Join(sb.Home, ".claude", "rules")
	newTarget := filepath.Join(sb.Home, ".cursor", "rules")
	os.MkdirAll(oldTarget, 0755)
	os.MkdirAll(newTarget, 0755)

	claudeTarget := sb.CreateTarget("claude")
	// Config already has an extra named "rules" pointing to oldTarget.
	sb.WriteConfig(`source: ` + sb.SourcePath + `
targets:
  claude:
    path: ` + claudeTarget + `
extras:
  - name: rules
    targets:
      - path: ` + oldTarget + `
`)

	// Re-init with --force and a different target.
	result := sb.RunCLI("extras", "init", "rules", "--target", newTarget, "--force", "-g")

	result.AssertSuccess(t)
	result.AssertAnyOutputContains(t, "Created extras/rules/")

	// Config should contain the new target, not the old one.
	configContent := sb.ReadFile(sb.ConfigPath)
	if !strings.Contains(configContent, newTarget) {
		t.Errorf("expected config to contain new target %s, got:\n%s", newTarget, configContent)
	}
	if strings.Contains(configContent, oldTarget) {
		t.Errorf("expected config to NOT contain old target %s, got:\n%s", oldTarget, configContent)
	}
}

// TestExtras_Init_Force_Project verifies that "extras init --force -p" overwrites
// an existing extra in project mode.
func TestExtras_Init_Force_Project(t *testing.T) {
	sb := testutil.NewSandbox(t)
	defer sb.Cleanup()

	projectRoot := sb.SetupProjectDir("claude")

	oldTarget := filepath.Join(projectRoot, ".claude", "rules")
	newTarget := filepath.Join(projectRoot, ".claude", "prompts")
	os.MkdirAll(oldTarget, 0755)
	os.MkdirAll(newTarget, 0755)

	// Project config already has an extra named "rules".
	sb.WriteProjectConfig(projectRoot, `targets:
  - claude
extras:
  - name: rules
    targets:
      - path: `+oldTarget+`
`)

	// Re-init with --force and a different target.
	result := sb.RunCLIInDir(projectRoot, "extras", "init", "rules", "--target", newTarget, "--force", "-p")

	result.AssertSuccess(t)
	result.AssertAnyOutputContains(t, "Created .skillshare/extras/rules/")

	// Project config should contain the new target, not the old one.
	projConfigPath := filepath.Join(projectRoot, ".skillshare", "config.yaml")
	configContent := sb.ReadFile(projConfigPath)
	if !strings.Contains(configContent, newTarget) {
		t.Errorf("expected project config to contain new target %s, got:\n%s", newTarget, configContent)
	}
	if strings.Contains(configContent, oldTarget) {
		t.Errorf("expected project config to NOT contain old target %s, got:\n%s", oldTarget, configContent)
	}
}

// TestExtras_Init_SourceNotSupportedInProjectMode verifies that --source is
// rejected in project mode with a clear error message.
func TestExtras_Init_SourceNotSupportedInProjectMode(t *testing.T) {
	sb := testutil.NewSandbox(t)
	defer sb.Cleanup()

	projectRoot := sb.SetupProjectDir("claude")

	target := filepath.Join(projectRoot, ".claude", "rules")
	os.MkdirAll(target, 0755)

	result := sb.RunCLIInDir(projectRoot, "extras", "init", "rules",
		"--target", target,
		"--source", "/some/custom/path",
		"-p")

	result.AssertFailure(t)
	result.AssertAnyOutputContains(t, "--source is not supported in project mode")
}

// TestExtras_Collect verifies that "extras collect" moves local (non-symlink) files
// from a target directory into the extras source and replaces them with symlinks.
func TestExtras_Collect(t *testing.T) {
	sb := testutil.NewSandbox(t)
	defer sb.Cleanup()

	// Create extras source directory.
	rulesSource := filepath.Join(sb.Home, ".config", "skillshare", "extras", "rules")
	os.MkdirAll(rulesSource, 0755)

	// Create target directory with a local (non-symlink) file.
	rulesTarget := filepath.Join(sb.Home, ".claude", "rules")
	os.MkdirAll(rulesTarget, 0755)
	localFile := filepath.Join(rulesTarget, "local-rule.md")
	os.WriteFile(localFile, []byte("# Local Rule"), 0644)

	claudeTarget := sb.CreateTarget("claude")
	sb.WriteConfig(`source: ` + sb.SourcePath + `
targets:
  claude:
    path: ` + claudeTarget + `
extras:
  - name: rules
    targets:
      - path: ` + rulesTarget + `
`)

	result := sb.RunCLI("extras", "collect", "rules", "-g")

	result.AssertSuccess(t)
	result.AssertAnyOutputContains(t, "collected")

	// File should now exist in source.
	sourceFile := filepath.Join(rulesSource, "local-rule.md")
	if !sb.FileExists(sourceFile) {
		t.Error("collected file should exist in extras source directory")
	}

	// Original location should now be a symlink.
	if !sb.IsSymlink(localFile) {
		t.Error("original file should be replaced with a symlink after collect")
	}

	// Symlink should point to the source file.
	if got := sb.SymlinkTarget(localFile); got != sourceFile {
		t.Errorf("symlink target = %q, want %q", got, sourceFile)
	}
}

// TestExtras_Collect_DryRun verifies that "extras collect --dry-run" reports what
// would be collected without actually moving any files.
func TestExtras_Collect_DryRun(t *testing.T) {
	sb := testutil.NewSandbox(t)
	defer sb.Cleanup()

	// Create extras source directory.
	rulesSource := filepath.Join(sb.Home, ".config", "skillshare", "extras", "rules")
	os.MkdirAll(rulesSource, 0755)

	// Create target directory with a local file.
	rulesTarget := filepath.Join(sb.Home, ".claude", "rules")
	os.MkdirAll(rulesTarget, 0755)
	localFile := filepath.Join(rulesTarget, "dry-rule.md")
	os.WriteFile(localFile, []byte("# Dry Rule"), 0644)

	claudeTarget := sb.CreateTarget("claude")
	sb.WriteConfig(`source: ` + sb.SourcePath + `
targets:
  claude:
    path: ` + claudeTarget + `
extras:
  - name: rules
    targets:
      - path: ` + rulesTarget + `
`)

	result := sb.RunCLI("extras", "collect", "rules", "--dry-run", "-g")

	result.AssertSuccess(t)
	result.AssertAnyOutputContains(t, "would collect")

	// File should NOT have been moved to source.
	sourceFile := filepath.Join(rulesSource, "dry-rule.md")
	if sb.FileExists(sourceFile) {
		t.Error("dry run should not move file to source")
	}

	// Original file should still be a regular file, not a symlink.
	if sb.IsSymlink(localFile) {
		t.Error("dry run should not replace file with symlink")
	}
}

// TestExtras_Status verifies that "status" shows extras information when extras are configured.
func TestExtras_Status(t *testing.T) {
	sb := testutil.NewSandbox(t)
	defer sb.Cleanup()

	// Create extras source with files.
	rulesSource := filepath.Join(sb.Home, ".config", "skillshare", "extras", "rules")
	os.MkdirAll(rulesSource, 0755)
	os.WriteFile(filepath.Join(rulesSource, "coding.md"), []byte("# Coding"), 0644)
	os.WriteFile(filepath.Join(rulesSource, "testing.md"), []byte("# Testing"), 0644)

	rulesTarget := filepath.Join(sb.Home, ".claude", "rules")
	os.MkdirAll(rulesTarget, 0755)

	claudeTarget := sb.CreateTarget("claude")
	sb.WriteConfig(`source: ` + sb.SourcePath + `
targets:
  claude:
    path: ` + claudeTarget + `
extras:
  - name: rules
    targets:
      - path: ` + rulesTarget + `
`)

	result := sb.RunCLI("status", "-g")

	result.AssertSuccess(t)
	// Status should show an "Extras" section.
	result.AssertAnyOutputContains(t, "Extras")
	// Should report file count and target path.
	result.AssertAnyOutputContains(t, "2 files")
	result.AssertAnyOutputContains(t, rulesTarget)
}

// TestExtras_DiffExtras verifies that "diff" automatically shows extras that need syncing.
func TestExtras_DiffExtras(t *testing.T) {
	sb := testutil.NewSandbox(t)
	defer sb.Cleanup()

	// Create extras source with a file.
	rulesSource := filepath.Join(sb.Home, ".config", "skillshare", "extras", "rules")
	os.MkdirAll(rulesSource, 0755)
	os.WriteFile(filepath.Join(rulesSource, "pending.md"), []byte("# Pending"), 0644)

	// Create target directory but do NOT sync yet.
	rulesTarget := filepath.Join(sb.Home, ".claude", "rules")
	os.MkdirAll(rulesTarget, 0755)

	claudeTarget := sb.CreateTarget("claude")
	sb.WriteConfig(`source: ` + sb.SourcePath + `
targets:
  claude:
    path: ` + claudeTarget + `
extras:
  - name: rules
    targets:
      - path: ` + rulesTarget + `
`)

	result := sb.RunCLI("diff", "--no-tui", "-g")

	result.AssertSuccess(t)
	// Output should show the extras diff section.
	result.AssertAnyOutputContains(t, "Extras")
	// Should indicate file needs syncing.
	result.AssertAnyOutputContains(t, "pending.md")
}

// TestExtras_SyncExtras_JSON verifies that "sync extras --json" produces valid JSON
// output with an extras array.
func TestExtras_SyncExtras_JSON(t *testing.T) {
	sb := testutil.NewSandbox(t)
	defer sb.Cleanup()

	// Create extras source with a file.
	rulesSource := filepath.Join(sb.Home, ".config", "skillshare", "extras", "rules")
	os.MkdirAll(rulesSource, 0755)
	os.WriteFile(filepath.Join(rulesSource, "coding.md"), []byte("# Coding"), 0644)

	rulesTarget := filepath.Join(sb.Home, ".claude", "rules")
	os.MkdirAll(rulesTarget, 0755)

	claudeTarget := sb.CreateTarget("claude")
	sb.WriteConfig(`source: ` + sb.SourcePath + `
targets:
  claude:
    path: ` + claudeTarget + `
extras:
  - name: rules
    targets:
      - path: ` + rulesTarget + `
`)

	result := sb.RunCLI("sync", "extras", "--json", "-g")

	result.AssertSuccess(t)
	// Verify JSON structure.
	if !strings.Contains(result.Stdout, `"extras"`) {
		t.Errorf("expected JSON to contain 'extras' key, got:\n%s", result.Stdout)
	}
	if !strings.Contains(result.Stdout, `"rules"`) {
		t.Errorf("expected JSON to contain 'rules' entry, got:\n%s", result.Stdout)
	}
	if !strings.Contains(result.Stdout, `"duration"`) {
		t.Errorf("expected JSON to contain 'duration' key, got:\n%s", result.Stdout)
	}
}

// TestExtras_SyncAll_Project verifies that "sync --all -p" syncs both skills and extras
// in project mode.
func TestExtras_SyncAll_Project(t *testing.T) {
	sb := testutil.NewSandbox(t)
	defer sb.Cleanup()

	// Set up a project directory with a claude target.
	projectRoot := sb.SetupProjectDir("claude")

	// Create a project skill.
	sb.CreateProjectSkill(projectRoot, "proj-skill", map[string]string{
		"SKILL.md": "# Project Skill",
	})

	// Create project extras source.
	extrasSource := filepath.Join(projectRoot, ".skillshare", "extras", "rules")
	os.MkdirAll(extrasSource, 0755)
	os.WriteFile(filepath.Join(extrasSource, "proj-rule.md"), []byte("# Project Rule"), 0644)

	// Create extras target directory.
	extrasTarget := filepath.Join(projectRoot, ".claude", "rules")
	os.MkdirAll(extrasTarget, 0755)

	// Write project config with both skills target and extras using absolute path.
	sb.WriteProjectConfig(projectRoot, `targets:
  - claude
extras:
  - name: rules
    targets:
      - path: `+extrasTarget+`
`)

	result := sb.RunCLIInDir(projectRoot, "sync", "--all", "-p")

	result.AssertSuccess(t)

	// Verify extras were synced (symlink created in target).
	ruleLink := filepath.Join(extrasTarget, "proj-rule.md")
	if !sb.IsSymlink(ruleLink) {
		t.Errorf("project extras should be synced as symlink after sync --all -p\nstdout: %s\nstderr: %s",
			result.Stdout, result.Stderr)
	}
}

// TestExtras_Doctor verifies that "doctor" reports a missing extras source directory as an error.
func TestExtras_Doctor(t *testing.T) {
	sb := testutil.NewSandbox(t)
	defer sb.Cleanup()

	rulesTarget := filepath.Join(sb.Home, ".claude", "rules")
	os.MkdirAll(rulesTarget, 0755)

	claudeTarget := sb.CreateTarget("claude")
	// Config references extras "rules" but we do NOT create the source directory.
	sb.WriteConfig(`source: ` + sb.SourcePath + `
targets:
  claude:
    path: ` + claudeTarget + `
extras:
  - name: rules
    targets:
      - path: ` + rulesTarget + `
`)

	result := sb.RunCLI("doctor", "-g")

	result.AssertSuccess(t)
	// Doctor should report the missing extras source.
	result.AssertAnyOutputContains(t, "rules")
	result.AssertAnyOutputContains(t, "missing")
}

// TestExtras_Migration verifies that "sync extras" migrates a legacy flat extras directory
// (configDir/name/) to the new location (configDir/extras/name/).
func TestExtras_Migration(t *testing.T) {
	sb := testutil.NewSandbox(t)
	defer sb.Cleanup()

	// Create the OLD-style (legacy) directory: configDir/rules/ (not extras/rules/).
	configDir := filepath.Join(sb.Home, ".config", "skillshare")
	legacySource := filepath.Join(configDir, "rules")
	os.MkdirAll(legacySource, 0755)
	os.WriteFile(filepath.Join(legacySource, "legacy.md"), []byte("# Legacy Rule"), 0644)

	rulesTarget := filepath.Join(sb.Home, ".claude", "rules")
	os.MkdirAll(rulesTarget, 0755)

	claudeTarget := sb.CreateTarget("claude")
	sb.WriteConfig(`source: ` + sb.SourcePath + `
targets:
  claude:
    path: ` + claudeTarget + `
extras:
  - name: rules
    targets:
      - path: ` + rulesTarget + `
`)

	result := sb.RunCLI("sync", "extras", "-g")

	result.AssertSuccess(t)

	// After migration, the new location should exist and legacy should be gone.
	newSource := filepath.Join(configDir, "extras", "rules")
	if _, err := os.Stat(newSource); os.IsNotExist(err) {
		t.Errorf("expected migrated directory at %s to exist", newSource)
	}
	if _, err := os.Stat(legacySource); err == nil {
		t.Errorf("expected legacy directory %s to be removed after migration", legacySource)
	}

	// The file should be accessible from the new location.
	migratedFile := filepath.Join(newSource, "legacy.md")
	if !sb.FileExists(migratedFile) {
		t.Error("migrated file should exist in new extras directory")
	}

	// File should be synced as symlink in target.
	ruleLink := filepath.Join(rulesTarget, "legacy.md")
	if !sb.IsSymlink(ruleLink) {
		t.Error("migrated file should be synced as symlink in target")
	}
}

// TestExtras_Mode_SingleTarget verifies that "extras mode" changes the sync mode
// of an extra's target when the extra has only one target (auto-resolved).
func TestExtras_Mode_SingleTarget(t *testing.T) {
	sb := testutil.NewSandbox(t)
	defer sb.Cleanup()

	rulesTarget := filepath.Join(sb.Home, ".claude", "rules")
	os.MkdirAll(rulesTarget, 0755)

	rulesSource := filepath.Join(sb.Home, ".config", "skillshare", "extras", "rules")
	os.MkdirAll(rulesSource, 0755)
	os.WriteFile(filepath.Join(rulesSource, "coding.md"), []byte("# Coding"), 0644)

	claudeTarget := sb.CreateTarget("claude")
	sb.WriteConfig(`source: ` + sb.SourcePath + `
targets:
  claude:
    path: ` + claudeTarget + `
extras:
  - name: rules
    targets:
      - path: ` + rulesTarget + `
`)

	// Change mode from merge (default) to copy
	result := sb.RunCLI("extras", "mode", "rules", "--mode", "copy", "-g")

	result.AssertSuccess(t)
	result.AssertAnyOutputContains(t, "copy")

	// Verify config was updated
	configContent := sb.ReadFile(sb.ConfigPath)
	if !strings.Contains(configContent, "mode: copy") {
		t.Errorf("expected config to contain 'mode: copy', got:\n%s", configContent)
	}
}

// TestExtras_Mode_Shorthand verifies the shorthand form:
// "extras <name> --mode <mode>" (without the "mode" subcommand).
func TestExtras_Mode_Shorthand(t *testing.T) {
	sb := testutil.NewSandbox(t)
	defer sb.Cleanup()

	rulesTarget := filepath.Join(sb.Home, ".claude", "rules")
	os.MkdirAll(rulesTarget, 0755)

	rulesSource := filepath.Join(sb.Home, ".config", "skillshare", "extras", "rules")
	os.MkdirAll(rulesSource, 0755)

	claudeTarget := sb.CreateTarget("claude")
	sb.WriteConfig(`source: ` + sb.SourcePath + `
targets:
  claude:
    path: ` + claudeTarget + `
extras:
  - name: rules
    targets:
      - path: ` + rulesTarget + `
`)

	// Use shorthand: extras rules --mode symlink
	result := sb.RunCLI("extras", "rules", "--mode", "symlink", "-g")

	result.AssertSuccess(t)
	result.AssertAnyOutputContains(t, "symlink")

	configContent := sb.ReadFile(sb.ConfigPath)
	if !strings.Contains(configContent, "mode: symlink") {
		t.Errorf("expected config to contain 'mode: symlink', got:\n%s", configContent)
	}
}

// TestExtras_Mode_WithTarget verifies that "extras mode" with --target
// changes the mode on a specific target when multiple targets exist.
func TestExtras_Mode_WithTarget(t *testing.T) {
	sb := testutil.NewSandbox(t)
	defer sb.Cleanup()

	rulesTarget1 := filepath.Join(sb.Home, ".claude", "rules")
	rulesTarget2 := filepath.Join(sb.Home, ".cursor", "rules")
	os.MkdirAll(rulesTarget1, 0755)
	os.MkdirAll(rulesTarget2, 0755)

	rulesSource := filepath.Join(sb.Home, ".config", "skillshare", "extras", "rules")
	os.MkdirAll(rulesSource, 0755)

	claudeTarget := sb.CreateTarget("claude")
	sb.WriteConfig(`source: ` + sb.SourcePath + `
targets:
  claude:
    path: ` + claudeTarget + `
extras:
  - name: rules
    targets:
      - path: ` + rulesTarget1 + `
      - path: ` + rulesTarget2 + `
`)

	// Change mode of second target only
	result := sb.RunCLI("extras", "mode", "rules", "--target", rulesTarget2, "--mode", "copy", "-g")

	result.AssertSuccess(t)

	configContent := sb.ReadFile(sb.ConfigPath)
	// The second target should have mode: copy, first should remain default (merge)
	if !strings.Contains(configContent, "mode: copy") {
		t.Errorf("expected config to contain 'mode: copy' for second target, got:\n%s", configContent)
	}
}

// TestExtras_Mode_MultipleTargets_NoTarget verifies that "extras mode" errors
// when the extra has multiple targets and --target is not specified.
func TestExtras_Mode_MultipleTargets_NoTarget(t *testing.T) {
	sb := testutil.NewSandbox(t)
	defer sb.Cleanup()

	rulesTarget1 := filepath.Join(sb.Home, ".claude", "rules")
	rulesTarget2 := filepath.Join(sb.Home, ".cursor", "rules")
	os.MkdirAll(rulesTarget1, 0755)
	os.MkdirAll(rulesTarget2, 0755)

	claudeTarget := sb.CreateTarget("claude")
	sb.WriteConfig(`source: ` + sb.SourcePath + `
targets:
  claude:
    path: ` + claudeTarget + `
extras:
  - name: rules
    targets:
      - path: ` + rulesTarget1 + `
      - path: ` + rulesTarget2 + `
`)

	result := sb.RunCLI("extras", "mode", "rules", "--mode", "copy", "-g")

	result.AssertFailure(t)
	result.AssertAnyOutputContains(t, "--target")
}

// TestExtras_Mode_InvalidMode verifies that an invalid mode is rejected.
func TestExtras_Mode_InvalidMode(t *testing.T) {
	sb := testutil.NewSandbox(t)
	defer sb.Cleanup()

	rulesTarget := filepath.Join(sb.Home, ".claude", "rules")
	os.MkdirAll(rulesTarget, 0755)

	claudeTarget := sb.CreateTarget("claude")
	sb.WriteConfig(`source: ` + sb.SourcePath + `
targets:
  claude:
    path: ` + claudeTarget + `
extras:
  - name: rules
    targets:
      - path: ` + rulesTarget + `
`)

	result := sb.RunCLI("extras", "mode", "rules", "--mode", "invalid", "-g")

	result.AssertFailure(t)
	result.AssertAnyOutputContains(t, "invalid")
}

// TestExtras_Mode_UnknownExtra verifies that mode change on non-existent extra fails.
func TestExtras_Mode_UnknownExtra(t *testing.T) {
	sb := testutil.NewSandbox(t)
	defer sb.Cleanup()

	claudeTarget := sb.CreateTarget("claude")
	sb.WriteConfig(`source: ` + sb.SourcePath + `
targets:
  claude:
    path: ` + claudeTarget + `
`)

	result := sb.RunCLI("extras", "mode", "nonexistent", "--mode", "copy", "-g")

	result.AssertFailure(t)
	result.AssertAnyOutputContains(t, "not found")
}

// TestInit_SetsExtrasSource verifies that "init" sets extras_source in the config
// to the default extras directory (sibling of source).
func TestInit_SetsExtrasSource(t *testing.T) {
	sb := testutil.NewSandbox(t)
	defer sb.Cleanup()

	// Remove config file to simulate fresh state
	os.Remove(sb.ConfigPath)

	// Run a fully non-interactive init
	result := sb.RunCLI("init", "--no-copy", "--no-targets", "--no-git", "--no-skill")
	result.AssertSuccess(t)

	// Verify config contains extras_source
	configContent := sb.ReadFile(sb.ConfigPath)
	if !strings.Contains(configContent, "extras_source:") {
		t.Errorf("expected config to contain 'extras_source:', got:\n%s", configContent)
	}

	// Verify it points to the extras dir (sibling of skills source)
	expected := filepath.Join(filepath.Dir(sb.SourcePath), "extras")
	if !strings.Contains(configContent, expected) {
		t.Errorf("expected extras_source to contain %s, got:\n%s", expected, configContent)
	}
}

// TestExtrasInit_BackfillsExtrasSource verifies that "extras init" backfills
// extras_source when it is missing from an existing config.
func TestExtrasInit_BackfillsExtrasSource(t *testing.T) {
	sb := testutil.NewSandbox(t)
	defer sb.Cleanup()

	claudeTarget := sb.CreateTarget("claude")
	rulesTarget := filepath.Join(sb.Home, ".claude", "rules")

	// Write config WITHOUT extras_source
	sb.WriteConfig(fmt.Sprintf("source: %s\ntargets:\n  claude:\n    path: %s",
		sb.SourcePath, claudeTarget))

	// Verify extras_source is absent before
	configBefore := sb.ReadFile(sb.ConfigPath)
	if strings.Contains(configBefore, "extras_source:") {
		t.Fatalf("precondition: expected no extras_source, got:\n%s", configBefore)
	}

	// Run extras init
	result := sb.RunCLI("extras", "init", "rules", "--target", rulesTarget, "-g")
	result.AssertSuccess(t)

	// Verify extras_source was backfilled
	configAfter := sb.ReadFile(sb.ConfigPath)
	if !strings.Contains(configAfter, "extras_source:") {
		t.Errorf("expected extras_source to be backfilled after extras init, got:\n%s", configAfter)
	}
	expected := filepath.Join(filepath.Dir(sb.SourcePath), "extras")
	if !strings.Contains(configAfter, expected) {
		t.Errorf("expected extras_source %s, got:\n%s", expected, configAfter)
	}
}

// TestExtrasInit_WithExtrasSource verifies that when extras_source is set in config,
// "extras init" creates the source dir under the custom extras_source path.
func TestExtrasInit_WithExtrasSource(t *testing.T) {
	sb := testutil.NewSandbox(t)
	defer sb.Cleanup()

	customExtras := filepath.Join(sb.Home, "custom-extras")
	claudeTarget := sb.CreateTarget("claude")
	rulesTarget := filepath.Join(sb.Home, ".claude", "rules")

	sb.WriteConfig("source: " + sb.SourcePath + "\nextras_source: " + customExtras + "\ntargets:\n  claude:\n    path: " + claudeTarget)

	result := sb.RunCLI("extras", "init", "rules", "--target", rulesTarget, "-g")

	result.AssertSuccess(t)

	// Source directory should be created under extras_source, not the default location.
	rulesDir := filepath.Join(customExtras, "rules")
	if _, err := os.Stat(rulesDir); os.IsNotExist(err) {
		t.Errorf("expected extras source dir at %s\nstdout: %s\nstderr: %s", rulesDir, result.Stdout, result.Stderr)
	}

	// The default location should NOT have been created.
	defaultDir := filepath.Join(sb.Home, ".config", "skillshare", "extras", "rules")
	if _, err := os.Stat(defaultDir); err == nil {
		t.Errorf("default extras dir %s should not exist when extras_source is set", defaultDir)
	}
}

// TestExtrasInit_WithPerExtraSource verifies that --source flag writes the per-extra
// source path to config and creates the source directory at that location.
func TestExtrasInit_WithPerExtraSource(t *testing.T) {
	sb := testutil.NewSandbox(t)
	defer sb.Cleanup()

	customSource := filepath.Join(sb.Home, "my-rules")
	os.MkdirAll(customSource, 0o755)

	claudeTarget := sb.CreateTarget("claude")
	rulesTarget := filepath.Join(sb.Home, ".claude", "rules")

	sb.WriteConfig("source: " + sb.SourcePath + "\ntargets:\n  claude:\n    path: " + claudeTarget)

	result := sb.RunCLI("extras", "init", "rules",
		"--target", rulesTarget,
		"--source", customSource,
		"-g")

	result.AssertSuccess(t)

	// Config should contain the per-extra source path.
	configContent := sb.ReadFile(sb.ConfigPath)
	if !strings.Contains(configContent, customSource) {
		t.Errorf("expected config to contain source %s, got:\n%s", customSource, configContent)
	}
}

// TestExtrasList_SourceType verifies JSON output includes correct source_type
// for per-extra source overrides.
func TestExtrasList_SourceType(t *testing.T) {
	sb := testutil.NewSandbox(t)
	defer sb.Cleanup()

	customSource := filepath.Join(sb.Home, "my-rules")
	os.MkdirAll(customSource, 0o755)

	claudeTarget := sb.CreateTarget("claude")
	rulesTarget := filepath.Join(sb.Home, ".claude", "rules")
	os.MkdirAll(rulesTarget, 0o755)

	sb.WriteConfig("source: " + sb.SourcePath + "\ntargets:\n  claude:\n    path: " + claudeTarget + "\nextras:\n  - name: rules\n    source: " + customSource + "\n    targets:\n      - path: " + rulesTarget)

	result := sb.RunCLI("extras", "list", "--json", "-g")

	result.AssertSuccess(t)
	result.AssertAnyOutputContains(t, "per-extra")
}

// TestExtrasSourcePriority verifies the 3-level source resolution priority chain:
// per-extra source > extras_source > default.
func TestExtrasSourcePriority(t *testing.T) {
	sb := testutil.NewSandbox(t)
	defer sb.Cleanup()

	perExtra := filepath.Join(sb.Home, "per-extra-rules")
	globalExtras := filepath.Join(sb.Home, "global-extras")
	os.MkdirAll(perExtra, 0o755)
	os.MkdirAll(filepath.Join(globalExtras, "commands"), 0o755)

	claudeTarget := sb.CreateTarget("claude")
	rulesTarget := filepath.Join(sb.Home, ".claude", "rules")
	commandsTarget := filepath.Join(sb.Home, ".claude", "commands")
	os.MkdirAll(rulesTarget, 0o755)
	os.MkdirAll(commandsTarget, 0o755)

	sb.WriteConfig("source: " + sb.SourcePath +
		"\nextras_source: " + globalExtras +
		"\ntargets:\n  claude:\n    path: " + claudeTarget +
		"\nextras:\n  - name: rules\n    source: " + perExtra +
		"\n    targets:\n      - path: " + rulesTarget +
		"\n  - name: commands\n    targets:\n      - path: " + commandsTarget)

	result := sb.RunCLI("extras", "list", "--json", "-g")

	result.AssertSuccess(t)
	// "rules" has per-extra source → source_type should be "per-extra"
	result.AssertAnyOutputContains(t, "per-extra")
	// "commands" falls back to extras_source → source_type should be "extras_source"
	result.AssertAnyOutputContains(t, "extras_source")
}

// TestExtrasList_DefaultSourceType verifies that "extras list --json" shows
// source_type "default" when neither extras_source nor per-extra source is set.
func TestExtrasList_DefaultSourceType(t *testing.T) {
	sb := testutil.NewSandbox(t)
	defer sb.Cleanup()

	claudeTarget := sb.CreateTarget("claude")
	rulesTarget := filepath.Join(sb.Home, ".claude", "rules")
	os.MkdirAll(rulesTarget, 0o755)

	// Create default extras source directory with a file so it reports as existing.
	defaultSource := filepath.Join(sb.Home, ".config", "skillshare", "extras", "rules")
	os.MkdirAll(defaultSource, 0o755)
	os.WriteFile(filepath.Join(defaultSource, "coding.md"), []byte("# Coding"), 0o644)

	// Config with extras but NO extras_source and NO per-extra source.
	sb.WriteConfig("source: " + sb.SourcePath +
		"\ntargets:\n  claude:\n    path: " + claudeTarget +
		"\nextras:\n  - name: rules\n    targets:\n      - path: " + rulesTarget)

	result := sb.RunCLI("extras", "list", "--json", "-g")

	result.AssertSuccess(t)
	result.AssertAnyOutputContains(t, `"source_type"`)
	result.AssertAnyOutputContains(t, `"default"`)
}

// TestExtrasSync_WithPerExtraSource verifies that sync uses the per-extra source path
// instead of the default location.
func TestExtrasSync_WithPerExtraSource(t *testing.T) {
	sb := testutil.NewSandbox(t)
	defer sb.Cleanup()

	claudeTarget := sb.CreateTarget("claude")

	// Create a custom source directory with a file.
	customSource := filepath.Join(sb.Home, "my-rules")
	os.MkdirAll(customSource, 0o755)
	os.WriteFile(filepath.Join(customSource, "custom.md"), []byte("# Custom Rule"), 0o644)

	// Create target directory.
	rulesTarget := filepath.Join(sb.Home, ".claude", "rules")
	os.MkdirAll(rulesTarget, 0o755)

	// Config with per-extra source pointing to custom location.
	sb.WriteConfig("source: " + sb.SourcePath +
		"\ntargets:\n  claude:\n    path: " + claudeTarget +
		"\nextras:\n  - name: rules\n    source: " + customSource +
		"\n    targets:\n      - path: " + rulesTarget)

	result := sb.RunCLI("sync", "extras", "-g")

	result.AssertSuccess(t)

	// Verify symlink was created in target.
	customLink := filepath.Join(rulesTarget, "custom.md")
	if !sb.IsSymlink(customLink) {
		t.Errorf("custom.md should be a symlink in target after sync\nstdout: %s\nstderr: %s",
			result.Stdout, result.Stderr)
	}

	// Verify symlink points to the custom source, not the default.
	expectedTarget := filepath.Join(customSource, "custom.md")
	if got := sb.SymlinkTarget(customLink); got != expectedTarget {
		t.Errorf("symlink target = %q, want %q", got, expectedTarget)
	}
}

// TestExtrasSync_WithExtrasSource verifies that sync uses extras_source when set.
func TestExtrasSync_WithExtrasSource(t *testing.T) {
	sb := testutil.NewSandbox(t)
	defer sb.Cleanup()

	claudeTarget := sb.CreateTarget("claude")

	// Create custom extras_source directory with the extra's subdirectory.
	globalExtras := filepath.Join(sb.Home, "custom-extras")
	rulesSource := filepath.Join(globalExtras, "rules")
	os.MkdirAll(rulesSource, 0o755)
	os.WriteFile(filepath.Join(rulesSource, "global-extra.md"), []byte("# Global Extra"), 0o644)

	// Create target directory.
	rulesTarget := filepath.Join(sb.Home, ".claude", "rules")
	os.MkdirAll(rulesTarget, 0o755)

	// Config with extras_source set (no per-extra source).
	sb.WriteConfig("source: " + sb.SourcePath +
		"\nextras_source: " + globalExtras +
		"\ntargets:\n  claude:\n    path: " + claudeTarget +
		"\nextras:\n  - name: rules\n    targets:\n      - path: " + rulesTarget)

	result := sb.RunCLI("sync", "extras", "-g")

	result.AssertSuccess(t)

	// Verify symlink was created from the extras_source location.
	link := filepath.Join(rulesTarget, "global-extra.md")
	if !sb.IsSymlink(link) {
		t.Errorf("global-extra.md should be a symlink after sync\nstdout: %s\nstderr: %s",
			result.Stdout, result.Stderr)
	}

	expectedTarget := filepath.Join(rulesSource, "global-extra.md")
	if got := sb.SymlinkTarget(link); got != expectedTarget {
		t.Errorf("symlink target = %q, want %q", got, expectedTarget)
	}
}

// TestExtrasCollect_WithPerExtraSource verifies that collect moves files into
// the per-extra source directory, not the default location.
func TestExtrasCollect_WithPerExtraSource(t *testing.T) {
	sb := testutil.NewSandbox(t)
	defer sb.Cleanup()

	claudeTarget := sb.CreateTarget("claude")

	// Create empty custom source directory.
	customSource := filepath.Join(sb.Home, "my-rules")
	os.MkdirAll(customSource, 0o755)

	// Create target directory with a real (non-symlink) file.
	rulesTarget := filepath.Join(sb.Home, ".claude", "rules")
	os.MkdirAll(rulesTarget, 0o755)
	localFile := filepath.Join(rulesTarget, "local-rule.md")
	os.WriteFile(localFile, []byte("# Local Rule"), 0o644)

	// Config with per-extra source.
	sb.WriteConfig("source: " + sb.SourcePath +
		"\ntargets:\n  claude:\n    path: " + claudeTarget +
		"\nextras:\n  - name: rules\n    source: " + customSource +
		"\n    targets:\n      - path: " + rulesTarget)

	result := sb.RunCLI("extras", "collect", "rules", "-g")

	result.AssertSuccess(t)
	result.AssertAnyOutputContains(t, "collected")

	// File should now exist in the custom source, not the default.
	collectedFile := filepath.Join(customSource, "local-rule.md")
	if !sb.FileExists(collectedFile) {
		t.Errorf("collected file should exist in custom source %s", customSource)
	}

	// Default source location should NOT have the file.
	defaultSource := filepath.Join(sb.Home, ".config", "skillshare", "extras", "rules", "local-rule.md")
	if sb.FileExists(defaultSource) {
		t.Error("file should NOT be collected to the default source when per-extra source is set")
	}

	// Original location should now be a symlink pointing to custom source.
	if !sb.IsSymlink(localFile) {
		t.Error("original file should be replaced with a symlink after collect")
	}
	if got := sb.SymlinkTarget(localFile); got != collectedFile {
		t.Errorf("symlink target = %q, want %q", got, collectedFile)
	}
}

// TestExtrasDoctor_WithCustomSource verifies that doctor reports the correct
// custom source path when per-extra source is configured.
func TestExtrasDoctor_WithCustomSource(t *testing.T) {
	sb := testutil.NewSandbox(t)
	defer sb.Cleanup()

	claudeTarget := sb.CreateTarget("claude")
	rulesTarget := filepath.Join(sb.Home, ".claude", "rules")
	os.MkdirAll(rulesTarget, 0o755)

	// Config with per-extra source pointing to a non-existent directory.
	nonExistentSource := filepath.Join(sb.Home, "does-not-exist-rules")
	sb.WriteConfig("source: " + sb.SourcePath +
		"\ntargets:\n  claude:\n    path: " + claudeTarget +
		"\nextras:\n  - name: rules\n    source: " + nonExistentSource +
		"\n    targets:\n      - path: " + rulesTarget)

	result := sb.RunCLI("doctor", "-g")

	result.AssertSuccess(t)
	// Doctor should mention the custom path in its extras check.
	result.AssertAnyOutputContains(t, "rules")
	result.AssertAnyOutputContains(t, "missing")
}

// TestExtrasRemove_WithPerExtraSource verifies that remove works correctly
// when per-extra source is configured.
func TestExtrasRemove_WithPerExtraSource(t *testing.T) {
	sb := testutil.NewSandbox(t)
	defer sb.Cleanup()

	claudeTarget := sb.CreateTarget("claude")
	rulesTarget := filepath.Join(sb.Home, ".claude", "rules")
	os.MkdirAll(rulesTarget, 0o755)

	// Create custom source directory with a file.
	customSource := filepath.Join(sb.Home, "my-rules")
	os.MkdirAll(customSource, 0o755)
	os.WriteFile(filepath.Join(customSource, "coding.md"), []byte("# Coding"), 0o644)

	// Config with per-extra source.
	sb.WriteConfig("source: " + sb.SourcePath +
		"\ntargets:\n  claude:\n    path: " + claudeTarget +
		"\nextras:\n  - name: rules\n    source: " + customSource +
		"\n    targets:\n      - path: " + rulesTarget)

	result := sb.RunCLI("extras", "remove", "rules", "--force", "-g")

	result.AssertSuccess(t)

	// Config should no longer contain the extras entry.
	configContent := sb.ReadFile(sb.ConfigPath)
	if strings.Contains(configContent, "name: rules") {
		t.Errorf("expected config to no longer contain 'name: rules', got:\n%s", configContent)
	}

	// Source files should still exist (remove only removes from config).
	if !sb.FileExists(filepath.Join(customSource, "coding.md")) {
		t.Error("source file in custom path should be preserved after remove")
	}
}

// TestExtrasSync_AutoCreatesSourceDir verifies that "sync extras" auto-creates
// the extras source directory when it does not exist.
func TestExtrasSync_AutoCreatesSourceDir(t *testing.T) {
	sb := testutil.NewSandbox(t)
	defer sb.Cleanup()

	// Set extras_source to a non-existent directory
	customExtras := filepath.Join(sb.Home, "new-extras-dir")
	// Don't create it — sync should auto-create

	claudeTarget := sb.CreateTarget("claude")
	rulesTarget := filepath.Join(sb.Home, ".claude", "rules")
	os.MkdirAll(rulesTarget, 0755)

	sb.WriteConfig(fmt.Sprintf("source: %s\nextras_source: %s\ntargets:\n  claude:\n    path: %s\nextras:\n  - name: rules\n    targets:\n      - path: %s",
		sb.SourcePath, customExtras, claudeTarget, rulesTarget))

	result := sb.RunCLI("sync", "extras", "-g")
	result.AssertSuccess(t)

	// Verify source dir was auto-created
	rulesDir := filepath.Join(customExtras, "rules")
	if _, err := os.Stat(rulesDir); os.IsNotExist(err) {
		t.Errorf("expected sync to auto-create source dir at %s", rulesDir)
	}

	// Verify output mentions creation
	result.AssertAnyOutputContains(t, "Created source directory")
}

// TestExtrasInit_WithExtrasSource_AutoCreatesDir verifies that "extras init"
// creates the source directory under extras_source even if the extras_source
// directory itself doesn't exist yet.
func TestExtrasInit_WithExtrasSource_AutoCreatesDir(t *testing.T) {
	sb := testutil.NewSandbox(t)
	defer sb.Cleanup()

	claudeTarget := sb.CreateTarget("claude")
	rulesTarget := filepath.Join(sb.Home, ".claude", "rules")

	// extras_source points to a path where parent exists but extras_source itself doesn't.
	customExtras := filepath.Join(sb.Home, "brand-new-extras")
	// Intentionally do NOT create customExtras — init should create it.

	sb.WriteConfig("source: " + sb.SourcePath +
		"\nextras_source: " + customExtras +
		"\ntargets:\n  claude:\n    path: " + claudeTarget)

	result := sb.RunCLI("extras", "init", "rules", "--target", rulesTarget, "-g")

	result.AssertSuccess(t)

	// The directory should have been auto-created at extras_source/rules.
	rulesDir := filepath.Join(customExtras, "rules")
	if _, err := os.Stat(rulesDir); os.IsNotExist(err) {
		t.Errorf("expected extras source dir at %s to be auto-created\nstdout: %s\nstderr: %s",
			rulesDir, result.Stdout, result.Stderr)
	}

	// The default location should NOT have been created.
	defaultDir := filepath.Join(sb.Home, ".config", "skillshare", "extras", "rules")
	if _, err := os.Stat(defaultDir); err == nil {
		t.Errorf("default extras dir %s should not exist when extras_source is set", defaultDir)
	}
}
