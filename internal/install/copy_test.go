package install

import (
	"os"
	"path/filepath"
	"testing"
)

func TestCopyDir_Basic(t *testing.T) {
	src := t.TempDir()
	dst := filepath.Join(t.TempDir(), "dest")

	os.WriteFile(filepath.Join(src, "file.txt"), []byte("hello"), 0644)
	sub := filepath.Join(src, "sub")
	os.MkdirAll(sub, 0755)
	os.WriteFile(filepath.Join(sub, "nested.txt"), []byte("world"), 0644)

	if err := copyDir(src, dst); err != nil {
		t.Fatal(err)
	}

	// Verify files were copied
	data, err := os.ReadFile(filepath.Join(dst, "file.txt"))
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != "hello" {
		t.Errorf("expected 'hello', got %q", string(data))
	}

	data, err = os.ReadFile(filepath.Join(dst, "sub", "nested.txt"))
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != "world" {
		t.Errorf("expected 'world', got %q", string(data))
	}
}

func TestCopyDir_SkipsGit(t *testing.T) {
	src := t.TempDir()
	dst := filepath.Join(t.TempDir(), "dest")

	os.WriteFile(filepath.Join(src, "SKILL.md"), []byte("# Skill"), 0644)
	gitDir := filepath.Join(src, ".git")
	os.MkdirAll(gitDir, 0755)
	os.WriteFile(filepath.Join(gitDir, "HEAD"), []byte("ref: refs/heads/main"), 0644)

	if err := copyDir(src, dst); err != nil {
		t.Fatal(err)
	}

	// SKILL.md should exist
	if _, err := os.Stat(filepath.Join(dst, "SKILL.md")); err != nil {
		t.Error("expected SKILL.md to be copied")
	}

	// .git should NOT exist
	if _, err := os.Stat(filepath.Join(dst, ".git")); !os.IsNotExist(err) {
		t.Error("expected .git to be skipped during copy")
	}
}

func TestCopyDir_PreservesPermissions(t *testing.T) {
	src := t.TempDir()
	dst := filepath.Join(t.TempDir(), "dest")

	// Create file with executable permission
	srcFile := filepath.Join(src, "script.sh")
	os.WriteFile(srcFile, []byte("#!/bin/bash\necho hi"), 0755)

	if err := copyDir(src, dst); err != nil {
		t.Fatal(err)
	}

	info, err := os.Stat(filepath.Join(dst, "script.sh"))
	if err != nil {
		t.Fatal(err)
	}
	// Check that executable bit is preserved (at least user execute)
	if info.Mode()&0100 == 0 {
		t.Errorf("expected executable permission to be preserved, got %o", info.Mode())
	}
}
