package sync

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"gopkg.in/yaml.v3"

	"skillshare/internal/config"
)

func writeAnalyzeSkill(t *testing.T, root, name, body string) {
	t.Helper()
	dir := filepath.Join(root, name)
	if err := os.MkdirAll(dir, 0755); err != nil {
		t.Fatal(err)
	}
	content := "---\nname: " + name + "\ndescription: about " + name + "\n---\n" + body
	if err := os.WriteFile(filepath.Join(dir, "SKILL.md"), []byte(content), 0644); err != nil {
		t.Fatal(err)
	}
}

func skillTarget(path, mode string) config.TargetConfig {
	return config.TargetConfig{Skills: &config.ResourceTargetConfig{Path: path, Mode: mode}}
}

func namesOf(skills []DiscoveredSkill) map[string]DiscoveredSkill {
	m := map[string]DiscoveredSkill{}
	for _, s := range skills {
		m[s.FlatName] = s
	}
	return m
}

func TestEstimateTokens_WideRunesCostOneEach(t *testing.T) {
	if got := EstimateTokens("abcdefgh"); got != 2 {
		t.Errorf("ascii: got %d, want 2", got)
	}
	if got := EstimateTokens("中文字"); got != 3 {
		t.Errorf("cjk: got %d, want 3", got)
	}
}

func TestTargetSkills_SymlinkKeepsDisabledMergeDropsIt(t *testing.T) {
	src := t.TempDir()
	writeAnalyzeSkill(t, src, "kept", "body")
	writeAnalyzeSkill(t, src, "off", "body")
	os.WriteFile(filepath.Join(src, ".skillignore"), []byte("off\n"), 0644)

	discovered, err := DiscoverSourceSkillsForAnalyze(src)
	if err != nil {
		t.Fatal(err)
	}

	symlinked, err := TargetSkills("claude", skillTarget(src, "symlink"), "merge", src, discovered)
	if err != nil {
		t.Fatal(err)
	}
	if got := namesOf(symlinked); len(got) != 2 || !got["off"].Disabled || got["off"].BodyTokens == 0 {
		t.Errorf("symlink target: want kept + measured disabled off, got %+v", symlinked)
	}

	merged, err := TargetSkills("claude", skillTarget(t.TempDir(), "merge"), "merge", src, discovered)
	if err != nil {
		t.Fatal(err)
	}
	if got := namesOf(merged); len(got) != 1 || got["kept"].FlatName == "" {
		t.Errorf("merge target: want only kept, got %+v", merged)
	}
}

func TestTargetSkills_MergeAddsUnmanagedLocalSkills(t *testing.T) {
	src, tgt := t.TempDir(), t.TempDir()
	writeAnalyzeSkill(t, src, "linked", "body")
	if err := os.Symlink(filepath.Join(src, "linked"), filepath.Join(tgt, "linked")); err != nil {
		t.Skip("symlinks unavailable:", err)
	}
	writeAnalyzeSkill(t, tgt, "mine", "手寫的 body")
	writeAnalyzeSkill(t, tgt, "copied", "body")
	if err := WriteManifest(tgt, &Manifest{Managed: map[string]string{"copied": "sha"}}); err != nil {
		t.Fatal(err)
	}

	discovered, err := DiscoverSourceSkillsForAnalyze(src)
	if err != nil {
		t.Fatal(err)
	}
	got, err := TargetSkills("claude", skillTarget(tgt, "merge"), "merge", src, discovered)
	if err != nil {
		t.Fatal(err)
	}
	byName := namesOf(got)
	if len(byName) != 3 || byName["linked"].Local || !byName["mine"].Local || byName["mine"].BodyTokens == 0 || byName["copied"].Local || byName["copied"].BodyTokens == 0 {
		t.Errorf("want linked (source) + mine (local, measured) + copied (managed, measured); got %+v", got)
	}
}

// With skills off nothing is synced, but links a sharing target keeps and
// copies left from copy mode are still in the folder, so the tool loads them.
func TestTargetSkills_SkillsOffCountsEverythingInTheFolder(t *testing.T) {
	src, tgt := t.TempDir(), t.TempDir()
	writeAnalyzeSkill(t, src, "linked", "body")
	writeAnalyzeSkill(t, src, "unlinked", "body")
	if err := os.Symlink(filepath.Join(src, "linked"), filepath.Join(tgt, "linked")); err != nil {
		t.Skip("symlinks unavailable:", err)
	}
	writeAnalyzeSkill(t, tgt, "mine", "body")
	writeAnalyzeSkill(t, tgt, "copied", "body")
	if err := WriteManifest(tgt, &Manifest{Managed: map[string]string{"copied": "sha"}}); err != nil {
		t.Fatal(err)
	}

	discovered, err := DiscoverSourceSkillsForAnalyze(src)
	if err != nil {
		t.Fatal(err)
	}
	off := false
	target := config.TargetConfig{Skills: &config.ResourceTargetConfig{Path: tgt, Mode: "merge", Enabled: &off}}
	got, err := TargetSkills("claude", target, "merge", src, discovered)
	if err != nil {
		t.Fatal(err)
	}
	byName := namesOf(got)
	if len(byName) != 3 || byName["linked"].Local || byName["copied"].Local || !byName["mine"].Local {
		t.Errorf("want linked + copied (managed) + mine (local), not unlinked; got %+v", got)
	}
}

func TestTargetSkills_IncludesProfilePreservedCopies(t *testing.T) {
	for _, mode := range []string{"merge", "copy"} {
		for _, naming := range []string{"flat", "standard", "prefixed"} {
			t.Run(mode+"/"+naming, func(t *testing.T) {
				source, targetPath := t.TempDir(), t.TempDir()
				writeAnalyzeSkill(t, source, "keep", "selected body")
				writeAnalyzeSkill(t, source, "clean", "clean body")
				writeAnalyzeSkill(t, filepath.Join(source, "group"), "edit", "original body")
				skills, err := DiscoverSourceSkillsForAnalyze(source)
				if err != nil {
					t.Fatal(err)
				}
				target := skillTarget(targetPath, "copy")
				target.Skills.TargetNaming = naming
				opts := SkillRunOptions{Source: source, ProfileAware: true}
				if r := SyncSkillTarget(SkillTarget{Name: "claude", Target: target, Mode: "copy"}, skills, opts); r.Err != nil || len(r.Linked) != 3 {
					t.Fatalf("seed: %+v", r)
				}
				resolution, err := ResolveTargetSkillsForTarget("claude", target.SkillsConfig(), skills)
				if err != nil {
					t.Fatal(err)
				}
				editName := ""
				for _, s := range resolution.Skills {
					if s.Skill.RelPath == "group/edit" {
						editName = s.TargetName
					}
				}
				if editName == "" {
					t.Fatal("edited skill not resolved")
				}
				path := filepath.Join(targetPath, editName, "SKILL.md")
				data, err := os.ReadFile(path)
				if err != nil {
					t.Fatal(err)
				}
				if err := os.WriteFile(path, append(data, []byte(strings.Repeat("authored context ", 100))...), 0644); err != nil {
					t.Fatal(err)
				}
				target.Skills.Mode = mode
				var cfg config.Config
				if err := yaml.Unmarshal([]byte("profiles:\n  subset:\n    skills:\n      include: [keep]\n"), &cfg); err != nil {
					t.Fatal(err)
				}
				cfg.Targets = map[string]config.TargetConfig{"claude": target}
				target = cfg.WithActiveProfile("subset").Targets["claude"]
				if selected, err := target.SkillsConfig().Profile.Selects("group__edit"); err != nil || selected {
					t.Fatalf("fixture did not activate subset: selected=%v, %v", selected, err)
				}
				if r := SyncSkillTarget(SkillTarget{Name: "claude", Target: target, Mode: mode}, skills, opts); r.Err != nil || r.PruneErr != nil || len(r.KeptModified) != 1 || len(r.Pruned) != 1 {
					t.Fatalf("profile sync: %+v", r)
				}
				got, err := TargetSkills("claude", target, mode, source, skills)
				if err != nil || len(got) != 2 {
					t.Fatalf("analysis: %+v, %v", got, err)
				}
				byName := namesOf(got)
				if byName["keep"].FlatName == "" || byName[editName].Local || byName[editName].BodyTokens < 100 || byName[editName].SourcePath != filepath.Dir(path) {
					t.Fatalf("preserved deployed content omitted or miscounted: %+v", got)
				}
			})
		}
	}
}

func TestTargetSkills_NamingRejectedCopiesCountOnce(t *testing.T) {
	for _, naming := range []string{"standard", "prefixed"} {
		for _, reason := range []string{"validation", "collision"} {
			t.Run(naming+"/"+reason, func(t *testing.T) {
				source, targetPath := t.TempDir(), t.TempDir()
				writeAnalyzeSkill(t, filepath.Join(source, "one"), "demo", "source body")
				if reason == "collision" {
					writeAnalyzeSkill(t, filepath.Join(source, "two"), "demo", "second body")
				}
				skills, err := DiscoverSourceSkillsForAnalyze(source)
				if err != nil {
					t.Fatal(err)
				}
				target := skillTarget(targetPath, "copy")
				target.Skills.TargetNaming = "flat"
				opts := SkillRunOptions{Source: source, ProfileAware: true}
				if r := SyncSkillTarget(SkillTarget{Name: "test", Target: target, Mode: "copy"}, skills, opts); r.Err != nil || len(r.Linked) != len(skills) {
					t.Fatalf("seed: %+v", r)
				}
				for _, skill := range skills {
					path := filepath.Join(targetPath, skill.FlatName, "SKILL.md")
					data, err := os.ReadFile(path)
					if err != nil {
						t.Fatal(err)
					}
					if err := os.WriteFile(path, append(data, []byte(strings.Repeat("retained context ", 100))...), 0644); err != nil {
						t.Fatal(err)
					}
					if reason == "validation" {
						if err := os.WriteFile(filepath.Join(skill.SourcePath, "SKILL.md"), []byte("---\nname: bad name\n---\nsource body"), 0644); err != nil {
							t.Fatal(err)
						}
					}
				}
				skills, err = DiscoverSourceSkillsForAnalyze(source)
				if err != nil || len(skills) == 0 {
					t.Fatalf("rediscover: %+v, %v", skills, err)
				}
				target.Skills.TargetNaming = naming
				resolution, err := ResolveTargetSkillsForTarget("test", target.SkillsConfig(), skills)
				if err != nil || len(resolution.Skills) != 0 {
					t.Fatalf("naming must reject desired selection: %+v, %v", resolution, err)
				}
				if r := SyncSkillTarget(SkillTarget{Name: "test", Target: target, Mode: "copy"}, skills, opts); r.Err != nil || r.PruneErr != nil || len(r.KeptModified) != len(skills) {
					t.Fatalf("preserve rejected copies: %+v", r)
				}
				got, err := TargetSkills("test", target, "copy", source, skills)
				if err != nil || len(got) != len(skills) {
					t.Fatalf("want only %d deployed copies, got %+v, %v", len(skills), got, err)
				}
				for _, skill := range got {
					if skill.Local || skill.BodyTokens < 100 || !strings.HasPrefix(skill.SourcePath, targetPath+string(filepath.Separator)) {
						t.Fatalf("not measured from retained managed copy: %+v", skill)
					}
				}
			})
		}
	}
}
