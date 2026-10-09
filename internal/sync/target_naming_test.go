package sync

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"skillshare/internal/config"
)

func TestPrefixedTargetName(t *testing.T) {
	tests := []struct {
		repo string // RepoRelPath; empty means outside a tracked repo
		name string
		want string
	}{
		{"_emil-design", "prototype", "emil-design-prototype"},
		{"_mattpocock-skills", "prototype", "mattpocock-skills-prototype"},
		{"_bmad", "bmad-ux", "bmad-ux"},
		{"_bmad", "bmad", "bmad"},
		{"", "my-skill", "my-skill"},
		{"org/_My_Team..v2", "dev", "my-team-v2-dev"},
		{"_--", "dev", "dev"},
		{"_技能", "dev", "技能-dev"},
		{"_Café", "dev", "café-dev"},
	}
	for _, tt := range tests {
		skill := DiscoveredSkill{IsInRepo: tt.repo != "", RepoRelPath: tt.repo}
		if got := PrefixedTargetName(skill, tt.name); got != tt.want {
			t.Errorf("PrefixedTargetName(%q, %q) = %q, want %q", tt.repo, tt.name, got, tt.want)
		}
	}
}

func TestResolveTargetSkillsForTarget_BOMFrontmatter(t *testing.T) {
	dir := t.TempDir()
	skill := createTempSkill(t, dir, "notepad-skill", "notepad-skill")
	content := "\ufeff---\nname: notepad-skill\n---\n# notepad-skill"
	if err := os.WriteFile(filepath.Join(skill.SourcePath, "SKILL.md"), []byte(content), 0644); err != nil {
		t.Fatal(err)
	}

	for _, naming := range []string{"standard", "prefixed"} {
		resolution, err := ResolveTargetSkillsForTarget("claude", config.ResourceTargetConfig{TargetNaming: naming}, []DiscoveredSkill{skill})
		if err != nil {
			t.Fatalf("%s: %v", naming, err)
		}
		if len(resolution.Skills) != 1 || resolution.Skills[0].TargetName != "notepad-skill" {
			t.Errorf("%s: skills = %+v, want notepad-skill kept", naming, resolution.Skills)
		}
	}
}

func TestResolveTargetSkillsForTarget_PrefixedNaming(t *testing.T) {
	dir := t.TempDir()
	inRepo := func(s DiscoveredSkill, repo string) DiscoveredSkill {
		s.IsInRepo, s.RepoRelPath = true, repo
		return s
	}
	longRepo, longName := strings.Repeat("a", 40), strings.Repeat("b", 30)
	skills := []DiscoveredSkill{
		inRepo(createTempSkill(t, dir, "_emil-design/skills/prototype", "prototype"), "_emil-design"),
		inRepo(createTempSkill(t, dir, "_mattpocock-skills/skills/engineering/prototype", "prototype"), "_mattpocock-skills"),
		inRepo(createTempSkill(t, dir, "_bmad/skills/bmad-ux", "bmad-ux"), "_bmad"),
		createTempSkill(t, dir, "my-skill", "my-skill"),
		inRepo(createTempSkill(t, dir, "_"+longRepo+"/"+longName, longName), "_"+longRepo),
		inRepo(createTempSkill(t, dir, "_emil-design/skills/odd", "not-odd"), "_emil-design"),
	}

	resolution, err := ResolveTargetSkillsForTarget("claude", config.ResourceTargetConfig{TargetNaming: "prefixed"}, skills)
	if err != nil {
		t.Fatalf("ResolveTargetSkillsForTarget() error = %v", err)
	}

	var got []string
	for _, s := range resolution.Skills {
		got = append(got, s.TargetName)
	}
	want := []string{"emil-design-prototype", "mattpocock-skills-prototype", "bmad-ux", "my-skill"}
	if strings.Join(got, ",") != strings.Join(want, ",") {
		t.Fatalf("TargetNames = %v, want %v", got, want)
	}
	if len(resolution.Warnings) != 2 {
		t.Fatalf("Warnings = %v, want the over-long and the name/folder mismatch", resolution.Warnings)
	}
	if !strings.Contains(resolution.Warnings[0], "longer than 64") {
		t.Errorf("over-long warning = %q", resolution.Warnings[0])
	}
}

func TestResolveTargetSkillsForTarget_PrefixedNamingSkipsRemainingCollisions(t *testing.T) {
	dir := t.TempDir()
	repoSkill := createTempSkill(t, dir, "_a/b-c", "b-c")
	repoSkill.IsInRepo, repoSkill.RepoRelPath = true, "_a"
	skills := []DiscoveredSkill{repoSkill, createTempSkill(t, dir, "a-b-c", "a-b-c")}

	resolution, err := ResolveTargetSkillsForTarget("claude", config.ResourceTargetConfig{TargetNaming: "prefixed"}, skills)
	if err != nil {
		t.Fatalf("ResolveTargetSkillsForTarget() error = %v", err)
	}
	if len(resolution.Skills) != 0 || len(resolution.Collisions) != 1 || resolution.Collisions[0].Name != "a-b-c" {
		t.Fatalf("Skills = %v, Collisions = %v; want both excluded under a-b-c", resolution.Skills, resolution.Collisions)
	}
}

func TestCopySkillToTarget_RemovesCopyWhenNameRewriteFails(t *testing.T) {
	src := t.TempDir() // no SKILL.md, so the name rewrite cannot read it
	if err := os.WriteFile(filepath.Join(src, "notes.md"), []byte("x"), 0644); err != nil {
		t.Fatal(err)
	}
	dst := filepath.Join(t.TempDir(), "repo-skill")

	if err := copySkillToTarget(src, dst, "repo-skill", nil); err == nil {
		t.Fatal("expected the rewrite error")
	}
	// A copy left behind without a manifest entry would look user-owned and
	// never be refreshed again.
	if _, err := os.Lstat(dst); !os.IsNotExist(err) {
		t.Fatalf("copy left behind: %v", err)
	}
}

func TestCopySkillToTarget_RewritesReadOnlySkillFile(t *testing.T) {
	if os.Getuid() == 0 {
		t.Skip("a read-only file is still writable as root")
	}
	src := t.TempDir()
	if err := os.WriteFile(filepath.Join(src, "SKILL.md"), []byte("---\nname: skill\n---\n"), 0o444); err != nil {
		t.Fatal(err)
	}
	dst := filepath.Join(t.TempDir(), "repo-skill")

	if err := copySkillToTarget(src, dst, "repo-skill", nil); err != nil {
		t.Fatalf("copySkillToTarget: %v", err)
	}
	info, err := os.Stat(filepath.Join(dst, "SKILL.md"))
	if err != nil || info.Mode().Perm() != 0o444 {
		t.Fatalf("copied SKILL.md mode = %v, err = %v; want the source's 0444 kept", info, err)
	}
}
