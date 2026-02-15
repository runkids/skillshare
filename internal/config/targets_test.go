package config

import (
	"testing"
)

func TestGroupedProjectTargets_AgentsGrouped(t *testing.T) {
	grouped := GroupedProjectTargets()

	// Find the agents group entry
	var agentsGroup *GroupedProjectTarget
	for i, g := range grouped {
		if g.Name == "agents" {
			agentsGroup = &grouped[i]
			break
		}
	}

	if agentsGroup == nil {
		t.Fatal("expected 'agents' group in GroupedProjectTargets result")
	}

	if agentsGroup.Path != ".agents/skills" {
		t.Errorf("agents group path = %q, want %q", agentsGroup.Path, ".agents/skills")
	}

	if len(agentsGroup.Members) == 0 {
		t.Fatal("agents group should have members")
	}

	// Verify known members are present
	memberSet := make(map[string]bool)
	for _, m := range agentsGroup.Members {
		memberSet[m] = true
	}

	expectedMembers := []string{"amp", "codex", "replit"}
	for _, name := range expectedMembers {
		if !memberSet[name] {
			t.Errorf("expected %q in agents group members, got %v", name, agentsGroup.Members)
		}
	}

	// Canonical name should NOT be in members
	if memberSet["agents"] {
		t.Error("canonical name 'agents' should not appear in members list")
	}
}

func TestGroupedProjectTargets_SinglePathNotGrouped(t *testing.T) {
	grouped := GroupedProjectTargets()

	// cursor has a unique path (.cursor/skills), should not have members
	for _, g := range grouped {
		if g.Name == "cursor" {
			if len(g.Members) != 0 {
				t.Errorf("cursor should have no members, got %v", g.Members)
			}
			return
		}
	}

	t.Error("cursor not found in GroupedProjectTargets result")
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
