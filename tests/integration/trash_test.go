//go:build !online

package integration

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"skillshare/internal/testutil"
)

func TestTrash_List_Empty(t *testing.T) {
	sb := testutil.NewSandbox(t)
	defer sb.Cleanup()

	sb.WriteConfig(`source: ` + sb.SourcePath + `
targets: {}
`)

	result := sb.RunCLI("trash", "list")
	result.AssertSuccess(t)
	result.AssertAnyOutputContains(t, "empty")
}

func TestTrash_List_ShowsItems(t *testing.T) {
	sb := testutil.NewSandbox(t)
	defer sb.Cleanup()

	sb.CreateSkill("list-skill", map[string]string{"SKILL.md": "# List"})
	sb.WriteConfig(`source: ` + sb.SourcePath + `
targets: {}
`)

	// Uninstall to populate trash
	sb.RunCLI("uninstall", "list-skill", "--force")

	result := sb.RunCLI("trash", "list")
	result.AssertSuccess(t)
	result.AssertAnyOutputContains(t, "list-skill")
	result.AssertAnyOutputContains(t, "1 item")
}

func TestTrash_Restore_Success(t *testing.T) {
	sb := testutil.NewSandbox(t)
	defer sb.Cleanup()

	sb.CreateSkill("restore-me", map[string]string{"SKILL.md": "# Restore Me"})
	sb.WriteConfig(`source: ` + sb.SourcePath + `
targets: {}
`)

	// Uninstall
	sb.RunCLI("uninstall", "restore-me", "--force")

	// Verify gone from source
	if sb.FileExists(filepath.Join(sb.SourcePath, "restore-me")) {
		t.Fatal("skill should be in trash, not source")
	}

	// Restore
	result := sb.RunCLI("trash", "restore", "restore-me")
	result.AssertSuccess(t)
	result.AssertAnyOutputContains(t, "✓ Restore")

	// Should be back in source
	if !sb.FileExists(filepath.Join(sb.SourcePath, "restore-me", "SKILL.md")) {
		t.Error("skill should be restored to source")
	}

	// Trash should be empty now
	trashDir := filepath.Join(sb.Home, ".local", "share", "skillshare", "trash")
	entries, _ := os.ReadDir(trashDir)
	if len(entries) != 0 {
		t.Errorf("trash should be empty after restore, has %d entries", len(entries))
	}
}

// Regression for #679: a remote install restored from the trash keeps its
// source instead of coming back as a local skill.
func TestTrash_Restore_KeepsRemoteSource(t *testing.T) {
	sb := testutil.NewSandbox(t)
	defer sb.Cleanup()
	sb.WriteConfig(`source: ` + sb.SourcePath + `
targets: {}
`)
	repoPath := filepath.Join(sb.Root, "remote")
	os.MkdirAll(filepath.Join(repoPath, "skills", "remote-skill"), 0755)
	os.WriteFile(filepath.Join(repoPath, "skills", "remote-skill", "SKILL.md"), []byte("---\nname: remote-skill\n---\n# Remote"), 0644)
	initGitRepo(t, repoPath)
	sb.RunCLI("install", "file://"+repoPath+"//skills/remote-skill").AssertSuccess(t)

	sb.RunCLI("uninstall", "remote-skill", "--force").AssertSuccess(t)
	sb.RunCLI("trash", "restore", "remote-skill").AssertSuccess(t)

	meta := sb.ReadFile(filepath.Join(sb.SourcePath, ".metadata.json"))
	if !strings.Contains(meta, repoPath) {
		t.Errorf("restored skill should keep its source:\n%s", meta)
	}
}

// Project mode also records a remote skill in config.yaml and .gitignore,
// which uninstall drops; restore puts them back.
func TestTrash_Restore_ProjectKeepsConfigAndGitignore(t *testing.T) {
	sb := testutil.NewSandbox(t)
	defer sb.Cleanup()
	projectRoot := sb.SetupProjectDir("claude")
	repoPath := filepath.Join(sb.Root, "remote")
	os.MkdirAll(filepath.Join(repoPath, "skills", "remote-skill"), 0755)
	os.WriteFile(filepath.Join(repoPath, "skills", "remote-skill", "SKILL.md"), []byte("---\nname: remote-skill\n---\n# Remote"), 0644)
	initGitRepo(t, repoPath)
	sb.RunCLIInDir(projectRoot, "install", "-p", "file://"+repoPath+"//skills/remote-skill").AssertSuccess(t)

	sb.RunCLIInDir(projectRoot, "uninstall", "-p", "remote-skill", "--force").AssertSuccess(t)
	sb.RunCLIInDir(projectRoot, "trash", "restore", "-p", "remote-skill").AssertSuccess(t)

	if cfg := sb.ReadFile(filepath.Join(projectRoot, ".skillshare", "config.yaml")); !strings.Contains(cfg, "name: remote-skill") {
		t.Errorf("config.yaml should declare the restored skill:\n%s", cfg)
	}
	if ignore := sb.ReadFile(filepath.Join(projectRoot, ".skillshare", ".gitignore")); !strings.Contains(ignore, "skills/remote-skill/") {
		t.Errorf(".gitignore should ignore the restored skill:\n%s", ignore)
	}
}

func TestTrash_Restore_NotFound(t *testing.T) {
	sb := testutil.NewSandbox(t)
	defer sb.Cleanup()

	sb.WriteConfig(`source: ` + sb.SourcePath + `
targets: {}
`)

	result := sb.RunCLI("trash", "restore", "nonexistent")
	result.AssertFailure(t)
	result.AssertAnyOutputContains(t, "not found in trash")
}

func TestTrash_Delete_Success(t *testing.T) {
	sb := testutil.NewSandbox(t)
	defer sb.Cleanup()

	sb.CreateSkill("del-me", map[string]string{"SKILL.md": "# Delete"})
	sb.WriteConfig(`source: ` + sb.SourcePath + `
targets: {}
`)

	// Uninstall to populate trash
	sb.RunCLI("uninstall", "del-me", "--force")

	// Verify in trash
	listResult := sb.RunCLI("trash", "list")
	listResult.AssertAnyOutputContains(t, "del-me")

	// Delete permanently
	result := sb.RunCLI("trash", "delete", "del-me")
	result.AssertSuccess(t)
	result.AssertAnyOutputContains(t, "Permanently deleted")

	// Trash should be empty
	listResult2 := sb.RunCLI("trash", "list")
	listResult2.AssertAnyOutputContains(t, "empty")
}

func TestTrash_Delete_NotFound(t *testing.T) {
	sb := testutil.NewSandbox(t)
	defer sb.Cleanup()

	sb.WriteConfig(`source: ` + sb.SourcePath + `
targets: {}
`)

	result := sb.RunCLI("trash", "delete", "nonexistent")
	result.AssertFailure(t)
	result.AssertAnyOutputContains(t, "not found in trash")
}

func TestTrash_Empty_WithItems(t *testing.T) {
	sb := testutil.NewSandbox(t)
	defer sb.Cleanup()

	sb.CreateSkill("empty-a", map[string]string{"SKILL.md": "# A"})
	sb.CreateSkill("empty-b", map[string]string{"SKILL.md": "# B"})
	sb.WriteConfig(`source: ` + sb.SourcePath + `
targets: {}
`)

	sb.RunCLI("uninstall", "empty-a", "--force")
	sb.RunCLI("uninstall", "empty-b", "--force")

	// Pipe "y" to confirm
	result := sb.RunCLIWithInput("y\n", "trash", "empty")
	result.AssertSuccess(t)
	result.AssertAnyOutputContains(t, "Emptied trash")
	result.AssertAnyOutputContains(t, "2 item")

	// Trash should be empty
	listResult := sb.RunCLI("trash", "list")
	listResult.AssertAnyOutputContains(t, "empty")
}

func TestTrash_Empty_ForceSkipsConfirm(t *testing.T) {
	sb := testutil.NewSandbox(t)
	defer sb.Cleanup()

	sb.CreateSkill("force-a", map[string]string{"SKILL.md": "# A"})
	sb.WriteConfig(`source: ` + sb.SourcePath + `
targets: {}
`)
	sb.RunCLI("uninstall", "force-a", "--force")

	// No input: without --force the confirmation would cancel.
	result := sb.RunCLI("trash", "empty", "--force")
	result.AssertSuccess(t)
	result.AssertAnyOutputContains(t, "Emptied trash: 1 item")
}

func TestTrash_Empty_AlreadyEmpty(t *testing.T) {
	sb := testutil.NewSandbox(t)
	defer sb.Cleanup()

	sb.WriteConfig(`source: ` + sb.SourcePath + `
targets: {}
`)

	result := sb.RunCLI("trash", "empty")
	result.AssertSuccess(t)
	result.AssertAnyOutputContains(t, "already empty")
}

func TestTrash_List_NoTUI_Flag(t *testing.T) {
	sb := testutil.NewSandbox(t)
	defer sb.Cleanup()

	sb.CreateSkill("notui-skill", map[string]string{"SKILL.md": "# NoTUI"})
	sb.WriteConfig(`source: ` + sb.SourcePath + `
targets: {}
`)

	// Uninstall to populate trash
	sb.RunCLI("uninstall", "notui-skill", "--force")

	// --no-tui should be accepted and produce plain text output
	result := sb.RunCLI("trash", "list", "--no-tui")
	result.AssertSuccess(t)
	result.AssertAnyOutputContains(t, "notui-skill")
	result.AssertAnyOutputContains(t, "1 item")
}

func TestTrash_Help(t *testing.T) {
	sb := testutil.NewSandbox(t)
	defer sb.Cleanup()

	result := sb.RunCLI("trash", "--help")
	result.AssertSuccess(t)
	result.AssertAnyOutputContains(t, "restore")
	result.AssertAnyOutputContains(t, "list")
	result.AssertAnyOutputContains(t, "delete")
	result.AssertAnyOutputContains(t, "empty")
}
