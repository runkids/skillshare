package sync

import (
	"testing"

	"skillshare/internal/config"
	"skillshare/internal/resource"
)

func TestSelectTargetSkills_AppliesDisabledFiltersAndTargets(t *testing.T) {
	skills := []DiscoveredSkill{
		{FlatName: "keep"},
		{FlatName: "off", Disabled: true},
		{FlatName: "dropped"},
		{FlatName: "cursor-only", Targets: []string{"cursor"}},
	}
	got, err := SelectTargetSkills(skills, "claude", config.ResourceTargetConfig{Exclude: []string{"dropped"}})
	if err != nil {
		t.Fatal(err)
	}
	assertFlatNames(t, got, []string{"keep"})
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
