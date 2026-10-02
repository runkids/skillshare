package sync

import (
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"skillshare/internal/config"
)

func skillRunSource(t *testing.T) (string, []DiscoveredSkill) {
	t.Helper()
	sourceDir := t.TempDir()
	os.MkdirAll(filepath.Join(sourceDir, "alpha"), 0755)
	os.WriteFile(filepath.Join(sourceDir, "alpha", "SKILL.md"), []byte("---\nname: alpha\n---\n# Alpha"), 0644)
	skills, err := DiscoverSourceSkills(sourceDir)
	if err != nil {
		t.Fatal(err)
	}
	return sourceDir, skills
}

func TestSyncSkillTarget_MergeLinksAndPrunesOrphans(t *testing.T) {
	sourceDir, skills := skillRunSource(t)
	targetDir := t.TempDir()
	if err := os.Symlink(filepath.Join(sourceDir, "old"), filepath.Join(targetDir, "old")); err != nil {
		t.Fatal(err)
	}
	target := config.TargetConfig{Skills: &config.ResourceTargetConfig{Path: targetDir}}

	r := SyncSkillTarget(SkillTarget{Name: "claude", Target: target, Mode: "merge"}, skills, SkillRunOptions{Source: sourceDir})

	if r.Err != nil || !slices.Equal(r.Linked, []string{"alpha"}) || !slices.Equal(r.Pruned, []string{"old"}) {
		t.Fatalf("unexpected result: %+v", r)
	}
}

func TestSyncSkillTarget_ReportsUnmatchedIncludeWithTargetName(t *testing.T) {
	sourceDir, skills := skillRunSource(t)
	for _, mode := range []string{"merge", "copy"} {
		target := config.TargetConfig{Skills: &config.ResourceTargetConfig{Path: t.TempDir(), Include: []string{"missing"}}}

		r := SyncSkillTarget(SkillTarget{Name: "claude", Target: target, Mode: mode}, skills, SkillRunOptions{Source: sourceDir})

		want := `claude: include pattern "missing" matches no skill in the source`
		if r.Err != nil || !slices.Contains(r.Warnings, want) {
			t.Fatalf("%s: warnings = %q, want %q", mode, r.Warnings, want)
		}
	}
}

func TestSyncSkillTarget_SymlinkConflictNeedsForce(t *testing.T) {
	sourceDir, skills := skillRunSource(t)
	elsewhere := t.TempDir()
	linkPath := filepath.Join(t.TempDir(), "skills")
	if err := os.Symlink(elsewhere, linkPath); err != nil {
		t.Fatal(err)
	}
	target := config.TargetConfig{Skills: &config.ResourceTargetConfig{Path: linkPath}}
	st := SkillTarget{Name: "claude", Target: target, Mode: "symlink"}

	r := SyncSkillTarget(st, skills, SkillRunOptions{Source: sourceDir})
	if r.Err == nil || !strings.HasPrefix(r.Err.Error(), "conflict - symlink points to ") {
		t.Fatalf("expected conflict error, got %+v", r)
	}

	r = SyncSkillTarget(st, skills, SkillRunOptions{Source: sourceDir, Force: true})
	if r.Err != nil || r.SymlinkStatus != StatusConflict || CheckStatus(linkPath, sourceDir) != StatusLinked {
		t.Fatalf("expected forced relink, got %+v", r)
	}
}
