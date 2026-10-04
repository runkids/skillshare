package sync

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"skillshare/internal/config"
	"skillshare/internal/sourcewalk"
)

// followFixture is a skills source whose `_f` entry links to an external group
// with skills a and b, plus an in-source skill local.
type followFixture struct {
	src, ext, tgt string
}

// newFollowFixture builds the fixture under root. src and tgt are where the
// source and target are addressed, which may cross links created by the caller.
func newFollowFixture(t *testing.T, src, tgt string) followFixture {
	t.Helper()
	ext := t.TempDir()
	for _, name := range []string{"a", "b"} {
		writeSkillMD(t, filepath.Join(ext, "f", name), "---\nname: "+name+"\n---\n# "+name)
	}
	writeSkillMD(t, filepath.Join(src, "local"), "---\nname: local\n---\n# local")
	if err := os.Symlink(filepath.Join(ext, "f"), filepath.Join(src, "_f")); err != nil {
		t.Fatal(err)
	}
	writeFile(t, filepath.Join(src, ".skillfollow"), "_f\n")
	if err := os.MkdirAll(tgt, 0755); err != nil {
		t.Fatal(err)
	}
	return followFixture{src: src, ext: ext, tgt: tgt}
}

func writeFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}
}

// discover builds a fresh set and discovery for one operation, as the CLI does.
func (f followFixture) discover(t *testing.T) (*sourcewalk.FollowSet, []DiscoveredSkill) {
	t.Helper()
	set := sourcewalk.Follow(f.src, sourcewalk.FollowOptions{TargetPaths: []string{f.tgt}})
	skills, _, err := DiscoverSourceSkillsWithOptions(f.src, DiscoveryOptions{Follow: &set})
	if err != nil {
		t.Fatal(err)
	}
	return &set, skills
}

func (f followFixture) merge(t *testing.T, target config.TargetConfig, projectRoot string) *MergeResult {
	t.Helper()
	set, skills := f.discover(t)
	result, err := SyncTargetMergeWithSkillsOptions("test", target, skills, f.src, false, false, projectRoot, MergeOptions{Follow: set})
	if err != nil {
		t.Fatal(err)
	}
	return result
}

// assertConverges syncs twice and requires the second sync to update nothing.
func (f followFixture) assertConverges(t *testing.T, target config.TargetConfig, projectRoot string) {
	t.Helper()
	f.merge(t, target, projectRoot)
	second := f.merge(t, target, projectRoot)
	if len(second.Updated) != 0 || len(second.Linked) != 3 {
		t.Fatalf("second sync: linked %v, updated %v", second.Linked, second.Updated)
	}
}

func TestSkillfollowIdentity_GlobalAbsoluteLink(t *testing.T) {
	f := newFollowFixture(t, filepath.Join(t.TempDir(), "src"), filepath.Join(t.TempDir(), "tgt"))
	f.assertConverges(t, config.TargetConfig{Path: f.tgt, Mode: "merge"}, "")

	if dest, _ := os.Readlink(filepath.Join(f.tgt, "_f__a")); dest != filepath.Join(f.src, "_f", "a") {
		t.Fatalf("link text = %s", dest)
	}
}

func TestSkillfollowIdentity_ProjectRelativeLinkKeepsEntry(t *testing.T) {
	root := t.TempDir()
	f := newFollowFixture(t, filepath.Join(root, ".skillshare", "skills"), filepath.Join(root, ".claude", "skills"))
	f.assertConverges(t, config.TargetConfig{Path: f.tgt, Mode: "merge"}, root)

	dest, _ := os.Readlink(filepath.Join(f.tgt, "_f__a"))
	if want := filepath.Join("..", "..", ".skillshare", "skills", "_f", "a"); dest != want {
		t.Fatalf("link text = %s, want %s", dest, want)
	}
}

func TestSkillfollowIdentity_LinkedSourceRootAndTargetParent(t *testing.T) {
	root := t.TempDir()
	realSrc := filepath.Join(root, "real-src")
	realTgtParent := filepath.Join(root, "real-claude")
	if err := os.MkdirAll(realSrc, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(realTgtParent, "skills"), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(root, ".skillshare"), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(realSrc, filepath.Join(root, ".skillshare", "skills")); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(realTgtParent, filepath.Join(root, ".claude")); err != nil {
		t.Fatal(err)
	}
	f := newFollowFixture(t, filepath.Join(root, ".skillshare", "skills"), filepath.Join(root, ".claude", "skills"))
	f.assertConverges(t, config.TargetConfig{Path: f.tgt, Mode: "merge"}, root)

	dest, _ := os.Readlink(filepath.Join(f.tgt, "_f__a"))
	if want := filepath.Join("..", "..", "real-src", "_f", "a"); dest != want {
		t.Fatalf("link text = %s, want %s", dest, want)
	}
}

func TestSkillfollowIdentity_FullyResolvedLinkIsSameSkill(t *testing.T) {
	f := newFollowFixture(t, filepath.Join(t.TempDir(), "src"), filepath.Join(t.TempDir(), "tgt"))
	resolved := filepath.Join(f.ext, "f", "a")
	if err := os.Symlink(resolved, filepath.Join(f.tgt, "_f__a")); err != nil {
		t.Fatal(err)
	}
	target := config.TargetConfig{Path: f.tgt, Mode: "merge"}
	f.assertConverges(t, target, "")

	if dest, _ := os.Readlink(filepath.Join(f.tgt, "_f__a")); dest != resolved {
		t.Fatalf("fully resolved link was replaced: %s", dest)
	}
}

func TestSameSkillLink_RejectsSiblingUnderOwnedRoot(t *testing.T) {
	f := newFollowFixture(t, filepath.Join(t.TempDir(), "src"), filepath.Join(t.TempDir(), "tgt"))
	_, skills := f.discover(t)
	link := filepath.Join(f.tgt, "_f__a")
	if err := os.Symlink(filepath.Join(f.src, "_f", "b"), link); err != nil {
		t.Fatal(err)
	}
	for _, skill := range skills {
		if skill.RelPath == "_f/a" && sameSkillLink(link, skill, newFollowScope(f.src, nil)) {
			t.Fatal("a link to _f/b must not count as _f/a")
		}
	}
}

func TestRelativeLinkText_SkillKeepsLogicalTail(t *testing.T) {
	f := newFollowFixture(t, filepath.Join(t.TempDir(), "src"), filepath.Join(t.TempDir(), "tgt"))
	text, err := relativeLinkText(filepath.Join(f.tgt, "_f__a"), filepath.Join(f.src, "_f", "a"), &skillLinkPath{root: f.src, tail: "_f/a"})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasSuffix(text, filepath.Join("src", "_f", "a")) {
		t.Fatalf("skill link text resolved the followed entry: %s", text)
	}
	legacy, err := relativeLinkText(filepath.Join(f.tgt, "_f__a"), filepath.Join(f.src, "_f", "a"), nil)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasSuffix(legacy, filepath.Join("f", "a")) || strings.Contains(legacy, "_f") {
		t.Fatalf("agents and extras keep resolving the whole destination: %s", legacy)
	}
}

func (f followFixture) prune(t *testing.T, exclude []string, force bool) *PruneResult {
	t.Helper()
	set, skills := f.discover(t)
	result, err := PruneOrphanLinksWithSkills(PruneOptions{
		TargetPath: f.tgt, SourcePath: f.src, Skills: skills, Exclude: exclude,
		TargetName: "test", Force: force, Follow: set,
	})
	if err != nil {
		t.Fatal(err)
	}
	return result
}

func assertGone(t *testing.T, path string) {
	t.Helper()
	if _, err := os.Lstat(path); !os.IsNotExist(err) {
		t.Fatalf("%s still exists (%v)", path, err)
	}
}

func assertPresent(t *testing.T, path string) {
	t.Helper()
	if _, err := os.Lstat(path); err != nil {
		t.Fatalf("%s was removed: %v", path, err)
	}
}

func TestSkillfollowOwnership_PrunesFullyResolvedManagedLink(t *testing.T) {
	f := newFollowFixture(t, filepath.Join(t.TempDir(), "src"), filepath.Join(t.TempDir(), "tgt"))
	if err := os.Symlink(filepath.Join(f.ext, "f", "a"), filepath.Join(f.tgt, "_f__a")); err != nil {
		t.Fatal(err)
	}
	f.merge(t, config.TargetConfig{Path: f.tgt, Mode: "merge"}, "")

	f.prune(t, []string{"_f__a"}, false)
	assertGone(t, filepath.Join(f.tgt, "_f__a"))
}

func TestSkillfollowOwnership_UnmanagedLinkIntoFollowedTargetIsLocal(t *testing.T) {
	f := newFollowFixture(t, filepath.Join(t.TempDir(), "src"), filepath.Join(t.TempDir(), "tgt"))
	if err := os.Symlink(filepath.Join(f.ext, "f", "a"), filepath.Join(f.tgt, "mine")); err != nil {
		t.Fatal(err)
	}

	result := f.prune(t, nil, false)
	assertPresent(t, filepath.Join(f.tgt, "mine"))
	if len(result.LocalDirs) != 1 || result.LocalDirs[0] != "mine" {
		t.Fatalf("local dirs = %v", result.LocalDirs)
	}
}

func TestSkillfollowOwnership_UnfollowPrunesLinksThroughEntry(t *testing.T) {
	for _, linkedRoot := range []bool{false, true} {
		t.Run(map[bool]string{false: "plain source", true: "linked source root"}[linkedRoot], func(t *testing.T) {
			root := t.TempDir()
			src := filepath.Join(root, ".skillshare", "skills")
			if linkedRoot {
				real := filepath.Join(root, "real-src")
				if err := os.MkdirAll(real, 0755); err != nil {
					t.Fatal(err)
				}
				if err := os.MkdirAll(filepath.Dir(src), 0755); err != nil {
					t.Fatal(err)
				}
				if err := os.Symlink(real, src); err != nil {
					t.Fatal(err)
				}
			}
			f := newFollowFixture(t, src, filepath.Join(root, ".claude", "skills"))
			f.merge(t, config.TargetConfig{Path: f.tgt, Mode: "merge"}, root)

			writeFile(t, filepath.Join(f.src, ".skillfollow"), "# _f unfollowed\n")
			result := f.prune(t, nil, false)
			for _, name := range []string{"_f__a", "_f__b"} {
				assertGone(t, filepath.Join(f.tgt, name))
			}
			if len(result.Removed) != 2 {
				t.Fatalf("removed = %v, warnings = %v", result.Removed, result.Warnings)
			}
		})
	}
}

func TestSkillfollowOwnership_UnfollowKeepsPhysicalManagedLink(t *testing.T) {
	f := newFollowFixture(t, filepath.Join(t.TempDir(), "src"), filepath.Join(t.TempDir(), "tgt"))
	link := filepath.Join(f.tgt, "_f__a")
	if err := os.Symlink(filepath.Join(f.ext, "f", "a"), link); err != nil {
		t.Fatal(err)
	}
	f.merge(t, config.TargetConfig{Path: f.tgt, Mode: "merge"}, "")
	writeFile(t, filepath.Join(f.src, ".skillfollow"), "")

	result := f.prune(t, nil, false)
	assertPresent(t, link)
	want := "_f__a: managed link resolves outside the source after unfollow; remove it or re-run with --force"
	if len(result.Warnings) != 1 || result.Warnings[0] != want {
		t.Fatalf("warnings = %v", result.Warnings)
	}

	f.prune(t, nil, true)
	assertGone(t, link)
}

func TestSkillfollowOwnership_HandMadeExternalLinkWithoutDeclarations(t *testing.T) {
	src, tgt := setupMergeTest(t, "alpha")
	ext := t.TempDir()
	writeSkillMD(t, filepath.Join(ext, "a"), "---\nname: a\n---\n# a")
	if err := os.Symlink(filepath.Join(ext, "a"), filepath.Join(tgt, "a")); err != nil {
		t.Fatal(err)
	}
	skills, err := DiscoverSourceSkills(src)
	if err != nil {
		t.Fatal(err)
	}

	result, err := PruneOrphanLinksWithSkills(PruneOptions{TargetPath: tgt, SourcePath: src, Skills: skills, TargetName: "test"})
	if err != nil {
		t.Fatal(err)
	}
	assertPresent(t, filepath.Join(tgt, "a"))
	if len(result.Warnings) != 1 || !strings.Contains(result.Warnings[0], "symlink to external location") {
		t.Fatalf("warnings = %v", result.Warnings)
	}
}

// run syncs and prunes one target the way the CLI does.
func (f followFixture) run(t *testing.T, mode, naming string, force bool) SkillTargetResult {
	t.Helper()
	set, skills := f.discover(t)
	target := config.TargetConfig{Skills: &config.ResourceTargetConfig{Path: f.tgt, Mode: mode, TargetNaming: naming}}
	result := SyncSkillTarget(SkillTarget{Name: "test", Target: target, Mode: mode}, skills, SkillRunOptions{Source: f.src, Force: force, Follow: set})
	if result.Err != nil {
		t.Fatal(result.Err)
	}
	return result
}

// targetNames lists what the target holds, without the manifest.
func targetNames(t *testing.T, dir string) []string {
	t.Helper()
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	var names []string
	for _, entry := range entries {
		if entry.Name() != ManifestFile {
			names = append(names, entry.Name())
		}
	}
	return names
}

func TestSkillfollowPause_MissingEntryRemovesNothing(t *testing.T) {
	for _, mode := range []string{"merge", "copy"} {
		for _, naming := range []string{"flat", "standard"} {
			for _, force := range []bool{false, true} {
				t.Run(fmt.Sprintf("%s/%s/force=%v", mode, naming, force), func(t *testing.T) {
					f := newFollowFixture(t, filepath.Join(t.TempDir(), "src"), filepath.Join(t.TempDir(), "tgt"))
					f.run(t, mode, naming, false)
					before := targetNames(t, f.tgt)

					// The drive holding the followed group is unmounted.
					if err := os.Rename(filepath.Join(f.ext, "f"), filepath.Join(f.ext, "f.off")); err != nil {
						t.Fatal(err)
					}
					result := f.run(t, mode, naming, force)

					if got := targetNames(t, f.tgt); strings.Join(got, ",") != strings.Join(before, ",") {
						t.Fatalf("target changed: %v -> %v", before, got)
					}
					if len(result.Pruned) != 0 || strings.Join(result.PrunePaused, ",") != "_f (missing)" {
						t.Fatalf("pruned %v, paused %v", result.Pruned, result.PrunePaused)
					}
				})
			}
		}
	}
}

func TestSkillfollowPause_InvalidTargetKeepsCopies(t *testing.T) {
	f := newFollowFixture(t, filepath.Join(t.TempDir(), "src"), filepath.Join(t.TempDir(), "tgt"))
	f.run(t, "copy", "flat", false)

	link := filepath.Join(f.src, "_f")
	if err := os.Remove(link); err != nil {
		t.Fatal(err)
	}
	writeFile(t, link, "not a directory")
	result := f.run(t, "copy", "flat", true)

	for _, name := range []string{"_f__a", "_f__b"} {
		assertPresent(t, filepath.Join(f.tgt, name, "SKILL.md"))
	}
	if strings.Join(result.PrunePaused, ",") != "_f (invalid-target)" {
		t.Fatalf("paused = %v", result.PrunePaused)
	}
}

// newOfflineFixture serves a from `_offline`, then makes `_offline` missing
// while another a, under other/, becomes available.
func newOfflineFixture(t *testing.T, mode, naming string) (followFixture, SkillTargetResult) {
	t.Helper()
	src, ext := filepath.Join(t.TempDir(), "src"), t.TempDir()
	writeSkillMD(t, filepath.Join(ext, "a"), "---\nname: a\n---\n# offline a")
	if err := os.MkdirAll(src, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(ext, filepath.Join(src, "_offline")); err != nil {
		t.Fatal(err)
	}
	writeFile(t, filepath.Join(src, ".skillfollow"), "_offline\n")
	f := followFixture{src: src, ext: ext, tgt: filepath.Join(t.TempDir(), "tgt")}
	f.run(t, mode, naming, false)

	if err := os.Rename(ext, ext+".off"); err != nil {
		t.Fatal(err)
	}
	writeSkillMD(t, filepath.Join(src, "other", "a"), "---\nname: a\n---\n# other a")
	return f, f.run(t, mode, naming, true)
}

func TestSkillfollowPause_StandardCopyKeepsUnprovableCopy(t *testing.T) {
	f, result := newOfflineFixture(t, "copy", "standard")

	content, err := os.ReadFile(filepath.Join(f.tgt, "a", "SKILL.md"))
	if err != nil || !strings.Contains(string(content), "offline a") {
		t.Fatalf("copy a was replaced: %q %v", content, err)
	}
	if strings.Join(result.Kept, ",") != "a" || len(result.Updated) != 0 {
		t.Fatalf("kept %v, updated %v", result.Kept, result.Updated)
	}
}

func TestSkillfollowPause_FlatCopyProceeds(t *testing.T) {
	f, result := newOfflineFixture(t, "copy", "flat")

	assertPresent(t, filepath.Join(f.tgt, "_offline__a", "SKILL.md"))
	assertPresent(t, filepath.Join(f.tgt, "other__a", "SKILL.md"))
	if len(result.Kept) != 0 || strings.Join(result.Linked, ",") != "other__a" {
		t.Fatalf("kept %v, copied %v", result.Kept, result.Linked)
	}
}

func TestSkillfollowPause_MergeRelinksWithWarning(t *testing.T) {
	f, result := newOfflineFixture(t, "merge", "standard")

	if dest, _ := os.Readlink(filepath.Join(f.tgt, "a")); dest != filepath.Join(f.src, "other", "a") {
		t.Fatalf("a -> %s", dest)
	}
	want := "test: a: relinked while a followed entry is unavailable; this name may previously have come from that entry and may collide when it returns"
	if strings.Join(result.Warnings, "\n") != want {
		t.Fatalf("warnings = %v", result.Warnings)
	}
}

func TestSkillfollowPause_ResumesWhenEntryReturns(t *testing.T) {
	f := newFollowFixture(t, filepath.Join(t.TempDir(), "src"), filepath.Join(t.TempDir(), "tgt"))
	f.run(t, "merge", "flat", false)
	writeFile(t, filepath.Join(f.src, ".skillfollow"), "_f\n_gone\n")
	if paused := f.run(t, "merge", "flat", false).PrunePaused; len(paused) != 1 {
		t.Fatalf("paused = %v", paused)
	}

	writeFile(t, filepath.Join(f.src, ".skillfollow"), "")
	result := f.run(t, "merge", "flat", false)
	if len(result.PrunePaused) != 0 || len(result.Pruned) != 2 {
		t.Fatalf("paused %v, pruned %v", result.PrunePaused, result.Pruned)
	}
}
