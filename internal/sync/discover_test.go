package sync

import (
	"os"
	"path/filepath"
	"strings"
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

// --- .skillignore tests (Issue #83) ---

func TestDiscoverSourceSkills_RespectsSkillIgnore(t *testing.T) {
	src := t.TempDir()

	// Create a tracked repo with .skillignore excluding .venv
	repoDir := filepath.Join(src, "_team-skills")
	os.MkdirAll(filepath.Join(repoDir, ".git"), 0755)
	os.WriteFile(filepath.Join(repoDir, ".skillignore"), []byte(".venv\n"), 0644)

	// Vendored SKILL.md inside .venv — should be ignored
	venvSkill := filepath.Join(repoDir, ".venv", "lib", "python3.13", "site-packages", "fastapi", ".agents", "skills", "fastapi")
	writeSkillMD(t, venvSkill, "not a real skill")

	// Normal skill — should be discovered
	writeSkillMD(t, filepath.Join(repoDir, "my-skill"), "---\nname: my-skill\n---\n# My Skill")

	skills, err := DiscoverSourceSkills(src)
	if err != nil {
		t.Fatal(err)
	}

	for _, s := range skills {
		if strings.Contains(s.RelPath, ".venv") {
			t.Errorf("expected .venv skill to be filtered by .skillignore, got %s", s.RelPath)
		}
	}
	if len(skills) != 1 {
		t.Errorf("expected 1 skill, got %d", len(skills))
	}
}

func TestDiscoverSourceSkillsLite_RespectsSkillIgnore(t *testing.T) {
	src := t.TempDir()

	// Same layout as above
	repoDir := filepath.Join(src, "_team-skills")
	os.MkdirAll(filepath.Join(repoDir, ".git"), 0755)
	os.WriteFile(filepath.Join(repoDir, ".skillignore"), []byte(".venv\n"), 0644)

	venvSkill := filepath.Join(repoDir, ".venv", "lib", "fastapi")
	writeSkillMD(t, venvSkill, "not a real skill")

	writeSkillMD(t, filepath.Join(repoDir, "my-skill"), "---\nname: my-skill\n---\n# My Skill")

	skills, repos, err := DiscoverSourceSkillsLite(src)
	if err != nil {
		t.Fatal(err)
	}

	for _, s := range skills {
		if strings.Contains(s.RelPath, ".venv") {
			t.Errorf("expected .venv skill to be filtered by .skillignore, got %s", s.RelPath)
		}
	}
	if len(skills) != 1 {
		t.Errorf("expected 1 skill, got %d", len(skills))
	}
	if len(repos) != 1 || repos[0] != "_team-skills" {
		t.Errorf("expected tracked repo [_team-skills], got %v", repos)
	}
}

// --- SkipDir + root-level .skillignore tests ---

func TestDiscoverSourceSkills_SkipDirDoesNotDescend(t *testing.T) {
	src := t.TempDir()

	repoDir := filepath.Join(src, "_team")
	os.MkdirAll(filepath.Join(repoDir, ".git"), 0755)
	os.WriteFile(filepath.Join(repoDir, ".skillignore"), []byte(".venv\n"), 0644)

	// Deep nested SKILL.md inside .venv
	deepPath := filepath.Join(repoDir, ".venv", "a", "b", "c", "d", "e")
	writeSkillMD(t, deepPath, "deep vendored skill")

	writeSkillMD(t, filepath.Join(repoDir, "real-skill"), "---\nname: real\n---\n# Real")

	skills, err := DiscoverSourceSkills(src)
	if err != nil {
		t.Fatal(err)
	}
	if len(skills) != 1 {
		t.Errorf("expected 1 skill, got %d", len(skills))
	}
}

func TestDiscoverSourceSkills_RootLevelSkillIgnore(t *testing.T) {
	src := t.TempDir()

	os.WriteFile(filepath.Join(src, ".skillignore"), []byte("draft-*\nmy-hidden\n"), 0644)

	writeSkillMD(t, filepath.Join(src, "draft-feature"), "---\nname: draft-feature\n---\n")
	writeSkillMD(t, filepath.Join(src, "draft-experiment"), "---\nname: draft-experiment\n---\n")
	writeSkillMD(t, filepath.Join(src, "my-hidden"), "---\nname: my-hidden\n---\n")
	writeSkillMD(t, filepath.Join(src, "visible-skill"), "---\nname: visible\n---\n")

	skills, err := DiscoverSourceSkills(src)
	if err != nil {
		t.Fatal(err)
	}
	if len(skills) != 1 {
		t.Errorf("expected 1 skill (visible-skill), got %d", len(skills))
	}
	if len(skills) == 1 && skills[0].FlatName != "visible-skill" {
		t.Errorf("expected 'visible-skill', got %q", skills[0].FlatName)
	}
}

func TestDiscoverSourceSkillsLite_RootLevelSkillIgnore(t *testing.T) {
	src := t.TempDir()

	os.WriteFile(filepath.Join(src, ".skillignore"), []byte("hidden\n"), 0644)

	writeSkillMD(t, filepath.Join(src, "hidden"), "---\nname: hidden\n---\n")
	writeSkillMD(t, filepath.Join(src, "visible"), "---\nname: visible\n---\n")

	skills, _, err := DiscoverSourceSkillsLite(src)
	if err != nil {
		t.Fatal(err)
	}
	if len(skills) != 1 {
		t.Errorf("expected 1 skill, got %d", len(skills))
	}
}

func TestDiscoverSourceSkills_RootSkipsEntireTrackedRepo(t *testing.T) {
	src := t.TempDir()

	os.WriteFile(filepath.Join(src, ".skillignore"), []byte("_unwanted\n"), 0644)

	repoDir := filepath.Join(src, "_unwanted")
	os.MkdirAll(filepath.Join(repoDir, ".git"), 0755)
	writeSkillMD(t, filepath.Join(repoDir, "skill-a"), "---\nname: a\n---\n")
	writeSkillMD(t, filepath.Join(repoDir, "skill-b"), "---\nname: b\n---\n")

	writeSkillMD(t, filepath.Join(src, "kept"), "---\nname: kept\n---\n")

	skills, err := DiscoverSourceSkills(src)
	if err != nil {
		t.Fatal(err)
	}
	if len(skills) != 1 {
		t.Errorf("expected 1 skill, got %d", len(skills))
	}
}

func TestDiscoverSourceSkills_RootAndRepoLayering(t *testing.T) {
	src := t.TempDir()

	os.WriteFile(filepath.Join(src, ".skillignore"), []byte("draft-*\n"), 0644)

	repoDir := filepath.Join(src, "_team")
	os.MkdirAll(filepath.Join(repoDir, ".git"), 0755)
	os.WriteFile(filepath.Join(repoDir, ".skillignore"), []byte(".venv\n"), 0644)

	writeSkillMD(t, filepath.Join(src, "draft-wip"), "---\nname: wip\n---\n")
	writeSkillMD(t, filepath.Join(repoDir, ".venv", "pkg"), "vendored")
	writeSkillMD(t, filepath.Join(repoDir, "real"), "---\nname: real\n---\n")

	skills, err := DiscoverSourceSkills(src)
	if err != nil {
		t.Fatal(err)
	}
	if len(skills) != 1 {
		t.Errorf("expected 1 skill (real), got %d", len(skills))
	}
}

func TestDiscoverSourceSkills_EmptyRootSkillIgnore(t *testing.T) {
	src := t.TempDir()

	os.WriteFile(filepath.Join(src, ".skillignore"), []byte("# just a comment\n\n"), 0644)

	writeSkillMD(t, filepath.Join(src, "skill-a"), "---\nname: a\n---\n")
	writeSkillMD(t, filepath.Join(src, "skill-b"), "---\nname: b\n---\n")

	skills, err := DiscoverSourceSkills(src)
	if err != nil {
		t.Fatal(err)
	}
	if len(skills) != 2 {
		t.Errorf("expected 2 skills, got %d", len(skills))
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
