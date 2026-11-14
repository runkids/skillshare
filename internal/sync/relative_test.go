package sync

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"skillshare/internal/config"
	"skillshare/internal/sourcewalk"
	"skillshare/internal/utils"
)

func TestShouldUseRelative(t *testing.T) {
	tests := []struct {
		name        string
		projectRoot string
		sourcePath  string
		targetPath  string
		want        bool
	}{
		{
			name:        "both under project root",
			projectRoot: "/home/user/project",
			sourcePath:  "/home/user/project/.skillshare/skills/foo",
			targetPath:  "/home/user/project/.claude/skills",
			want:        true,
		},
		{
			name:        "source outside project root",
			projectRoot: "/home/user/project",
			sourcePath:  "/opt/shared/skills/foo",
			targetPath:  "/home/user/project/.claude/skills",
			want:        false,
		},
		{
			name:        "target outside project root",
			projectRoot: "/home/user/project",
			sourcePath:  "/home/user/project/.skillshare/skills/foo",
			targetPath:  "/opt/shared/claude/skills",
			want:        false,
		},
		{
			name:        "empty project root (global mode)",
			projectRoot: "",
			sourcePath:  "/home/user/.config/skillshare/skills/foo",
			targetPath:  "/home/user/.claude/skills",
			want:        false,
		},
		{
			name:        "both outside project root",
			projectRoot: "/home/user/project",
			sourcePath:  "/opt/a",
			targetPath:  "/opt/b",
			want:        false,
		},
		{
			name:        "project root itself as source",
			projectRoot: "/home/user/project",
			sourcePath:  "/home/user/project",
			targetPath:  "/home/user/project/.claude/skills",
			want:        true,
		},
		{
			name:        "filesystem root as project root",
			projectRoot: "/",
			sourcePath:  "/skills/foo",
			targetPath:  "/claude/skills",
			want:        true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := shouldUseRelative(tt.projectRoot, tt.sourcePath, tt.targetPath)
			if got != tt.want {
				t.Errorf("shouldUseRelative(%q, %q, %q) = %v, want %v",
					tt.projectRoot, tt.sourcePath, tt.targetPath, got, tt.want)
			}
		})
	}
}

func TestShouldUseRelative_CleansPaths(t *testing.T) {
	got := shouldUseRelative(
		"/home/user/project/",
		"/home/user/project/.skillshare/skills/../skills/foo",
		"/home/user/project/.claude/skills/",
	)
	if !got {
		t.Error("expected true for unclean but valid paths under project root")
	}
}

func TestShouldUseRelative_SymlinkedProjectRoot(t *testing.T) {
	tmp, _ := filepath.EvalSymlinks(t.TempDir())
	realProject := filepath.Join(tmp, "real-project")
	os.MkdirAll(filepath.Join(realProject, ".skillshare", "skills"), 0755)
	os.MkdirAll(filepath.Join(realProject, ".claude", "skills"), 0755)

	symlinkProject := filepath.Join(tmp, "workspace")
	if err := os.Symlink(realProject, symlinkProject); err != nil {
		t.Skip("symlinks not supported:", err)
	}

	// Symlinked root: both resolve under the real project
	got := shouldUseRelative(
		symlinkProject,
		filepath.Join(symlinkProject, ".skillshare", "skills"),
		filepath.Join(symlinkProject, ".claude", "skills"),
	)
	if !got {
		t.Error("expected true: both paths under symlinked project root")
	}
}

func TestShouldUseRelative_DivergentSymlink(t *testing.T) {
	tmp, _ := filepath.EvalSymlinks(t.TempDir())
	realProject := filepath.Join(tmp, "project")
	otherDir := filepath.Join(tmp, "other")
	os.MkdirAll(filepath.Join(realProject, ".skillshare", "skills"), 0755)
	os.MkdirAll(filepath.Join(otherDir, "skills"), 0755)

	// Symlink .claude/skills to outside the project
	os.MkdirAll(filepath.Join(realProject, ".claude"), 0755)
	if err := os.Symlink(filepath.Join(otherDir, "skills"), filepath.Join(realProject, ".claude", "skills")); err != nil {
		t.Skip("symlinks not supported:", err)
	}

	// Target resolves to outside project root → should use absolute
	got := shouldUseRelative(
		realProject,
		filepath.Join(realProject, ".skillshare", "skills"),
		filepath.Join(realProject, ".claude", "skills"),
	)
	if got {
		t.Error("expected false: target symlink resolves outside project root")
	}
}

// posixAbs gives a slash-form path the volume it needs to be absolute on Windows.
func posixAbs(p string) string {
	return filepath.VolumeName(os.TempDir()) + filepath.FromSlash(p)
}

func TestResolveReadlink(t *testing.T) {
	tests := []struct {
		name     string
		dest     string
		linkPath string
		want     string
	}{
		{
			"absolute dest unchanged",
			posixAbs("/abs/source/skill"),
			posixAbs("/project/.claude/skills/skill"),
			posixAbs("/abs/source/skill"),
		},
		{
			"relative dest resolved from link parent",
			"../../.skillshare/skills/skill",
			posixAbs("/project/.claude/skills/skill"),
			posixAbs("/project/.skillshare/skills/skill"),
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := resolveReadlink(tt.dest, tt.linkPath)
			if got != tt.want {
				t.Errorf("resolveReadlink(%q, %q) = %q, want %q", tt.dest, tt.linkPath, got, tt.want)
			}
		})
	}
}

func TestLinkNeedsReformat(t *testing.T) {
	tests := []struct {
		name         string
		dest         string
		wantRelative bool
		expected     bool
	}{
		{"absolute dest wants relative", posixAbs("/abs/path/to/skill"), true, true},
		{"absolute dest wants absolute", posixAbs("/abs/path/to/skill"), false, false},
		{"relative dest wants relative", "../../.skillshare/skills/foo", true, false},
		{"relative dest wants absolute", "../../.skillshare/skills/foo", false, true},
		{"empty dest", "", true, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := linkNeedsReformat(tt.dest, tt.wantRelative); got != tt.expected {
				t.Errorf("linkNeedsReformat(%q, %v) = %v, want %v", tt.dest, tt.wantRelative, got, tt.expected)
			}
		})
	}
}

func TestReformatLink(t *testing.T) {
	tmp, _ := filepath.EvalSymlinks(t.TempDir())
	source := filepath.Join(tmp, "source")
	os.MkdirAll(source, 0755)

	link := filepath.Join(tmp, "link")
	os.Symlink(source, link)

	// Verify setup: absolute link
	dest, _ := os.Readlink(link)
	if !filepath.IsAbs(dest) {
		t.Fatal("setup: link should be absolute")
	}

	// Reformat to relative (atomic)
	if err := reformatLink(link, source, true); err != nil {
		t.Fatalf("reformatLink: %v", err)
	}

	// Verify: now relative
	dest, _ = os.Readlink(link)
	if filepath.IsAbs(dest) {
		t.Errorf("after reformat, link should be relative, got %q", dest)
	}

	// Verify: still resolves correctly
	resolved, err := filepath.EvalSymlinks(link)
	if err != nil {
		t.Fatalf("symlink should resolve: %v", err)
	}
	if resolved != source {
		t.Errorf("resolved = %q, want %q", resolved, source)
	}

	// Verify: no temp file left behind
	if _, err := os.Lstat(link + ".ss-reformat"); err == nil {
		t.Error("temp file should be cleaned up")
	}
}

func TestRelativeLinkTargetThroughDirectoryAlias(t *testing.T) {
	root := t.TempDir()
	sourceRoot := filepath.Join(root, "source")
	source := filepath.Join(sourceRoot, "demo")
	physical := filepath.Join(root, "physical", "nested")
	for _, dir := range []string{source, physical} {
		if err := os.MkdirAll(dir, 0755); err != nil {
			t.Fatal(err)
		}
	}
	alias := filepath.Join(root, "target")
	if runtime.GOOS == "windows" {
		if err := utils.CreateJunction(alias, physical); err != nil {
			t.Fatal(err)
		}
	} else if err := os.Symlink(physical, alias); err != nil {
		t.Fatal(err)
	}
	if runtime.GOOS == "windows" && shouldUseRelative(root, source, alias) {
		t.Fatal("junction target must retain absolute-link mode across syncs")
	}
	link := filepath.Join(alias, "skill")
	if err := createLink(link, source, true, source); err != nil {
		t.Fatal(err)
	}
	actual, err := os.Stat(link)
	want, wantErr := os.Stat(source)
	if err != nil || wantErr != nil || !os.SameFile(actual, want) {
		raw, readErr := os.Readlink(link)
		t.Fatalf("relative link through alias does not reach source: %v, %v (raw=%q, read=%v, parent=%q, source=%q)", err, wantErr, raw, readErr, evalOrClean(filepath.Dir(link)), evalOrClean(source))
	}
	resolved, err := utils.ResolveLinkTarget(link)
	if err != nil || !utils.PathsEqual(evalOrClean(resolved), evalOrClean(source)) {
		t.Fatalf("resolved link = %q, %v; want %q", resolved, err, source)
	}
	raw, err := os.Readlink(link)
	if runtime.GOOS == "windows" && err == nil && !filepath.IsAbs(raw) {
		t.Fatalf("link through junction must be absolute, got %q", raw)
	}
	if err != nil || !utils.PathsEqual(evalOrClean(resolveReadlink(raw, link)), evalOrClean(source)) {
		t.Fatalf("resolved raw link = %q, %v; want %q", raw, err, source)
	}
	if err := os.Remove(link); err != nil {
		t.Fatal(err)
	}
	content := "---\nname: demo\n---\n# Demo\n"
	if err := os.WriteFile(filepath.Join(source, "SKILL.md"), []byte(content), 0644); err != nil {
		t.Fatal(err)
	}
	skills, err := DiscoverSourceSkills(sourceRoot)
	if err != nil || len(skills) != 1 {
		t.Fatalf("discover: %+v, %v", skills, err)
	}
	target := SkillTarget{Name: "test", Target: config.TargetConfig{Path: alias}, Mode: "merge"}
	opts := SkillRunOptions{Source: sourceRoot, ProjectRoot: root, ProfileAware: true}
	for i := 0; i < 2; i++ {
		if r := SyncSkillTarget(target, skills, opts); r.Err != nil || len(r.Blocked)+len(r.KeptModified)+len(r.Updated) != 0 || len(r.Linked) != 1 {
			t.Fatalf("sync %d: %+v", i, r)
		}
		manifest, err := ReadManifest(alias)
		if err != nil {
			t.Fatal(err)
		}
		if got, allowed := PreviewEntry("demo", alias, sourceRoot, skills, manifest, nil); !allowed || got.Class != EntryManagedLink {
			t.Fatalf("preview %d: %+v, %v", i, got, allowed)
		}
	}
	target.Target.Exclude = []string{"*"}
	if r := SyncSkillTarget(target, skills, opts); r.Err != nil || r.PruneErr != nil || len(r.Pruned) != 1 {
		t.Fatalf("exclude through alias: %+v", r)
	}
	if _, err := os.Lstat(filepath.Join(physical, "demo")); !os.IsNotExist(err) {
		t.Fatalf("excluded link remains: %v", err)
	}
	if data, err := os.ReadFile(filepath.Join(source, "SKILL.md")); err != nil || string(data) != content {
		t.Fatalf("sync changed source: %q, %v", data, err)
	}
	missing := filepath.Join(alias, "not-created", "child")
	if got := evalOrClean(missing); !utils.PathsEqual(got, filepath.Join(evalOrClean(physical), "not-created", "child")) {
		t.Fatalf("missing tail did not retain physical ancestor: %s", got)
	}
}

func TestRepeatSyncThroughSharedProjectAlias(t *testing.T) {
	sourceReal := linkedRepoSource(t)
	writeAnalyzeSkill(t, sourceReal, "demo", "body")
	root := filepath.Dir(sourceReal)
	alias := filepath.Join(t.TempDir(), "project")
	if runtime.GOOS == "windows" {
		if err := utils.CreateJunction(alias, root); err != nil {
			t.Fatal(err)
		}
	} else if err := os.Symlink(root, alias); err != nil {
		t.Fatal(err)
	}
	source := filepath.Join(alias, filepath.Base(sourceReal))
	skills, err := DiscoverSourceSkills(source, sourcewalk.Options{Follow: sourcewalk.NewFollow(source, nil)})
	if err != nil || len(skills) != 2 {
		t.Fatalf("discovery: %+v, %v", skills, err)
	}
	file := ExtraFile{Source: filepath.Join(source, "demo", "SKILL.md"), Target: filepath.Join(alias, "extra-file")}
	if platformCanCreateFileLink() {
		if err := createLink(file.Target, file.Source, true); err != nil {
			t.Fatal(err)
		}
		if !file.isOurLink() {
			t.Fatal("single-file extra lost ownership through project alias")
		}
	}
	whole := config.TargetConfig{Path: filepath.Join(alias, "whole")}
	merged := config.TargetConfig{Path: filepath.Join(alias, "merged")}
	for i := 0; i < 2; i++ {
		if err := SyncTarget("whole", whole, source, false, alias); err != nil {
			t.Fatalf("symlink sync %d: %v", i, err)
		}
		if !LinksTo(whole.Path, source) {
			t.Fatal("whole-directory link lost ownership through project alias")
		}
		if got := CheckStatus(whole.Path, source); got != StatusLinked {
			t.Fatalf("symlink status %d: %v", i, got)
		}
		if got, _, _ := CheckStatusMerge(whole.Path, source); got != StatusLinked {
			t.Fatalf("whole-source merge status %d: %v", i, got)
		}
		r := SyncSkillTarget(SkillTarget{Name: "merged", Target: merged, Mode: "merge"}, skills, SkillRunOptions{Source: source, ProjectRoot: alias, ProfileAware: true})
		if r.Err != nil || r.PruneErr != nil || len(r.Updated)+len(r.Blocked)+len(r.KeptModified) != 0 || len(r.Linked) != 2 {
			t.Fatalf("merge sync %d: %+v", i, r)
		}
		if got, linked, local := CheckStatusMerge(merged.Path, source); got != StatusMerged || linked != 2 || local != 0 {
			t.Fatalf("merge status %d: %v, linked=%d, local=%d", i, got, linked, local)
		}
	}
}
