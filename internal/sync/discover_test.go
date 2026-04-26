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

func TestDiscoverSourceSkills_InfersTargetsFromHostPaths(t *testing.T) {
	src := t.TempDir()

	writeSkillMD(t, filepath.Join(src, ".cursor", "skills", "gstack-browse"), "---\nname: gstack-browse\n---\n# Cursor")
	writeSkillMD(t, filepath.Join(src, "openclaw", "skills", "gstack-openclaw-investigate"), "---\nname: gstack-openclaw-investigate\n---\n# Openclaw")
	writeSkillMD(t, filepath.Join(src, ".factory", "skills", "gstack-benchmark"), "---\nname: gstack-benchmark\n---\n# Factory")
	writeSkillMD(t, filepath.Join(src, ".skills", "letta-skill"), "---\nname: letta-skill\n---\n# Letta")
	writeSkillMD(t, filepath.Join(src, ".agents", "skills", "universal-skill"), "---\nname: universal-skill\n---\n# Universal")
	writeSkillMD(t, filepath.Join(src, "shared", "generic"), "---\nname: generic\n---\n# Generic")

	skills, err := DiscoverSourceSkills(src)
	if err != nil {
		t.Fatal(err)
	}
	if len(skills) != 6 {
		t.Fatalf("expected 6 skills, got %d", len(skills))
	}

	byRel := make(map[string]DiscoveredSkill)
	for _, s := range skills {
		byRel[s.RelPath] = s
	}

	cursorSkill := byRel[".cursor/skills/gstack-browse"]
	if cursorSkill.RelPath == "" {
		t.Fatalf("missing .cursor/skills/gstack-browse")
	}
	if len(cursorSkill.Targets) != 1 || cursorSkill.Targets[0] != "cursor" {
		t.Errorf("expected cursor host skill to target [cursor], got %v", cursorSkill.Targets)
	}

	openclawSkill := byRel["openclaw/skills/gstack-openclaw-investigate"]
	if openclawSkill.RelPath == "" {
		t.Fatalf("missing openclaw/skills/gstack-openclaw-investigate")
	}
	if len(openclawSkill.Targets) != 1 || openclawSkill.Targets[0] != "openclaw" {
		t.Errorf("expected openclaw host skill to target [openclaw], got %v", openclawSkill.Targets)
	}

	factorySkill := byRel[".factory/skills/gstack-benchmark"]
	if factorySkill.RelPath == "" {
		t.Fatalf("missing .factory/skills/gstack-benchmark")
	}
	if len(factorySkill.Targets) != 1 || factorySkill.Targets[0] != "droid" {
		t.Errorf("expected .factory host skill to target [droid], got %v", factorySkill.Targets)
	}

	lettaSkill := byRel[".skills/letta-skill"]
	if lettaSkill.RelPath == "" {
		t.Fatalf("missing .skills/letta-skill")
	}
	if len(lettaSkill.Targets) != 1 || lettaSkill.Targets[0] != "letta" {
		t.Errorf("expected .skills host skill to target [letta], got %v", lettaSkill.Targets)
	}

	universalSkill := byRel[".agents/skills/universal-skill"]
	if universalSkill.RelPath == "" {
		t.Fatalf("missing .agents/skills/universal-skill")
	}
	if len(universalSkill.Targets) != 1 || universalSkill.Targets[0] != "universal" {
		t.Errorf("expected .agents host skill to target [universal], got %v", universalSkill.Targets)
	}

	genericSkill := byRel["shared/generic"]
	if genericSkill.RelPath == "" {
		t.Fatalf("missing shared/generic")
	}
	if genericSkill.Targets != nil {
		t.Errorf("expected generic skill targets to be nil, got %v", genericSkill.Targets)
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

func TestDiscoverSourceSkillsLite_DoesNotInferTargetsFromPath(t *testing.T) {
	src := t.TempDir()
	writeSkillMD(t, filepath.Join(src, ".cursor", "skills", "gstack-browse"), "---\nname: gstack-browse\n---\n# Cursor")

	skills, repos, err := DiscoverSourceSkillsLite(src)
	if err != nil {
		t.Fatal(err)
	}
	if len(skills) != 1 {
		t.Fatalf("expected 1 skill, got %d", len(skills))
	}
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

// --- Gitignore syntax integration tests ---

func TestDiscoverSourceSkills_DoubleStarPattern(t *testing.T) {
	src := t.TempDir()

	repoDir := filepath.Join(src, "_team")
	os.MkdirAll(filepath.Join(repoDir, ".git"), 0755)
	// ** should match at any depth
	os.WriteFile(filepath.Join(repoDir, ".skillignore"), []byte("**/temp\n"), 0644)

	writeSkillMD(t, filepath.Join(repoDir, "temp"), "ignored")
	writeSkillMD(t, filepath.Join(repoDir, "sub", "temp"), "deep ignored")
	writeSkillMD(t, filepath.Join(repoDir, "real-skill"), "# Real")

	skills, err := DiscoverSourceSkills(src)
	if err != nil {
		t.Fatal(err)
	}
	for _, s := range skills {
		if strings.Contains(s.RelPath, "temp") {
			t.Errorf("temp skill should be ignored by **/temp, got %s", s.RelPath)
		}
	}
	if len(skills) != 1 {
		t.Errorf("expected 1 skill (real-skill), got %d: %v", len(skills), skills)
	}
}

func TestDiscoverSourceSkills_NegationPattern(t *testing.T) {
	src := t.TempDir()

	repoDir := filepath.Join(src, "_team")
	os.MkdirAll(filepath.Join(repoDir, ".git"), 0755)
	// Ignore all test-* but keep test-important
	os.WriteFile(filepath.Join(repoDir, ".skillignore"), []byte("test-*\n!test-important\n"), 0644)

	writeSkillMD(t, filepath.Join(repoDir, "test-alpha"), "ignored")
	writeSkillMD(t, filepath.Join(repoDir, "test-beta"), "ignored")
	writeSkillMD(t, filepath.Join(repoDir, "test-important"), "kept")
	writeSkillMD(t, filepath.Join(repoDir, "prod-skill"), "kept")

	skills, err := DiscoverSourceSkills(src)
	if err != nil {
		t.Fatal(err)
	}

	nameSet := map[string]bool{}
	for _, s := range skills {
		nameSet[s.FlatName] = true
	}

	if nameSet["_team__test-alpha"] {
		t.Error("test-alpha should be ignored")
	}
	if nameSet["_team__test-beta"] {
		t.Error("test-beta should be ignored")
	}
	if !nameSet["_team__test-important"] {
		t.Error("test-important should be kept (negation)")
	}
	if !nameSet["_team__prod-skill"] {
		t.Error("prod-skill should be kept")
	}
}

func TestDiscoverSourceSkills_DirOnlyPattern(t *testing.T) {
	src := t.TempDir()

	repoDir := filepath.Join(src, "_team")
	os.MkdirAll(filepath.Join(repoDir, ".git"), 0755)
	// demo/ with trailing slash — should ignore the demo directory
	os.WriteFile(filepath.Join(repoDir, ".skillignore"), []byte("demo/\n"), 0644)

	writeSkillMD(t, filepath.Join(repoDir, "demo"), "ignored dir")
	writeSkillMD(t, filepath.Join(repoDir, "demo-skill"), "NOT ignored — different name")
	writeSkillMD(t, filepath.Join(repoDir, "real"), "kept")

	skills, err := DiscoverSourceSkills(src)
	if err != nil {
		t.Fatal(err)
	}

	nameSet := map[string]bool{}
	for _, s := range skills {
		nameSet[s.FlatName] = true
	}

	if nameSet["_team__demo"] {
		t.Error("demo should be ignored by demo/ pattern")
	}
	if !nameSet["_team__demo-skill"] {
		t.Error("demo-skill should NOT be ignored (different from demo/)")
	}
	if !nameSet["_team__real"] {
		t.Error("real should be kept")
	}
}

func TestDiscoverSourceSkills_QuestionMarkPattern(t *testing.T) {
	src := t.TempDir()

	repoDir := filepath.Join(src, "_team")
	os.MkdirAll(filepath.Join(repoDir, ".git"), 0755)
	os.WriteFile(filepath.Join(repoDir, ".skillignore"), []byte("?.md-test\n"), 0644)

	writeSkillMD(t, filepath.Join(repoDir, "a.md-test"), "ignored")
	writeSkillMD(t, filepath.Join(repoDir, "ab.md-test"), "kept")
	writeSkillMD(t, filepath.Join(repoDir, "real-skill"), "kept")

	skills, err := DiscoverSourceSkills(src)
	if err != nil {
		t.Fatal(err)
	}

	for _, s := range skills {
		if s.FlatName == "_team__a.md-test" {
			t.Error("a.md-test should be ignored by ?.md-test pattern")
		}
	}
}

func TestDiscoverSourceSkills_CharClassPattern(t *testing.T) {
	src := t.TempDir()

	repoDir := filepath.Join(src, "_team")
	os.MkdirAll(filepath.Join(repoDir, ".git"), 0755)
	os.WriteFile(filepath.Join(repoDir, ".skillignore"), []byte("[Tt]emp\n"), 0644)

	writeSkillMD(t, filepath.Join(repoDir, "Temp"), "ignored")
	writeSkillMD(t, filepath.Join(repoDir, "temp"), "ignored")
	writeSkillMD(t, filepath.Join(repoDir, "hemp"), "kept")

	skills, err := DiscoverSourceSkills(src)
	if err != nil {
		t.Fatal(err)
	}

	nameSet := map[string]bool{}
	for _, s := range skills {
		nameSet[s.FlatName] = true
	}

	if nameSet["_team__Temp"] {
		t.Error("Temp should be ignored by [Tt]emp")
	}
	if nameSet["_team__temp"] {
		t.Error("temp should be ignored by [Tt]emp")
	}
	if !nameSet["_team__hemp"] {
		t.Error("hemp should be kept")
	}
}

func TestDiscoverSourceSkillsLite_NegationWithCanSkipDir(t *testing.T) {
	src := t.TempDir()

	repoDir := filepath.Join(src, "_team")
	os.MkdirAll(filepath.Join(repoDir, ".git"), 0755)
	// Ignore vendor/ but un-ignore vendor/important
	os.WriteFile(filepath.Join(repoDir, ".skillignore"), []byte("vendor/\n!vendor/important\n"), 0644)

	writeSkillMD(t, filepath.Join(repoDir, "vendor", "lib-a"), "ignored")
	writeSkillMD(t, filepath.Join(repoDir, "vendor", "important"), "kept by negation")
	writeSkillMD(t, filepath.Join(repoDir, "real"), "kept")

	skills, _, err := DiscoverSourceSkillsLite(src)
	if err != nil {
		t.Fatal(err)
	}

	nameSet := map[string]bool{}
	for _, s := range skills {
		nameSet[s.FlatName] = true
	}

	if nameSet["_team__vendor__lib-a"] {
		t.Error("vendor/lib-a should be ignored by vendor/")
	}
	if !nameSet["_team__vendor__important"] {
		t.Error("vendor/important should be kept by !vendor/important negation")
	}
	if !nameSet["_team__real"] {
		t.Error("real should be kept")
	}
}

func TestDiscoverSourceSkills_RootLevelGitignoreSyntax(t *testing.T) {
	src := t.TempDir()

	// Root-level .skillignore with gitignore syntax
	os.WriteFile(filepath.Join(src, ".skillignore"), []byte("**/temp\ndemo/\n!demo/keep\n"), 0644)

	writeSkillMD(t, filepath.Join(src, "temp"), "ignored by **/temp")
	writeSkillMD(t, filepath.Join(src, "sub", "temp"), "ignored by **/temp")
	writeSkillMD(t, filepath.Join(src, "demo", "sub"), "ignored by demo/")
	writeSkillMD(t, filepath.Join(src, "demo", "keep"), "kept by !demo/keep")
	writeSkillMD(t, filepath.Join(src, "real"), "kept")

	skills, err := DiscoverSourceSkills(src)
	if err != nil {
		t.Fatal(err)
	}

	nameSet := map[string]bool{}
	for _, s := range skills {
		nameSet[s.RelPath] = true
	}

	if nameSet["temp"] {
		t.Error("temp should be ignored by **/temp")
	}
	if nameSet["sub/temp"] {
		t.Error("sub/temp should be ignored by **/temp")
	}
	if nameSet["demo/sub"] {
		t.Error("demo/sub should be ignored by demo/")
	}
	if !nameSet["demo/keep"] {
		t.Error("demo/keep should be kept by !demo/keep")
	}
	if !nameSet["real"] {
		t.Error("real should be kept")
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

// --- DiscoverSourceSkillsWithStats tests ---

func TestDiscoverSourceSkillsWithStats_ReturnsIgnoredSkills(t *testing.T) {
	src := t.TempDir()

	// Create a tracked repo with .skillignore
	repoDir := filepath.Join(src, "_team")
	os.MkdirAll(filepath.Join(repoDir, ".git"), 0755)
	os.WriteFile(filepath.Join(repoDir, ".skillignore"), []byte("test-*\nvendor/\n"), 0644)

	// Skills that should be ignored
	writeSkillMD(t, filepath.Join(repoDir, "test-alpha"), "ignored")
	writeSkillMD(t, filepath.Join(repoDir, "test-beta"), "ignored")
	writeSkillMD(t, filepath.Join(repoDir, "vendor", "lib"), "ignored")

	// Skills that should be discovered
	writeSkillMD(t, filepath.Join(repoDir, "real-skill"), "---\nname: real\n---\n# Real")

	skills, stats, err := DiscoverSourceSkillsWithStats(src)
	if err != nil {
		t.Fatal(err)
	}

	// Should discover only the non-ignored skill
	if len(skills) != 1 {
		t.Fatalf("expected 1 skill, got %d", len(skills))
	}
	if skills[0].FlatName != "_team__real-skill" {
		t.Errorf("expected '_team__real-skill', got %q", skills[0].FlatName)
	}

	// Stats should be active
	if stats == nil {
		t.Fatal("expected non-nil stats")
	}
	if !stats.Active() {
		t.Error("expected stats.Active() = true")
	}

	// Repo file should be recorded
	if len(stats.RepoFiles) != 1 {
		t.Fatalf("expected 1 repo file, got %d", len(stats.RepoFiles))
	}
	if !strings.HasSuffix(stats.RepoFiles[0], "_team/.skillignore") &&
		!strings.HasSuffix(stats.RepoFiles[0], "_team\\.skillignore") {
		t.Errorf("expected repo file ending with _team/.skillignore, got %q", stats.RepoFiles[0])
	}

	// Patterns should be collected
	if stats.PatternCount() != 2 {
		t.Errorf("expected 2 patterns, got %d: %v", stats.PatternCount(), stats.Patterns)
	}

	// Ignored skills should be recorded
	// Note: vendor/lib may be skipped by CanSkipDir (directory-level skip),
	// so only skills that pass through the file-level Match check are recorded.
	// test-alpha and test-beta should be recorded as ignored.
	if stats.IgnoredCount() < 2 {
		t.Errorf("expected at least 2 ignored skills, got %d: %v", stats.IgnoredCount(), stats.IgnoredSkills)
	}

	// Verify ignored paths contain the expected skills
	ignoredSet := map[string]bool{}
	for _, p := range stats.IgnoredSkills {
		ignoredSet[p] = true
	}
	if !ignoredSet["_team/test-alpha"] {
		t.Error("expected _team/test-alpha in ignored skills")
	}
	if !ignoredSet["_team/test-beta"] {
		t.Error("expected _team/test-beta in ignored skills")
	}
}

func TestDiscoverSourceSkillsWithStats_NoSkillignore(t *testing.T) {
	src := t.TempDir()

	writeSkillMD(t, filepath.Join(src, "skill-a"), "---\nname: a\n---\n# A")
	writeSkillMD(t, filepath.Join(src, "skill-b"), "---\nname: b\n---\n# B")

	skills, stats, err := DiscoverSourceSkillsWithStats(src)
	if err != nil {
		t.Fatal(err)
	}

	if len(skills) != 2 {
		t.Fatalf("expected 2 skills, got %d", len(skills))
	}

	if stats == nil {
		t.Fatal("expected non-nil stats")
	}
	if stats.Active() {
		t.Error("expected stats.Active() = false when no .skillignore exists")
	}
	if stats.RootFile != "" {
		t.Errorf("expected empty RootFile, got %q", stats.RootFile)
	}
	if len(stats.RepoFiles) != 0 {
		t.Errorf("expected 0 repo files, got %d", len(stats.RepoFiles))
	}
	if stats.PatternCount() != 0 {
		t.Errorf("expected 0 patterns, got %d", stats.PatternCount())
	}
	if stats.IgnoredCount() != 0 {
		t.Errorf("expected 0 ignored skills, got %d", stats.IgnoredCount())
	}
}

func TestDiscoverSourceSkillsWithStats_RootLevelSkillignore(t *testing.T) {
	src := t.TempDir()

	os.WriteFile(filepath.Join(src, ".skillignore"), []byte("draft-*\nhidden\n"), 0644)

	writeSkillMD(t, filepath.Join(src, "draft-feature"), "---\nname: draft\n---\n")
	writeSkillMD(t, filepath.Join(src, "hidden"), "---\nname: hidden\n---\n")
	writeSkillMD(t, filepath.Join(src, "visible"), "---\nname: visible\n---\n")

	skills, stats, err := DiscoverSourceSkillsWithStats(src)
	if err != nil {
		t.Fatal(err)
	}

	// Only visible should be discovered
	if len(skills) != 1 {
		t.Fatalf("expected 1 skill, got %d", len(skills))
	}
	if skills[0].FlatName != "visible" {
		t.Errorf("expected 'visible', got %q", skills[0].FlatName)
	}

	// Stats checks
	if stats == nil {
		t.Fatal("expected non-nil stats")
	}
	if !stats.Active() {
		t.Error("expected stats.Active() = true")
	}
	if stats.RootFile == "" {
		t.Error("expected RootFile to be set")
	}
	if !strings.HasSuffix(stats.RootFile, ".skillignore") {
		t.Errorf("expected RootFile ending with .skillignore, got %q", stats.RootFile)
	}
	if stats.PatternCount() != 2 {
		t.Errorf("expected 2 patterns (draft-*, hidden), got %d: %v", stats.PatternCount(), stats.Patterns)
	}

	// Both draft-feature and hidden should be in ignored list
	if stats.IgnoredCount() != 2 {
		t.Errorf("expected 2 ignored skills, got %d: %v", stats.IgnoredCount(), stats.IgnoredSkills)
	}

	ignoredSet := map[string]bool{}
	for _, p := range stats.IgnoredSkills {
		ignoredSet[p] = true
	}
	if !ignoredSet["draft-feature"] {
		t.Error("expected draft-feature in ignored skills")
	}
	if !ignoredSet["hidden"] {
		t.Error("expected hidden in ignored skills")
	}
}
