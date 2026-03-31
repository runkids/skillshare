package sync

import (
	"os"
	"path/filepath"
	"testing"

	"skillshare/internal/config"
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

	if resolution.Naming != "flat" {
		t.Fatalf("Naming = %q, want flat", resolution.Naming)
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

	if resolution.Naming != "standard" {
		t.Fatalf("Naming = %q, want standard", resolution.Naming)
	}
	if len(resolution.Skills) != 1 {
		t.Fatalf("len(Skills) = %d, want 1", len(resolution.Skills))
	}
	if resolution.Skills[0].TargetName != "dev" {
		t.Fatalf("TargetName = %q, want dev", resolution.Skills[0].TargetName)
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

func TestResolveTargetSkillsForTarget_StandardNamingSkipsCollisions(t *testing.T) {
	dir := t.TempDir()
	skills := []DiscoveredSkill{
		createTempSkill(t, dir, "frontend/dev", "dev"),
		createTempSkill(t, dir, "backend/dev", "dev"),
		createTempSkill(t, dir, "alpha", "alpha"),
	}

	resolution, err := ResolveTargetSkillsForTarget("claude", config.ResourceTargetConfig{TargetNaming: "standard"}, skills)
	if err != nil {
		t.Fatalf("ResolveTargetSkillsForTarget() error = %v", err)
	}

	if len(resolution.Collisions) != 1 {
		t.Fatalf("Collisions = %v, want 1 collision", resolution.Collisions)
	}
	if resolution.Collisions[0].Name != "dev" {
		t.Fatalf("collision name = %q, want dev", resolution.Collisions[0].Name)
	}
	if len(resolution.Skills) != 1 {
		t.Fatalf("len(Skills) = %d, want 1", len(resolution.Skills))
	}
	if resolution.Skills[0].TargetName != "alpha" {
		t.Fatalf("TargetName = %q, want alpha", resolution.Skills[0].TargetName)
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
