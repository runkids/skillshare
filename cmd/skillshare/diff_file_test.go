package main

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"skillshare/internal/sync"
	"skillshare/internal/utils"
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

func TestCollectTargetDiff_ConfiguredRootAlias(t *testing.T) {
	for _, mode := range []string{"copy", "merge"} {
		for _, scenario := range []string{"profile_alias", "plain_alias", "profile_source", "plain_source"} {
			t.Run(mode+"/"+scenario, func(t *testing.T) {
				source, outside := t.TempDir(), t.TempDir()
				writeSyncSkill(t, source, "alpha", "alpha")
				sourceBytes, err := os.ReadFile(filepath.Join(source, "alpha", "SKILL.md"))
				if err != nil {
					t.Fatal(err)
				}
				linkedDir := outside
				if strings.HasSuffix(scenario, "_source") {
					linkedDir = source
				}
				target := filepath.Join(t.TempDir(), "target")
				if runtime.GOOS == "windows" {
					if err := utils.CreateJunction(target, linkedDir); err != nil {
						t.Fatal(err)
					}
				} else if err := os.Symlink(linkedDir, target); err != nil {
					t.Fatal(err)
				}
				rootLink, err := os.Readlink(target)
				if err != nil {
					t.Fatal(err)
				}
				skills, err := sync.DiscoverSourceSkills(source)
				if err != nil || len(skills) != 1 {
					t.Fatalf("setup requires alpha: %+v, %v", skills, err)
				}
				var manifestBefore []byte
				if linkedDir == outside {
					out, err := sync.SyncTargetCopyWithSkills("custom", skillTarget(target, "copy", ""), skills, source, false, false, nil)
					if err != nil || len(out.Copied) != 1 {
						t.Fatalf("setup requires writer-managed copy: %+v, %v", out, err)
					}
					manifest, err := sync.ReadManifest(target)
					if err != nil || !manifest.OwnsCopy("alpha", filepath.Join(target, "alpha")) {
						t.Fatalf("setup copy ownership not recorded: %+v, %v", manifest, err)
					}
					writeTestFile(t, outside, "alpha/SKILL.md", "authored alpha")
					manifestBefore, err = os.ReadFile(filepath.Join(outside, sync.ManifestFile))
					if err != nil {
						t.Fatal(err)
					}
				}
				tc := skillTarget(target, mode, "")
				resolution, err := sync.ResolveTargetSkillsForTarget("custom", tc.SkillsConfig(), skills)
				if err != nil {
					t.Fatal(err)
				}
				aware := strings.HasPrefix(scenario, "profile_")
				r := collectTargetDiff("custom", tc, source, mode, resolution, nil, nil, profileDiffPreview{aware: aware, skills: skills})
				switch {
				case linkedDir == source:
					if r.mode != "symlink" || r.errMsg != "" || !r.synced || len(r.items) != 0 {
						t.Errorf("whole-source link behavior changed: %+v", r)
					}
				case !aware:
					if r.mode != "symlink" || !strings.Contains(r.errMsg, "Symlink points to different location") {
						t.Errorf("no-profile alias behavior changed: %+v", r)
					}
				default:
					if r.mode != mode || r.errMsg != "" || r.syncCount != 0 || r.synced {
						t.Errorf("profile alias bypassed directory preview: %+v", r)
					}
					if len(r.items) != 1 || r.items[0].name != "alpha" || r.items[0].action != "kept" || r.items[0].isSync || r.items[0].reason == "" {
						t.Errorf("profile alias omitted non-executable authored copy: %+v", r.items)
					}
				}
				if linkedDir == outside {
					if data, err := os.ReadFile(filepath.Join(outside, "alpha", "SKILL.md")); err != nil || string(data) != "authored alpha" {
						t.Errorf("preview changed authored bytes: %q, %v", data, err)
					}
					if data, err := os.ReadFile(filepath.Join(outside, sync.ManifestFile)); err != nil || string(data) != string(manifestBefore) {
						t.Error("preview changed managed manifest", err)
					}
				}
				if data, err := os.ReadFile(filepath.Join(source, "alpha", "SKILL.md")); err != nil || string(data) != string(sourceBytes) {
					t.Errorf("preview changed source: %q, %v", data, err)
				}
				if after, err := os.Readlink(target); err != nil || after != rootLink || !utils.IsSymlinkOrJunction(target) {
					t.Errorf("preview changed configured root link: %q, %v", after, err)
				}
			})
		}
	}
}
