package sync

import (
	"os"
	"path/filepath"
	"slices"
	"testing"

	"skillshare/internal/config"
)

func leakTargets(t *testing.T, scanner string) map[string]config.TargetConfig {
	dir := t.TempDir()
	return map[string]config.TargetConfig{
		scanner:  {Skills: &config.ResourceTargetConfig{Path: filepath.Join(dir, scanner)}},
		"claude": {Skills: &config.ResourceTargetConfig{Path: filepath.Join(dir, "claude")}},
	}
}

func TestLeakedSkills_ListsSkillsFilteredFromScanner(t *testing.T) {
	discovered := []DiscoveredSkill{
		{FlatName: "shared"},
		{FlatName: "claude-only", Targets: []string{"claude"}},
	}
	leaked, known := LeakedSkills("opencode", "claude", leakTargets(t, "opencode"), "merge", discovered)
	if !known || !slices.Equal(leaked, []string{"claude-only"}) {
		t.Errorf("LeakedSkills = %v, %v; want [claude-only], true", leaked, known)
	}
}

func TestLeakedSkills_AccountTarget(t *testing.T) {
	discovered := []DiscoveredSkill{
		{FlatName: "shared"},
		{FlatName: "claude-only", Targets: []string{"claude"}},
	}
	dir := t.TempDir()
	targets := map[string]config.TargetConfig{
		"opencode-work": {
			Agent:     "opencode",
			ConfigDir: filepath.Join(dir, "opencode-work"),
			Skills:    &config.ResourceTargetConfig{Path: filepath.Join(dir, "opencode-work", "skills")},
		},
		"claude": {Skills: &config.ResourceTargetConfig{Path: filepath.Join(dir, "claude")}},
	}
	leaked, known := LeakedSkills("opencode-work", "claude", targets, "merge", discovered)
	if !known || !slices.Equal(leaked, []string{"claude-only"}) {
		t.Errorf("LeakedSkills = %v, %v; want [claude-only], true", leaked, known)
	}
}

func TestLeakedSkills_NoneWhenScannerGetsEverySkill(t *testing.T) {
	discovered := []DiscoveredSkill{{FlatName: "shared"}}
	leaked, known := LeakedSkills("opencode", "claude", leakTargets(t, "opencode"), "merge", discovered)
	if !known || len(leaked) != 0 {
		t.Errorf("LeakedSkills = %v, %v; want none, true", leaked, known)
	}
}

func TestLeakedSkills_CountsLocalSkillsInWriterFolder(t *testing.T) {
	// Merge mode keeps skills the user put in claude's folder; opencode loads
	// them unless its own folder has a skill of the same name. Only a folder
	// with SKILL.md is a skill.
	targets := leakTargets(t, "opencode")
	claude, opencode := targets["claude"].Skills.Path, targets["opencode"].Skills.Path
	for _, skill := range []string{
		filepath.Join(claude, "my-local"),
		filepath.Join(claude, "shared"),
		filepath.Join(claude, "in-both"),
		filepath.Join(opencode, "in-both"),
		filepath.Join(claude, "not-a-skill-there"),
	} {
		if err := os.MkdirAll(skill, 0755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(skill, "SKILL.md"), []byte("---\nname: x\n---\n"), 0644); err != nil {
			t.Fatal(err)
		}
	}
	for _, dir := range []string{filepath.Join(claude, "cache"), filepath.Join(opencode, "not-a-skill-there")} {
		if err := os.MkdirAll(dir, 0755); err != nil {
			t.Fatal(err)
		}
	}
	leaked, known := LeakedSkills("opencode", "claude", targets, "merge", []DiscoveredSkill{{FlatName: "shared"}})
	if !known || !slices.Equal(leaked, []string{"my-local", "not-a-skill-there"}) {
		t.Errorf("LeakedSkills = %v, %v; want [my-local not-a-skill-there], true", leaked, known)
	}
}

func TestLeakedSkills_CountsSkillsStandardNamingSkips(t *testing.T) {
	// Two skills named dev collide in opencode's standard folder, so sync skips
	// both there; claude's filter keeps one, which opencode then loads from it.
	src := t.TempDir()
	discovered := []DiscoveredSkill{
		createTempSkill(t, src, "frontend/dev", "dev"),
		createTempSkill(t, src, "backend/dev", "dev"),
	}
	targets := leakTargets(t, "opencode")
	targets["opencode"].Skills.TargetNaming = "standard"
	targets["claude"].Skills.TargetNaming = "standard"
	targets["claude"].Skills.Include = []string{"frontend/dev"}

	leaked, known := LeakedSkills("opencode", "claude", targets, "merge", discovered)
	if !known || !slices.Equal(leaked, []string{"frontend/dev"}) {
		t.Errorf("LeakedSkills = %v, %v; want [frontend/dev], true", leaked, known)
	}
}

func TestLeakedSkills_UnknownForSymlinkWriter(t *testing.T) {
	// A symlink-mode folder exposes .skillignore'd skills that discovery omits.
	targets := leakTargets(t, "opencode")
	targets["claude"].Skills.Mode = "symlink"
	if _, known := LeakedSkills("opencode", "claude", targets, "merge", []DiscoveredSkill{{FlatName: "shared"}}); known {
		t.Error("want known=false for a symlink-mode writer")
	}
}

func TestLeakedSkills_UnknownForOtherRuntimes(t *testing.T) {
	discovered := []DiscoveredSkill{{FlatName: "shared"}}
	if _, known := LeakedSkills("codex", "claude", leakTargets(t, "codex"), "merge", discovered); known {
		t.Error("codex's rule for same-named skills is not known; want known=false")
	}
}
