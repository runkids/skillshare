//go:build !online

package integration

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"skillshare/internal/install"
	"skillshare/internal/testutil"
)

// TestSync_BothSourceAndTargetAreSymlinks verifies the combined scenario.
func TestSync_BothSourceAndTargetAreSymlinks(t *testing.T) {
	sb := testutil.NewSandbox(t)
	defer sb.Cleanup()

	// Setup symlinked source
	realSource := filepath.Join(sb.Root, "dotfiles", "skills")
	os.MkdirAll(realSource, 0755)
	skillDir := filepath.Join(realSource, "beta")
	os.MkdirAll(skillDir, 0755)
	os.WriteFile(filepath.Join(skillDir, "SKILL.md"), []byte("---\nname: beta\n---\n# Beta"), 0644)

	os.RemoveAll(sb.SourcePath)
	if err := os.Symlink(realSource, sb.SourcePath); err != nil {
		t.Fatal(err)
	}

	// Setup symlinked target
	realTarget := filepath.Join(sb.Root, "dotfiles", "cursor-skills")
	os.MkdirAll(realTarget, 0755)

	cursorSkillsDir := filepath.Join(sb.Home, ".cursor", "skills")
	os.RemoveAll(cursorSkillsDir)
	os.MkdirAll(filepath.Dir(cursorSkillsDir), 0755)
	if err := os.Symlink(realTarget, cursorSkillsDir); err != nil {
		t.Fatal(err)
	}

	sb.WriteConfig(`source: ` + sb.SourcePath + `
mode: merge
targets:
  cursor:
    path: ` + cursorSkillsDir + `
`)

	result := sb.RunCLI("sync")
	result.AssertSuccess(t)

	result.AssertRowContains(t, "cursor", "1 linked")
	if !sb.IsSymlink(filepath.Join(cursorSkillsDir, "beta")) {
		t.Fatal("skill should be a symlink through the symlinked target")
	}

	// Verify through all paths
	for _, path := range []string{
		filepath.Join(cursorSkillsDir, "beta"),
		filepath.Join(realTarget, "beta"),
	} {
		if _, err := os.Stat(path); err != nil {
			t.Errorf("skill not accessible at %q: %v", path, err)
		}
	}
}

// TestStatus_WithSymlinkedSource verifies `status` reports correctly
// when source is a symlink.
func TestStatus_WithSymlinkedSource(t *testing.T) {
	sb := testutil.NewSandbox(t)
	defer sb.Cleanup()

	realSource := filepath.Join(sb.Root, "dotfiles", "skills")
	os.MkdirAll(realSource, 0755)
	skillDir := filepath.Join(realSource, "delta")
	os.MkdirAll(skillDir, 0755)
	os.WriteFile(filepath.Join(skillDir, "SKILL.md"), []byte("---\nname: delta\n---\n# Delta"), 0644)

	os.RemoveAll(sb.SourcePath)
	if err := os.Symlink(realSource, sb.SourcePath); err != nil {
		t.Fatal(err)
	}

	targetPath := sb.CreateTarget("claude")
	sb.WriteConfig(`source: ` + sb.SourcePath + `
mode: merge
targets:
  claude:
    path: ` + targetPath + `
`)

	// Sync first
	sb.RunCLI("sync").AssertSuccess(t)

	// Status should show the skill linked
	result := sb.RunCLI("status")
	result.AssertSuccess(t)
	result.AssertOutputContains(t, "✓ 1 linked")
}

// TestList_WithSymlinkedSource verifies `list` works with symlinked source.
func TestList_WithSymlinkedSource(t *testing.T) {
	sb := testutil.NewSandbox(t)
	defer sb.Cleanup()

	realSource := filepath.Join(sb.Root, "dotfiles", "skills")
	os.MkdirAll(realSource, 0755)
	skillDir := filepath.Join(realSource, "epsilon")
	os.MkdirAll(skillDir, 0755)
	os.WriteFile(filepath.Join(skillDir, "SKILL.md"), []byte("---\nname: epsilon\n---\n# Epsilon"), 0644)

	os.RemoveAll(sb.SourcePath)
	if err := os.Symlink(realSource, sb.SourcePath); err != nil {
		t.Fatal(err)
	}

	sb.WriteConfig(`source: ` + sb.SourcePath + `
mode: merge
targets: {}
`)

	result := sb.RunCLI("list", "--no-tui")
	result.AssertSuccess(t)
	result.AssertAnyOutputContains(t, "epsilon")
}

// TestUninstallGroup_SymlinkedSource verifies that `uninstall --group` works
// when the source directory is a symlink.
func TestUninstallGroup_SymlinkedSource(t *testing.T) {
	sb := testutil.NewSandbox(t)
	defer sb.Cleanup()

	realSource := filepath.Join(sb.Root, "dotfiles", "skills")
	os.MkdirAll(realSource, 0755)
	groupDir := filepath.Join(realSource, "mygroup", "skill-a")
	os.MkdirAll(groupDir, 0755)
	os.WriteFile(filepath.Join(groupDir, "SKILL.md"), []byte("---\nname: skill-a\n---\n# A"), 0644)

	os.RemoveAll(sb.SourcePath)
	if err := os.Symlink(realSource, sb.SourcePath); err != nil {
		t.Fatal(err)
	}

	sb.WriteConfig(`source: ` + sb.SourcePath + "\ntargets: {}\n")

	result := sb.RunCLI("uninstall", "--group", "mygroup", "-f")
	result.AssertSuccess(t)

	if sb.FileExists(filepath.Join(realSource, "mygroup", "skill-a")) {
		t.Error("skill-a should be removed from real source")
	}
}

// TestUninstallGroup_ExternalSymlinkRejected verifies that `uninstall --group`
// refuses to operate when a group directory is a symlink pointing outside the
// source tree.
func TestUninstallGroup_ExternalSymlinkRejected(t *testing.T) {
	sb := testutil.NewSandbox(t)
	defer sb.Cleanup()

	sb.CreateSkill("legit-skill", map[string]string{
		"SKILL.md": "---\nname: legit\n---\n# Legit",
	})

	// Create an external directory with a skill
	externalDir := filepath.Join(sb.Root, "external-danger")
	os.MkdirAll(filepath.Join(externalDir, "victim"), 0755)
	os.WriteFile(filepath.Join(externalDir, "victim", "SKILL.md"),
		[]byte("---\nname: victim\n---\n# Victim"), 0644)

	// Symlink a group inside source to the external location
	os.Symlink(externalDir, filepath.Join(sb.SourcePath, "evil-group"))

	sb.WriteConfig(`source: ` + sb.SourcePath + "\ntargets: {}\n")

	result := sb.RunCLI("uninstall", "--group", "evil-group", "-f")
	result.AssertFailure(t)
	result.AssertAnyOutputContains(t, "outside source directory")

	// Verify external directory was NOT touched
	if !sb.FileExists(filepath.Join(externalDir, "victim", "SKILL.md")) {
		t.Error("external victim skill should NOT have been deleted")
	}
}

// TestUpdateGroup_ExternalSymlinkRejected verifies that `update --group`
// refuses to operate when a group directory is a symlink pointing outside the
// source tree.
func TestUpdateGroup_ExternalSymlinkRejected(t *testing.T) {
	sb := testutil.NewSandbox(t)
	defer sb.Cleanup()

	sb.CreateSkill("legit-skill", map[string]string{
		"SKILL.md": "---\nname: legit\n---\n# Legit",
	})

	// Create an external directory with a skill that has metadata
	externalDir := filepath.Join(sb.Root, "external-danger")
	os.MkdirAll(filepath.Join(externalDir, "victim"), 0755)
	os.WriteFile(filepath.Join(externalDir, "victim", "SKILL.md"),
		[]byte("---\nname: victim\n---\n# Victim"), 0644)
	os.WriteFile(filepath.Join(externalDir, install.MetadataFileName),
		[]byte(`{"version":1,"entries":{"victim":{"source":"github.com/example/victim","installed_at":"2025-01-01T00:00:00Z"}}}`), 0644)

	// Symlink a group inside source to the external location
	os.Symlink(externalDir, filepath.Join(sb.SourcePath, "evil-group"))

	sb.WriteConfig(`source: ` + sb.SourcePath + "\ntargets: {}\n")

	result := sb.RunCLI("update", "--group", "evil-group", "-n")
	result.AssertFailure(t)
	result.AssertAnyOutputContains(t, "outside source directory")
}

// TestUpdateAll_SymlinkedSource verifies that `update --all` discovers skills
// through a symlinked source directory.
func TestUpdateAll_SymlinkedSource(t *testing.T) {
	sb := testutil.NewSandbox(t)
	defer sb.Cleanup()

	realSource := filepath.Join(sb.Root, "dotfiles", "skills")
	os.MkdirAll(realSource, 0755)

	// Create a skill with metadata in centralized store
	skillDir := filepath.Join(realSource, "remote-skill")
	os.MkdirAll(skillDir, 0755)
	os.WriteFile(filepath.Join(skillDir, "SKILL.md"),
		[]byte("---\nname: remote-skill\n---\n# Remote"), 0644)
	os.WriteFile(filepath.Join(realSource, install.MetadataFileName),
		[]byte(`{"version":1,"entries":{"remote-skill":{"source":"github.com/example/remote","installed_at":"2025-01-01T00:00:00Z"}}}`), 0644)

	os.RemoveAll(sb.SourcePath)
	if err := os.Symlink(realSource, sb.SourcePath); err != nil {
		t.Fatal(err)
	}

	sb.WriteConfig(`source: ` + sb.SourcePath + "\ntargets: {}\n")

	result := sb.RunCLI("update", "--all", "-n")
	result.AssertSuccess(t)
	result.AssertAnyOutputContains(t, "remote-skill")
}

// TestDiff_SymlinkedTarget_InSyncAfterSync verifies diff follows a target
// reached through another tool's directory link, as sync does (#1070).
func TestDiff_SymlinkedTarget_InSyncAfterSync(t *testing.T) {
	for _, mode := range []string{"merge", "copy"} {
		t.Run(mode, func(t *testing.T) {
			sb := testutil.NewSandbox(t)
			defer sb.Cleanup()

			sb.CreateSkill("demo", map[string]string{"SKILL.md": "---\nname: demo\n---\n# Demo"})
			realTarget := filepath.Join(sb.Root, "dotfiles", "skills")
			os.MkdirAll(realTarget, 0755)
			alias := filepath.Join(sb.Root, "alias")
			if err := os.Symlink(realTarget, alias); err != nil {
				t.Skipf("symlinks unavailable: %v", err)
			}
			sb.WriteConfig(singleTargetConfig(sb.SourcePath, alias, mode))

			sb.RunCLI("sync").AssertSuccess(t)

			result := sb.RunCLI("diff", "--json")
			result.AssertSuccess(t)
			var out struct {
				Targets []struct {
					Mode   string `json:"mode"`
					Synced bool   `json:"synced"`
					Error  string `json:"error"`
				} `json:"targets"`
			}
			if err := json.Unmarshal([]byte(result.Stdout), &out); err != nil {
				t.Fatalf("parse diff JSON: %v\n%s", err, result.Stdout)
			}
			if len(out.Targets) != 1 {
				t.Fatalf("got %d targets, want 1:\n%s", len(out.Targets), result.Stdout)
			}
			if got := out.Targets[0]; got.Mode != mode || !got.Synced {
				t.Errorf("got mode %q synced %v (error %q), want mode %q in sync", got.Mode, got.Synced, got.Error, mode)
			}
		})
	}
}

// symlinkedTarget makes a target folder that is another tool's link to a real
// directory, and returns both paths.
func symlinkedTarget(t *testing.T, sb *testutil.Sandbox) (alias, real string) {
	t.Helper()
	real = filepath.Join(sb.Root, "dotfiles", "skills")
	os.MkdirAll(real, 0755)
	alias = filepath.Join(sb.Root, "alias")
	if err := os.Symlink(real, alias); err != nil {
		t.Skipf("symlinks unavailable: %v", err)
	}
	return alias, real
}

// TestTargetRemove_SymlinkedTarget_KeepsLink verifies target remove keeps
// another tool's link and replaces the skill links behind it with copies (#1070).
func TestTargetRemove_SymlinkedTarget_KeepsLink(t *testing.T) {
	sb := testutil.NewSandbox(t)
	defer sb.Cleanup()

	sb.CreateSkill("demo", map[string]string{"SKILL.md": "---\nname: demo\n---\n# Demo"})
	alias, real := symlinkedTarget(t, sb)
	sb.WriteConfig(singleTargetConfig(sb.SourcePath, alias, "merge"))
	sb.RunCLI("sync").AssertSuccess(t)

	sb.RunCLI("target", "remove", "claude").AssertSuccess(t)

	if !sb.IsSymlink(alias) {
		t.Error("the target link should stay a link")
	}
	if demo := filepath.Join(real, "demo"); sb.IsSymlink(demo) || !sb.FileExists(filepath.Join(demo, "SKILL.md")) {
		t.Error("demo should be a copy behind the link")
	}
}

// The project path once followed the link too; it must keep doing so.
func TestTargetRemove_Project_SymlinkedTarget_KeepsLink(t *testing.T) {
	sb := testutil.NewSandbox(t)
	defer sb.Cleanup()
	projectRoot := sb.SetupProjectDir("claude")
	sb.CreateProjectSkill(projectRoot, "demo", map[string]string{"SKILL.md": "# Demo"})
	real := filepath.Join(sb.Root, "dotfiles", "project-skills")
	os.MkdirAll(real, 0755)
	alias := filepath.Join(projectRoot, ".claude", "skills")
	os.RemoveAll(alias)
	if err := os.Symlink(real, alias); err != nil {
		t.Skipf("symlinks unavailable: %v", err)
	}
	sb.RunCLIInDir(projectRoot, "sync", "-p").AssertSuccess(t)

	sb.RunCLIInDir(projectRoot, "target", "remove", "claude", "-p").AssertSuccess(t)

	if !sb.IsSymlink(alias) {
		t.Error("the target link should stay a link")
	}
	if demo := filepath.Join(real, "demo"); sb.IsSymlink(demo) || !sb.FileExists(filepath.Join(demo, "SKILL.md")) {
		t.Error("demo should be a copy behind the link")
	}
}

// TestBackupRestore_SymlinkedTarget verifies backup reads, and restore writes,
// the directory behind another tool's link and keeps the link (#1070).
func TestBackupRestore_SymlinkedTarget(t *testing.T) {
	sb := testutil.NewSandbox(t)
	defer sb.Cleanup()

	alias, real := symlinkedTarget(t, sb)
	local := filepath.Join(real, "mine", "SKILL.md")
	os.MkdirAll(filepath.Dir(local), 0755)
	os.WriteFile(local, []byte("# Original"), 0644)
	sb.WriteConfig(singleTargetConfig(sb.SourcePath, alias, "merge"))

	sb.RunCLI("backup").AssertSuccess(t)
	os.WriteFile(local, []byte("# Modified"), 0644)
	sb.RunCLI("restore", "claude", "--force").AssertSuccess(t)

	if !sb.IsSymlink(alias) {
		t.Error("restore should keep the target link")
	}
	if content, err := os.ReadFile(local); err != nil || string(content) != "# Original" {
		t.Errorf("restored file = %q, %v; want the backed-up content", content, err)
	}
}
