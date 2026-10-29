package main

import (
	ssync "skillshare/internal/sync"
	"testing"
)

func TestListedRepoCountsIncludeDisabledRows(t *testing.T) {
	skills := []ssync.DiscoveredSkill{
		{RelPath: "_repo/one", RepoRelPath: "_repo"},
		{RelPath: "_repo/two", RepoRelPath: "_repo", Disabled: true},
		{RelPath: "_repo/skills/claude/fix", RepoRelPath: "_repo", LogicalID: "fix"},
		{RelPath: "_repo/skills/codex/fix", RepoRelPath: "_repo", LogicalID: "fix", Disabled: true},
	}
	counts := listedRepoCounts(skills)
	if counts["_repo"] != (ssync.SkillCounts{Skills: 3, Variants: 4}) {
		t.Fatalf("listed repo counts = %+v", counts)
	}
	if !skills[1].Disabled || !skills[3].Disabled {
		t.Fatal("list counting changed disabled state")
	}
	if enabled := ssync.CountSkills(skills); enabled != (ssync.SkillCounts{Skills: 2, Variants: 2}) {
		t.Fatalf("enabled counts changed: %+v", enabled)
	}
}
