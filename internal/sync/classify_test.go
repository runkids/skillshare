package sync

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
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
	for _, filters := range []config.ResourceTargetConfig{
		{Exclude: []string{"other", "target-only"}},
		{Include: []string{"keep"}},
	} {
		filters.Profile = sc.Profile
		got, err := ProfileDeselected("claude", filters, skills)
		if err != nil || len(got) != 1 || got["other"] != "frontend" {
			t.Fatal("profile reason lost to overlapping target filter or target-only mislabeled", got, err)
		}
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

func TestProfileDeselectedRetainsDisambiguation(t *testing.T) {
	for _, naming := range []string{"standard", "prefixed"} {
		for _, filter := range []string{"include", "exclude"} {
			for _, layout := range []string{"collision", "variant", "variant-shadow"} {
				t.Run(naming+"/"+filter+"/"+layout, func(t *testing.T) {
					source := t.TempDir()
					selected, rejected := "a/review", "b/review"
					want := "review"
					if strings.HasPrefix(layout, "variant") {
						selected, rejected = "_repo/skills/claude/review", "_repo/skills/claude-code/review"
						if naming == "prefixed" {
							want = "repo-review"
						}
					}
					for _, rel := range []string{selected, rejected} {
						writeSkillMD(t, filepath.Join(source, rel), "---\nname: review\n---\n# Fixture")
					}
					if layout == "variant-shadow" {
						writeSkillMD(t, filepath.Join(source, want), "---\nname: "+want+"\n---\n# Ordinary")
					}
					physicalCount := 2
					if layout == "variant-shadow" {
						physicalCount++
					}
					skills, err := DiscoverSourceSkills(source)
					if err != nil || len(skills) != physicalCount {
						t.Fatalf("discovery: %+v, %v", skills, err)
					}
					if strings.HasPrefix(layout, "variant") {
						for _, skill := range skills {
							if skill.RelPath != want && (skill.LogicalID == "" || !skill.variantRouting) {
								t.Fatalf("fixture did not discover actual variants: %+v", skill)
							}
						}
					}
					sc := config.ResourceTargetConfig{TargetNaming: naming}
					if filter == "include" {
						sc.Include = []string{strings.ReplaceAll(selected, "/", "__")}
					} else {
						sc.Exclude = []string{strings.ReplaceAll(rejected, "/", "__")}
						if layout == "variant-shadow" {
							sc.Exclude = append(sc.Exclude, want)
						}
					}
					resolved, err := ResolveTargetSkillsForTarget("claude", sc, skills)
					if err != nil || len(resolved.Skills) != 1 || resolved.Skills[0].Skill.RelPath != selected || resolved.Skills[0].TargetName != want {
						t.Fatalf("target filter must select the physical fixture: %+v, %v", resolved, err)
					}
					catalog := profile.Catalog{Definitions: map[string]profile.Definition{
						"empty": {Name: "empty", Usable: true, Skills: &profile.SkillSelection{IncludePresent: true}},
						"keep":  {Name: "keep", Usable: true, Skills: &profile.SkillSelection{Include: []string{strings.ReplaceAll(selected, "/", "__")}, IncludePresent: true}},
					}}
					for _, active := range []string{"", "keep", "empty"} {
						sc.Profile = profile.Resolve(catalog, profile.State{Version: 1, Active: active}, "fixture.yaml", nil)
						got, err := ProfileDeselected("claude", sc, skills)
						if err != nil {
							t.Fatal(err)
						}
						if active == "empty" {
							if len(got) != 1 || got[want] != "empty" {
								t.Errorf("profile removal lost filtered identity: got %v, want %s:empty", got, want)
							}
						} else if len(got) != 0 {
							t.Errorf("selected/no-profile entry falsely labeled: %v", got)
						}
					}
					// Unfiltered ambiguity cannot label either variant; an ordinary entry remains eligible.
					sc.Include, sc.Exclude = nil, nil
					wantCount := 0
					if layout == "variant-shadow" {
						wantCount = 1
					}
					if got, err := ProfileDeselected("claude", sc, skills); err != nil || len(got) != wantCount || wantCount == 1 && got[want] != "empty" {
						t.Fatalf("ambiguous candidates falsely labeled: %v, %v", got, err)
					}
				})
			}
		}
	}
}
