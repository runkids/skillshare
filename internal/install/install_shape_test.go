package install

import (
	"maps"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

// installCollectionFixture commits a repository whose root SKILL.md sits above
// other skills, one of them with a skill nested inside it, and installs the
// root as "root" and that parent as "parent" the way `install <repo> --all`
// does: the root as its SKILL.md alone, the parent without its nested skill.
func installCollectionFixture(t *testing.T) (repo, sourceDir string) {
	t.Helper()
	repo = initTestRepo(t)
	writeRepoFile(t, repo, "SKILL.md", mergeBaseSkill)
	writeRepoFile(t, repo, "README.md", "readme\n")
	writeRepoFile(t, repo, "skills/child/SKILL.md", "---\nname: child\n---\nchild\n")
	writeRepoFile(t, repo, "skills/parent/SKILL.md", "---\nname: parent\n---\nparent v1\n")
	writeRepoFile(t, repo, "skills/parent/notes.md", "notes\n")
	writeRepoFile(t, repo, "skills/parent/nested/SKILL.md", "---\nname: nested\n---\nnested\n")
	gitAdd(t, repo, ".")
	gitCommit(t, repo, "v1")

	source, err := ParseSource("file://" + repo)
	if err != nil {
		t.Fatal(err)
	}
	discovery, err := DiscoverFromGit(source)
	if err != nil {
		t.Fatal(err)
	}
	defer CleanupDiscovery(discovery)
	sourceDir = t.TempDir()
	for _, skill := range discovery.Skills {
		name := map[string]string{".": "root", "skills/parent": "parent"}[skill.Path]
		if name == "" {
			continue
		}
		if _, err := InstallFromDiscovery(discovery, skill, filepath.Join(sourceDir, name), InstallOptions{SourceDir: sourceDir, SkipAudit: true}); err != nil {
			t.Fatal(err)
		}
	}
	return repo, sourceDir
}

// updateRecorded updates the installed skill name from its recorded source,
// as `skillshare update <name>` does.
func updateRecorded(t *testing.T, sourceDir, name string, merge bool) (*InstallResult, error) {
	t.Helper()
	entry := LoadMetadataOrNew(sourceDir).Get(name)
	if entry == nil {
		t.Fatalf("no record for %s", name)
	}
	source, err := RecordedSource(entry, ParseOptions{})
	if err != nil {
		t.Fatal(err)
	}
	opts := InstallOptions{SourceDir: sourceDir, Update: true, SkipAudit: true}
	if merge {
		opts.MergeBase = entry
	}
	return Install(source, filepath.Join(sourceDir, name), opts)
}

// skillFiles lists the files of an installed skill, slash-separated.
func skillFiles(t *testing.T, dir string) []string {
	t.Helper()
	hashes, err := ComputeFileHashes(dir)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(dir, ".git")); err == nil {
		hashes[".git/"] = ""
	}
	return slices.Sorted(maps.Keys(hashes))
}

func TestUpdate_DiscoveryInstallKeepsItsLayout(t *testing.T) {
	repo, sourceDir := installCollectionFixture(t)
	want := map[string][]string{
		"root":   {"SKILL.md"},
		"parent": {"SKILL.md", "notes.md"},
	}
	// Two rounds: the first update must keep the shape, and so must the
	// next one, which starts from what the first left behind.
	for round := 2; round <= 3; round++ {
		v := map[int]string{2: "v2", 3: "v3"}[round]
		editSkillFile(t, repo, "SKILL.md", "five", "five "+v)
		editSkillFile(t, repo, "skills/parent/SKILL.md", "parent v", "parent v"+v+" ")
		gitAdd(t, repo, ".")
		gitCommit(t, repo, v)

		for name, files := range want {
			if _, err := updateRecorded(t, sourceDir, name, false); err != nil {
				t.Fatalf("%s: update %s: %v", v, name, err)
			}
			dest := filepath.Join(sourceDir, name)
			if got := skillFiles(t, dest); !slices.Equal(got, files) {
				t.Errorf("%s: %s holds %v, want %v", v, name, got, files)
			}
			if got := readSkillFile(t, dest, "SKILL.md"); !strings.Contains(got, v) {
				t.Errorf("%s: %s SKILL.md = %q, want the upstream change", v, name, got)
			}
			entry := LoadMetadataOrNew(sourceDir).Get(name)
			if got := slices.Sorted(maps.Keys(entry.FileHashes)); !slices.Equal(got, files) {
				t.Errorf("%s: %s records %v, want %v", v, name, got, files)
			}
		}
	}
}

func TestUpdate_KeepsNestedSkillTheInstallCopied(t *testing.T) {
	repo, sourceDir := installCollectionFixture(t)
	// A direct install of the parent's path copies its nested skill too.
	source, err := ParseSource("file://" + repo + "//skills/parent")
	if err != nil {
		t.Fatal(err)
	}
	dest := filepath.Join(sourceDir, "whole")
	if _, err := Install(source, dest, InstallOptions{SourceDir: sourceDir, SkipAudit: true}); err != nil {
		t.Fatal(err)
	}
	editSkillFile(t, repo, "skills/parent/nested/SKILL.md", "nested", "nested v2")
	gitAdd(t, repo, ".")
	gitCommit(t, repo, "v2")

	if _, err := updateRecorded(t, sourceDir, "whole", false); err != nil {
		t.Fatal(err)
	}
	if got := readSkillFile(t, dest, "nested/SKILL.md"); !strings.Contains(got, "nested v2") {
		t.Errorf("nested/SKILL.md = %q, want it kept and updated", got)
	}
}

func TestMergeUpdate_DiscoveryInstalledRoot(t *testing.T) {
	repo, sourceDir := installCollectionFixture(t)
	dest := filepath.Join(sourceDir, "root")
	editSkillFile(t, dest, "SKILL.md", "one", "one (mine)")
	editSkillFile(t, repo, "SKILL.md", "five", "five (upstream)")
	gitAdd(t, repo, ".")
	gitCommit(t, repo, "v2")

	result, err := updateRecorded(t, sourceDir, "root", true)
	if err != nil {
		t.Fatalf("merge: %v", err)
	}
	if result.Action != "merged" {
		t.Errorf("action = %q, want merged", result.Action)
	}
	if got := skillFiles(t, dest); !slices.Equal(got, []string{"SKILL.md"}) {
		t.Errorf("root holds %v, want SKILL.md alone", got)
	}
	want := strings.NewReplacer("one", "one (mine)", "five", "five (upstream)").Replace(mergeBaseSkill)
	if got := readSkillFile(t, dest, "SKILL.md"); got != want {
		t.Errorf("SKILL.md =\n%s\nwant\n%s", got, want)
	}
	if !LoadMetadataOrNew(sourceDir).Get("root").LocallyModified(dest) {
		t.Error("merged skill is not reported as modified")
	}
}
