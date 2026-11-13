package sync

import (
	"errors"
	"os"
	"path/filepath"
	"testing"

	"skillshare/internal/config"
	"skillshare/internal/profile"
)

func TestProfiles_SelectionSeam(t *testing.T) {
	catalog := profile.Catalog{Definitions: map[string]profile.Definition{
		"web": {Name: "web", Usable: true, Skills: &profile.SkillSelection{Include: []string{"web*"}, IncludePresent: true}},
	}}
	selector := profile.Resolve(catalog, profile.State{Version: 1, Active: "web"}, "config.yaml", nil)
	skills := []DiscoveredSkill{
		{FlatName: "web-one", RelPath: "web-one"},
		{FlatName: "web-blocked", RelPath: "web-blocked"},
		{FlatName: "other", RelPath: "other"},
		{FlatName: "web-disabled", RelPath: "web-disabled", Disabled: true},
		{FlatName: "web-routed", RelPath: "web-routed", Targets: []string{"other-target"}},
	}
	sc := config.ResourceTargetConfig{Profile: selector, Exclude: []string{"web-blocked"}}
	got, err := SelectTargetSkills(skills, "claude", sc)
	if err != nil || len(got) != 1 || got[0].FlatName != "web-one" {
		t.Fatalf("selection: %+v, %v", got, err)
	}
	rows, err := ClassifySkillsForTarget(skills, "claude", sc)
	if err != nil {
		t.Fatal(err)
	}
	if rows["other"] != (SkillTargetClassification{Status: "not_in_profile", Reason: "web"}) {
		t.Fatal("profile-excluded skill lost its diagnostic reason", rows["other"])
	}
	sc.Profile = profile.Resolve(catalog, profile.State{Version: 1, Active: "missing"}, "config.yaml", nil)
	for _, input := range [][]DiscoveredSkill{skills, nil} {
		if _, err := SelectTargetSkills(input, "claude", sc); !errors.Is(err, profile.ErrProfileUnavailable) {
			t.Fatalf("unavailable selector accepted (skills %d): %v", len(input), err)
		}
	}
}

func TestProfiles_MergePruneUsesSelector(t *testing.T) {
	source, target := t.TempDir(), t.TempDir()
	skillPath := filepath.Join(source, "other")
	if err := os.MkdirAll(skillPath, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(skillPath, filepath.Join(target, "other")); err != nil {
		t.Skipf("symlink unavailable: %v", err)
	}
	if err := WriteManifest(target, &Manifest{Managed: map[string]string{"other": manifestSymlink}}); err != nil {
		t.Fatal(err)
	}
	catalog := profile.Catalog{Definitions: map[string]profile.Definition{
		"web": {Name: "web", Usable: true, Skills: &profile.SkillSelection{Include: []string{"web*"}, IncludePresent: true}},
	}}
	opts := PruneOptions{TargetPath: target, SourcePath: source, TargetName: "claude", ProfileAware: true,
		Skills:  []DiscoveredSkill{{FlatName: "other", RelPath: "other", SourcePath: skillPath}},
		Profile: profile.Resolve(catalog, profile.State{Version: 1, Active: "web"}, "config.yaml", nil)}
	result, err := PruneOrphanLinksWithSkills(opts)
	if err != nil || len(result.Removed) != 1 || result.Removed[0] != "other" {
		t.Fatalf("profile-excluded entry was not pruned: %+v, %v", result, err)
	}
	opts.Profile = profile.Resolve(catalog, profile.State{Version: 1, Active: "missing"}, "config.yaml", nil)
	if _, err := PruneOrphanLinksWithSkills(opts); !errors.Is(err, profile.ErrProfileUnavailable) {
		t.Fatalf("unavailable selector accepted by prune: %v", err)
	}
}

func TestProfiles_SharedFolderVariants(t *testing.T) {
	catalog := profile.Catalog{Definitions: map[string]profile.Definition{
		"web": {Name: "web", Usable: true, Skills: &profile.SkillSelection{Include: []string{"web*"}, IncludePresent: true}},
	}}
	sc := config.ResourceTargetConfig{
		Profile:       profile.Resolve(catalog, profile.State{Version: 1, Active: "web"}, "config.yaml", nil),
		SharedTargets: []config.SkillTargetIdentity{{Name: "claude"}, {Name: "codex"}},
	}
	skills := []DiscoveredSkill{
		{FlatName: "web-claude", RelPath: "web-claude", Targets: []string{"claude"}},
		{FlatName: "web-codex", RelPath: "web-codex", Targets: []string{"codex"}},
		{FlatName: "other", RelPath: "other", Targets: []string{"codex"}},
	}
	got, err := SelectTargetSkills(skills, "claude", sc)
	if err != nil || len(got) != 2 || got[0].FlatName != "web-claude" || got[1].FlatName != "web-codex" {
		t.Fatalf("shared selection: %+v, %v", got, err)
	}
}

func TestProfiles_UnavailableDoesNotCreateTarget(t *testing.T) {
	for _, mode := range []string{"merge", "copy", "symlink"} {
		t.Run(mode, func(t *testing.T) {
			targetPath := filepath.Join(t.TempDir(), "missing")
			var cfg config.Config
			cfg.Targets = map[string]config.TargetConfig{"claude": {Path: targetPath}}
			candidate := cfg.WithActiveProfile("missing")
			run := SyncSkillTarget(SkillTarget{Name: "claude", Target: candidate.Targets["claude"], Mode: mode}, nil, SkillRunOptions{Source: t.TempDir()})
			if !errors.Is(run.Err, profile.ErrProfileUnavailable) {
				t.Fatalf("expected unavailable error: %v", run.Err)
			}
			if _, err := os.Lstat(targetPath); !os.IsNotExist(err) {
				t.Fatalf("unavailable sync created target: %v", err)
			}
			var err error
			if mode == "copy" {
				_, err = SyncTargetCopyWithSkillsOptions("claude", candidate.Targets["claude"], nil, t.TempDir(), false, false, nil, CopyOptions{})
			} else if mode == "merge" {
				_, err = SyncTargetMergeWithPolicy("claude", candidate.Targets["claude"], nil, t.TempDir(), false, false, "", false, nil)
			}
			if mode != "symlink" && !errors.Is(err, profile.ErrProfileUnavailable) {
				t.Fatalf("writer accepted unavailable selection: %v", err)
			}
			if _, err := os.Lstat(targetPath); !os.IsNotExist(err) {
				t.Fatalf("writer created unavailable target: %v", err)
			}
		})
	}
}
