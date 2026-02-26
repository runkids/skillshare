package install

import (
	"testing"
)

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
