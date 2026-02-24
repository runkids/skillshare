//go:build !online

package integration

import (
	"path/filepath"
	"strings"
	"testing"

	"skillshare/internal/testutil"
)

func TestUninstall_ExistingSkill_Removes(t *testing.T) {
	sb := testutil.NewSandbox(t)
	defer sb.Cleanup()

	// Create existing skill in source
	sb.CreateSkill("my-skill", map[string]string{"SKILL.md": "# My Skill"})

	sb.WriteConfig(`source: ` + sb.SourcePath + `
targets: {}
`)

	// Uninstall with --force to skip confirmation
	result := sb.RunCLI("uninstall", "my-skill", "--force")

	result.AssertSuccess(t)
	result.AssertOutputContains(t, "Uninstalled")
	result.AssertOutputContains(t, "my-skill")

	// Verify skill was removed
	skillPath := filepath.Join(sb.SourcePath, "my-skill")
	if sb.FileExists(skillPath) {
		t.Error("skill should be removed after uninstall")
	}
}

func TestUninstall_NotFound_Errors(t *testing.T) {
	sb := testutil.NewSandbox(t)
	defer sb.Cleanup()

	sb.WriteConfig(`source: ` + sb.SourcePath + `
targets: {}
`)

	result := sb.RunCLI("uninstall", "nonexistent-skill", "--force")

	result.AssertFailure(t)
	result.AssertAnyOutputContains(t, "not found")
}

func TestUninstall_DryRun_NoChanges(t *testing.T) {
	sb := testutil.NewSandbox(t)
	defer sb.Cleanup()

	// Create existing skill in source
	sb.CreateSkill("dry-run-skill", map[string]string{"SKILL.md": "# Dry Run"})

	sb.WriteConfig(`source: ` + sb.SourcePath + `
targets: {}
`)

	result := sb.RunCLI("uninstall", "dry-run-skill", "--dry-run")

	result.AssertSuccess(t)
	result.AssertOutputContains(t, "dry-run")
	result.AssertOutputContains(t, "would move to trash")

	// Verify skill was NOT removed
	skillPath := filepath.Join(sb.SourcePath, "dry-run-skill")
	if !sb.FileExists(skillPath) {
		t.Error("skill should not be removed in dry-run mode")
	}
}

func TestUninstall_Force_SkipsConfirm(t *testing.T) {
	sb := testutil.NewSandbox(t)
	defer sb.Cleanup()

	// Create existing skill
	sb.CreateSkill("force-skill", map[string]string{"SKILL.md": "# Force"})

	sb.WriteConfig(`source: ` + sb.SourcePath + `
targets: {}
`)

	// Without --force, would wait for stdin (but RunCLI provides no input)
	// With --force, should complete immediately
	result := sb.RunCLI("uninstall", "force-skill", "--force")

	result.AssertSuccess(t)
	result.AssertOutputContains(t, "Uninstalled")

	// Verify skill was removed
	skillPath := filepath.Join(sb.SourcePath, "force-skill")
	if sb.FileExists(skillPath) {
		t.Error("skill should be removed with --force")
	}
}

func TestUninstall_Help_ShowsUsage(t *testing.T) {
	sb := testutil.NewSandbox(t)
	defer sb.Cleanup()

	result := sb.RunCLI("uninstall", "--help")

	result.AssertSuccess(t)
	result.AssertOutputContains(t, "Usage:")
	result.AssertOutputContains(t, "--force")
	result.AssertOutputContains(t, "--dry-run")
}

func TestUninstall_NoArgs_ShowsHelp(t *testing.T) {
	sb := testutil.NewSandbox(t)
	defer sb.Cleanup()

	sb.WriteConfig(`source: ` + sb.SourcePath + `
targets: {}
`)

	result := sb.RunCLI("uninstall")

	result.AssertFailure(t)
	result.AssertAnyOutputContains(t, "skill name or --group is required")
}

func TestUninstall_NestedSkill_ResolvesByBasename(t *testing.T) {
	sb := testutil.NewSandbox(t)
	defer sb.Cleanup()

	// Create nested skill in organizational folder
	sb.CreateSkill("frontend/react/react-best-practices", map[string]string{
		"SKILL.md": "# React Best Practices",
	})

	sb.WriteConfig(`source: ` + sb.SourcePath + `
targets: {}
`)

	// Uninstall by short name (basename only)
	result := sb.RunCLI("uninstall", "react-best-practices", "--force")
	result.AssertSuccess(t)
	result.AssertOutputContains(t, "Uninstalled")

	// Verify nested skill was removed
	skillPath := filepath.Join(sb.SourcePath, "frontend", "react", "react-best-practices")
	if sb.FileExists(skillPath) {
		t.Error("nested skill should be removed after uninstall by basename")
	}
}

func TestUninstall_NestedSkill_FullPathAlsoWorks(t *testing.T) {
	sb := testutil.NewSandbox(t)
	defer sb.Cleanup()

	sb.CreateSkill("backend/go-patterns", map[string]string{
		"SKILL.md": "# Go Patterns",
	})

	sb.WriteConfig(`source: ` + sb.SourcePath + `
targets: {}
`)

	// Uninstall by full nested path
	result := sb.RunCLI("uninstall", "backend/go-patterns", "--force")
	result.AssertSuccess(t)
	result.AssertOutputContains(t, "Uninstalled")
}

func TestUninstall_ShowsMetadata(t *testing.T) {
	sb := testutil.NewSandbox(t)
	defer sb.Cleanup()

	// Create skill with metadata (simulating installed skill)
	sb.CreateSkill("meta-skill", map[string]string{
		"SKILL.md": "# Meta Skill",
		".skillshare-meta.json": `{
  "source": "github.com/user/repo",
  "type": "github",
  "installed_at": "2024-01-15T10:30:00Z"
}`,
	})

	sb.WriteConfig(`source: ` + sb.SourcePath + `
targets: {}
`)

	result := sb.RunCLI("uninstall", "meta-skill", "--dry-run")

	result.AssertSuccess(t)
	result.AssertOutputContains(t, "github.com/user/repo")
}

// --- Multi-skill tests ---

func TestUninstall_MultipleSkills(t *testing.T) {
	sb := testutil.NewSandbox(t)
	defer sb.Cleanup()

	sb.CreateSkill("alpha", map[string]string{"SKILL.md": "# Alpha"})
	sb.CreateSkill("beta", map[string]string{"SKILL.md": "# Beta"})
	sb.CreateSkill("gamma", map[string]string{"SKILL.md": "# Gamma"})

	sb.WriteConfig(`source: ` + sb.SourcePath + `
targets: {}
`)

	result := sb.RunCLI("uninstall", "alpha", "beta", "gamma", "-f")
	result.AssertSuccess(t)

	for _, name := range []string{"alpha", "beta", "gamma"} {
		if sb.FileExists(filepath.Join(sb.SourcePath, name)) {
			t.Errorf("skill %s should be removed", name)
		}
	}
}

func TestUninstall_MultipleSkills_PartialNotFound(t *testing.T) {
	sb := testutil.NewSandbox(t)
	defer sb.Cleanup()

	sb.CreateSkill("exists-a", map[string]string{"SKILL.md": "# A"})
	sb.CreateSkill("exists-b", map[string]string{"SKILL.md": "# B"})

	sb.WriteConfig(`source: ` + sb.SourcePath + `
targets: {}
`)

	result := sb.RunCLI("uninstall", "exists-a", "nonexistent", "exists-b", "-f")
	result.AssertSuccess(t) // partial success = exit 0
	result.AssertAnyOutputContains(t, "not found")

	if sb.FileExists(filepath.Join(sb.SourcePath, "exists-a")) {
		t.Error("exists-a should be removed")
	}
	if sb.FileExists(filepath.Join(sb.SourcePath, "exists-b")) {
		t.Error("exists-b should be removed")
	}
}

func TestUninstall_MultipleSkills_AllNotFound(t *testing.T) {
	sb := testutil.NewSandbox(t)
	defer sb.Cleanup()

	sb.WriteConfig(`source: ` + sb.SourcePath + `
targets: {}
`)

	result := sb.RunCLI("uninstall", "x", "y", "z", "-f")
	result.AssertFailure(t)
}

func TestUninstall_MultipleSkills_DryRun(t *testing.T) {
	sb := testutil.NewSandbox(t)
	defer sb.Cleanup()

	sb.CreateSkill("dry-a", map[string]string{"SKILL.md": "# A"})
	sb.CreateSkill("dry-b", map[string]string{"SKILL.md": "# B"})

	sb.WriteConfig(`source: ` + sb.SourcePath + `
targets: {}
`)

	result := sb.RunCLI("uninstall", "dry-a", "dry-b", "-n")
	result.AssertSuccess(t)
	result.AssertOutputContains(t, "would move to trash")

	// Both should still exist
	if !sb.FileExists(filepath.Join(sb.SourcePath, "dry-a")) {
		t.Error("dry-a should not be removed in dry-run")
	}
	if !sb.FileExists(filepath.Join(sb.SourcePath, "dry-b")) {
		t.Error("dry-b should not be removed in dry-run")
	}
}

// --- Group tests ---

func TestUninstall_Group(t *testing.T) {
	sb := testutil.NewSandbox(t)
	defer sb.Cleanup()

	sb.CreateSkill("frontend/skill-a", map[string]string{"SKILL.md": "# A"})
	sb.CreateSkill("frontend/skill-b", map[string]string{"SKILL.md": "# B"})
	sb.CreateSkill("backend/skill-c", map[string]string{"SKILL.md": "# C"})

	sb.WriteConfig(`source: ` + sb.SourcePath + `
targets: {}
`)

	result := sb.RunCLI("uninstall", "--group", "frontend", "-f")
	result.AssertSuccess(t)

	if sb.FileExists(filepath.Join(sb.SourcePath, "frontend", "skill-a")) {
		t.Error("frontend/skill-a should be removed")
	}
	if sb.FileExists(filepath.Join(sb.SourcePath, "frontend", "skill-b")) {
		t.Error("frontend/skill-b should be removed")
	}
	// backend should be untouched
	if !sb.FileExists(filepath.Join(sb.SourcePath, "backend", "skill-c")) {
		t.Error("backend/skill-c should NOT be removed")
	}
}

func TestUninstall_Group_PrefixMatch(t *testing.T) {
	sb := testutil.NewSandbox(t)
	defer sb.Cleanup()

	sb.CreateSkill("frontend/react/hooks", map[string]string{"SKILL.md": "# Hooks"})
	sb.CreateSkill("frontend/vue/composables", map[string]string{"SKILL.md": "# Composables"})

	sb.WriteConfig(`source: ` + sb.SourcePath + `
targets: {}
`)

	result := sb.RunCLI("uninstall", "--group", "frontend", "-f")
	result.AssertSuccess(t)

	if sb.FileExists(filepath.Join(sb.SourcePath, "frontend", "react", "hooks")) {
		t.Error("frontend/react/hooks should be removed via prefix match")
	}
	if sb.FileExists(filepath.Join(sb.SourcePath, "frontend", "vue", "composables")) {
		t.Error("frontend/vue/composables should be removed via prefix match")
	}
}

func TestUninstall_Group_NotFound(t *testing.T) {
	sb := testutil.NewSandbox(t)
	defer sb.Cleanup()

	sb.WriteConfig(`source: ` + sb.SourcePath + `
targets: {}
`)

	result := sb.RunCLI("uninstall", "--group", "nonexistent", "-f")
	result.AssertFailure(t)
}

func TestUninstall_Group_DryRun(t *testing.T) {
	sb := testutil.NewSandbox(t)
	defer sb.Cleanup()

	sb.CreateSkill("frontend/dry-skill", map[string]string{"SKILL.md": "# Dry"})

	sb.WriteConfig(`source: ` + sb.SourcePath + `
targets: {}
`)

	result := sb.RunCLI("uninstall", "--group", "frontend", "-n")
	result.AssertSuccess(t)
	result.AssertOutputContains(t, "would move to trash")

	if !sb.FileExists(filepath.Join(sb.SourcePath, "frontend", "dry-skill")) {
		t.Error("dry-run should not remove skills")
	}
}

// --- Mixed tests ---

func TestUninstall_Mixed(t *testing.T) {
	sb := testutil.NewSandbox(t)
	defer sb.Cleanup()

	sb.CreateSkill("standalone", map[string]string{"SKILL.md": "# Standalone"})
	sb.CreateSkill("frontend/react-hooks", map[string]string{"SKILL.md": "# Hooks"})

	sb.WriteConfig(`source: ` + sb.SourcePath + `
targets: {}
`)

	result := sb.RunCLI("uninstall", "standalone", "-G", "frontend", "-f")
	result.AssertSuccess(t)

	if sb.FileExists(filepath.Join(sb.SourcePath, "standalone")) {
		t.Error("standalone should be removed")
	}
	if sb.FileExists(filepath.Join(sb.SourcePath, "frontend", "react-hooks")) {
		t.Error("frontend/react-hooks should be removed via group")
	}
}

func TestUninstall_Mixed_Dedup(t *testing.T) {
	sb := testutil.NewSandbox(t)
	defer sb.Cleanup()

	sb.CreateSkill("frontend/my-skill", map[string]string{"SKILL.md": "# My Skill"})

	sb.WriteConfig(`source: ` + sb.SourcePath + `
targets: {}
`)

	// Specify the same skill by name AND by group — should only uninstall once
	result := sb.RunCLI("uninstall", "frontend/my-skill", "-G", "frontend", "-f")
	result.AssertSuccess(t)

	if sb.FileExists(filepath.Join(sb.SourcePath, "frontend", "my-skill")) {
		t.Error("skill should be removed")
	}
}

// TestUninstall_GroupDir_RemovesConfigEntries verifies that uninstalling a group
// directory (not --group flag, but a directory name) removes all member skills
// from the config.yaml skills manifest.
func TestUninstall_GroupDir_RemovesConfigEntries(t *testing.T) {
	sb := testutil.NewSandbox(t)
	defer sb.Cleanup()

	sb.CreateSkill("mygroup/skill-a", map[string]string{"SKILL.md": "# A"})
	sb.CreateSkill("mygroup/skill-b", map[string]string{"SKILL.md": "# B"})
	sb.CreateSkill("other/skill-c", map[string]string{"SKILL.md": "# C"})

	sb.WriteConfig(`source: ` + sb.SourcePath + `
targets: {}
skills:
  - name: skill-a
    source: github.com/org/repo/skill-a
    group: mygroup
  - name: skill-b
    source: github.com/org/repo/skill-b
    group: mygroup
  - name: skill-c
    source: github.com/org/repo/skill-c
    group: other
`)

	result := sb.RunCLI("uninstall", "mygroup", "-f")
	result.AssertSuccess(t)

	// Group directory should be removed from disk
	if sb.FileExists(filepath.Join(sb.SourcePath, "mygroup")) {
		t.Error("mygroup directory should be removed")
	}

	// Registry should no longer contain mygroup skills
	registryPath := filepath.Join(filepath.Dir(sb.ConfigPath), "registry.yaml")
	registryContent := sb.ReadFile(registryPath)
	if strings.Contains(registryContent, "skill-a") {
		t.Error("registry should not contain skill-a after group uninstall")
	}
	if strings.Contains(registryContent, "skill-b") {
		t.Error("registry should not contain skill-b after group uninstall")
	}
	// other group should be untouched
	if !strings.Contains(registryContent, "skill-c") {
		t.Error("registry should still contain skill-c from other group")
	}
}

func TestUninstall_MultipleGroupDirs_UsesGroupCopy(t *testing.T) {
	sb := testutil.NewSandbox(t)
	defer sb.Cleanup()

	sb.CreateSkill("fix-review/checklist", map[string]string{"SKILL.md": "# Checklist"})
	sb.CreateSkill("security/audit", map[string]string{"SKILL.md": "# Audit"})
	sb.CreateSkill("other/keep", map[string]string{"SKILL.md": "# Keep"})

	sb.WriteConfig(`source: ` + sb.SourcePath + `
targets: {}
`)

	result := sb.RunCLI("uninstall", "fix-review", "security/", "--force")
	result.AssertSuccess(t)
	result.AssertOutputContains(t, "Uninstalling 2 group(s)")
	result.AssertAnyOutputContains(t, "Uninstalled 2 group(s)")
	result.AssertAnyOutputContains(t, "fix-review")
	result.AssertAnyOutputContains(t, "security")

	if sb.FileExists(filepath.Join(sb.SourcePath, "fix-review")) {
		t.Error("fix-review group should be removed")
	}
	if sb.FileExists(filepath.Join(sb.SourcePath, "security")) {
		t.Error("security group should be removed")
	}
	if !sb.FileExists(filepath.Join(sb.SourcePath, "other", "keep")) {
		t.Error("other/keep should NOT be removed")
	}
}

func TestUninstall_GroupDirWithTrailingSlash_RemovesConfigEntries(t *testing.T) {
	sb := testutil.NewSandbox(t)
	defer sb.Cleanup()

	sb.CreateSkill("security/scan", map[string]string{"SKILL.md": "# Scan"})
	sb.CreateSkill("security/hardening", map[string]string{"SKILL.md": "# Hardening"})
	sb.CreateSkill("other/keep", map[string]string{"SKILL.md": "# Keep"})

	sb.WriteConfig(`source: ` + sb.SourcePath + `
targets: {}
skills:
  - name: scan
    source: github.com/org/repo/scan
    group: security
  - name: hardening
    source: github.com/org/repo/hardening
    group: security
  - name: keep
    source: github.com/org/repo/keep
    group: other
`)

	result := sb.RunCLI("uninstall", "security/", "-f")
	result.AssertSuccess(t)

	registryPath := filepath.Join(filepath.Dir(sb.ConfigPath), "registry.yaml")
	registryContent := sb.ReadFile(registryPath)
	if strings.Contains(registryContent, "scan") {
		t.Error("registry should not contain scan after security/ uninstall")
	}
	if strings.Contains(registryContent, "hardening") {
		t.Error("registry should not contain hardening after security/ uninstall")
	}
	if !strings.Contains(registryContent, "keep") {
		t.Error("registry should still contain keep from other group")
	}
}

// --- --all tests ---

func TestUninstall_AllFlag(t *testing.T) {
	sb := testutil.NewSandbox(t)
	defer sb.Cleanup()

	sb.CreateSkill("alpha", map[string]string{"SKILL.md": "# Alpha"})
	sb.CreateSkill("beta", map[string]string{"SKILL.md": "# Beta"})
	sb.CreateSkill("gamma", map[string]string{"SKILL.md": "# Gamma"})

	sb.WriteConfig(`source: ` + sb.SourcePath + `
targets: {}
skills:
  - name: alpha
    source: github.com/org/alpha
  - name: beta
    source: github.com/org/beta
  - name: gamma
    source: github.com/org/gamma
`)

	result := sb.RunCLI("uninstall", "--all", "--force")
	result.AssertSuccess(t)

	for _, name := range []string{"alpha", "beta", "gamma"} {
		if sb.FileExists(filepath.Join(sb.SourcePath, name)) {
			t.Errorf("skill %s should be removed after --all", name)
		}
	}

	// Registry skills should be cleared
	registryPath := filepath.Join(filepath.Dir(sb.ConfigPath), "registry.yaml")
	registryContent := sb.ReadFile(registryPath)
	if strings.Contains(registryContent, "alpha") || strings.Contains(registryContent, "beta") || strings.Contains(registryContent, "gamma") {
		t.Error("registry should not contain any skills after --all uninstall")
	}
}

func TestUninstall_AllProject(t *testing.T) {
	sb := testutil.NewSandbox(t)
	defer sb.Cleanup()

	projectRoot := sb.SetupProjectDir("claude")
	projectSource := filepath.Join(projectRoot, ".skillshare", "skills")

	sb.CreateProjectSkill(projectRoot, "skill-a", map[string]string{"SKILL.md": "# A"})
	sb.CreateProjectSkill(projectRoot, "skill-b", map[string]string{"SKILL.md": "# B"})

	sb.WriteProjectConfig(projectRoot, `skills:
  - name: skill-a
    source: github.com/org/a
  - name: skill-b
    source: github.com/org/b
targets: {}
`)

	result := sb.RunCLIInDir(projectRoot, "uninstall", "--all", "--force", "-p")
	result.AssertSuccess(t)

	for _, name := range []string{"skill-a", "skill-b"} {
		if sb.FileExists(filepath.Join(projectSource, name)) {
			t.Errorf("skill %s should be removed after --all in project mode", name)
		}
	}
}

func TestUninstall_AllMutualExclusion(t *testing.T) {
	sb := testutil.NewSandbox(t)
	defer sb.Cleanup()

	sb.WriteConfig(`source: ` + sb.SourcePath + `
targets: {}
`)

	// --all + skill names
	result := sb.RunCLI("uninstall", "--all", "skill-a")
	result.AssertFailure(t)
	result.AssertAnyOutputContains(t, "--all cannot be used with skill names")

	// --all + --group
	result = sb.RunCLI("uninstall", "--all", "--group", "frontend")
	result.AssertFailure(t)
	result.AssertAnyOutputContains(t, "--all cannot be used with --group")
}

func TestUninstall_AllDryRun(t *testing.T) {
	sb := testutil.NewSandbox(t)
	defer sb.Cleanup()

	sb.CreateSkill("dry-all-a", map[string]string{"SKILL.md": "# A"})
	sb.CreateSkill("dry-all-b", map[string]string{"SKILL.md": "# B"})

	sb.WriteConfig(`source: ` + sb.SourcePath + `
targets: {}
`)

	result := sb.RunCLI("uninstall", "--all", "-n")
	result.AssertSuccess(t)
	result.AssertOutputContains(t, "would move to trash")

	// Skills should NOT be removed
	if !sb.FileExists(filepath.Join(sb.SourcePath, "dry-all-a")) {
		t.Error("dry-all-a should not be removed in dry-run")
	}
	if !sb.FileExists(filepath.Join(sb.SourcePath, "dry-all-b")) {
		t.Error("dry-all-b should not be removed in dry-run")
	}
}

func TestUninstall_ShellGlobDetection(t *testing.T) {
	sb := testutil.NewSandbox(t)
	defer sb.Cleanup()

	sb.WriteConfig(`source: ` + sb.SourcePath + `
targets: {}
`)

	// Simulate what happens when shell expands * in a Go project directory
	result := sb.RunCLI("uninstall", "README.md", "go.mod", "go.sum", "cmd", "internal", "--force")
	result.AssertFailure(t)
	result.AssertAnyOutputContains(t, "--all")
}
