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

func TestCopyDirExcluding_SkipsChildSkillDirs(t *testing.T) {
	src := t.TempDir()
	dst := filepath.Join(t.TempDir(), "dest")

	// Orchestrator-style repo layout (issue #124):
	//   SKILL.md              (root skill)
	//   README.md
	//   src/helper.go         (root-owned asset, must remain)
	//   skills/child-a/SKILL.md  (child skill, must be excluded)
	//   skills/child-b/SKILL.md  (child skill, must be excluded)
	//   skills/shared/README.md  (NOT a skill dir, must remain)
	os.WriteFile(filepath.Join(src, "SKILL.md"), []byte("# Root"), 0644)
	os.WriteFile(filepath.Join(src, "README.md"), []byte("root readme"), 0644)
	os.MkdirAll(filepath.Join(src, "src"), 0755)
	os.WriteFile(filepath.Join(src, "src", "helper.go"), []byte("package main"), 0644)
	os.MkdirAll(filepath.Join(src, "skills", "child-a"), 0755)
	os.WriteFile(filepath.Join(src, "skills", "child-a", "SKILL.md"), []byte("# A"), 0644)
	os.MkdirAll(filepath.Join(src, "skills", "child-b"), 0755)
	os.WriteFile(filepath.Join(src, "skills", "child-b", "SKILL.md"), []byte("# B"), 0644)
	os.MkdirAll(filepath.Join(src, "skills", "shared"), 0755)
	os.WriteFile(filepath.Join(src, "skills", "shared", "README.md"), []byte("shared"), 0644)

	excludes := map[string]bool{
		"skills/child-a": true,
		"skills/child-b": true,
	}
	if err := copyDirExcluding(src, dst, excludes); err != nil {
		t.Fatal(err)
	}

	// Files that MUST exist in the copy
	mustExist := []string{
		"SKILL.md",
		"README.md",
		filepath.Join("src", "helper.go"),
		filepath.Join("skills", "shared", "README.md"),
	}
	for _, rel := range mustExist {
		if _, err := os.Stat(filepath.Join(dst, rel)); err != nil {
			t.Errorf("expected %q to be copied, got %v", rel, err)
		}
	}

	// Files that MUST NOT exist (excluded child skill dirs)
	mustNotExist := []string{
		filepath.Join("skills", "child-a"),
		filepath.Join("skills", "child-b"),
		filepath.Join("skills", "child-a", "SKILL.md"),
	}
	for _, rel := range mustNotExist {
		if _, err := os.Stat(filepath.Join(dst, rel)); !os.IsNotExist(err) {
			t.Errorf("expected %q to be excluded, but it exists (err=%v)", rel, err)
		}
	}
}

func TestCopyDirExcluding_NilMapEqualsCopyDir(t *testing.T) {
	src := t.TempDir()
	dst := filepath.Join(t.TempDir(), "dest")

	os.WriteFile(filepath.Join(src, "a.txt"), []byte("a"), 0644)
	os.MkdirAll(filepath.Join(src, "sub"), 0755)
	os.WriteFile(filepath.Join(src, "sub", "b.txt"), []byte("b"), 0644)

	if err := copyDirExcluding(src, dst, nil); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(dst, "sub", "b.txt")); err != nil {
		t.Errorf("expected sub/b.txt to be copied, got %v", err)
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
