package sync

import (
	"os"
	"path/filepath"
	"testing"
)

func TestDirChecksum_Deterministic(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "a.txt"), []byte("hello"), 0644)
	os.WriteFile(filepath.Join(dir, "b.txt"), []byte("world"), 0644)

	c1, err := DirChecksum(dir)
	if err != nil {
		t.Fatal(err)
	}
	c2, err := DirChecksum(dir)
	if err != nil {
		t.Fatal(err)
	}
	if c1 != c2 {
		t.Errorf("checksums not deterministic: %q vs %q", c1, c2)
	}
	if len(c1) != 64 {
		t.Errorf("expected 64-char hex SHA256, got len=%d", len(c1))
	}
}

func TestDirChecksum_DifferentContent(t *testing.T) {
	dir1 := t.TempDir()
	dir2 := t.TempDir()
	os.WriteFile(filepath.Join(dir1, "file.txt"), []byte("content-a"), 0644)
	os.WriteFile(filepath.Join(dir2, "file.txt"), []byte("content-b"), 0644)

	c1, err := DirChecksum(dir1)
	if err != nil {
		t.Fatal(err)
	}
	c2, err := DirChecksum(dir2)
	if err != nil {
		t.Fatal(err)
	}
	if c1 == c2 {
		t.Error("different content should produce different checksums")
	}
}

func TestDirChecksum_SkipsGit(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "file.txt"), []byte("main"), 0644)
	os.MkdirAll(filepath.Join(dir, ".git"), 0755)
	os.WriteFile(filepath.Join(dir, ".git", "HEAD"), []byte("ref: refs/heads/main"), 0644)

	// Checksum without .git
	dirClean := t.TempDir()
	os.WriteFile(filepath.Join(dirClean, "file.txt"), []byte("main"), 0644)

	c1, err := DirChecksum(dir)
	if err != nil {
		t.Fatal(err)
	}
	c2, err := DirChecksum(dirClean)
	if err != nil {
		t.Fatal(err)
	}
	if c1 != c2 {
		t.Error("checksum should be the same when .git is ignored")
	}
}

func TestDirChecksum_EmptyDir(t *testing.T) {
	dir := t.TempDir()

	c, err := DirChecksum(dir)
	if err != nil {
		t.Fatal(err)
	}
	if c == "" {
		t.Error("expected non-empty checksum for empty dir")
	}
}

func TestDirChecksum_NestedFiles(t *testing.T) {
	dir := t.TempDir()
	sub := filepath.Join(dir, "sub")
	os.MkdirAll(sub, 0755)
	os.WriteFile(filepath.Join(dir, "root.txt"), []byte("root"), 0644)
	os.WriteFile(filepath.Join(sub, "nested.txt"), []byte("nested"), 0644)

	c, err := DirChecksum(dir)
	if err != nil {
		t.Fatal(err)
	}
	if c == "" {
		t.Error("expected non-empty checksum for dir with nested files")
	}

	// Changing nested file should change checksum
	os.WriteFile(filepath.Join(sub, "nested.txt"), []byte("changed"), 0644)
	c2, err := DirChecksum(dir)
	if err != nil {
		t.Fatal(err)
	}
	if c == c2 {
		t.Error("changing nested file should change checksum")
	}
}
