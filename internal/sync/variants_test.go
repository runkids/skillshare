package sync

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"skillshare/internal/config"
	"skillshare/internal/install"
)

func TestTrackedAgentCategoriesRemainOrdinarySkills(t *testing.T) {
	src := t.TempDir()
	for _, category := range []string{"agents", "continue", "pi", "warp", "factory", "copilot"} {
		writeSkillMD(t, filepath.Join(src, "_repo/skills", category, category+"-planner"), "# Instructions")
	}
	for _, lite := range []bool{false, true} {
		var skills []DiscoveredSkill
		var err error
		if lite {
			skills, _, err = DiscoverSourceSkillsLite(src)
		} else {
			skills, err = DiscoverSourceSkills(src)
		}
		if err != nil {
			t.Fatal(err)
		}
		for _, sk := range skills {
			if sk.LogicalID != "" || sk.VariantAgent != "" {
				t.Fatalf("category inferred as variant: %+v", sk)
			}
		}
		if !lite {
			selected, err := SelectTargetSkills(skills, "claude", config.ResourceTargetConfig{})
			if err != nil || len(selected) != 6 {
				t.Fatalf("category selection = %d, %v", len(selected), err)
			}
		}
	}
}

func TestTrackedVariantsSharedFolder(t *testing.T) {
	for _, mode := range []string{"merge", "copy"} {
		t.Run(mode, func(t *testing.T) {
			src, dest := t.TempDir(), t.TempDir()
			for _, agent := range []string{"claude", "codex"} {
				writeSkillMD(t, filepath.Join(src, "_repo/skills", agent, "fix"), "# Instructions for "+agent)
			}
			skills, err := DiscoverSourceSkills(src)
			if err != nil {
				t.Fatal(err)
			}
			targets := map[string]config.TargetConfig{
				"codex": {Path: dest}, "universal": {Path: dest},
			}
			cfg := &config.Config{Source: src, Mode: mode, Targets: targets}
			for i := 0; i < 2; i++ {
				result, err := ExecuteTargets(context.Background(), cfg, targets, ExecuteOptions{Kinds: []string{"skill"}, Discovery: TargetDiscovery{Skills: skills}})
				if err != nil || len(result.Failed) != 0 {
					t.Fatalf("sync: %+v, %v", result, err)
				}
				for _, r := range result.Results {
					if len(r.Pruned) != 0 {
						t.Fatalf("shared variant pruned: %+v", r)
					}
				}
				got, err := os.ReadFile(filepath.Join(dest, "_repo__skills__codex__fix", "SKILL.md"))
				if err != nil || string(got) != "# Instructions for codex" {
					t.Fatalf("Codex lost variant: %q, %v", got, err)
				}
			}
		})
	}
}

func TestTrackedVariantsRouteAndPreserveBytes(t *testing.T) {
	src := t.TempDir()
	for _, agent := range []string{"claude-code", "codex"} {
		for _, name := range []string{"fix", "review/fix"} {
			writeSkillMD(t, filepath.Join(src, "_repo/internal/skills", agent, name), "---\nname: fix\ndescription: Review changes\n---\nInstructions for "+agent+"\n")
		}
	}
	skills, err := DiscoverSourceSkills(src)
	if err != nil {
		t.Fatal(err)
	}
	if len(skills) != 4 {
		t.Fatalf("physical entries = %d, want 4", len(skills))
	}
	for _, tc := range []struct {
		name, agent string
		want        int
	}{
		{"claude", "", 2}, {"claude-code", "", 2}, {"codex", "", 2},
		{"codex-work", "codex", 2}, {"universal", "", 0}, {"cursor", "", 0},
	} {
		t.Run(tc.name, func(t *testing.T) {
			for _, mode := range []string{"merge", "copy"} {
				target := config.TargetConfig{Agent: tc.agent, Path: filepath.Join(t.TempDir(), "skills"), Mode: mode}
				selected, err := SelectTargetSkills(skills, tc.name, target.SkillsConfig())
				if err != nil || len(selected) != tc.want {
					t.Fatalf("selected = %d, want %d: %v", len(selected), tc.want, err)
				}
				run := SyncSkillTarget(SkillTarget{Name: tc.name, Target: target, Mode: mode}, skills, SkillRunOptions{Source: src})
				if run.Err != nil || len(run.Linked) != tc.want {
					t.Fatalf("%s sync = %+v", mode, run)
				}
				for _, sk := range selected {
					want, err := os.ReadFile(filepath.Join(sk.SourcePath, "SKILL.md"))
					if err != nil {
						t.Fatal(err)
					}
					got, err := os.ReadFile(filepath.Join(target.Path, sk.FlatName, "SKILL.md"))
					if err != nil {
						t.Fatal(err)
					}
					if !bytes.Equal(got, want) {
						t.Fatalf("%s changed bytes for %s", mode, sk.RelPath)
					}
				}
			}
		})
	}
}

func TestTrackedVariantRoutingPrecedence(t *testing.T) {
	src := t.TempDir()
	paths := []string{"_repo/claude/skills/codex/inferred", "_repo/skills/codex/explicit", "_repo/skills/codex/all", "_repo/skills/codex/override"}
	for _, path := range paths {
		// A second canonical agent distinguishes this family from a category.
		peer := strings.Replace(path, "/skills/codex/", "/skills/claude/", 1)
		writeSkillMD(t, filepath.Join(src, peer), "---\nmetadata:\n  targets: [unused]\n---\n# Peer")
		content := "# Instructions"
		if filepath.Base(path) == "explicit" || filepath.Base(path) == "override" {
			content = "---\nmetadata:\n  targets: [custom]\n---\n# Instructions"
		}
		writeSkillMD(t, filepath.Join(src, path), content)
	}
	store := install.NewMetadataStore()
	store.SetTargetOverride(paths[2], nil)
	store.SetTargetOverride(paths[3], []string{"claude"})
	if err := store.Save(src); err != nil {
		t.Fatal(err)
	}
	skills, err := DiscoverSourceSkills(src)
	if err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		name string
		want []string
	}{
		{"codex", []string{paths[0], paths[2]}}, {"universal", []string{paths[2]}},
		{"custom", []string{paths[1], paths[2]}}, {"claude", []string{paths[2], paths[3]}},
	} {
		got, err := SelectTargetSkills(skills, tc.name, config.ResourceTargetConfig{})
		if err != nil {
			t.Fatal(err)
		}
		if len(got) != len(tc.want) {
			t.Errorf("%s paths = %v, want %v", tc.name, got, tc.want)
			continue
		}
		for _, sk := range got {
			found := false
			for _, want := range tc.want {
				found = found || sk.RelPath == want
			}
			if !found {
				t.Errorf("%s unexpectedly receives %s", tc.name, sk.RelPath)
			}
		}
	}
}

func TestTrackedVariantAliasAmbiguity(t *testing.T) {
	src := t.TempDir()
	for _, agent := range []string{"claude", "claude-code", "codex"} {
		writeSkillMD(t, filepath.Join(src, "_repo/skills", agent, "fix"), "# Instructions")
	}
	skills, err := DiscoverSourceSkills(src)
	if err != nil {
		t.Fatal(err)
	}
	got, err := SelectTargetSkills(skills, "claude", config.ResourceTargetConfig{})
	if err != nil || len(got) != 0 {
		t.Fatalf("ambiguous Claude variants selected = %v: %v", got, err)
	}
	resolution, err := ResolveTargetSkillsForTarget("claude", config.ResourceTargetConfig{}, skills)
	if err != nil || len(resolution.SelectionWarnings) == 0 || len(resolution.Warnings) != 0 || resolution.NamingSkippedCount != 0 {
		t.Fatalf("ambiguous variants need a diagnostic: %+v, %v", resolution, err)
	}
	got, err = SelectTargetSkills(skills, "claude", config.ResourceTargetConfig{Include: []string{"_repo__skills__claude__fix"}})
	if err != nil || len(got) != 1 {
		t.Fatalf("explicit include should resolve ambiguity: %v, %v", got, err)
	}
}

func TestTrackedVariantCountsKeepRepositoryPrefixAndSuffix(t *testing.T) {
	src := t.TempDir()
	for _, repo := range []string{"_one", "org/_two"} {
		if err := os.MkdirAll(filepath.Join(src, repo, ".git"), 0755); err != nil {
			t.Fatal(err)
		}
		for _, prefix := range []string{"internal/skills", "other/skills"} {
			for _, suffix := range []string{"fix", "review/skills/codex/fix"} {
				for _, agent := range []string{"claude", "codex"} {
					writeSkillMD(t, filepath.Join(src, repo, prefix, agent, suffix), "# Instructions")
				}
			}
		}
	}
	// Unknown agent layouts and ordinary skills remain independent physical skills.
	for _, path := range []string{"_one/skills/unknown/fix", "_one/skills/shared/fix", "skills/claude/fix", "skills/codex/fix"} {
		writeSkillMD(t, filepath.Join(src, path), "# Instructions")
	}
	full, err := DiscoverSourceSkills(src)
	if err != nil {
		t.Fatal(err)
	}
	lite, _, err := DiscoverSourceSkillsLite(src)
	if err != nil {
		t.Fatal(err)
	}
	for _, skills := range [][]DiscoveredSkill{full, lite} {
		counts, repos := CountSkillsByRepo(skills)
		if counts.Skills != 12 || counts.Variants != 20 {
			t.Errorf("source counts = %+v, want 12 skills/20 variants", counts)
		}
		if repos["_one"].Skills != 6 || repos["org/_two"].Skills != 4 {
			t.Errorf("repo counts = %+v", repos)
		}
	}
}

func TestTrackedVariantIgnoresAndPhysicalContext(t *testing.T) {
	src := t.TempDir()
	for _, path := range []string{"_repo/skills/claude/fix", "_repo/skills/codex/fix", "_repo/skills/claude/hidden", "_repo/skills/codex/hidden"} {
		writeSkillMD(t, filepath.Join(src, path), "---\nname: fix\ndescription: Review changes\n---\n# Instructions")
	}
	if err := os.WriteFile(filepath.Join(src, ".skillignore"), []byte("**/hidden\n_repo/skills/claude/fix\n"), 0644); err != nil {
		t.Fatal(err)
	}
	skills, err := DiscoverSourceSkillsForAnalyze(src)
	if err != nil {
		t.Fatal(err)
	}
	counts := CountSkills(skills)
	if counts.Skills != 1 || counts.Variants != 1 {
		t.Fatalf("active counts = %+v", counts)
	}
	selected, err := SelectTargetSkills(skills, "claude", config.ResourceTargetConfig{Include: []string{"*"}})
	if err != nil || len(selected) != 0 {
		t.Fatalf("ignored Claude gets substitute: %+v, %v", selected, err)
	}
	for _, mode := range []string{"merge", "symlink"} {
		target := config.TargetConfig{Mode: mode}
		loaded, err := TargetSkills("codex", target, mode, src, skills)
		if err != nil {
			t.Fatal(err)
		}
		want := 1
		if mode == "symlink" {
			want = 4
		}
		if len(loaded) != want {
			t.Fatalf("%s physical context entries = %d, want %d", mode, len(loaded), want)
		}
		for _, sk := range loaded {
			if sk.BodyChars == 0 {
				t.Fatalf("missing physical context for %s", sk.RelPath)
			}
		}
	}
}

func TestTrackedVariantUpgradeDisclosesPrunedEntries(t *testing.T) {
	for _, mode := range []string{"merge", "copy"} {
		t.Run(mode, func(t *testing.T) {
			src := t.TempDir()
			store := install.NewMetadataStore()
			for _, agent := range []string{"claude", "codex"} {
				rel := "_repo/skills/" + agent + "/fix"
				writeSkillMD(t, filepath.Join(src, rel), "# Instructions for "+agent)
				store.SetTargetOverride(rel, nil)
			}
			if err := store.Save(src); err != nil {
				t.Fatal(err)
			}
			// Empty overrides reproduce the unrestricted selection of pre-upgrade discovery.
			legacy, err := DiscoverSourceSkills(src)
			if err != nil {
				t.Fatal(err)
			}
			target := SkillTarget{Name: "universal", Target: config.TargetConfig{Path: t.TempDir(), Mode: mode}, Mode: mode}
			before := SyncSkillTarget(target, legacy, SkillRunOptions{Source: src})
			if before.Err != nil || len(before.Linked) != 2 {
				t.Fatalf("unrestricted sync: %+v", before)
			}
			store.RemoveTargetOverrides("_repo")
			if err := store.Save(src); err != nil {
				t.Fatal(err)
			}
			inferred, err := DiscoverSourceSkills(src)
			if err != nil {
				t.Fatal(err)
			}
			for _, dryRun := range []bool{true, false} {
				after := SyncSkillTarget(target, inferred, SkillRunOptions{Source: src, DryRun: dryRun})
				warnings := strings.Join(after.Warnings, "\n")
				if after.Err != nil || len(after.Pruned) != 2 {
					t.Fatalf("upgrade prune: %+v", after)
				}
				if !strings.Contains(warnings, "automatic agent routing") || !strings.Contains(warnings, "target override") {
					t.Fatalf("upgrade removals need routing and recovery explanation: %+v", after)
				}
			}
			for _, sk := range inferred {
				if _, err := os.Stat(filepath.Join(sk.SourcePath, "SKILL.md")); err != nil {
					t.Fatal(err)
				}
				if _, err := os.Lstat(filepath.Join(target.Target.Path, sk.FlatName)); !os.IsNotExist(err) {
					t.Fatalf("mismatched target entry survived: %s, %v", sk.FlatName, err)
				}
			}
			store.SetTargetOverride("_repo/skills/codex/fix", []string{"universal"})
			if err := store.Save(src); err != nil {
				t.Fatal(err)
			}
			restored, err := DiscoverSourceSkills(src)
			if err != nil {
				t.Fatal(err)
			}
			recovery := SyncSkillTarget(target, restored, SkillRunOptions{Source: src})
			if recovery.Err != nil || len(recovery.Linked) != 1 || len(recovery.Pruned) != 0 {
				t.Fatalf("override recovery: %+v", recovery)
			}
		})
	}
}

func TestTrackedVariantAmbiguousPruneExplainsRemoval(t *testing.T) {
	for _, mode := range []string{"merge", "copy"} {
		t.Run(mode, func(t *testing.T) {
			src := t.TempDir()
			for _, agent := range []string{"claude", "codex"} {
				writeSkillMD(t, filepath.Join(src, "_repo/skills", agent, "fix"), "# Instructions")
			}
			target := SkillTarget{Name: "claude", Target: config.TargetConfig{Path: t.TempDir()}, Mode: mode}
			skills, err := DiscoverSourceSkills(src)
			if err != nil {
				t.Fatal(err)
			}
			if result := SyncSkillTarget(target, skills, SkillRunOptions{Source: src}); result.Err != nil || len(result.Linked) != 1 {
				t.Fatalf("initial sync: %+v", result)
			}
			writeSkillMD(t, filepath.Join(src, "_repo/skills/claude-code/fix"), "# Alias makes selection ambiguous")
			skills, err = DiscoverSourceSkills(src)
			if err != nil {
				t.Fatal(err)
			}
			result := SyncSkillTarget(target, skills, SkillRunOptions{Source: src})
			if result.Err != nil || len(result.Pruned) != 1 || !strings.Contains(strings.Join(result.Warnings, "\n"), "ambiguous") {
				t.Fatalf("silent ambiguous removal: %+v", result)
			}
		})
	}
}

func TestNestedAgentNamedSkillsRemainOrdinary(t *testing.T) {
	src := t.TempDir()
	for _, agent := range []string{"codex", "gemini"} {
		writeSkillMD(t, filepath.Join(src, "_repo/skills", agent), "# Parent skill")
		writeSkillMD(t, filepath.Join(src, "_repo/skills", agent, "review"), "# Nested skill")
	}
	skills, err := DiscoverSourceSkills(src)
	if err != nil {
		t.Fatal(err)
	}
	for _, skill := range skills {
		if skill.LogicalID != "" {
			t.Fatalf("ordinary nested skill inferred as variant: %+v", skill)
		}
	}
	selected, err := SelectTargetSkills(skills, "claude", config.ResourceTargetConfig{})
	if err != nil || len(selected) != 4 {
		t.Fatalf("selected = %v, %v", selected, err)
	}
}
