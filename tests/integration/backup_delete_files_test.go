//go:build !online

package integration

import (
	"crypto/sha256"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"skillshare/internal/testutil"
)

func TestBackup_Delete_RemovesOneSnapshot(t *testing.T) {
	sb := testutil.NewSandbox(t)
	defer sb.Cleanup()
	sb.WriteConfig("source: " + sb.SourcePath + "\ntargets: {}\n")

	backupDir := filepath.Join(sb.Home, ".local", "share", "skillshare", "backups")
	for _, ts := range []string{"2024-01-15_14-30-45", "2024-01-16_09-00-00"} {
		sb.WriteFile(filepath.Join(backupDir, ts, "claude", "s", "SKILL.md"), "# s")
	}

	dry := sb.RunCLI("backup", "--delete", "2024-01-15_14-30-45", "--dry-run")
	dry.AssertSuccess(t)
	dry.AssertAnyOutputContains(t, "Would delete backup")
	if !sb.FileExists(filepath.Join(backupDir, "2024-01-15_14-30-45")) {
		t.Fatal("dry run deleted the backup")
	}

	result := sb.RunCLI("backup", "--delete", "2024-01-15_14-30-45")
	result.AssertSuccess(t)
	if sb.FileExists(filepath.Join(backupDir, "2024-01-15_14-30-45")) || !sb.FileExists(filepath.Join(backupDir, "2024-01-16_09-00-00")) {
		t.Fatal("expected only the named backup to be deleted")
	}

	sb.RunCLI("backup", "--delete", "2024-01-15_14-30-45").AssertFailure(t)
	sb.RunCLI("backup", "--delete", "../backups").AssertFailure(t)
}

// seedFileBackupDir writes a history version of path the way skillshare stores it.
func seedFileBackupDir(sb *testutil.Sandbox, path, name, content string) {
	sum := sha256.Sum256([]byte(filepath.Clean(path)))
	dir := filepath.Join(sb.Home, ".local", "state", "skillshare", "extras", "backups", fmt.Sprintf("%x", sum[:8]))
	sb.WriteFile(filepath.Join(dir, "path"), filepath.Clean(path))
	sb.WriteFile(filepath.Join(dir, name), content)
}

func TestBackupFiles_ListShowRestore(t *testing.T) {
	sb := testutil.NewSandbox(t)
	defer sb.Cleanup()
	sb.WriteConfig("source: " + sb.SourcePath + "\ntargets: {}\n")

	path := filepath.Join(sb.Home, ".claude", "CLAUDE.md")
	sb.WriteFile(path, "# now")
	seedFileBackupDir(sb, path, "1700000000000000001.edit.bak", "# before edit")

	list := sb.RunCLI("backup", "files", "-g")
	list.AssertSuccess(t)
	list.AssertOutputContains(t, "~/.claude/CLAUDE.md")

	show := sb.RunCLI("backup", "files", "show", path, "-g")
	show.AssertSuccess(t)
	show.AssertOutputContains(t, "history/edit")
	show.AssertOutputContains(t, "# before edit")

	restore := sb.RunCLI("backup", "files", "restore", path, "1700000000000000001.edit", "-g")
	restore.AssertSuccess(t)
	if got := sb.ReadFile(path); got != "# before edit" {
		t.Fatalf("content = %q", got)
	}
	show = sb.RunCLI("backup", "files", "show", path, "-g")
	show.AssertOutputContains(t, "history/restore")
}

func TestBackupFiles_RestoreRefusesLinkWithoutUnlink(t *testing.T) {
	sb := testutil.NewSandbox(t)
	defer sb.Cleanup()
	sb.WriteConfig("source: " + sb.SourcePath + "\ntargets: {}\n")

	shared := filepath.Join(sb.Home, "shared", "AGENTS.md")
	sb.WriteFile(shared, "# shared")
	path := filepath.Join(sb.Home, ".claude", "CLAUDE.md")
	os.MkdirAll(filepath.Dir(path), 0755)
	sb.CreateSymlink(shared, path)
	seedFileBackupDir(sb, path, "1700000000000000001.bak", "# old")

	refused := sb.RunCLI("backup", "files", "restore", path, "1700000000000000001", "-g")
	refused.AssertFailure(t)
	refused.AssertAnyOutputContains(t, "--unlink")
	if sb.ReadFile(shared) != "# shared" {
		t.Fatal("shared file changed")
	}

	sb.RunCLI("backup", "files", "restore", path, "1700000000000000001", "--unlink", "-g").AssertSuccess(t)
	if sb.IsSymlink(path) || !strings.Contains(sb.ReadFile(path), "# old") {
		t.Fatal("link was not replaced by the restored file")
	}
}

func TestBackup_ListAndCleanup_ProjectMode(t *testing.T) {
	sb := testutil.NewSandbox(t)
	defer sb.Cleanup()
	projectDir := sb.SetupProjectDir("claude")

	projectBackups := filepath.Join(projectDir, ".skillshare", "backups")
	sb.WriteFile(filepath.Join(projectBackups, "2024-01-15_14-30-45", "claude-agents", "a.md"), "# a")
	globalBackups := filepath.Join(sb.Home, ".local", "share", "skillshare", "backups")
	sb.WriteFile(filepath.Join(globalBackups, "2024-02-01_10-00-00", "claude", "s", "SKILL.md"), "# s")

	list := sb.RunCLIInDir(projectDir, "backup", "--list", "-p")
	list.AssertSuccess(t)
	list.AssertAnyOutputContains(t, "2024-01-15_14-30-45")
	list.AssertOutputNotContains(t, "2024-02-01_10-00-00")

	cleanup := sb.RunCLIInDir(projectDir, "backup", "--cleanup", "--dry-run", "-p")
	cleanup.AssertSuccess(t)
	cleanup.AssertAnyOutputContains(t, "1 backup, ")
}
