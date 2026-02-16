//go:build !online

package integration

import (
	"os"
	"path/filepath"
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
	result.AssertAnyOutputContains(t, "Restored")

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

func TestTrash_Restore_AlreadyExists(t *testing.T) {
	sb := testutil.NewSandbox(t)
	defer sb.Cleanup()

	sb.CreateSkill("conflict", map[string]string{"SKILL.md": "# V1"})
	sb.WriteConfig(`source: ` + sb.SourcePath + `
targets: {}
`)

	// Uninstall
	sb.RunCLI("uninstall", "conflict", "--force")

	// Recreate a skill with the same name
	sb.CreateSkill("conflict", map[string]string{"SKILL.md": "# V2"})

	// Restore should fail
	result := sb.RunCLI("trash", "restore", "conflict")
	result.AssertFailure(t)
	result.AssertAnyOutputContains(t, "already exists")
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
