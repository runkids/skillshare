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

	expectedMembers := []string{"amp", "codex", "droid", "gemini-cli", "github-copilot", "opencode", "replit"}
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
