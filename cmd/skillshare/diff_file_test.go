package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"skillshare/internal/sync"
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

// Sync keeps another tool's link on a skill's name, so diff lists it as a
// local override instead of reporting the target in sync or a skill to add.
func TestDiffListsLinkFromAnotherToolAsOverride(t *testing.T) {
	source, target := t.TempDir(), t.TempDir()
	skill := filepath.Join(source, "alpha")
	if err := os.MkdirAll(skill, 0o755); err != nil {
		t.Fatal(err)
	}
	// Broken, so a check that follows the link would see nothing there.
	if err := os.Symlink(filepath.Join(t.TempDir(), "gone"), filepath.Join(target, "alpha")); err != nil {
		t.Skipf("symlinks unavailable: %v", err)
	}
	resolved := []sync.ResolvedTargetSkill{{Skill: sync.DiscoveredSkill{SourcePath: skill, FlatName: "alpha"}, TargetName: "alpha"}}
	names := map[string]bool{"alpha": true}
	manifest, _ := sync.ReadManifest(target)

	var merge, copyMode targetDiffResult
	collectMergeDiff(&merge, target, source, names, map[string]string{"alpha": skill}, nil)
	collectCopyDiff(&copyMode, "claude", target, source, resolved, "flat", names, nil, manifest, nil, nil)

	for mode, r := range map[string]targetDiffResult{"merge": merge, "copy": copyMode} {
		cats := categorizeItems(r.items)
		if len(cats) != 1 || cats[0].kind != "override" {
			t.Errorf("%s: want one local override, got items %+v", mode, r.items)
		}
	}
}
