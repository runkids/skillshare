//go:build !online

package integration

import (
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

func TestSync_IdempotentWithSymlinkedDirs(t *testing.T) {
	sb := testutil.NewSandbox(t)
	defer sb.Cleanup()

	// Setup symlinked source
	realSource := filepath.Join(sb.Root, "dotfiles", "skills")
	os.MkdirAll(realSource, 0755)
	skillDir := filepath.Join(realSource, "gamma")
	os.MkdirAll(skillDir, 0755)
	os.WriteFile(filepath.Join(skillDir, "SKILL.md"), []byte("---\nname: gamma\n---\n# Gamma"), 0644)

	os.RemoveAll(sb.SourcePath)
	if err := os.Symlink(realSource, sb.SourcePath); err != nil {
		t.Fatal(err)
	}

	realTarget := filepath.Join(sb.Root, "dotfiles", "claude-skills")
	if err := os.MkdirAll(realTarget, 0755); err != nil {
		t.Fatal(err)
	}
	targetPath := filepath.Join(sb.Home, ".claude", "skills")
	if err := os.Symlink(realTarget, targetPath); err != nil {
		t.Fatal(err)
	}
	sb.WriteConfig(`source: ` + sb.SourcePath + `
mode: merge
targets:
  claude:
    path: ` + targetPath + `
`)

	// First sync
	r1 := sb.RunCLI("sync")
	r1.AssertSuccess(t)

	// Second sync — should still succeed
	r2 := sb.RunCLI("sync")
	r2.AssertSuccess(t)

	// Verify link is still correct
	skillLink := filepath.Join(targetPath, "gamma")
	if !sb.IsSymlink(skillLink) {
		t.Fatal("skill should still be a symlink after double sync")
	}
	if _, err := os.Stat(skillLink); err != nil {
		t.Fatalf("symlink does not resolve after double sync: %v", err)
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

	// Status should show merged
	result := sb.RunCLI("status")
	result.AssertSuccess(t)
	result.AssertOutputContains(t, "merged")
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
