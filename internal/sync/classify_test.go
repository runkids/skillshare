package sync

import (
	"errors"
	"os"
	"path/filepath"
	"testing"

	"skillshare/internal/config"
	"skillshare/internal/profile"
)

func TestClassifySkillForTarget(t *testing.T) {
	tests := []struct {
		name       string
		flatName   string
		targets    []string
		targetName string
		include    []string
		exclude    []string
		wantStatus string
		wantReason string
	}{
		{name: "synced_no_filters", flatName: "my-skill", targets: nil, targetName: "claude", wantStatus: "synced"},
		{name: "excluded_by_pattern", flatName: "legacy-tool", targets: nil, targetName: "claude", exclude: []string{"legacy*"}, wantStatus: "excluded", wantReason: "legacy*"},
		{name: "not_included", flatName: "backend-api", targets: nil, targetName: "claude", include: []string{"frontend*"}, wantStatus: "not_included"},
		{name: "included_by_pattern", flatName: "frontend-design", targets: nil, targetName: "claude", include: []string{"frontend*"}, wantStatus: "synced"},
		{name: "skill_target_mismatch", flatName: "my-skill", targets: []string{"cursor"}, targetName: "claude", wantStatus: "skill_target_mismatch", wantReason: "cursor"},
		{name: "skill_target_match", flatName: "my-skill", targets: []string{"claude", "cursor"}, targetName: "claude", wantStatus: "synced"},
		{name: "exclude_wins_over_include", flatName: "frontend-legacy", targets: nil, targetName: "claude", include: []string{"frontend*"}, exclude: []string{"*legacy*"}, wantStatus: "excluded", wantReason: "*legacy*"},
		{name: "include_miss_beats_exclude_match", flatName: "legacy-tool", targets: nil, targetName: "claude", include: []string{"frontend*"}, exclude: []string{"legacy*"}, wantStatus: "not_included"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			status, reason := ClassifySkillForTarget(tt.flatName, tt.targets, tt.targetName, tt.include, tt.exclude)
			if status != tt.wantStatus {
				t.Errorf("status = %q, want %q", status, tt.wantStatus)
			}
			if tt.wantReason != "" && reason != tt.wantReason {
				t.Errorf("reason = %q, want %q", reason, tt.wantReason)
			}
		})
	}
}

func TestClassifySkillsForTarget_ProfileReasonPrecedence(t *testing.T) {
	skills := []DiscoveredSkill{{FlatName: "keep", RelPath: "keep"}, {FlatName: "other", RelPath: "other"}}
	selector := profile.Resolve(profile.Catalog{Definitions: map[string]profile.Definition{
		"frontend": {Name: "frontend", Usable: true, Skills: &profile.SkillSelection{Include: []string{"keep"}, IncludePresent: true}},
	}}, profile.State{Version: 1, Active: "frontend"}, "fixture.yaml", nil)
	for _, sc := range []config.ResourceTargetConfig{
		{Profile: selector},
		{Profile: selector, Include: []string{"keep"}},
		{Profile: selector, Exclude: []string{"other"}},
	} {
		rows, err := ClassifySkillsForTarget(skills, "claude", sc)
		if err != nil || rows["other"] != (SkillTargetClassification{Status: "not_in_profile", Reason: "frontend"}) {
			t.Fatal(rows, err)
		}
		selected, err := SelectTargetSkills(skills, "claude", sc)
		if err != nil || len(selected) != 1 || selected[0].FlatName != "keep" {
			t.Fatal("diagnostic row changed source selection", selected, err)
		}
	}
}

func TestProfileDeselected(t *testing.T) {
	skills := []DiscoveredSkill{
		{FlatName: "keep", RelPath: "keep"}, {FlatName: "other", RelPath: "other"},
		{FlatName: "target-only", RelPath: "target-only"}, {FlatName: "ignored", RelPath: "ignored", Disabled: true},
	}
	catalog := profile.Catalog{Definitions: map[string]profile.Definition{
		"frontend": {Name: "frontend", Usable: true, Skills: &profile.SkillSelection{Include: []string{"keep", "target-only"}, IncludePresent: true}},
	}}
	sc := config.ResourceTargetConfig{Exclude: []string{"target-only"}, Profile: profile.Resolve(catalog, profile.State{Version: 1, Active: "frontend"}, "fixture.yaml", nil)}
	got, err := ProfileDeselected("claude", sc, skills)
	if err != nil || len(got) != 1 || got["other"] != "frontend" {
		t.Fatal(got, err)
	}
	sc.Profile = profile.Selector{}
	if got, err := ProfileDeselected("claude", sc, skills); err != nil || len(got) != 0 {
		t.Fatal("no-profile behavior changed", got, err)
	}
	sc.Profile = profile.Resolve(catalog, profile.State{Version: 1, Active: "missing"}, "fixture.yaml", nil)
	if _, err := ProfileDeselected("claude", sc, nil); !errors.Is(err, profile.ErrProfileUnavailable) {
		t.Fatal("unavailable profile accepted", err)
	}
}

func TestProfileDeselectedUsesTargetNaming(t *testing.T) {
	root := t.TempDir()
	source := filepath.Join(root, "_repo", "skills", "review")
	if err := os.MkdirAll(source, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(source, "SKILL.md"), []byte("---\nname: review\n---\n# Fixture"), 0600); err != nil {
		t.Fatal(err)
	}
	skills := []DiscoveredSkill{{SourcePath: source, RelPath: "_repo/skills/review", FlatName: "_repo__skills__review", IsInRepo: true, RepoRelPath: "_repo"}}
	selector := profile.Resolve(profile.Catalog{Definitions: map[string]profile.Definition{
		"frontend": {Name: "frontend", Usable: true, Skills: &profile.SkillSelection{IncludePresent: true}},
	}}, profile.State{Version: 1, Active: "frontend"}, "fixture.yaml", nil)
	for naming, want := range map[string]string{"flat": "_repo__skills__review", "standard": "review", "prefixed": "repo-review"} {
		got, err := ProfileDeselected("claude", config.ResourceTargetConfig{Profile: selector, TargetNaming: naming}, skills)
		if err != nil || len(got) != 1 || got[want] != "frontend" {
			t.Fatal(naming, got, err)
		}
	}
}
