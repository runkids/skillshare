package sync

import (
	"os"
	"path/filepath"
	"testing"

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
	if len(byName) != 2 || byName["linked"].Local || !byName["mine"].Local || byName["mine"].BodyTokens == 0 {
		t.Errorf("want linked (source) + mine (local, measured), not copied; got %+v", got)
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

// Claude Code syncs claude.ai skills into synced/<id>/<name>; the id differs per
// account, so those read as "<name> (claude.ai)" on any Claude target. A skill
// the user keeps under the same name elsewhere keeps its own name.
func TestSkillLabel_NamesClaudeAISkillsOnClaudeTargets(t *testing.T) {
	synced := DiscoveredSkill{FlatName: "synced__org_user__skill-creator", RelPath: "synced/org_user/skill-creator", Local: true}
	tests := []struct {
		name, target string
		tc           config.TargetConfig
		skill        DiscoveredSkill
		want         string
	}{
		{"claude target", "claude", config.TargetConfig{}, synced, "skill-creator (claude.ai)"},
		{"second claude account", "claude-work", config.TargetConfig{Agent: "claude"}, synced, "skill-creator (claude.ai)"},
		{"other agent", "cursor", config.TargetConfig{}, synced, "synced__org_user__skill-creator"},
		{"source skill", "claude", config.TargetConfig{}, DiscoveredSkill{FlatName: "skill-creator", RelPath: "skill-creator"}, "skill-creator"},
		{"other local skill", "claude", config.TargetConfig{}, DiscoveredSkill{FlatName: "tools__lint", RelPath: "tools/lint", Local: true}, "tools__lint"},
	}
	for _, tt := range tests {
		if got := SkillLabel(tt.target, tt.tc, tt.skill); got != tt.want {
			t.Errorf("%s: SkillLabel = %q, want %q", tt.name, got, tt.want)
		}
	}
}
