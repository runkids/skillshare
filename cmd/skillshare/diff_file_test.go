package main

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"skillshare/internal/sync"
	"skillshare/internal/testutil"
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

func TestCollectTargetDiff_ProfileEqualMtime(t *testing.T) {
	root := t.TempDir()
	source, target := filepath.Join(root, "source"), filepath.Join(root, "target")
	writeSyncSkill(t, source, "demo", "demo")
	cfg := skillTarget(target, "copy", "standard")
	if _, err := sync.SyncTargetCopy("custom", cfg, source, false, false); err != nil {
		t.Fatal(err)
	}
	file := filepath.Join(source, "demo", "SKILL.md")
	info, err := os.Stat(file)
	if err != nil {
		t.Fatal(err)
	}
	writeTestFile(t, source, "demo/SKILL.md", "---\nname: demo\n---\nnew selected bytes\n")
	if err := os.Chtimes(file, info.ModTime(), info.ModTime()); err != nil {
		t.Fatal(err)
	}
	skills, err := sync.DiscoverSourceSkills(source)
	resolution, resolutionErr := sync.ResolveTargetSkillsForTarget("custom", cfg.SkillsConfig(), skills)
	if err != nil || resolutionErr != nil {
		t.Fatal(err, resolutionErr)
	}
	manifest, manifestErr := sync.ReadManifest(target)
	stamp, stampErr := sync.DirMaxMtimeWithIgnore(filepath.Dir(file), sync.DefaultFileIgnorePatterns())
	if manifestErr != nil || stampErr != nil || stamp != manifest.Mtimes["demo"] || !strings.HasPrefix(manifest.Outputs["demo"], "sha256-v2:") {
		t.Fatalf("equal-mtime current-output precondition: %+v %d %v %v", manifest, stamp, manifestErr, stampErr)
	}
	before := fmt.Sprint(testutil.SnapshotTree(t, true, root))
	for _, aware := range []bool{false, true} {
		r := collectTargetDiff("custom", cfg, source, "copy", resolution, nil, nil, profileDiffPreview{aware: aware, skills: skills})
		if r.errMsg != "" || aware && (len(r.items) != 1 || r.items[0].action != "modify" || !r.items[0].isSync || r.items[0].reason != "content changed") || !aware && len(r.items) != 0 {
			t.Errorf("equal-mtime selected bytes prediction aware=%v: %+v", aware, r)
		}
	}
	if fmt.Sprint(testutil.SnapshotTree(t, true, root)) != before {
		t.Error("preview changed file paths, modes, source, copy or manifest bytes")
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

func TestRenderDiffGroup_PreservedFileInventory(t *testing.T) {
	for _, mode := range []string{"stat", "patch"} {
		t.Run(mode, func(t *testing.T) {
			dst := t.TempDir()
			const authored = "# Authored locally\n"
			writeTestFile(t, dst, "SKILL.md", authored)
			kept := copyDiffEntry{action: "kept", name: "alpha", reason: "modified managed copy preserved", dstDir: dst}
			changes := copyDiffEntry{action: "modify", name: "beta", reason: "content changed", isSync: true, files: []fileDiffEntry{
				{RelPath: "added.md", Action: "add", SrcSize: 10},
				{RelPath: "changed.md", Action: "modify", DstSize: 10, SrcSize: 20},
				{RelPath: "removed.md", Action: "delete", DstSize: 10},
			}}
			opts := diffRenderOpts{showStat: mode == "stat", showPatch: mode == "patch"}
			var next, controlNext diffNext
			output := stripANSIWarnings(captureStdout(t, func() {
				renderDiffGroup(targetDiffResult{items: []copyDiffEntry{kept}}, opts, &next, 8)
				renderDiffGroup(targetDiffResult{items: []copyDiffEntry{changes}}, opts, &controlNext, 8)
			}))
			if !strings.Contains(output, "· SKILL.md") || strings.Contains(output, "- SKILL.md") {
				t.Errorf("kept file must be neutral inventory, not a deletion:\n%s", output)
			}
			for _, want := range []string{"+ added.md", "~ changed.md", "- removed.md", "alpha", kept.reason} {
				if !strings.Contains(output, want) {
					t.Errorf("missing real-change icon or preservation detail %q:\n%s", want, output)
				}
			}
			if next != (diffNext{}) || !controlNext.skills {
				t.Errorf("kept inventory became sync work or real work disappeared: kept=%+v changes=%+v", next, controlNext)
			}
			raw := diffItemToJSON(kept)
			if raw.Action != "kept" || raw.Name != "alpha" || raw.Reason != kept.reason || raw.IsSync {
				t.Errorf("rendering changed raw preservation semantics: %+v", raw)
			}
			if data, err := os.ReadFile(filepath.Join(dst, "SKILL.md")); err != nil || string(data) != authored {
				t.Errorf("rendering changed authored bytes: %q, %v", data, err)
			}
		})
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

func TestRenderGroupedDiffs_PreservedInventoryIdentity(t *testing.T) {
	for _, layout := range []string{"targets", "current_former"} {
		for _, mode := range []string{"stat", "patch"} {
			t.Run(layout+"/"+mode, func(t *testing.T) {
				root := t.TempDir()
				dirs := []string{filepath.Join(root, "current"), filepath.Join(root, "former")}
				files := []string{"current-only.md", "former-only.md"}
				contents := []string{"authored current bytes", "different authored former bytes"}
				const reason = "kept: copy was edited or could not be verified; collect or remove manually, or use verified --discard-edits"
				items := make([]copyDiffEntry, 2)
				for i := range items {
					writeTestFile(t, dirs[i], files[i], contents[i])
					items[i] = copyDiffEntry{name: "alpha", action: "kept", reason: reason, dstDir: dirs[i]}
				}
				results := []targetDiffResult{
					{name: "first", mode: "copy", items: items[:1]},
					{name: "second", mode: "copy", items: items[1:]},
				}
				if layout == "current_former" {
					items[0].reason += " (in " + dirs[0] + ")"
					items[1].reason += " (in " + dirs[1] + ")"
					results = []targetDiffResult{{name: "goose", mode: "copy", items: items}}
				}
				opts := diffRenderOpts{showStat: mode == "stat", showPatch: mode == "patch"}
				output := stripANSIWarnings(captureStdout(t, func() {
					renderGroupedDiffs(results, nil, opts)
				}))
				for i, file := range files {
					if strings.Count(output, "· "+file) != 1 || strings.Contains(output, "- "+file) {
						t.Errorf("inventory %s must appear once and stay neutral:\n%s", dirs[i], output)
					}
					if !strings.Contains(output, items[i].reason) {
						t.Errorf("lost full preservation reason %q:\n%s", items[i].reason, output)
					}
					if wire := diffItemToJSON(items[i]); wire.Action != "kept" || wire.Name != "alpha" || wire.IsSync || wire.Reason != items[i].reason {
						t.Errorf("inventory changed raw identity/reason: %+v", wire)
					}
					if data, err := os.ReadFile(filepath.Join(dirs[i], file)); err != nil || string(data) != contents[i] {
						t.Errorf("preview changed authored inventory %s: %q, %v", dirs[i], data, err)
					}
				}
				firstFile, secondFile := strings.Index(output, files[0]), strings.Index(output, files[1])
				if layout == "targets" {
					firstTarget, secondTarget := strings.Index(output, "first\n"), strings.Index(output, "second\n")
					if firstTarget < 0 || secondTarget < 0 || !(firstTarget < firstFile && firstFile < secondTarget && secondTarget < secondFile) {
						t.Errorf("detailed inventories lost their owning target association:\n%s", output)
					}
				} else {
					firstReason, secondReason := strings.Index(output, items[0].reason), strings.Index(output, items[1].reason)
					if firstFile < 0 || secondFile < 0 || !(firstFile < firstReason && firstReason < secondFile && secondFile < secondReason) {
						t.Errorf("same-name inventories were attributed to the wrong current/former reason:\n%s", output)
					}
				}
				if strings.Contains(output, "to sync") {
					t.Errorf("preserved inventories became executable work:\n%s", output)
				}
			})
		}
	}
}
