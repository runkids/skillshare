package sync

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"skillshare/internal/config"
	"skillshare/internal/utils"
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
		inRepo(createTempSkill(t, dir, "_ooux", "ooux"), "_ooux"),
	}

	resolution, err := ResolveTargetSkillsForTarget("claude", config.ResourceTargetConfig{TargetNaming: "prefixed"}, skills)
	if err != nil {
		t.Fatalf("ResolveTargetSkillsForTarget() error = %v", err)
	}

	var got []string
	for _, s := range resolution.Skills {
		got = append(got, s.TargetName)
	}
	// name: need not match the folder: the copy's name: is rewritten anyway.
	want := []string{"emil-design-prototype", "mattpocock-skills-prototype", "bmad-ux", "my-skill", "emil-design-not-odd", "ooux"}
	if strings.Join(got, ",") != strings.Join(want, ",") {
		t.Fatalf("TargetNames = %v, want %v", got, want)
	}
	if len(resolution.Warnings) != 1 {
		t.Fatalf("Warnings = %v, want only the over-long name", resolution.Warnings)
	}
	if !strings.Contains(resolution.Warnings[0], "longer than 64") {
		t.Errorf("over-long warning = %q", resolution.Warnings[0])
	}
}

func TestPrintResolutionSummary_ListsSkillsNamedApartFromFolder(t *testing.T) {
	dir := t.TempDir()
	var skills []DiscoveredSkill
	for i := range 7 {
		skills = append(skills, createTempSkill(t, dir, fmt.Sprintf("dir-%d", i), fmt.Sprintf("name-%d", i)))
	}
	summary := func(naming string) string {
		var out bytes.Buffer
		prev := DiagOutput
		DiagOutput = &out
		defer func() { DiagOutput = prev }()
		resolution, err := ResolveTargetSkillsForTarget("claude", config.ResourceTargetConfig{TargetNaming: naming}, skills)
		if err != nil {
			t.Fatal(err)
		}
		printResolutionSummary(resolution)
		return out.String()
	}

	t.Run("prefixed lists the first few", func(t *testing.T) {
		want := "  7 skill(s) use their SKILL.md name instead of their folder name:\n" +
			"    dir-0 → name-0\n    dir-1 → name-1\n    dir-2 → name-2\n    dir-3 → name-3\n    dir-4 → name-4\n" +
			"    ... and 2 more\n"
		if got := summary("prefixed"); got != want {
			t.Fatalf("summary =\n%s\nwant\n%s", got, want)
		}
	})
	t.Run("standard skips them instead", func(t *testing.T) {
		if got := summary("standard"); strings.Contains(got, "instead of their folder name") {
			t.Fatalf("standard listed skills it skips:\n%s", got)
		}
	})
}

func TestResolveTargetSkillsForTarget_PrefixedNamingKeepsRemainingCollisionsUnderFlatNames(t *testing.T) {
	dir := t.TempDir()
	repoSkill := createTempSkill(t, dir, "_a/b-c", "b-c")
	repoSkill.IsInRepo, repoSkill.RepoRelPath, repoSkill.FlatName = true, "_a", "_a__b-c"
	skills := []DiscoveredSkill{repoSkill, createTempSkill(t, dir, "a-b-c", "a-b-c")}

	resolution, err := ResolveTargetSkillsForTarget("claude", config.ResourceTargetConfig{TargetNaming: "prefixed"}, skills)
	if err != nil {
		t.Fatalf("ResolveTargetSkillsForTarget() error = %v", err)
	}
	if len(resolution.Collisions) != 1 || resolution.Collisions[0].Name != "a-b-c" {
		t.Fatalf("Collisions = %v, want one under a-b-c", resolution.Collisions)
	}
	got := map[string]string{}
	for _, s := range resolution.Skills {
		got[s.TargetName] = s.Naming
	}
	// The top-level a-b-c keeps its name; the tracked skill moves to its flat name.
	if len(got) != 2 || got["a-b-c"] != "prefixed" || got["_a__b-c"] != "flat" {
		t.Fatalf("skills = %v; want a-b-c under prefixed and _a__b-c under flat", got)
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

func TestKeptLocalWarning_CapsTheNameList(t *testing.T) {
	names := []string{"a", "b", "c", "d", "e", "f", "g"}
	got := keptLocalWarning(names)
	want := "kept local: a, b, c, d, e ... and 2 more (sync --force replaces them)"
	if got != want {
		t.Fatalf("keptLocalWarning = %q, want %q", got, want)
	}
}

// A flat name that still clashes with another skill's name would put two
// skills in one entry, so those skills are skipped as before.
func TestResolveTargetSkillsForTarget_SkipsFlatNamesThatStillClash(t *testing.T) {
	dir := t.TempDir()
	flat := func(s DiscoveredSkill) DiscoveredSkill {
		s.FlatName = utils.PathToFlatName(s.RelPath)
		return s
	}
	t.Run("prefixed", func(t *testing.T) {
		// bar's name: is foo; it shares foo with x/foo, and its flat name bar is y/bar's name.
		skills := []DiscoveredSkill{
			flat(createTempSkill(t, dir, "p/bar", "foo")),
			flat(createTempSkill(t, dir, "p/x/foo", "foo")),
			flat(createTempSkill(t, dir, "p/y/bar", "bar")),
		}
		skills[0].FlatName = "bar" // a top-level skill's flat name is its folder
		resolution, err := ResolveTargetSkillsForTarget("claude", config.ResourceTargetConfig{TargetNaming: "prefixed"}, skills)
		if err != nil {
			t.Fatal(err)
		}
		got := map[string]string{}
		for _, s := range resolution.Skills {
			got[s.Skill.RelPath] = s.TargetName
		}
		if len(got) != 2 || got["p/x/foo"] != "p__x__foo" || got["p/y/bar"] != "bar" || resolution.SkippedCount != 1 {
			t.Fatalf("skills = %v, skipped = %d; want p/x/foo as p__x__foo, p/y/bar as bar, one skipped", got, resolution.SkippedCount)
		}
		if resolution.SharedNameSkippedCount != 1 || resolution.NamingSkippedCount != 0 || !strings.Contains(strings.Join(resolution.Warnings, "\n"), "p/bar") {
			t.Fatalf("warnings = %v, naming = %d, shared = %d; want one shared-name warning for p/bar", resolution.Warnings, resolution.NamingSkippedCount, resolution.SharedNameSkippedCount)
		}
	})
	t.Run("standard", func(t *testing.T) {
		skills := []DiscoveredSkill{
			flat(createTempSkill(t, dir, "s/a__b/dev", "dev")),
			flat(createTempSkill(t, dir, "s/a/b/dev", "dev")),
		}
		resolution, err := ResolveTargetSkillsForTarget("claude", config.ResourceTargetConfig{TargetNaming: "standard"}, skills)
		if err != nil {
			t.Fatal(err)
		}
		if len(resolution.Skills) != 0 || resolution.SkippedCount != 2 || resolution.SharedNameSkippedCount != 2 || resolution.NamingSkippedCount != 0 {
			t.Fatalf("skills = %v, skipped = %d, warnings = %v; want both skipped with a warning each", resolution.Skills, resolution.SkippedCount, resolution.Warnings)
		}
	})
}

// A top-level skill's flat name is its own name, so it keeps it, and with it
// the target's naming, while the nested one moves to its flat name.
func TestResolveTargetSkillsForTarget_TopLevelSkillKeepsItsNameInACollision(t *testing.T) {
	dir := t.TempDir()
	top := createTempSkill(t, dir, "dev", "dev")
	nested := createTempSkill(t, dir, "frontend/dev", "dev")
	nested.FlatName = "frontend__dev"

	resolution, err := ResolveTargetSkillsForTarget("claude", config.ResourceTargetConfig{TargetNaming: "standard"}, []DiscoveredSkill{top, nested})
	if err != nil {
		t.Fatal(err)
	}
	got := map[string]string{}
	for _, s := range resolution.Skills {
		got[s.TargetName] = s.Naming
	}
	if len(got) != 2 || got["dev"] != "standard" || got["frontend__dev"] != "flat" {
		t.Fatalf("skills = %v; want dev under standard and frontend__dev under flat", got)
	}
}

// When a collision appears, both skills can claim the copy under the shared
// name. Sync renames it for the first, so diff must report the same.
func TestLegacyNames_FirstSkillClaimsASharedLegacyCopy(t *testing.T) {
	src, target := t.TempDir(), t.TempDir()
	a := createTempSkill(t, src, "a/dev", "dev")
	b := createTempSkill(t, src, "b/dev", "dev")
	a.FlatName, b.FlatName = "a__dev", "b__dev"
	if err := os.MkdirAll(filepath.Join(target, "dev"), 0o755); err != nil {
		t.Fatal(err)
	}
	manifest := &Manifest{Managed: map[string]string{"dev": "sum"}, Naming: map[string]string{"dev": "standard"}}

	resolution, err := ResolveTargetSkillsForTarget("claude", config.ResourceTargetConfig{TargetNaming: "standard"}, []DiscoveredSkill{a, b})
	if err != nil {
		t.Fatal(err)
	}
	legacy := resolution.LegacyNames("copy", target, manifest)
	if got := legacy["dev"].Skill.RelPath; got != "a/dev" {
		t.Fatalf("dev is claimed by %q, want a/dev", got)
	}
}

// A dry run leaves the shared copy where it is, so the second skill must not
// also claim it.
func TestSelectActiveTargetNameForSync_DryRunRenamesASharedCopyOnce(t *testing.T) {
	src, target := t.TempDir(), t.TempDir()
	a := createTempSkill(t, src, "a/dev", "dev")
	b := createTempSkill(t, src, "b/dev", "dev")
	a.FlatName, b.FlatName = "a__dev", "b__dev"
	if err := os.MkdirAll(filepath.Join(target, "dev"), 0o755); err != nil {
		t.Fatal(err)
	}
	manifest := &Manifest{Managed: map[string]string{"dev": "sum"}, Naming: map[string]string{"dev": "standard"}}

	resolution, err := ResolveTargetSkillsForTarget("claude", config.ResourceTargetConfig{TargetNaming: "standard"}, []DiscoveredSkill{a, b})
	if err != nil {
		t.Fatal(err)
	}
	taken := resolution.ValidTargetNames()
	var got []string
	for _, s := range resolution.Skills {
		name, err := selectActiveTargetNameForSync("copy", target, s, taken, manifest, true)
		if err != nil {
			t.Fatal(err)
		}
		got = append(got, name)
	}
	if strings.Join(got, ",") != "dev,b__dev" {
		t.Fatalf("active names = %v, want dev for a/dev (renamed later) and b__dev", got)
	}
}
