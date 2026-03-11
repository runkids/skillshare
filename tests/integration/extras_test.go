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
	// Header for the extra should be present.
	result.AssertAnyOutputContains(t, "Rules")
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
	// Header for extra name
	result.AssertAnyOutputContains(t, "Rules")
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
