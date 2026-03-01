package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestDiffSkillFiles_IdenticalDirs(t *testing.T) {
	src := t.TempDir()
	dst := t.TempDir()

	writeTestFile(t, src, "SKILL.md", "# Hello")
	writeTestFile(t, dst, "SKILL.md", "# Hello")

	diffs := diffSkillFiles(src, dst)
	if len(diffs) != 0 {
		t.Fatalf("expected 0 diffs for identical dirs, got %d: %+v", len(diffs), diffs)
	}
}

func TestDiffSkillFiles_NewFile(t *testing.T) {
	src := t.TempDir()
	dst := t.TempDir()

	writeTestFile(t, src, "SKILL.md", "# Hello")
	writeTestFile(t, src, "extra.md", "extra content")
	writeTestFile(t, dst, "SKILL.md", "# Hello")

	diffs := diffSkillFiles(src, dst)
	if len(diffs) != 1 {
		t.Fatalf("expected 1 diff, got %d: %+v", len(diffs), diffs)
	}
	if diffs[0].Action != "add" {
		t.Errorf("expected action 'add', got %q", diffs[0].Action)
	}
	if diffs[0].RelPath != "extra.md" {
		t.Errorf("expected RelPath 'extra.md', got %q", diffs[0].RelPath)
	}
}

func TestDiffSkillFiles_DeletedFile(t *testing.T) {
	src := t.TempDir()
	dst := t.TempDir()

	writeTestFile(t, src, "SKILL.md", "# Hello")
	writeTestFile(t, dst, "SKILL.md", "# Hello")
	writeTestFile(t, dst, "old.md", "old content")

	diffs := diffSkillFiles(src, dst)
	if len(diffs) != 1 {
		t.Fatalf("expected 1 diff, got %d: %+v", len(diffs), diffs)
	}
	if diffs[0].Action != "delete" {
		t.Errorf("expected action 'delete', got %q", diffs[0].Action)
	}
	if diffs[0].RelPath != "old.md" {
		t.Errorf("expected RelPath 'old.md', got %q", diffs[0].RelPath)
	}
}

func TestDiffSkillFiles_ModifiedFile(t *testing.T) {
	src := t.TempDir()
	dst := t.TempDir()

	writeTestFile(t, src, "SKILL.md", "# Hello v2")
	writeTestFile(t, dst, "SKILL.md", "# Hello v1")

	diffs := diffSkillFiles(src, dst)
	if len(diffs) != 1 {
		t.Fatalf("expected 1 diff, got %d: %+v", len(diffs), diffs)
	}
	if diffs[0].Action != "modify" {
		t.Errorf("expected action 'modify', got %q", diffs[0].Action)
	}
}

func TestDiffSkillFiles_SrcOnly(t *testing.T) {
	src := t.TempDir()
	writeTestFile(t, src, "a.md", "aaa")
	writeTestFile(t, src, "b.md", "bbb")

	diffs := diffSkillFiles(src, "/nonexistent_dir_that_does_not_exist")
	if len(diffs) != 2 {
		t.Fatalf("expected 2 diffs, got %d: %+v", len(diffs), diffs)
	}
	for _, d := range diffs {
		if d.Action != "add" {
			t.Errorf("expected action 'add', got %q for %s", d.Action, d.RelPath)
		}
	}
}

func TestDiffSkillFiles_NestedDir(t *testing.T) {
	src := t.TempDir()
	dst := t.TempDir()

	writeTestFile(t, src, "SKILL.md", "# Hello")
	writeTestFile(t, dst, "SKILL.md", "# Hello")

	// Add a nested file only in source
	subDir := filepath.Join(src, "sub")
	if err := os.MkdirAll(subDir, 0755); err != nil {
		t.Fatal(err)
	}
	writeTestFile(t, src, filepath.Join("sub", "extra.md"), "nested content")

	diffs := diffSkillFiles(src, dst)
	if len(diffs) != 1 {
		t.Fatalf("expected 1 diff, got %d: %+v", len(diffs), diffs)
	}
	if diffs[0].RelPath != "sub/extra.md" {
		t.Errorf("expected RelPath 'sub/extra.md', got %q", diffs[0].RelPath)
	}
	if diffs[0].Action != "add" {
		t.Errorf("expected action 'add', got %q", diffs[0].Action)
	}
}

func TestGenerateUnifiedDiff(t *testing.T) {
	dir := t.TempDir()
	oldPath := filepath.Join(dir, "old.txt")
	newPath := filepath.Join(dir, "new.txt")

	if err := os.WriteFile(oldPath, []byte("line1\nline2\nline3\n"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(newPath, []byte("line1\nmodified\nline3\nextra\n"), 0644); err != nil {
		t.Fatal(err)
	}

	// newPath is src (new), oldPath is dst (old)
	result := generateUnifiedDiff(newPath, oldPath)

	if !strings.Contains(result, "+") && !strings.Contains(result, "-") {
		t.Errorf("expected diff output to contain '+' and '-' lines, got:\n%s", result)
	}
	if !strings.Contains(result, "modified") {
		t.Errorf("expected diff to mention 'modified', got:\n%s", result)
	}
	if !strings.Contains(result, "extra") {
		t.Errorf("expected diff to mention 'extra', got:\n%s", result)
	}
}

func TestIsBinary(t *testing.T) {
	if !isBinary("hello\x00world") {
		t.Error("expected isBinary to return true for string with null byte")
	}
	if isBinary("hello world, this is normal text") {
		t.Error("expected isBinary to return false for normal text")
	}
	if isBinary("") {
		t.Error("expected isBinary to return false for empty string")
	}
}

func TestRenderFileStat(t *testing.T) {
	files := []fileDiffEntry{
		{RelPath: "new.md", Action: "add", SrcSize: 100},
		{RelPath: "changed.md", Action: "modify", DstSize: 200, SrcSize: 250},
		{RelPath: "removed.md", Action: "delete", DstSize: 50},
	}

	result := renderFileStat(files)

	if !strings.Contains(result, "+ new.md (100 bytes)") {
		t.Errorf("expected add line for new.md, got:\n%s", result)
	}
	if !strings.Contains(result, "~ changed.md (200") {
		t.Errorf("expected modify line for changed.md, got:\n%s", result)
	}
	if !strings.Contains(result, "- removed.md (50 bytes)") {
		t.Errorf("expected delete line for removed.md, got:\n%s", result)
	}
}

// writeTestFile creates a file relative to dir with the given content.
func writeTestFile(t *testing.T, dir, relPath, content string) {
	t.Helper()
	full := filepath.Join(dir, relPath)
	parent := filepath.Dir(full)
	if err := os.MkdirAll(parent, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(full, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}
}
