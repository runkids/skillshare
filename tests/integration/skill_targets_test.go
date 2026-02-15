//go:build !online

package integration

import (
	"path/filepath"
	"testing"

	"skillshare/internal/testutil"
)

func TestSkillTargets_OnlySyncsToMatchingTarget(t *testing.T) {
	sb := testutil.NewSandbox(t)
	defer sb.Cleanup()

	sb.CreateSkill("claude-skill", map[string]string{
		"SKILL.md": "---\nname: claude-skill\ntargets: [claude]\n---\n# Claude only",
	})
	sb.CreateSkill("universal-skill", map[string]string{
		"SKILL.md": "---\nname: universal-skill\n---\n# Universal",
	})

	claudePath := sb.CreateTarget("claude")
	cursorPath := sb.CreateTarget("cursor")

	sb.WriteConfig(`source: ` + sb.SourcePath + `
targets:
  claude:
    path: ` + claudePath + `
  cursor:
    path: ` + cursorPath + `
`)

	result := sb.RunCLI("sync")
	result.AssertSuccess(t)

	// claude-skill should only be in claude target
	if !sb.IsSymlink(filepath.Join(claudePath, "claude-skill")) {
		t.Error("claude-skill should be synced to claude target")
	}
	if sb.FileExists(filepath.Join(cursorPath, "claude-skill")) {
		t.Error("claude-skill should NOT be synced to cursor target")
	}

	// universal-skill (no targets field) should be in both
	if !sb.IsSymlink(filepath.Join(claudePath, "universal-skill")) {
		t.Error("universal-skill should be synced to claude target")
	}
	if !sb.IsSymlink(filepath.Join(cursorPath, "universal-skill")) {
		t.Error("universal-skill should be synced to cursor target")
	}
}

func TestSkillTargets_CrossModeMatching(t *testing.T) {
	sb := testutil.NewSandbox(t)
	defer sb.Cleanup()

	// Skill declares "claude" (global name), target is configured as "claude"
	// but skill should also match if target were "claude-code" (project name)
	sb.CreateSkill("cross-skill", map[string]string{
		"SKILL.md": "---\nname: cross-skill\ntargets: [claude]\n---\n# Cross",
	})

	projectRoot := sb.SetupProjectDir("claude-code")
	sb.CreateProjectSkill(projectRoot, "cross-skill", map[string]string{
		"SKILL.md": "---\nname: cross-skill\ntargets: [claude]\n---\n# Cross",
	})

	result := sb.RunCLIInDir(projectRoot, "sync", "-p")
	result.AssertSuccess(t)

	// claude-code target path
	targetPath := filepath.Join(projectRoot, ".claude", "skills")
	if !sb.IsSymlink(filepath.Join(targetPath, "cross-skill")) {
		t.Error("skill with targets: [claude] should match claude-code target")
	}
}

func TestSkillTargets_MultipleTargetsListed(t *testing.T) {
	sb := testutil.NewSandbox(t)
	defer sb.Cleanup()

	sb.CreateSkill("multi-skill", map[string]string{
		"SKILL.md": "---\nname: multi-skill\ntargets: [claude, cursor]\n---\n# Multi",
	})
	sb.CreateSkill("single-skill", map[string]string{
		"SKILL.md": "---\nname: single-skill\ntargets: [cursor]\n---\n# Single",
	})

	claudePath := sb.CreateTarget("claude")
	cursorPath := sb.CreateTarget("cursor")

	sb.WriteConfig(`source: ` + sb.SourcePath + `
targets:
  claude:
    path: ` + claudePath + `
  cursor:
    path: ` + cursorPath + `
`)

	result := sb.RunCLI("sync")
	result.AssertSuccess(t)

	// multi-skill should be in both
	if !sb.IsSymlink(filepath.Join(claudePath, "multi-skill")) {
		t.Error("multi-skill should be in claude")
	}
	if !sb.IsSymlink(filepath.Join(cursorPath, "multi-skill")) {
		t.Error("multi-skill should be in cursor")
	}

	// single-skill should only be in cursor
	if sb.FileExists(filepath.Join(claudePath, "single-skill")) {
		t.Error("single-skill should NOT be in claude")
	}
	if !sb.IsSymlink(filepath.Join(cursorPath, "single-skill")) {
		t.Error("single-skill should be in cursor")
	}
}

func TestSkillTargets_DoctorNoDriftWarning(t *testing.T) {
	sb := testutil.NewSandbox(t)
	defer sb.Cleanup()

	sb.CreateSkill("claude-only", map[string]string{
		"SKILL.md": "---\nname: claude-only\ntargets: [claude]\n---\n# Claude only",
	})
	sb.CreateSkill("universal", map[string]string{
		"SKILL.md": "---\nname: universal\n---\n# Universal",
	})

	claudePath := sb.CreateTarget("claude")
	cursorPath := sb.CreateTarget("cursor")

	sb.WriteConfig(`source: ` + sb.SourcePath + `
targets:
  claude:
    path: ` + claudePath + `
  cursor:
    path: ` + cursorPath + `
`)

	sb.RunCLI("sync").AssertSuccess(t)

	// Doctor should NOT warn about drift — cursor correctly has 1 skill (not 2)
	result := sb.RunCLI("doctor")
	result.AssertSuccess(t)
	result.AssertOutputNotContains(t, "not synced")
}

func TestSkillTargets_StatusNoDriftWarning(t *testing.T) {
	sb := testutil.NewSandbox(t)
	defer sb.Cleanup()

	sb.CreateSkill("claude-only", map[string]string{
		"SKILL.md": "---\nname: claude-only\ntargets: [claude]\n---\n# Claude only",
	})
	sb.CreateSkill("universal", map[string]string{
		"SKILL.md": "---\nname: universal\n---\n# Universal",
	})

	claudePath := sb.CreateTarget("claude")
	cursorPath := sb.CreateTarget("cursor")

	sb.WriteConfig(`source: ` + sb.SourcePath + `
targets:
  claude:
    path: ` + claudePath + `
  cursor:
    path: ` + cursorPath + `
`)

	sb.RunCLI("sync").AssertSuccess(t)

	// Status should NOT warn about drift
	result := sb.RunCLI("status")
	result.AssertSuccess(t)
	result.AssertOutputNotContains(t, "not synced")
}

func TestSkillTargets_PrunesWhenTargetRestricted(t *testing.T) {
	sb := testutil.NewSandbox(t)
	defer sb.Cleanup()

	// First sync with universal skill
	sb.CreateSkill("my-skill", map[string]string{
		"SKILL.md": "---\nname: my-skill\n---\n# Universal",
	})
	claudePath := sb.CreateTarget("claude")
	cursorPath := sb.CreateTarget("cursor")

	sb.WriteConfig(`source: ` + sb.SourcePath + `
targets:
  claude:
    path: ` + claudePath + `
  cursor:
    path: ` + cursorPath + `
`)

	sb.RunCLI("sync").AssertSuccess(t)
	if !sb.IsSymlink(filepath.Join(cursorPath, "my-skill")) {
		t.Fatal("my-skill should be in cursor after first sync")
	}

	// Update SKILL.md to restrict to claude only
	sb.CreateSkill("my-skill", map[string]string{
		"SKILL.md": "---\nname: my-skill\ntargets: [claude]\n---\n# Claude only now",
	})

	sb.RunCLI("sync").AssertSuccess(t)
	if !sb.IsSymlink(filepath.Join(claudePath, "my-skill")) {
		t.Error("my-skill should still be in claude")
	}
	if sb.FileExists(filepath.Join(cursorPath, "my-skill")) {
		t.Error("my-skill should be pruned from cursor after adding targets restriction")
	}
}
