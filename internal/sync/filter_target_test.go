package sync

import (
	"testing"

	"skillshare/internal/resource"
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

func TestFilterAgentsByTarget_NilPassesThrough(t *testing.T) {
	agents := []resource.DiscoveredResource{{FlatName: "no-targets.md", Targets: nil}}
	if got := FilterAgentsByTarget(agents, "claude"); len(got) != 1 {
		t.Errorf("nil Targets should pass through, got %d", len(got))
	}
}

func TestFilterAgentsByTarget_RestrictsToDeclaredTarget(t *testing.T) {
	agents := []resource.DiscoveredResource{
		{FlatName: "all.md", Targets: nil},
		{FlatName: "claude-only.md", Targets: []string{"claude"}},
		{FlatName: "opencode-only.md", Targets: []string{"opencode"}},
	}
	got := FilterAgentsByTarget(agents, "claude-code") // alias of claude
	if len(got) != 2 || got[0].FlatName != "all.md" || got[1].FlatName != "claude-only.md" {
		t.Errorf("expected [all.md claude-only.md], got %v", got)
	}
}
