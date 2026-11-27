package sync

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"skillshare/internal/config"
	"skillshare/internal/utils"
)

// createTempSkill creates a skill directory with a SKILL.md containing the given name.
func createTempSkill(t *testing.T, dir, relPath, skillName string) DiscoveredSkill {
	t.Helper()
	skillDir := filepath.Join(dir, filepath.FromSlash(relPath))
	if err := os.MkdirAll(skillDir, 0755); err != nil {
		t.Fatalf("mkdir %s: %v", skillDir, err)
	}
	content := "---\nname: " + skillName + "\n---\n# " + skillName
	if err := os.WriteFile(filepath.Join(skillDir, "SKILL.md"), []byte(content), 0644); err != nil {
		t.Fatalf("write SKILL.md: %v", err)
	}
	return DiscoveredSkill{
		SourcePath: skillDir,
		RelPath:    filepath.ToSlash(relPath),
		FlatName:   filepath.ToSlash(relPath),
	}
}

func TestResolveTargetSkillsForTarget_FlatNamingUsesFlatNames(t *testing.T) {
	dir := t.TempDir()
	skills := []DiscoveredSkill{
		createTempSkill(t, dir, "frontend/dev", "dev"),
		createTempSkill(t, dir, "alpha", "alpha"),
	}

	resolution, err := ResolveTargetSkillsForTarget("claude", config.ResourceTargetConfig{}, skills)
	if err != nil {
		t.Fatalf("ResolveTargetSkillsForTarget() error = %v", err)
	}

	if resolution.Skills[0].Naming != "flat" {
		t.Fatalf("Naming = %q, want flat", resolution.Skills[0].Naming)
	}
	if len(resolution.Warnings) != 0 {
		t.Fatalf("Warnings = %v, want none", resolution.Warnings)
	}
	if len(resolution.Skills) != 2 {
		t.Fatalf("len(Skills) = %d, want 2", len(resolution.Skills))
	}
	if resolution.Skills[0].TargetName != "frontend/dev" {
		t.Fatalf("first TargetName = %q, want %q", resolution.Skills[0].TargetName, "frontend/dev")
	}
}

func TestResolveTargetSkillsForTarget_StandardNamingUsesSkillName(t *testing.T) {
	dir := t.TempDir()
	skills := []DiscoveredSkill{
		createTempSkill(t, dir, "frontend/dev", "dev"),
	}

	resolution, err := ResolveTargetSkillsForTarget("claude", config.ResourceTargetConfig{TargetNaming: "standard"}, skills)
	if err != nil {
		t.Fatalf("ResolveTargetSkillsForTarget() error = %v", err)
	}

	if len(resolution.Skills) != 1 {
		t.Fatalf("len(Skills) = %d, want 1", len(resolution.Skills))
	}
	if resolution.Skills[0].Naming != "standard" {
		t.Fatalf("Naming = %q, want standard", resolution.Skills[0].Naming)
	}
	if resolution.Skills[0].TargetName != "dev" {
		t.Fatalf("TargetName = %q, want dev", resolution.Skills[0].TargetName)
	}
}

func TestResolveTargetSkillsForTarget_ReportsUnmatchedInclude(t *testing.T) {
	dir := t.TempDir()
	skill := createTempSkill(t, dir, "frontend/dev", "dev")
	skill.FlatName = utils.PathToFlatName(skill.RelPath)
	skills := []DiscoveredSkill{skill}

	const want = `include filter "dev" matches no skill in the source, so it adds nothing to this target ` +
		`(filters use the source path name; did you mean "frontend__dev"?)`

	for _, naming := range []string{"flat", "standard"} {
		resolution, err := ResolveTargetSkillsForTarget("claude", config.ResourceTargetConfig{
			TargetNaming: naming,
			Include:      []string{"dev"},
		}, skills)
		if err != nil {
			t.Fatalf("ResolveTargetSkillsForTarget(%s) error = %v", naming, err)
		}
		if len(resolution.Skills) != 0 {
			t.Fatalf("%s: len(Skills) = %d, want 0", naming, len(resolution.Skills))
		}
		warnings := resolution.UnmatchedIncludeWarnings()
		if len(warnings) != 1 || warnings[0] != want {
			t.Fatalf("%s: warnings = %v, want [%q]", naming, warnings, want)
		}
	}

	resolution, err := ResolveTargetSkillsForTarget("claude", config.ResourceTargetConfig{
		TargetNaming: "standard",
		Include:      []string{"frontend__dev"},
	}, skills)
	if err != nil {
		t.Fatalf("ResolveTargetSkillsForTarget() error = %v", err)
	}
	if len(resolution.Skills) != 1 || resolution.Skills[0].TargetName != "dev" {
		t.Fatalf("Skills = %+v, want one entry named dev", resolution.Skills)
	}
	if got := resolution.UnmatchedIncludeWarnings(); len(got) != 0 {
		t.Fatalf("warnings = %v, want none for a pattern that selects the skill", got)
	}
}

func TestResolveTargetSkillsForTarget_StandardNamingSkipsInvalidSkill(t *testing.T) {
	dir := t.TempDir()
	skills := []DiscoveredSkill{
		createTempSkill(t, dir, "frontend/dev", "wrong-name"),
		createTempSkill(t, dir, "alpha", "alpha"),
	}

	resolution, err := ResolveTargetSkillsForTarget("claude", config.ResourceTargetConfig{TargetNaming: "standard"}, skills)
	if err != nil {
		t.Fatalf("ResolveTargetSkillsForTarget() error = %v", err)
	}

	if len(resolution.Skills) != 1 {
		t.Fatalf("len(Skills) = %d, want 1", len(resolution.Skills))
	}
	if resolution.Skills[0].TargetName != "alpha" {
		t.Fatalf("TargetName = %q, want alpha", resolution.Skills[0].TargetName)
	}
	if len(resolution.Warnings) != 1 {
		t.Fatalf("Warnings = %v, want 1 warning", resolution.Warnings)
	}
}

func TestResolveTargetSkillsForTarget_StandardNamingKeepsCollisionsUnderFlatNames(t *testing.T) {
	dir := t.TempDir()
	frontend := createTempSkill(t, dir, "frontend/dev", "dev")
	backend := createTempSkill(t, dir, "backend/dev", "dev")
	frontend.FlatName, backend.FlatName = "frontend__dev", "backend__dev"
	skills := []DiscoveredSkill{frontend, backend, createTempSkill(t, dir, "alpha", "alpha")}

	resolution, err := ResolveTargetSkillsForTarget("claude", config.ResourceTargetConfig{TargetNaming: "standard"}, skills)
	if err != nil {
		t.Fatalf("ResolveTargetSkillsForTarget() error = %v", err)
	}

	if len(resolution.Collisions) != 1 || resolution.Collisions[0].Name != "dev" {
		t.Fatalf("Collisions = %v, want one under dev", resolution.Collisions)
	}
	got := map[string]string{}
	for _, s := range resolution.Skills {
		got[s.TargetName] = s.Naming
	}
	want := map[string]string{"frontend__dev": "flat", "backend__dev": "flat", "alpha": "standard"}
	if len(got) != len(want) {
		t.Fatalf("skills = %v, want %v", got, want)
	}
	for name, naming := range want {
		if got[name] != naming {
			t.Fatalf("skills = %v, want %v", got, want)
		}
	}
	if resolution.SkippedCount != 0 {
		t.Fatalf("SkippedCount = %d, want 0", resolution.SkippedCount)
	}
}

func TestCheckNameCollisionsForTargets_NoCollisions(t *testing.T) {
	dir := t.TempDir()
	skills := []DiscoveredSkill{
		createTempSkill(t, dir, "skill-a", "alpha"),
		createTempSkill(t, dir, "skill-b", "beta"),
	}

	targets := map[string]config.TargetConfig{
		"claude": {Path: "/tmp/claude", Mode: "merge"},
	}

	global, perTarget := CheckNameCollisionsForTargets(skills, targets)
	if len(global) != 0 {
		t.Errorf("expected no global collisions, got %d", len(global))
	}
	if len(perTarget) != 0 {
		t.Errorf("expected no per-target collisions, got %d", len(perTarget))
	}
}

func TestCheckNameCollisionsForTargets_GlobalCollisionIsolatedByFilter(t *testing.T) {
	dir := t.TempDir()
	skills := []DiscoveredSkill{
		createTempSkill(t, dir, "codex-plan", "planner"),
		createTempSkill(t, dir, "gemini-plan", "planner"),
	}

	targets := map[string]config.TargetConfig{
		"codex-target": {
			Skills: &config.ResourceTargetConfig{
				Path:         "/tmp/codex",
				Mode:         "merge",
				TargetNaming: "standard",
				Include:      []string{"codex-*"},
			},
		},
		"gemini-target": {
			Skills: &config.ResourceTargetConfig{
				Path:         "/tmp/gemini",
				Mode:         "merge",
				TargetNaming: "standard",
				Include:      []string{"gemini-*"},
			},
		},
	}

	global, perTarget := CheckNameCollisionsForTargets(skills, targets)
	if len(global) != 1 {
		t.Fatalf("expected 1 global collision, got %d", len(global))
	}
	if global[0].Name != "planner" {
		t.Errorf("expected collision name 'planner', got '%s'", global[0].Name)
	}
	if len(perTarget) != 0 {
		t.Errorf("expected no per-target collisions (filters isolate), got %d", len(perTarget))
	}
}

func TestCheckNameCollisionsForTargets_UnresolvedPerTargetCollision(t *testing.T) {
	dir := t.TempDir()
	skills := []DiscoveredSkill{
		createTempSkill(t, dir, "frontend/dev", "dev"),
		createTempSkill(t, dir, "backend/dev", "dev"),
	}

	targets := map[string]config.TargetConfig{
		"claude": {
			Skills: &config.ResourceTargetConfig{
				Path:         "/tmp/claude",
				Mode:         "merge",
				TargetNaming: "standard",
			},
		},
	}

	global, perTarget := CheckNameCollisionsForTargets(skills, targets)
	if len(global) != 1 {
		t.Fatalf("expected 1 global collision, got %d", len(global))
	}
	if len(perTarget) != 1 {
		t.Fatalf("expected 1 per-target collision, got %d", len(perTarget))
	}
	if perTarget[0].TargetName != "claude" {
		t.Errorf("expected target 'claude', got '%s'", perTarget[0].TargetName)
	}
}

func TestCheckNameCollisionsForTargets_SymlinkModeSkipped(t *testing.T) {
	dir := t.TempDir()
	skills := []DiscoveredSkill{
		createTempSkill(t, dir, "frontend/dev", "dev"),
		createTempSkill(t, dir, "backend/dev", "dev"),
	}

	targets := map[string]config.TargetConfig{
		"symlink-target": {
			Skills: &config.ResourceTargetConfig{
				Path:         "/tmp/sym",
				Mode:         "symlink",
				TargetNaming: "standard",
				Include:      []string{"*"},
			},
		},
	}

	global, perTarget := CheckNameCollisionsForTargets(skills, targets)
	if len(global) != 1 {
		t.Fatalf("expected 1 global collision, got %d", len(global))
	}
	if len(perTarget) != 0 {
		t.Errorf("expected no per-target collisions for symlink mode, got %d", len(perTarget))
	}
}

// Skills that share a name reach the target under their full path names, but
// a tool that loads skills by name: still sees one name, so each is reported.
// A symlink-mode target, set or inherited, holds the source as it is.
func TestSharedNameWarnings_NamesEachTargetAndSkill(t *testing.T) {
	dir := t.TempDir()
	skills := []DiscoveredSkill{
		createTempSkill(t, dir, "frontend/dev", "dev"),
		createTempSkill(t, dir, "backend/dev", "dev"),
	}
	standard := func(path, mode string) config.TargetConfig {
		return config.TargetConfig{Skills: &config.ResourceTargetConfig{Path: path, Mode: mode, TargetNaming: "standard"}}
	}
	targets := map[string]config.TargetConfig{
		"cursor": standard("/tmp/cursor", "merge"),
		"claude": standard("/tmp/claude", "copy"),
		"codex":  standard("/tmp/codex", ""), // inherits symlink
	}

	got := SharedNameWarnings(skills, targets, "symlink")
	want := []string{
		"claude: backend/dev and frontend/dev share the name dev; all but a top-level one sync under their full path names, and a tool that loads skills by name may load only one",
		"cursor: backend/dev and frontend/dev share the name dev; all but a top-level one sync under their full path names, and a tool that loads skills by name may load only one",
	}
	if strings.Join(got, "\n") != strings.Join(want, "\n") {
		t.Fatalf("warnings =\n%s\nwant\n%s", strings.Join(got, "\n"), strings.Join(want, "\n"))
	}
}

// Under prefixed naming two skills can want one entry while their name: differ;
// a tool then tells them apart, so the warning does not say it loads one.
func TestSharedNameWarnings_PrefixedEntryClashWithDifferentNames(t *testing.T) {
	dir := t.TempDir()
	tracked := createTempSkill(t, dir, "_repo/ui", "ui")
	tracked.IsInRepo, tracked.RepoRelPath = true, "_repo"
	skills := []DiscoveredSkill{tracked, createTempSkill(t, dir, "repo-ui", "repo-ui")}
	targets := map[string]config.TargetConfig{"claude": {Skills: &config.ResourceTargetConfig{Path: "/tmp/claude", Mode: "copy", TargetNaming: "prefixed"}}}

	got := SharedNameWarnings(skills, targets, "")
	want := "claude: _repo/ui and repo-ui would both sync as repo-ui; all but a top-level one sync under their full path names instead"
	if len(got) != 1 || got[0] != want {
		t.Fatalf("warnings = %q, want %q", got, want)
	}
}
