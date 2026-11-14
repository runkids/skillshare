package install

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

type publicationContext struct {
	metadataInstallContext
	entries []SkillEntryDTO
	err     error
}

func (c publicationContext) ConfigSkills() []SkillEntryDTO { return c.entries }
func (c publicationContext) Reconcile() error              { return c.err }

func TestConfigInstalledPublicationPaths(t *testing.T) {
	for _, dry := range []bool{false, true} {
		root, origin := t.TempDir(), t.TempDir()
		if err := os.WriteFile(filepath.Join(origin, "SKILL.md"), []byte("# Fixture"), 0600); err != nil {
			t.Fatal(err)
		}
		ctx := publicationContext{metadataInstallContext{root}, []SkillEntryDTO{
			{Name: "review", Group: "team", Source: origin},
			{Name: "broken", Source: filepath.Join(origin, "absent")},
		}, errors.New("reconcile failed")}
		result, err := InstallFromConfig(ctx, InstallOptions{SkipAudit: true, Quiet: true, DryRun: dry})
		want := []string{filepath.Join(root, "team", "review")}
		if dry {
			if err != nil || len(result.InstalledPaths) != 0 || result.Installed != 0 {
				t.Fatalf("dry run published paths: %+v (%v)", result, err)
			}
			continue
		}
		if !errors.Is(err, ctx.err) || !reflect.DeepEqual(result.InstalledPaths, want) || result.Installed != 1 || len(result.FailedSkills) != 1 || !reflect.DeepEqual(result.InstalledSkills, []string{"team/review"}) {
			t.Fatalf("partial successes lost or public names changed: %+v (%v)", result, err)
		}
		if _, err := os.Stat(filepath.Join(want[0], "SKILL.md")); err != nil {
			t.Fatal("successful publication missing", err)
		}
		data, err := json.Marshal(result)
		if err != nil || strings.Contains(string(data), "InstalledPaths") || strings.Contains(string(data), want[0]) {
			t.Fatalf("internal paths leaked: %s (%v)", data, err)
		}
		ctx.err = nil
		skipped, err := InstallFromConfig(ctx, InstallOptions{SkipAudit: true, Quiet: true})
		if err != nil || skipped.Skipped != 1 || len(skipped.InstalledPaths) != 0 {
			t.Fatalf("already-present path published again: %+v (%v)", skipped, err)
		}
	}
}

func TestConfigTrackedAndLockedPublicationPaths(t *testing.T) {
	remote := makeRemote(t, "next")
	root := t.TempDir()
	ctx := publicationContext{metadataInstallContext{root}, []SkillEntryDTO{{Name: "_repo", Group: "team", Source: remote, Tracked: true}}, nil}
	result, err := InstallFromConfig(ctx, InstallOptions{SkipAudit: true, Quiet: true})
	want := filepath.Join(root, "team", "_repo")
	if err != nil || !reflect.DeepEqual(result.InstalledPaths, []string{want}) || result.InstalledRepos != 1 || result.InstalledRepoSkills != 1 {
		t.Fatalf("tracked publication: %+v (%v)", result, err)
	}
	gitOut, err := gitOutput("", "ls-remote", remote, "refs/heads/next")
	if err != nil {
		t.Fatal(err)
	}
	refs := strings.Fields(gitOut)
	if len(refs) != 2 {
		t.Fatal("missing fixture revision", gitOut)
	}
	commit := refs[0]
	lock := &Lock{Skills: map[string]LockEntry{"team/_repo": {Source: remote, Commit: commit}}}
	result, err = InstallFromConfig(ctx, InstallOptions{SkipAudit: true, Quiet: true, Lock: lock, Update: true})
	if err != nil || result.Installed != 1 || !reflect.DeepEqual(result.InstalledPaths, []string{want}) || InstalledCommit(want, nil) != commit {
		t.Fatalf("locked revision publication: %+v (%v)", result, err)
	}
}

func TestConfigGroupedAndTrackedRootsKeepPublicNames(t *testing.T) {
	repo := initTestRepo(t)
	for _, path := range []string{"one/review", "two/review"} {
		dir := filepath.Join(repo, filepath.FromSlash(path))
		if err := os.MkdirAll(dir, 0755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(dir, "SKILL.md"), []byte("# Fixture"), 0600); err != nil {
			t.Fatal(err)
		}
	}
	gitAdd(t, repo, ".")
	gitCommit(t, repo, "fixture")
	remote := localFileURL(repo)
	for _, tracked := range []bool{false, true} {
		root := t.TempDir()
		entries := []SkillEntryDTO{{Name: "a", Group: "team", Source: remote + "//one/review"}, {Name: "b", Group: "team", Source: remote + "//two/review"}}
		paths, names := []string{filepath.Join(root, "team", "a"), filepath.Join(root, "team", "b")}, []string{"team/a", "team/b"}
		if tracked {
			entries = []SkillEntryDTO{{Name: "_repo", Group: "team", Source: remote, Tracked: true}}
			paths, names = []string{filepath.Join(root, "team", "_repo")}, []string{"review", "review"}
		}
		ctx := publicationContext{metadataInstallContext{root}, entries, nil}
		result, err := InstallFromConfig(ctx, InstallOptions{SkipAudit: true, Quiet: true})
		if err != nil || result.Installed != len(entries) || len(result.FailedSkills) != 0 || !reflect.DeepEqual(result.InstalledPaths, paths) || !reflect.DeepEqual(result.InstalledSkills, names) {
			t.Fatalf("grouped/tracked identity changed: %+v (%v)", result, err)
		}
		for _, path := range paths {
			if _, err := os.Stat(path); err != nil {
				t.Fatal("publication root missing", err)
			}
		}
	}
	// A tracked repository with no skills still retains its display fallback.
	if err := os.RemoveAll(filepath.Join(repo, "one")); err != nil {
		t.Fatal(err)
	}
	if err := os.RemoveAll(filepath.Join(repo, "two")); err != nil {
		t.Fatal(err)
	}
	gitAdd(t, repo, ".")
	gitCommit(t, repo, "empty repository")
	root := t.TempDir()
	ctx := publicationContext{metadataInstallContext{root}, []SkillEntryDTO{{Name: "_empty", Source: remote, Tracked: true}}, nil}
	result, err := InstallFromConfig(ctx, InstallOptions{SkipAudit: true, Quiet: true})
	if err != nil || result.InstalledRepos != 1 || result.InstalledRepoSkills != 0 || !reflect.DeepEqual(result.InstalledSkills, []string{"_empty"}) || !reflect.DeepEqual(result.InstalledPaths, []string{filepath.Join(root, "_empty")}) {
		t.Fatalf("empty repo display semantics changed: %+v (%v)", result, err)
	}
}

func TestGroupConfigSkillsByRepo(t *testing.T) {
	mustParse := func(raw string) *Source {
		s, err := ParseSource(raw)
		if err != nil {
			t.Fatalf("ParseSource(%q): %v", raw, err)
		}
		return s
	}

	t.Run("two skills from same repo are grouped", func(t *testing.T) {
		entries := []configSkillEntry{
			{dto: SkillEntryDTO{Name: "skill-a", Source: "github.com/org/repo/skills/a"}, source: mustParse("github.com/org/repo/skills/a")},
			{dto: SkillEntryDTO{Name: "skill-b", Source: "github.com/org/repo/skills/b"}, source: mustParse("github.com/org/repo/skills/b")},
		}

		groups, singles := groupConfigSkillsByRepo(entries)

		if len(groups) != 1 {
			t.Fatalf("expected 1 group, got %d", len(groups))
		}
		if len(groups[0].skills) != 2 {
			t.Errorf("expected 2 skills in group, got %d", len(groups[0].skills))
		}
		if len(singles) != 0 {
			t.Errorf("expected 0 singles, got %d", len(singles))
		}
	})

	t.Run("single skill from repo stays as single", func(t *testing.T) {
		entries := []configSkillEntry{
			{dto: SkillEntryDTO{Name: "skill-a", Source: "github.com/org/repo/skills/a"}, source: mustParse("github.com/org/repo/skills/a")},
		}

		groups, singles := groupConfigSkillsByRepo(entries)

		if len(groups) != 0 {
			t.Errorf("expected 0 groups, got %d", len(groups))
		}
		if len(singles) != 1 {
			t.Errorf("expected 1 single, got %d", len(singles))
		}
	})

	t.Run("root-level repo (no subdir) stays as single", func(t *testing.T) {
		entries := []configSkillEntry{
			{dto: SkillEntryDTO{Name: "repo-a", Source: "github.com/org/repo-a"}, source: mustParse("github.com/org/repo-a")},
			{dto: SkillEntryDTO{Name: "repo-b", Source: "github.com/org/repo-b"}, source: mustParse("github.com/org/repo-b")},
		}

		groups, singles := groupConfigSkillsByRepo(entries)

		if len(groups) != 0 {
			t.Errorf("expected 0 groups, got %d", len(groups))
		}
		if len(singles) != 2 {
			t.Errorf("expected 2 singles, got %d", len(singles))
		}
	})

	t.Run("local path stays as single", func(t *testing.T) {
		src := &Source{Type: SourceTypeLocalPath, Path: "/tmp/skill", Name: "skill"}
		entries := []configSkillEntry{
			{dto: SkillEntryDTO{Name: "skill", Source: "/tmp/skill"}, source: src},
		}

		groups, singles := groupConfigSkillsByRepo(entries)

		if len(groups) != 0 {
			t.Errorf("expected 0 groups, got %d", len(groups))
		}
		if len(singles) != 1 {
			t.Errorf("expected 1 single, got %d", len(singles))
		}
	})

	t.Run("mixed repos group correctly", func(t *testing.T) {
		entries := []configSkillEntry{
			// Repo A: 3 skills → should group
			{dto: SkillEntryDTO{Name: "a1", Source: "github.com/org/repoA/skills/a1"}, source: mustParse("github.com/org/repoA/skills/a1")},
			{dto: SkillEntryDTO{Name: "a2", Source: "github.com/org/repoA/skills/a2"}, source: mustParse("github.com/org/repoA/skills/a2")},
			{dto: SkillEntryDTO{Name: "a3", Source: "github.com/org/repoA/skills/a3"}, source: mustParse("github.com/org/repoA/skills/a3")},
			// Repo B: 1 skill → single
			{dto: SkillEntryDTO{Name: "b1", Source: "github.com/org/repoB/skills/b1"}, source: mustParse("github.com/org/repoB/skills/b1")},
			// Root-level → single
			{dto: SkillEntryDTO{Name: "root", Source: "github.com/org/root-skill"}, source: mustParse("github.com/org/root-skill")},
		}

		groups, singles := groupConfigSkillsByRepo(entries)

		if len(groups) != 1 {
			t.Fatalf("expected 1 group, got %d", len(groups))
		}
		if len(groups[0].skills) != 3 {
			t.Errorf("expected 3 skills in group, got %d", len(groups[0].skills))
		}
		if len(singles) != 2 {
			t.Errorf("expected 2 singles, got %d", len(singles))
		}
	})

	t.Run("web URLs with a recorded branch share one clone", func(t *testing.T) {
		recorded := func(raw string) *Source {
			s := mustParse(raw)
			s.ApplyRecordedBranch("main")
			return s
		}
		entries := []configSkillEntry{
			{dto: SkillEntryDTO{Name: "a"}, source: recorded("github.com/org/repo/tree/main/skills/a")},
			{dto: SkillEntryDTO{Name: "b"}, source: recorded("github.com/org/repo/tree/main/skills/b")},
		}

		groups, singles := groupConfigSkillsByRepo(entries)

		if len(groups) != 1 || len(singles) != 0 {
			t.Errorf("got %d groups and %d singles, want 1 group and 0 singles", len(groups), len(singles))
		}
	})

	t.Run("empty input returns empty results", func(t *testing.T) {
		groups, singles := groupConfigSkillsByRepo(nil)

		if len(groups) != 0 {
			t.Errorf("expected 0 groups, got %d", len(groups))
		}
		if len(singles) != 0 {
			t.Errorf("expected 0 singles, got %d", len(singles))
		}
	})
}

func TestRepoSourceForConfigGroup(t *testing.T) {
	src, err := ParseSource("github.com/anthropics/skills/skills/pdf")
	if err != nil {
		t.Fatalf("ParseSource: %v", err)
	}

	repoSrc := repoSourceForConfigGroup(src)

	if repoSrc.Subdir != "" {
		t.Errorf("expected empty Subdir, got %q", repoSrc.Subdir)
	}
	if repoSrc.CloneURL != "https://github.com/anthropics/skills.git" {
		t.Errorf("unexpected CloneURL: %s", repoSrc.CloneURL)
	}
	if repoSrc.Name != "skills" {
		t.Errorf("expected Name 'skills', got %q", repoSrc.Name)
	}
}

func TestMatchDiscoveredSkillBySubdir(t *testing.T) {
	discovery := &DiscoveryResult{
		Skills: []SkillInfo{
			{Name: "pdf", Path: "skills/pdf"},
			{Name: "commit", Path: "skills/commit"},
			{Name: "review", Path: "tools/review"},
		},
	}

	t.Run("exact match", func(t *testing.T) {
		skill, found := matchDiscoveredSkillBySubdir(discovery, "skills/pdf")
		if !found {
			t.Fatal("expected to find skill")
		}
		if skill.Name != "pdf" {
			t.Errorf("expected 'pdf', got %q", skill.Name)
		}
	})

	t.Run("no match", func(t *testing.T) {
		_, found := matchDiscoveredSkillBySubdir(discovery, "skills/nonexistent")
		if found {
			t.Error("expected no match")
		}
	})
}
