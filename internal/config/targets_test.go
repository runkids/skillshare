package config

import (
	"testing"
)

func TestGroupedProjectTargets_UniversalGrouped(t *testing.T) {
	grouped := GroupedProjectTargets()

	// Find the universal group entry
	var universalGroup *GroupedProjectTarget
	for i, g := range grouped {
		if g.Name == "universal" {
			universalGroup = &grouped[i]
			break
		}
	}

	if universalGroup == nil {
		t.Fatal("expected 'universal' group in GroupedProjectTargets result")
	}

	if universalGroup.Path != ".agents/skills" {
		t.Errorf("universal group path = %q, want %q", universalGroup.Path, ".agents/skills")
	}

	if len(universalGroup.Members) == 0 {
		t.Fatal("universal group should have members")
	}

	// Verify known members are present
	memberSet := make(map[string]bool)
	for _, m := range universalGroup.Members {
		memberSet[m] = true
	}

	expectedMembers := []string{"amp", "codex", "cursor", "dexto", "kimi", "replit"}
	for _, name := range expectedMembers {
		if !memberSet[name] {
			t.Errorf("expected %q in universal group members, got %v", name, universalGroup.Members)
		}
	}

	// Canonical name should NOT be in members
	if memberSet["universal"] {
		t.Error("canonical name 'universal' should not appear in members list")
	}
}

func TestGroupedProjectTargets_SinglePathNotGrouped(t *testing.T) {
	grouped := GroupedProjectTargets()

	// copilot has a unique path (.github/skills), should not have members
	for _, g := range grouped {
		if g.Name == "copilot" {
			if len(g.Members) != 0 {
				t.Errorf("copilot should have no members, got %v", g.Members)
			}
			return
		}
	}

	t.Error("copilot not found in GroupedProjectTargets result")
}

func TestGroupedProjectTargets_NoDuplicatePaths(t *testing.T) {
	grouped := GroupedProjectTargets()

	seen := make(map[string]bool)
	for _, g := range grouped {
		if seen[g.Path] {
			t.Errorf("duplicate path %q in GroupedProjectTargets", g.Path)
		}
		seen[g.Path] = true
	}
}

func TestGroupedProjectTargets_MembersAreSorted(t *testing.T) {
	grouped := GroupedProjectTargets()

	for _, g := range grouped {
		if len(g.Members) < 2 {
			continue
		}
		for i := 1; i < len(g.Members); i++ {
			if g.Members[i] < g.Members[i-1] {
				t.Errorf("members of %q not sorted: %v", g.Name, g.Members)
				break
			}
		}
	}
}

func TestLookupProjectTarget_Alias(t *testing.T) {
	// Canonical name should resolve
	tc, ok := LookupProjectTarget("claude")
	if !ok {
		t.Fatal("LookupProjectTarget should find canonical name 'claude'")
	}
	if tc.Path == "" {
		t.Error("expected non-empty path for claude")
	}

	// Alias should also resolve to the same target
	tcAlias, ok := LookupProjectTarget("claude-code")
	if !ok {
		t.Fatal("LookupProjectTarget should find alias 'claude-code'")
	}
	if tcAlias.Path != tc.Path {
		t.Errorf("alias path %q != canonical path %q", tcAlias.Path, tc.Path)
	}

	// Unknown name should not resolve
	_, ok = LookupProjectTarget("nonexistent-tool")
	if ok {
		t.Error("LookupProjectTarget should not find unknown name")
	}
}

// --- Agent target tests ---

func TestDefaultAgentTargets_V1TargetsHaveAgentPaths(t *testing.T) {
	agents := DefaultAgentTargets()

	v1Targets := []string{"claude", "cursor", "opencode", "augment"}
	for _, name := range v1Targets {
		tc, ok := agents[name]
		if !ok {
			t.Errorf("expected %q in DefaultAgentTargets", name)
			continue
		}
		if tc.Path == "" {
			t.Errorf("expected non-empty agent path for %q", name)
		}
	}
}

func TestDefaultAgentTargets_NonV1Excluded(t *testing.T) {
	agents := DefaultAgentTargets()

	// copilot, codex, etc. should NOT have agent paths in v1
	for _, name := range []string{"copilot", "codex", "windsurf"} {
		if _, ok := agents[name]; ok {
			t.Errorf("%q should not be in DefaultAgentTargets (not v1 agent target)", name)
		}
	}
}

func TestProjectAgentTargets_V1TargetsHaveAgentPaths(t *testing.T) {
	agents := ProjectAgentTargets()

	v1Targets := []string{"claude", "cursor", "opencode", "augment"}
	for _, name := range v1Targets {
		tc, ok := agents[name]
		if !ok {
			t.Errorf("expected %q in ProjectAgentTargets", name)
			continue
		}
		if tc.Path == "" {
			t.Errorf("expected non-empty agent project path for %q", name)
		}
	}
}

func TestProjectAgentTargets_NonV1Excluded(t *testing.T) {
	agents := ProjectAgentTargets()

	for _, name := range []string{"copilot", "codex", "windsurf"} {
		if _, ok := agents[name]; ok {
			t.Errorf("%q should not be in ProjectAgentTargets", name)
		}
	}
}

func TestProjectTargetDotDirs_IncludesAgentPaths(t *testing.T) {
	dirs := ProjectTargetDotDirs()

	// .claude should be included (from both skill and agent project paths)
	if !dirs[".claude"] {
		t.Error("expected .claude in ProjectTargetDotDirs")
	}

	// .cursor should be included
	if !dirs[".cursor"] {
		t.Error("expected .cursor in ProjectTargetDotDirs")
	}

	// .skillshare always included
	if !dirs[".skillshare"] {
		t.Error("expected .skillshare in ProjectTargetDotDirs")
	}
}

func TestDefaultTargets_ClaudePath(t *testing.T) {
	targets := DefaultTargets()
	tc, ok := targets["claude"]
	if !ok {
		t.Fatal("expected claude in DefaultTargets")
	}
	// Path should contain "skills" (not "agents")
	if tc.Path == "" {
		t.Error("expected non-empty global path for claude")
	}
}

func TestProjectTargets_ClaudePath(t *testing.T) {
	targets := ProjectTargets()
	tc, ok := targets["claude"]
	if !ok {
		t.Fatal("expected claude in ProjectTargets")
	}
	if tc.Path != ".claude/skills" {
		t.Errorf("claude project path = %q, want %q", tc.Path, ".claude/skills")
	}
}
