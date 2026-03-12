//go:build !online

package integration

import (
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
