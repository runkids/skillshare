package backup

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestCopyDir_RegularFiles(t *testing.T) {
	src := t.TempDir()
	dst := t.TempDir()

	writeTestFile(t, filepath.Join(src, "file1.txt"), "hello")
	os.MkdirAll(filepath.Join(src, "subdir"), 0755)
	writeTestFile(t, filepath.Join(src, "subdir", "file2.txt"), "world")

	if err := copyDir(src, dst); err != nil {
		t.Fatalf("copyDir failed: %v", err)
	}

	assertFileContent(t, filepath.Join(dst, "file1.txt"), "hello")
	assertFileContent(t, filepath.Join(dst, "subdir", "file2.txt"), "world")
}

func TestCopyDir_SkipsSymlinks(t *testing.T) {
	src := t.TempDir()
	dst := t.TempDir()

	writeTestFile(t, filepath.Join(src, "real.txt"), "keep me")

	symlinkTarget := t.TempDir()
	writeTestFile(t, filepath.Join(symlinkTarget, "secret.txt"), "do not copy")

	// Symlink to a file
	if err := os.Symlink(
		filepath.Join(symlinkTarget, "secret.txt"),
		filepath.Join(src, "linked-file.txt"),
	); err != nil {
		t.Skipf("symlink not supported: %v", err)
	}

	// Symlink to a directory (simulates Windows junction)
	if err := os.Symlink(symlinkTarget, filepath.Join(src, "linked-dir")); err != nil {
		t.Skipf("symlink not supported: %v", err)
	}

	if err := copyDir(src, dst); err != nil {
		t.Fatalf("copyDir failed: %v", err)
	}

	assertFileContent(t, filepath.Join(dst, "real.txt"), "keep me")

	if _, err := os.Lstat(filepath.Join(dst, "linked-file.txt")); !os.IsNotExist(err) {
		t.Error("symlinked file should not be copied to backup")
	}
	if _, err := os.Lstat(filepath.Join(dst, "linked-dir")); !os.IsNotExist(err) {
		t.Error("symlinked directory should not be copied to backup")
	}
}

func TestCopyDir_MixedContent(t *testing.T) {
	src := t.TempDir()
	dst := t.TempDir()

	// Real local skills (should be backed up)
	os.MkdirAll(filepath.Join(src, "my-local-skill"), 0755)
	writeTestFile(t, filepath.Join(src, "my-local-skill", "SKILL.md"), "# Local Skill")
	os.MkdirAll(filepath.Join(src, "another-local"), 0755)
	writeTestFile(t, filepath.Join(src, "another-local", "SKILL.md"), "# Another")

	// Symlinked skill (simulates merge-mode junction)
	sourceDir := t.TempDir()
	os.MkdirAll(filepath.Join(sourceDir, "agent-browser"), 0755)
	writeTestFile(t, filepath.Join(sourceDir, "agent-browser", "SKILL.md"), "# Agent")

	if err := os.Symlink(
		filepath.Join(sourceDir, "agent-browser"),
		filepath.Join(src, "agent-browser"),
	); err != nil {
		t.Skipf("symlink not supported: %v", err)
	}

	if err := copyDir(src, dst); err != nil {
		t.Fatalf("copyDir failed: %v", err)
	}

	assertFileContent(t, filepath.Join(dst, "my-local-skill", "SKILL.md"), "# Local Skill")
	assertFileContent(t, filepath.Join(dst, "another-local", "SKILL.md"), "# Another")

	if _, err := os.Lstat(filepath.Join(dst, "agent-browser")); !os.IsNotExist(err) {
		t.Error("symlinked skill 'agent-browser' should not be copied to backup")
	}
}

func TestCopyDir_BrokenSymlink(t *testing.T) {
	src := t.TempDir()
	dst := t.TempDir()

	writeTestFile(t, filepath.Join(src, "real.txt"), "safe")

	if err := os.Symlink("/nonexistent/path", filepath.Join(src, "broken-link")); err != nil {
		t.Skipf("symlink not supported: %v", err)
	}

	if err := copyDir(src, dst); err != nil {
		t.Fatalf("copyDir should not fail on broken symlink: %v", err)
	}

	assertFileContent(t, filepath.Join(dst, "real.txt"), "safe")

	if _, err := os.Lstat(filepath.Join(dst, "broken-link")); !os.IsNotExist(err) {
		t.Error("broken symlink should not be copied to backup")
	}
}

func TestCopyDir_EmptyDir(t *testing.T) {
	src := t.TempDir()
	dst := t.TempDir()

	if err := copyDir(src, dst); err != nil {
		t.Fatalf("copyDir on empty dir failed: %v", err)
	}

	entries, _ := os.ReadDir(dst)
	if len(entries) != 0 {
		t.Errorf("expected empty dst, got %d entries", len(entries))
	}
}

func TestCopyDirFollowTopSymlinks_MergeMode(t *testing.T) {
	// Simulate merge-mode target: directory with per-skill symlinks
	source := t.TempDir() // acts as "source" skill repo
	target := t.TempDir() // acts as merge-mode target
	dst := t.TempDir()    // backup destination

	// Create real skills in source
	os.MkdirAll(filepath.Join(source, "skill-a"), 0755)
	writeTestFile(t, filepath.Join(source, "skill-a", "SKILL.md"), "# Skill A")
	writeTestFile(t, filepath.Join(source, "skill-a", "prompt.md"), "prompt content")

	os.MkdirAll(filepath.Join(source, "skill-b"), 0755)
	writeTestFile(t, filepath.Join(source, "skill-b", "SKILL.md"), "# Skill B")

	// Create symlinks in target (merge mode)
	if err := os.Symlink(
		filepath.Join(source, "skill-a"),
		filepath.Join(target, "skill-a"),
	); err != nil {
		t.Skipf("symlink not supported: %v", err)
	}
	os.Symlink(filepath.Join(source, "skill-b"), filepath.Join(target, "skill-b"))

	// Also add a local (non-symlink) skill
	os.MkdirAll(filepath.Join(target, "local-skill"), 0755)
	writeTestFile(t, filepath.Join(target, "local-skill", "SKILL.md"), "# Local")

	if err := copyDirFollowTopSymlinks(target, dst); err != nil {
		t.Fatalf("copyDirFollowTopSymlinks failed: %v", err)
	}

	// Symlinked skills should be resolved and copied
	assertFileContent(t, filepath.Join(dst, "skill-a", "SKILL.md"), "# Skill A")
	assertFileContent(t, filepath.Join(dst, "skill-a", "prompt.md"), "prompt content")
	assertFileContent(t, filepath.Join(dst, "skill-b", "SKILL.md"), "# Skill B")

	// Local skill should also be copied
	assertFileContent(t, filepath.Join(dst, "local-skill", "SKILL.md"), "# Local")
}

func TestCopyDirFollowTopSymlinks_BrokenSymlink(t *testing.T) {
	target := t.TempDir()
	dst := t.TempDir()

	// Create a broken symlink
	if err := os.Symlink("/nonexistent/path", filepath.Join(target, "broken")); err != nil {
		t.Skipf("symlink not supported: %v", err)
	}

	// Should not fail on broken symlink
	if err := copyDirFollowTopSymlinks(target, dst); err != nil {
		t.Fatalf("should not fail on broken symlink: %v", err)
	}

	// Broken link should be skipped
	if _, err := os.Lstat(filepath.Join(dst, "broken")); !os.IsNotExist(err) {
		t.Error("broken symlink should be skipped")
	}
}

func TestBackupDir_RespectsXDGDataHome(t *testing.T) {
	t.Setenv("XDG_DATA_HOME", "/custom/data")

	got := BackupDir()
	want := filepath.Join("/custom/data", "skillshare", "backups")
	if got != want {
		t.Errorf("BackupDir() = %q, want %q", got, want)
	}
}

func TestValidateRestore_SymlinkTarget_IsAllowed(t *testing.T) {
	tmp := t.TempDir()
	backupPath := filepath.Join(tmp, "backup")
	os.MkdirAll(filepath.Join(backupPath, "claude"), 0755)
	os.WriteFile(filepath.Join(backupPath, "claude", "SKILL.md"), []byte("# X"), 0644)

	// Create a symlink as destination pointing to a non-empty directory.
	// With os.Stat (buggy): resolves symlink, sees non-empty dir, requires --force.
	// With os.Lstat (fixed): detects symlink, returns nil immediately.
	destPath := filepath.Join(tmp, "target")
	realDir := filepath.Join(tmp, "real")
	os.MkdirAll(realDir, 0755)
	os.WriteFile(filepath.Join(realDir, "existing.txt"), []byte("data"), 0644)
	os.Symlink(realDir, destPath)

	err := ValidateRestore(backupPath, "claude", destPath, RestoreOptions{})
	if err != nil {
		t.Errorf("symlink target should be allowed without force, got: %v", err)
	}
}

func TestListTargetsWithBackups_Empty(t *testing.T) {
	dir := t.TempDir()

	summaries, err := ListTargetsWithBackups(dir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(summaries) != 0 {
		t.Errorf("expected 0 summaries, got %d", len(summaries))
	}
}

func TestListTargetsWithBackups_NonExistentDir(t *testing.T) {
	summaries, err := ListTargetsWithBackups("/nonexistent/path/backups")
	if err != nil {
		t.Fatalf("unexpected error for non-existent dir: %v", err)
	}
	if summaries != nil {
		t.Errorf("expected nil, got %v", summaries)
	}
}

func TestListTargetsWithBackups_MultiBacks(t *testing.T) {
	dir := t.TempDir()

	// Create 3 timestamp directories with various targets
	// Timestamp format matches backup.Create: 2006-01-02_15-04-05
	timestamps := []string{
		"2025-01-10_08-00-00",
		"2025-02-15_12-30-00",
		"2025-03-20_18-45-00",
	}

	// ts0: claude, cursor
	os.MkdirAll(filepath.Join(dir, timestamps[0], "claude"), 0755)
	os.MkdirAll(filepath.Join(dir, timestamps[0], "cursor"), 0755)

	// ts1: claude, windsurf
	os.MkdirAll(filepath.Join(dir, timestamps[1], "claude"), 0755)
	os.MkdirAll(filepath.Join(dir, timestamps[1], "windsurf"), 0755)

	// ts2: claude
	os.MkdirAll(filepath.Join(dir, timestamps[2], "claude"), 0755)

	summaries, err := ListTargetsWithBackups(dir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(summaries) != 3 {
		t.Fatalf("expected 3 targets, got %d", len(summaries))
	}

	// Should be sorted by target name: claude, cursor, windsurf
	if summaries[0].TargetName != "claude" {
		t.Errorf("summaries[0].TargetName = %q, want %q", summaries[0].TargetName, "claude")
	}
	if summaries[1].TargetName != "cursor" {
		t.Errorf("summaries[1].TargetName = %q, want %q", summaries[1].TargetName, "cursor")
	}
	if summaries[2].TargetName != "windsurf" {
		t.Errorf("summaries[2].TargetName = %q, want %q", summaries[2].TargetName, "windsurf")
	}

	// claude: 3 backups, oldest=ts0, latest=ts2
	if summaries[0].BackupCount != 3 {
		t.Errorf("claude BackupCount = %d, want 3", summaries[0].BackupCount)
	}
	wantOldest := time.Date(2025, 1, 10, 8, 0, 0, 0, time.Local)
	wantLatest := time.Date(2025, 3, 20, 18, 45, 0, 0, time.Local)
	if !summaries[0].Oldest.Equal(wantOldest) {
		t.Errorf("claude Oldest = %v, want %v", summaries[0].Oldest, wantOldest)
	}
	if !summaries[0].Latest.Equal(wantLatest) {
		t.Errorf("claude Latest = %v, want %v", summaries[0].Latest, wantLatest)
	}

	// cursor: 1 backup
	if summaries[1].BackupCount != 1 {
		t.Errorf("cursor BackupCount = %d, want 1", summaries[1].BackupCount)
	}

	// windsurf: 1 backup
	if summaries[2].BackupCount != 1 {
		t.Errorf("windsurf BackupCount = %d, want 1", summaries[2].BackupCount)
	}
}

func TestListTargetsWithBackups_SkipsFiles(t *testing.T) {
	dir := t.TempDir()

	// Create a timestamp dir with a target
	os.MkdirAll(filepath.Join(dir, "2025-01-10_08-00-00", "claude"), 0755)
	// Create a plain file at timestamp level (should be skipped)
	os.WriteFile(filepath.Join(dir, "notes.txt"), []byte("ignore"), 0644)
	// Create a plain file inside timestamp dir (should be skipped as target)
	os.WriteFile(filepath.Join(dir, "2025-01-10_08-00-00", "readme.txt"), []byte("ignore"), 0644)

	summaries, err := ListTargetsWithBackups(dir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(summaries) != 1 {
		t.Fatalf("expected 1 target, got %d", len(summaries))
	}
	if summaries[0].TargetName != "claude" {
		t.Errorf("TargetName = %q, want %q", summaries[0].TargetName, "claude")
	}
}

func TestListBackupVersions_Empty(t *testing.T) {
	dir := t.TempDir()

	result, err := ListBackupVersions(dir, "claude")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(result) != 0 {
		t.Errorf("expected 0 versions, got %d", len(result))
	}
}

func TestListBackupVersions_NonExistentDir(t *testing.T) {
	result, err := ListBackupVersions("/nonexistent/path/backups", "claude")
	if err != nil {
		t.Fatalf("unexpected error for non-existent dir: %v", err)
	}
	if result != nil {
		t.Errorf("expected nil, got %v", result)
	}
}

func TestListBackupVersions_ReturnsSkillInfo(t *testing.T) {
	dir := t.TempDir()

	// ts1 (older): claude with 1 skill
	ts1 := "2025-01-10_08-00-00"
	skillDir1 := filepath.Join(dir, ts1, "claude", "my-skill")
	os.MkdirAll(skillDir1, 0755)
	writeTestFile(t, filepath.Join(skillDir1, "SKILL.md"), "# My Skill")

	// ts2 (newer): claude with 2 skills
	ts2 := "2025-03-20_18-45-00"
	skillDir2a := filepath.Join(dir, ts2, "claude", "skill-a")
	skillDir2b := filepath.Join(dir, ts2, "claude", "skill-b")
	os.MkdirAll(skillDir2a, 0755)
	os.MkdirAll(skillDir2b, 0755)
	writeTestFile(t, filepath.Join(skillDir2a, "SKILL.md"), "# Skill A")
	writeTestFile(t, filepath.Join(skillDir2b, "SKILL.md"), "# Skill B content here")

	result, err := ListBackupVersions(dir, "claude")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(result) != 2 {
		t.Fatalf("expected 2 versions, got %d", len(result))
	}

	// Should be sorted newest first
	wantNewest := time.Date(2025, 3, 20, 18, 45, 0, 0, time.Local)
	wantOldest := time.Date(2025, 1, 10, 8, 0, 0, 0, time.Local)

	if !result[0].Timestamp.Equal(wantNewest) {
		t.Errorf("result[0].Timestamp = %v, want %v", result[0].Timestamp, wantNewest)
	}
	if !result[1].Timestamp.Equal(wantOldest) {
		t.Errorf("result[1].Timestamp = %v, want %v", result[1].Timestamp, wantOldest)
	}

	// Newer version: 2 skills
	if result[0].SkillCount != 2 {
		t.Errorf("result[0].SkillCount = %d, want 2", result[0].SkillCount)
	}
	if len(result[0].SkillNames) != 2 {
		t.Errorf("result[0].SkillNames len = %d, want 2", len(result[0].SkillNames))
	}

	// Older version: 1 skill
	if result[1].SkillCount != 1 {
		t.Errorf("result[1].SkillCount = %d, want 1", result[1].SkillCount)
	}
	if len(result[1].SkillNames) != 1 || result[1].SkillNames[0] != "my-skill" {
		t.Errorf("result[1].SkillNames = %v, want [my-skill]", result[1].SkillNames)
	}

	// Label format
	wantLabel := "2025-03-20 18:45:00"
	if result[0].Label != wantLabel {
		t.Errorf("result[0].Label = %q, want %q", result[0].Label, wantLabel)
	}

	// Dir should point to the target subdir
	wantDir := filepath.Join(dir, ts2, "claude")
	if result[0].Dir != wantDir {
		t.Errorf("result[0].Dir = %q, want %q", result[0].Dir, wantDir)
	}

	// TotalSize should be > 0 (we wrote files)
	if result[0].TotalSize <= 0 {
		t.Errorf("result[0].TotalSize = %d, want > 0", result[0].TotalSize)
	}
}

func TestListBackupVersions_IgnoresOtherTargets(t *testing.T) {
	dir := t.TempDir()

	ts := "2025-06-01_10-00-00"
	// claude and cursor both exist
	os.MkdirAll(filepath.Join(dir, ts, "claude", "skill-x"), 0755)
	os.MkdirAll(filepath.Join(dir, ts, "cursor", "skill-y"), 0755)

	result, err := ListBackupVersions(dir, "claude")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(result) != 1 {
		t.Fatalf("expected 1 version, got %d", len(result))
	}
	if result[0].SkillCount != 1 {
		t.Errorf("SkillCount = %d, want 1", result[0].SkillCount)
	}
	if result[0].SkillNames[0] != "skill-x" {
		t.Errorf("SkillNames = %v, want [skill-x]", result[0].SkillNames)
	}
}

func TestListBackupVersions_SkipsInvalidTimestamps(t *testing.T) {
	dir := t.TempDir()

	// Valid timestamp
	os.MkdirAll(filepath.Join(dir, "2025-01-10_08-00-00", "claude", "skill"), 0755)
	// Invalid timestamp directory name
	os.MkdirAll(filepath.Join(dir, "not-a-timestamp", "claude", "skill"), 0755)
	// Plain file at top level
	writeTestFile(t, filepath.Join(dir, "notes.txt"), "ignore")

	result, err := ListBackupVersions(dir, "claude")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(result) != 1 {
		t.Fatalf("expected 1 version, got %d", len(result))
	}
}

// --- helpers ---

func writeTestFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatalf("writeTestFile(%s): %v", path, err)
	}
}

func assertFileContent(t *testing.T, path, expected string) {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("cannot read %s: %v", path, err)
	}
	if string(data) != expected {
		t.Errorf("%s: got %q, want %q", path, string(data), expected)
	}
}
