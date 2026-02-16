package sync

import (
	"testing"
)

func TestFilterSkillsByTarget_NilPassesThrough(t *testing.T) {
	skills := []DiscoveredSkill{
		{FlatName: "no-targets", Targets: nil},
	}
	result := FilterSkillsByTarget(skills, "claude")
	if len(result) != 1 {
		t.Errorf("nil Targets should pass through, got %d", len(result))
	}
}

func TestFilterSkillsByTarget_ExactMatch(t *testing.T) {
	skills := []DiscoveredSkill{
		{FlatName: "claude-only", Targets: []string{"claude"}},
		{FlatName: "cursor-only", Targets: []string{"cursor"}},
	}
	result := FilterSkillsByTarget(skills, "claude")
	if len(result) != 1 || result[0].FlatName != "claude-only" {
		t.Errorf("expected only claude-only, got %v", result)
	}
}

func TestFilterSkillsByTarget_CrossModeMatch(t *testing.T) {
	skills := []DiscoveredSkill{
		{FlatName: "cross-mode", Targets: []string{"claude"}},
	}
	// "claude-code" is an alias for claude
	result := FilterSkillsByTarget(skills, "claude-code")
	if len(result) != 1 {
		t.Errorf("cross-mode match should work, got %d results", len(result))
	}
}

func TestFilterSkillsByTarget_NoMatch(t *testing.T) {
	skills := []DiscoveredSkill{
		{FlatName: "claude-only", Targets: []string{"claude"}},
	}
	result := FilterSkillsByTarget(skills, "cursor")
	if len(result) != 0 {
		t.Errorf("expected 0 results, got %d", len(result))
	}
}

func TestFilterSkillsByTarget_MultipleTargets(t *testing.T) {
	skills := []DiscoveredSkill{
		{FlatName: "multi", Targets: []string{"claude", "cursor"}},
	}
	result := FilterSkillsByTarget(skills, "cursor")
	if len(result) != 1 {
		t.Errorf("skill with multiple targets should match, got %d", len(result))
	}
}

func TestFilterSkillsByTarget_MixedNilAndSpecific(t *testing.T) {
	skills := []DiscoveredSkill{
		{FlatName: "all-targets", Targets: nil},
		{FlatName: "claude-only", Targets: []string{"claude"}},
		{FlatName: "cursor-only", Targets: []string{"cursor"}},
	}
	result := FilterSkillsByTarget(skills, "claude")
	if len(result) != 2 {
		t.Errorf("expected 2 results (nil + claude-only), got %d", len(result))
	}
}
