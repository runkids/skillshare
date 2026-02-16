package backup

import (
	"os"
	"path/filepath"
	"testing"
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

func TestBackupDir_RespectsXDGDataHome(t *testing.T) {
	t.Setenv("XDG_DATA_HOME", "/custom/data")

	got := BackupDir()
	want := filepath.Join("/custom/data", "skillshare", "backups")
	if got != want {
		t.Errorf("BackupDir() = %q, want %q", got, want)
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
