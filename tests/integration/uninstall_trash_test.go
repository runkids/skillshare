//go:build !online

package integration

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"skillshare/internal/testutil"
)

func TestUninstall_MovesToTrash(t *testing.T) {
	sb := testutil.NewSandbox(t)
	defer sb.Cleanup()

	sb.CreateSkill("trash-me", map[string]string{"SKILL.md": "# Trash Me"})
	sb.WriteConfig(`source: ` + sb.SourcePath + `
targets: {}
`)

	result := sb.RunCLI("uninstall", "trash-me", "--force")
	result.AssertSuccess(t)
	result.AssertOutputContains(t, "Moved to trash")

	// Source should be gone
	if sb.FileExists(filepath.Join(sb.SourcePath, "trash-me")) {
		t.Error("skill should be removed from source")
	}

	// Trash directory should contain the skill
	trashDir := filepath.Join(sb.Home, ".local", "share", "skillshare", "trash")
	entries, err := os.ReadDir(trashDir)
	if err != nil {
		t.Fatalf("trash directory should exist: %v", err)
	}
	if len(entries) != 1 {
		t.Fatalf("expected 1 trash entry, got %d", len(entries))
	}

	// Trashed skill should contain SKILL.md
	trashedSkill := filepath.Join(trashDir, entries[0].Name(), "SKILL.md")
	if _, err := os.Stat(trashedSkill); err != nil {
		t.Error("trashed skill should contain SKILL.md")
	}
}

func TestUninstall_WithMeta_PrintsReinstallHint(t *testing.T) {
	sb := testutil.NewSandbox(t)
	defer sb.Cleanup()

	sb.CreateSkill("remote-skill", map[string]string{
		"SKILL.md": "# Remote Skill",
		".skillshare-meta.json": `{
  "source": "github.com/user/skills/remote-skill",
  "type": "github",
  "installed_at": "2026-01-15T10:30:00Z"
}`,
	})
	sb.WriteConfig(`source: ` + sb.SourcePath + `
targets: {}
`)

	result := sb.RunCLI("uninstall", "remote-skill", "--force")
	result.AssertSuccess(t)
	result.AssertOutputContains(t, "Reinstall: skillshare install github.com/user/skills/remote-skill")
}

func TestUninstall_LocalSkill_NoReinstallHint(t *testing.T) {
	sb := testutil.NewSandbox(t)
	defer sb.Cleanup()

	// Local skill without metadata
	sb.CreateSkill("local-skill", map[string]string{"SKILL.md": "# Local"})
	sb.WriteConfig(`source: ` + sb.SourcePath + `
targets: {}
`)

	result := sb.RunCLI("uninstall", "local-skill", "--force")
	result.AssertSuccess(t)
	result.AssertOutputContains(t, "Moved to trash")

	// Should NOT contain reinstall hint
	combined := result.Stdout + result.Stderr
	if strings.Contains(combined, "Reinstall:") {
		t.Error("local skill without meta should not show reinstall hint")
	}
}

func TestUninstall_DryRun_ShowsTrashPreview(t *testing.T) {
	sb := testutil.NewSandbox(t)
	defer sb.Cleanup()

	sb.CreateSkill("preview-skill", map[string]string{
		"SKILL.md": "# Preview",
		".skillshare-meta.json": `{
  "source": "github.com/org/repo/preview-skill",
  "type": "github",
  "installed_at": "2026-01-15T10:30:00Z"
}`,
	})
	sb.WriteConfig(`source: ` + sb.SourcePath + `
targets: {}
`)

	result := sb.RunCLI("uninstall", "preview-skill", "--dry-run")
	result.AssertSuccess(t)
	result.AssertOutputContains(t, "would move to trash")
	result.AssertOutputContains(t, "Reinstall:")

	// Skill should still exist
	if !sb.FileExists(filepath.Join(sb.SourcePath, "preview-skill")) {
		t.Error("dry-run should not move skill to trash")
	}

	// Trash should be empty
	trashDir := filepath.Join(sb.Home, ".local", "share", "skillshare", "trash")
	entries, err := os.ReadDir(trashDir)
	if err == nil && len(entries) > 0 {
		t.Error("trash directory should be empty during dry-run")
	}
}
