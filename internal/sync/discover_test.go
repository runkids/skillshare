package sync

import (
	"os"
	"path/filepath"
	"testing"
)

func writeSkillMD(t *testing.T, dir, content string) {
	t.Helper()
	if err := os.MkdirAll(dir, 0755); err != nil {
		t.Fatalf("mkdir %s: %v", dir, err)
	}
	if err := os.WriteFile(filepath.Join(dir, "SKILL.md"), []byte(content), 0644); err != nil {
		t.Fatalf("write SKILL.md in %s: %v", dir, err)
	}
}

func TestDiscoverSourceSkills_SingleSkill(t *testing.T) {
	src := t.TempDir()
	writeSkillMD(t, filepath.Join(src, "my-skill"), "---\nname: my-skill\n---\n# My Skill")

	skills, err := DiscoverSourceSkills(src)
	if err != nil {
		t.Fatal(err)
	}
	if len(skills) != 1 {
		t.Fatalf("expected 1 skill, got %d", len(skills))
	}
	if skills[0].RelPath != "my-skill" {
		t.Errorf("expected relPath 'my-skill', got %q", skills[0].RelPath)
	}
	if skills[0].FlatName != "my-skill" {
		t.Errorf("expected flatName 'my-skill', got %q", skills[0].FlatName)
	}
	if skills[0].IsInRepo {
		t.Error("expected IsInRepo false for non-tracked skill")
	}
}

func TestDiscoverSourceSkills_Nested(t *testing.T) {
	src := t.TempDir()
	writeSkillMD(t, filepath.Join(src, "group", "sub-skill"), "---\nname: sub-skill\n---\n# Sub")

	skills, err := DiscoverSourceSkills(src)
	if err != nil {
		t.Fatal(err)
	}
	if len(skills) != 1 {
		t.Fatalf("expected 1 skill, got %d", len(skills))
	}
	if skills[0].FlatName != "group__sub-skill" {
		t.Errorf("expected flatName 'group__sub-skill', got %q", skills[0].FlatName)
	}
}

func TestDiscoverSourceSkills_SkipsGitDir(t *testing.T) {
	src := t.TempDir()
	writeSkillMD(t, filepath.Join(src, "real-skill"), "---\nname: real\n---\n# Real")
	// Put a SKILL.md inside .git — should be ignored
	writeSkillMD(t, filepath.Join(src, ".git", "hidden-skill"), "---\nname: hidden\n---\n# Hidden")

	skills, err := DiscoverSourceSkills(src)
	if err != nil {
		t.Fatal(err)
	}
	if len(skills) != 1 {
		t.Fatalf("expected 1 skill (skipping .git), got %d", len(skills))
	}
	if skills[0].FlatName != "real-skill" {
		t.Errorf("expected 'real-skill', got %q", skills[0].FlatName)
	}
}

func TestDiscoverSourceSkills_SkipsRoot(t *testing.T) {
	src := t.TempDir()
	// SKILL.md at root level should be skipped (relPath == ".")
	if err := os.WriteFile(filepath.Join(src, "SKILL.md"), []byte("---\nname: root\n---\n"), 0644); err != nil {
		t.Fatal(err)
	}
	writeSkillMD(t, filepath.Join(src, "child"), "---\nname: child\n---\n# Child")

	skills, err := DiscoverSourceSkills(src)
	if err != nil {
		t.Fatal(err)
	}
	if len(skills) != 1 {
		t.Fatalf("expected 1 skill (skipping root), got %d", len(skills))
	}
	if skills[0].FlatName != "child" {
		t.Errorf("expected 'child', got %q", skills[0].FlatName)
	}
}

func TestDiscoverSourceSkills_TrackedRepo(t *testing.T) {
	src := t.TempDir()
	// "_team" prefix indicates a tracked repo
	writeSkillMD(t, filepath.Join(src, "_team", "coding"), "---\nname: coding\n---\n# Coding")

	skills, err := DiscoverSourceSkills(src)
	if err != nil {
		t.Fatal(err)
	}
	if len(skills) != 1 {
		t.Fatalf("expected 1 skill, got %d", len(skills))
	}
	if !skills[0].IsInRepo {
		t.Error("expected IsInRepo true for _-prefixed parent")
	}
}

func TestDiscoverSourceSkills_ParsesTargets(t *testing.T) {
	src := t.TempDir()
	content := "---\nname: targeted\ntargets:\n  - claude\n  - cursor\n---\n# Targeted"
	writeSkillMD(t, filepath.Join(src, "targeted-skill"), content)

	skills, err := DiscoverSourceSkills(src)
	if err != nil {
		t.Fatal(err)
	}
	if len(skills) != 1 {
		t.Fatalf("expected 1 skill, got %d", len(skills))
	}
	if skills[0].Targets == nil {
		t.Fatal("expected Targets to be non-nil")
	}
	if len(skills[0].Targets) != 2 {
		t.Fatalf("expected 2 targets, got %d", len(skills[0].Targets))
	}
}

func TestDiscoverSourceSkills_EmptyDir(t *testing.T) {
	src := t.TempDir()

	skills, err := DiscoverSourceSkills(src)
	if err != nil {
		t.Fatal(err)
	}
	if len(skills) != 0 {
		t.Errorf("expected 0 skills for empty dir, got %d", len(skills))
	}
}

func TestDiscoverSourceSkills_NonExistent(t *testing.T) {
	// filepath.Walk skips inaccessible paths, so non-existent source returns empty list
	skills, err := DiscoverSourceSkills("/nonexistent/path/for/test")
	if err != nil {
		// Acceptable: some OS may return walk error
		return
	}
	if len(skills) != 0 {
		t.Errorf("expected 0 skills for non-existent path, got %d", len(skills))
	}
}

// --- DiscoverSourceSkillsLite tests ---

func TestDiscoverSourceSkillsLite_SkipsTargetsParsing(t *testing.T) {
	src := t.TempDir()
	content := "---\nname: targeted\ntargets:\n  - claude\n  - cursor\n---\n# Targeted"
	writeSkillMD(t, filepath.Join(src, "targeted-skill"), content)

	skills, repos, err := DiscoverSourceSkillsLite(src)
	if err != nil {
		t.Fatal(err)
	}
	if len(skills) != 1 {
		t.Fatalf("expected 1 skill, got %d", len(skills))
	}
	// Lite version should NOT parse targets
	if skills[0].Targets != nil {
		t.Errorf("expected Targets to be nil in Lite mode, got %v", skills[0].Targets)
	}
	if len(repos) != 0 {
		t.Errorf("expected 0 tracked repos, got %d", len(repos))
	}
}

func TestDiscoverSourceSkillsLite_CollectsTrackedRepos(t *testing.T) {
	src := t.TempDir()
	// Create a tracked repo with .git dir and a skill inside
	repoDir := filepath.Join(src, "_team")
	if err := os.MkdirAll(filepath.Join(repoDir, ".git"), 0755); err != nil {
		t.Fatal(err)
	}
	writeSkillMD(t, filepath.Join(repoDir, "coding"), "---\nname: coding\n---\n# Coding")

	skills, repos, err := DiscoverSourceSkillsLite(src)
	if err != nil {
		t.Fatal(err)
	}
	if len(skills) != 1 {
		t.Fatalf("expected 1 skill, got %d", len(skills))
	}
	if !skills[0].IsInRepo {
		t.Error("expected IsInRepo true")
	}
	if len(repos) != 1 {
		t.Fatalf("expected 1 tracked repo, got %d", len(repos))
	}
	if repos[0] != "_team" {
		t.Errorf("expected tracked repo '_team', got %q", repos[0])
	}
}

func TestDiscoverSourceSkillsLite_BasicDiscovery(t *testing.T) {
	src := t.TempDir()
	writeSkillMD(t, filepath.Join(src, "skill-a"), "---\nname: a\n---\n# A")
	writeSkillMD(t, filepath.Join(src, "group", "skill-b"), "---\nname: b\n---\n# B")

	skills, repos, err := DiscoverSourceSkillsLite(src)
	if err != nil {
		t.Fatal(err)
	}
	if len(skills) != 2 {
		t.Fatalf("expected 2 skills, got %d", len(skills))
	}
	if len(repos) != 0 {
		t.Errorf("expected 0 tracked repos, got %d", len(repos))
	}

	// Verify flat names are correct
	names := map[string]bool{}
	for _, s := range skills {
		names[s.FlatName] = true
	}
	if !names["skill-a"] {
		t.Error("missing skill-a")
	}
	if !names["group__skill-b"] {
		t.Error("missing group__skill-b")
	}
}

func TestDiscoverSourceSkillsLite_EmptyDir(t *testing.T) {
	src := t.TempDir()

	skills, repos, err := DiscoverSourceSkillsLite(src)
	if err != nil {
		t.Fatal(err)
	}
	if len(skills) != 0 {
		t.Errorf("expected 0 skills, got %d", len(skills))
	}
	if len(repos) != 0 {
		t.Errorf("expected 0 repos, got %d", len(repos))
	}
}
