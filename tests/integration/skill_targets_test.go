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

func TestSkillTargets_InfersTargetsFromHostPaths(t *testing.T) {
	sb := testutil.NewSandbox(t)
	defer sb.Cleanup()

	sb.CreateNestedSkill(".cursor/skills/gstack-browse", map[string]string{
		"SKILL.md": "---\nname: gstack-browse\n---\n# Cursor host skill",
	})
	sb.CreateNestedSkill(".factory/skills/gstack-benchmark", map[string]string{
		"SKILL.md": "---\nname: gstack-benchmark\n---\n# Factory host skill",
	})
	sb.CreateSkill("generic", map[string]string{
		"SKILL.md": "---\nname: generic\n---\n# Generic",
	})

	cursorPath := sb.CreateTarget("cursor")
	droidPath := sb.CreateTarget("droid")

	sb.WriteConfig(`source: ` + sb.SourcePath + `
targets:
  cursor:
    path: ` + cursorPath + `
  droid:
    path: ` + droidPath + `
`)

	sb.RunCLI("sync").AssertSuccess(t)

	if !sb.IsSymlink(filepath.Join(cursorPath, ".cursor__skills__gstack-browse")) {
		t.Error("cursor host skill should be synced to cursor target")
	}
	if sb.FileExists(filepath.Join(cursorPath, ".factory__skills__gstack-benchmark")) {
		t.Error("factory host skill should NOT be synced to cursor target")
	}
	if !sb.IsSymlink(filepath.Join(cursorPath, "generic")) {
		t.Error("generic skill should still sync to cursor target")
	}

	if !sb.IsSymlink(filepath.Join(droidPath, ".factory__skills__gstack-benchmark")) {
		t.Error("factory host skill should be synced to droid target")
	}
	if sb.FileExists(filepath.Join(droidPath, ".cursor__skills__gstack-browse")) {
		t.Error("cursor host skill should NOT be synced to droid target")
	}
	if !sb.IsSymlink(filepath.Join(droidPath, "generic")) {
		t.Error("generic skill should still sync to droid target")
	}
}

func TestSkillTargets_CrossModeMatching(t *testing.T) {
	sb := testutil.NewSandbox(t)
	defer sb.Cleanup()

	// Skill declares "claude" (global name), target is configured as "claude"
	// but skill should also match if target were "claude" (project name)
	sb.CreateSkill("cross-skill", map[string]string{
		"SKILL.md": "---\nname: cross-skill\ntargets: [claude]\n---\n# Cross",
	})

	projectRoot := sb.SetupProjectDir("claude")
	sb.CreateProjectSkill(projectRoot, "cross-skill", map[string]string{
		"SKILL.md": "---\nname: cross-skill\ntargets: [claude]\n---\n# Cross",
	})

	result := sb.RunCLIInDir(projectRoot, "sync", "-p")
	result.AssertSuccess(t)

	// claude target path
	targetPath := filepath.Join(projectRoot, ".claude", "skills")
	if !sb.IsSymlink(filepath.Join(targetPath, "cross-skill")) {
		t.Error("skill with targets: [claude] should match claude target")
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

// --- metadata.targets integration tests ---

func TestSkillTargets_MetadataTargetsFiltersCorrectly(t *testing.T) {
	sb := testutil.NewSandbox(t)
	defer sb.Cleanup()

	// Use new metadata.targets format
	sb.CreateSkill("meta-skill", map[string]string{
		"SKILL.md": "---\nname: meta-skill\nmetadata:\n  targets: [claude]\n---\n# Metadata targets",
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

	result := sb.RunCLI("sync")
	result.AssertSuccess(t)

	// meta-skill should only be in claude (metadata.targets: [claude])
	if !sb.IsSymlink(filepath.Join(claudePath, "meta-skill")) {
		t.Error("meta-skill should be synced to claude target")
	}
	if sb.FileExists(filepath.Join(cursorPath, "meta-skill")) {
		t.Error("meta-skill should NOT be synced to cursor target")
	}

	// universal should be in both
	if !sb.IsSymlink(filepath.Join(claudePath, "universal")) {
		t.Error("universal should be synced to claude")
	}
	if !sb.IsSymlink(filepath.Join(cursorPath, "universal")) {
		t.Error("universal should be synced to cursor")
	}
}

func TestSkillTargets_MetadataAndLegacyMixed(t *testing.T) {
	sb := testutil.NewSandbox(t)
	defer sb.Cleanup()

	// One skill uses metadata.targets, another uses top-level targets
	sb.CreateSkill("new-format", map[string]string{
		"SKILL.md": "---\nname: new-format\nmetadata:\n  targets: [claude]\n---\n# New format",
	})
	sb.CreateSkill("old-format", map[string]string{
		"SKILL.md": "---\nname: old-format\ntargets: [cursor]\n---\n# Old format",
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

	// new-format (metadata.targets: [claude]) → only claude
	if !sb.IsSymlink(filepath.Join(claudePath, "new-format")) {
		t.Error("new-format should be in claude")
	}
	if sb.FileExists(filepath.Join(cursorPath, "new-format")) {
		t.Error("new-format should NOT be in cursor")
	}

	// old-format (targets: [cursor]) → only cursor
	if sb.FileExists(filepath.Join(claudePath, "old-format")) {
		t.Error("old-format should NOT be in claude")
	}
	if !sb.IsSymlink(filepath.Join(cursorPath, "old-format")) {
		t.Error("old-format should be in cursor")
	}
}

func TestSkillTargets_MetadataOverridesTopLevelInSync(t *testing.T) {
	sb := testutil.NewSandbox(t)
	defer sb.Cleanup()

	// Both top-level and metadata.targets present — metadata wins
	sb.CreateSkill("override-skill", map[string]string{
		"SKILL.md": "---\nname: override-skill\ntargets: [claude, cursor]\nmetadata:\n  targets: [claude]\n---\n# Metadata overrides",
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

	// metadata.targets: [claude] wins over top-level targets: [claude, cursor]
	if !sb.IsSymlink(filepath.Join(claudePath, "override-skill")) {
		t.Error("override-skill should be in claude (metadata wins)")
	}
	if sb.FileExists(filepath.Join(cursorPath, "override-skill")) {
		t.Error("override-skill should NOT be in cursor (metadata.targets only has claude)")
	}
}
