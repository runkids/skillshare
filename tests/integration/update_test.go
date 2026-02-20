//go:build !online

package integration

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"skillshare/internal/testutil"
)

// writeMeta writes a minimal .skillshare-meta.json to make a skill updatable.
func writeMeta(t *testing.T, skillDir string) {
	t.Helper()
	meta := map[string]any{"source": "/tmp/fake-source", "type": "local"}
	data, _ := json.Marshal(meta)
	if err := os.WriteFile(filepath.Join(skillDir, ".skillshare-meta.json"), data, 0644); err != nil {
		t.Fatalf("failed to write meta: %v", err)
	}
}

func setupGlobalConfig(sb *testutil.Sandbox) {
	sb.WriteConfig(`source: ` + sb.SourcePath + "\ntargets: {}\n")
}

// --- Global mode tests ---

func TestUpdate_MultipleNames_DryRun(t *testing.T) {
	sb := testutil.NewSandbox(t)
	defer sb.Cleanup()
	setupGlobalConfig(sb)

	d1 := sb.CreateSkill("skill-a", map[string]string{"SKILL.md": "# A"})
	writeMeta(t, d1)
	d2 := sb.CreateSkill("skill-b", map[string]string{"SKILL.md": "# B"})
	writeMeta(t, d2)

	result := sb.RunCLI("update", "skill-a", "skill-b", "--dry-run")
	result.AssertSuccess(t)
	result.AssertAnyOutputContains(t, "skill-a")
	result.AssertAnyOutputContains(t, "skill-b")
	result.AssertAnyOutputContains(t, "dry-run")
}

func TestUpdate_MultipleNames_PartialNotFound(t *testing.T) {
	sb := testutil.NewSandbox(t)
	defer sb.Cleanup()
	setupGlobalConfig(sb)

	d1 := sb.CreateSkill("real-skill", map[string]string{"SKILL.md": "# Real"})
	writeMeta(t, d1)

	result := sb.RunCLI("update", "real-skill", "ghost", "--dry-run")
	result.AssertSuccess(t)
	result.AssertAnyOutputContains(t, "real-skill")
	result.AssertAnyOutputContains(t, "ghost")
}

func TestUpdate_Group_DryRun(t *testing.T) {
	sb := testutil.NewSandbox(t)
	defer sb.Cleanup()
	setupGlobalConfig(sb)

	d1 := sb.CreateNestedSkill("frontend/react", map[string]string{"SKILL.md": "# React"})
	writeMeta(t, d1)
	d2 := sb.CreateNestedSkill("frontend/vue", map[string]string{"SKILL.md": "# Vue"})
	writeMeta(t, d2)

	result := sb.RunCLI("update", "--group", "frontend", "--dry-run")
	result.AssertSuccess(t)
	result.AssertAnyOutputContains(t, "react")
	result.AssertAnyOutputContains(t, "vue")
	result.AssertAnyOutputContains(t, "dry-run")
}

func TestUpdate_Group_SkipsLocal(t *testing.T) {
	sb := testutil.NewSandbox(t)
	defer sb.Cleanup()
	setupGlobalConfig(sb)

	d1 := sb.CreateNestedSkill("backend/api", map[string]string{"SKILL.md": "# API"})
	writeMeta(t, d1)
	sb.CreateNestedSkill("backend/local-only", map[string]string{"SKILL.md": "# Local"})

	result := sb.RunCLI("update", "--group", "backend", "--dry-run")
	result.AssertSuccess(t)
	result.AssertAnyOutputContains(t, "api")
	result.AssertOutputNotContains(t, "local-only")
}

func TestUpdate_Group_NotFound(t *testing.T) {
	sb := testutil.NewSandbox(t)
	defer sb.Cleanup()
	setupGlobalConfig(sb)

	result := sb.RunCLI("update", "--group", "nonexistent", "--dry-run")
	result.AssertFailure(t)
	result.AssertAnyOutputContains(t, "nonexistent")
}

func TestUpdate_Mixed_NamesAndGroup(t *testing.T) {
	sb := testutil.NewSandbox(t)
	defer sb.Cleanup()
	setupGlobalConfig(sb)

	d1 := sb.CreateSkill("standalone", map[string]string{"SKILL.md": "# Standalone"})
	writeMeta(t, d1)

	d2 := sb.CreateNestedSkill("frontend/react", map[string]string{"SKILL.md": "# React"})
	writeMeta(t, d2)

	result := sb.RunCLI("update", "standalone", "--group", "frontend", "--dry-run")
	result.AssertSuccess(t)
	result.AssertAnyOutputContains(t, "standalone")
	result.AssertAnyOutputContains(t, "react")
}

func TestUpdate_AllMutuallyExclusive(t *testing.T) {
	sb := testutil.NewSandbox(t)
	defer sb.Cleanup()
	setupGlobalConfig(sb)

	result := sb.RunCLI("update", "--all", "some-name")
	result.AssertFailure(t)
	result.AssertAnyOutputContains(t, "cannot be used with")
}

func TestUpdate_PositionalGroupAutoDetect(t *testing.T) {
	sb := testutil.NewSandbox(t)
	defer sb.Cleanup()
	setupGlobalConfig(sb)

	d1 := sb.CreateNestedSkill("mygroup/s1", map[string]string{"SKILL.md": "# S1"})
	writeMeta(t, d1)

	result := sb.RunCLI("update", "mygroup", "--dry-run")
	result.AssertSuccess(t)
	result.AssertAnyOutputContains(t, "is a group")
	result.AssertAnyOutputContains(t, "s1")
}
