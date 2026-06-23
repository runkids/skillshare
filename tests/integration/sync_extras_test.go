//go:build !online

package integration

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"skillshare/internal/testutil"
)

func TestSyncExtras_MergeMode(t *testing.T) {
	sb := testutil.NewSandbox(t)
	defer sb.Cleanup()

	// Create a skill so config.Load() has a valid source
	sb.CreateSkill("placeholder", map[string]string{
		"SKILL.md": "# Placeholder",
	})
	targetPath := sb.CreateTarget("claude")

	// Create extras source: rules directory with 2 .md files
	sourceRoot := filepath.Dir(sb.SourcePath) // ~/.config/skillshare/
	rulesSource := filepath.Join(sourceRoot, "extras", "rules")
	os.MkdirAll(rulesSource, 0755)
	os.WriteFile(filepath.Join(rulesSource, "coding.md"), []byte("# Coding Rules"), 0644)
	os.WriteFile(filepath.Join(rulesSource, "testing.md"), []byte("# Testing Rules"), 0644)

	// Create extras target directory
	rulesTarget := filepath.Join(sb.Home, ".claude", "rules")
	os.MkdirAll(rulesTarget, 0755)

	sb.WriteConfig(`source: ` + sb.SourcePath + `
targets:
  claude:
    path: ` + targetPath + `
extras:
  - name: rules
    targets:
      - path: ` + rulesTarget + `
`)

	result := sb.RunCLI("sync", "extras")

	result.AssertSuccess(t)
	result.AssertAnyOutputContains(t, "Syncing extras")
	result.AssertAnyOutputContains(t, "2 files")

	// Verify files are symlinks
	codingLink := filepath.Join(rulesTarget, "coding.md")
	if !sb.IsSymlink(codingLink) {
		t.Error("coding.md should be a symlink in merge mode")
	}

	testingLink := filepath.Join(rulesTarget, "testing.md")
	if !sb.IsSymlink(testingLink) {
		t.Error("testing.md should be a symlink in merge mode")
	}

	// Verify symlink targets point to the source files
	expectedCoding := filepath.Join(rulesSource, "coding.md")
	if got := sb.SymlinkTarget(codingLink); got != expectedCoding {
		t.Errorf("coding.md symlink target = %q, want %q", got, expectedCoding)
	}
}

func TestSyncExtras_CopyMode(t *testing.T) {
	sb := testutil.NewSandbox(t)
	defer sb.Cleanup()

	sb.CreateSkill("placeholder", map[string]string{
		"SKILL.md": "# Placeholder",
	})
	targetPath := sb.CreateTarget("claude")

	// Create extras source
	sourceRoot := filepath.Dir(sb.SourcePath)
	rulesSource := filepath.Join(sourceRoot, "extras", "rules")
	os.MkdirAll(rulesSource, 0755)
	os.WriteFile(filepath.Join(rulesSource, "coding.md"), []byte("# Coding Rules"), 0644)

	// Create extras target
	rulesTarget := filepath.Join(sb.Home, ".cursor", "rules")
	os.MkdirAll(rulesTarget, 0755)

	sb.WriteConfig(`source: ` + sb.SourcePath + `
targets:
  claude:
    path: ` + targetPath + `
extras:
  - name: rules
    targets:
      - path: ` + rulesTarget + `
        mode: copy
`)

	result := sb.RunCLI("sync", "extras")

	result.AssertSuccess(t)
	result.AssertAnyOutputContains(t, "Syncing extras")

	// Verify file exists and is a real copy (not a symlink)
	copiedFile := filepath.Join(rulesTarget, "coding.md")
	if !sb.FileExists(copiedFile) {
		t.Fatal("coding.md should exist in target")
	}
	if sb.IsSymlink(copiedFile) {
		t.Error("coding.md should be a real copy, not a symlink, in copy mode")
	}

	// Verify content matches
	content := sb.ReadFile(copiedFile)
	if content != "# Coding Rules" {
		t.Errorf("copied file content = %q, want %q", content, "# Coding Rules")
	}
}

func TestSyncExtras_NoExtrasConfigured(t *testing.T) {
	sb := testutil.NewSandbox(t)
	defer sb.Cleanup()

	sb.CreateSkill("placeholder", map[string]string{
		"SKILL.md": "# Placeholder",
	})
	targetPath := sb.CreateTarget("claude")

	// Config with no extras section
	sb.WriteConfig(`source: ` + sb.SourcePath + `
targets:
  claude:
    path: ` + targetPath + `
`)

	result := sb.RunCLI("sync", "extras")

	result.AssertSuccess(t)
	result.AssertAnyOutputContains(t, "No extras configured")
}

func TestSyncExtras_PrunesOrphans(t *testing.T) {
	sb := testutil.NewSandbox(t)
	defer sb.Cleanup()

	sb.CreateSkill("placeholder", map[string]string{
		"SKILL.md": "# Placeholder",
	})
	targetPath := sb.CreateTarget("claude")

	// Create extras source with 1 file
	sourceRoot := filepath.Dir(sb.SourcePath)
	rulesSource := filepath.Join(sourceRoot, "extras", "rules")
	os.MkdirAll(rulesSource, 0755)
	os.WriteFile(filepath.Join(rulesSource, "keep.md"), []byte("# Keep"), 0644)

	// Create extras target with an orphan symlink pointing to non-existent source
	rulesTarget := filepath.Join(sb.Home, ".claude", "rules")
	os.MkdirAll(rulesTarget, 0755)

	orphanSource := filepath.Join(rulesSource, "deleted.md")
	orphanLink := filepath.Join(rulesTarget, "deleted.md")
	os.Symlink(orphanSource, orphanLink)

	sb.WriteConfig(`source: ` + sb.SourcePath + `
targets:
  claude:
    path: ` + targetPath + `
extras:
  - name: rules
    targets:
      - path: ` + rulesTarget + `
`)

	result := sb.RunCLI("sync", "extras")

	result.AssertSuccess(t)

	// Verify orphan is removed
	if sb.FileExists(orphanLink) {
		t.Error("orphan symlink should have been pruned")
	}

	// Verify real file is synced
	keepLink := filepath.Join(rulesTarget, "keep.md")
	if !sb.IsSymlink(keepLink) {
		t.Error("keep.md should be synced as a symlink")
	}
}

func TestSync_AllFlag(t *testing.T) {
	sb := testutil.NewSandbox(t)
	defer sb.Cleanup()

	// Setup skills
	sb.CreateSkill("my-skill", map[string]string{
		"SKILL.md": "# My Skill\n\nDescription here.",
	})
	targetPath := sb.CreateTarget("claude")

	// Setup extras
	sourceRoot := filepath.Dir(sb.SourcePath)
	rulesSource := filepath.Join(sourceRoot, "extras", "rules")
	os.MkdirAll(rulesSource, 0755)
	os.WriteFile(filepath.Join(rulesSource, "coding.md"), []byte("# Coding"), 0644)

	rulesTarget := filepath.Join(sb.Home, ".claude", "rules")
	os.MkdirAll(rulesTarget, 0755)

	sb.WriteConfig(`source: ` + sb.SourcePath + `
mode: merge
targets:
  claude:
    path: ` + targetPath + `
extras:
  - name: rules
    targets:
      - path: ` + rulesTarget + `
`)

	result := sb.RunCLI("sync", "--all")

	result.AssertSuccess(t)

	// Verify skill sync happened
	result.AssertAnyOutputContains(t, "merged")

	// Verify extras sync happened
	result.AssertAnyOutputContains(t, "Syncing extras")

	// Verify skill symlink
	if !sb.IsSymlink(filepath.Join(targetPath, "my-skill")) {
		t.Error("skill should be synced to target")
	}

	// Verify extras symlink
	if !sb.IsSymlink(filepath.Join(rulesTarget, "coding.md")) {
		t.Error("extras rule should be synced to target")
	}
}

// TestSyncExtras_ModeSwitchMergeToCopy verifies that switching mode from merge
// to copy and re-syncing replaces symlinks with copies without needing --force.
func TestSyncExtras_ModeSwitchMergeToCopy(t *testing.T) {
	sb := testutil.NewSandbox(t)
	defer sb.Cleanup()

	sb.CreateSkill("placeholder", map[string]string{
		"SKILL.md": "# Placeholder",
	})
	targetPath := sb.CreateTarget("claude")

	// Create extras source
	sourceRoot := filepath.Dir(sb.SourcePath)
	rulesSource := filepath.Join(sourceRoot, "extras", "rules")
	os.MkdirAll(rulesSource, 0755)
	os.WriteFile(filepath.Join(rulesSource, "coding.md"), []byte("# Coding Rules"), 0644)

	rulesTarget := filepath.Join(sb.Home, ".claude", "rules")
	os.MkdirAll(rulesTarget, 0755)

	// Step 1: Sync with merge mode (creates symlinks)
	sb.WriteConfig(`source: ` + sb.SourcePath + `
targets:
  claude:
    path: ` + targetPath + `
extras:
  - name: rules
    targets:
      - path: ` + rulesTarget + `
`)

	result := sb.RunCLI("sync", "extras")
	result.AssertSuccess(t)

	codingFile := filepath.Join(rulesTarget, "coding.md")
	if !sb.IsSymlink(codingFile) {
		t.Fatal("coding.md should be a symlink after merge sync")
	}

	// Step 2: Change mode to copy and re-sync
	sb.WriteConfig(`source: ` + sb.SourcePath + `
targets:
  claude:
    path: ` + targetPath + `
extras:
  - name: rules
    targets:
      - path: ` + rulesTarget + `
        mode: copy
`)

	result = sb.RunCLI("sync", "extras")
	result.AssertSuccess(t)

	// Verify: file should now be a regular copy, not a symlink
	if sb.IsSymlink(codingFile) {
		t.Error("coding.md should be a regular file after switching to copy mode, but is still a symlink")
	}
	if !sb.FileExists(codingFile) {
		t.Fatal("coding.md should exist in target after copy sync")
	}

	content := sb.ReadFile(codingFile)
	if content != "# Coding Rules" {
		t.Errorf("copied file content = %q, want %q", content, "# Coding Rules")
	}
}

// TestSyncExtras_ModeSwitchCopyToMerge verifies that switching mode from copy
// to merge and re-syncing replaces copies with symlinks (requires --force since
// regular files are not auto-replaced).
func TestSyncExtras_ModeSwitchCopyToMerge(t *testing.T) {
	sb := testutil.NewSandbox(t)
	defer sb.Cleanup()

	sb.CreateSkill("placeholder", map[string]string{
		"SKILL.md": "# Placeholder",
	})
	targetPath := sb.CreateTarget("claude")

	// Create extras source
	sourceRoot := filepath.Dir(sb.SourcePath)
	rulesSource := filepath.Join(sourceRoot, "extras", "rules")
	os.MkdirAll(rulesSource, 0755)
	os.WriteFile(filepath.Join(rulesSource, "coding.md"), []byte("# Coding Rules"), 0644)

	rulesTarget := filepath.Join(sb.Home, ".claude", "rules")
	os.MkdirAll(rulesTarget, 0755)

	// Step 1: Sync with copy mode
	sb.WriteConfig(`source: ` + sb.SourcePath + `
targets:
  claude:
    path: ` + targetPath + `
extras:
  - name: rules
    targets:
      - path: ` + rulesTarget + `
        mode: copy
`)

	result := sb.RunCLI("sync", "extras")
	result.AssertSuccess(t)

	codingFile := filepath.Join(rulesTarget, "coding.md")
	if sb.IsSymlink(codingFile) {
		t.Fatal("coding.md should be a regular file after copy sync")
	}

	// Step 2: Change mode to merge and sync WITHOUT --force (should skip)
	sb.WriteConfig(`source: ` + sb.SourcePath + `
targets:
  claude:
    path: ` + targetPath + `
extras:
  - name: rules
    targets:
      - path: ` + rulesTarget + `
`)

	result = sb.RunCLI("sync", "extras")
	result.AssertSuccess(t)

	// File should still be a regular file (skipped because no --force)
	if sb.IsSymlink(codingFile) {
		t.Error("coding.md should still be a regular file without --force")
	}

	// Step 3: Sync WITH --force (should replace with symlink)
	result = sb.RunCLI("sync", "extras", "--force")
	result.AssertSuccess(t)

	if !sb.IsSymlink(codingFile) {
		t.Error("coding.md should be a symlink after --force sync with merge mode")
	}

	expectedTarget := filepath.Join(rulesSource, "coding.md")
	if got := sb.SymlinkTarget(codingFile); got != expectedTarget {
		t.Errorf("symlink target = %q, want %q", got, expectedTarget)
	}
}

func TestSyncExtras_SourceNotExist(t *testing.T) {
	sb := testutil.NewSandbox(t)
	defer sb.Cleanup()

	sb.CreateSkill("placeholder", map[string]string{
		"SKILL.md": "# Placeholder",
	})
	targetPath := sb.CreateTarget("claude")

	// Config with extras pointing to non-existent source directory
	// The source name "nonexistent" resolves to ~/.config/skillshare/nonexistent
	sb.WriteConfig(`source: ` + sb.SourcePath + `
targets:
  claude:
    path: ` + targetPath + `
extras:
  - name: nonexistent
    targets:
      - path: ` + filepath.Join(sb.Home, ".claude", "rules") + `
`)

	result := sb.RunCLI("sync", "extras")

	result.AssertSuccess(t)
	// Sync auto-creates missing extras source directories (same as target dirs)
	result.AssertAnyOutputContains(t, "Created source directory")
}

func TestSyncExtras_FlattenMerge(t *testing.T) {
	sb := testutil.NewSandbox(t)
	defer sb.Cleanup()

	sb.CreateSkill("placeholder", map[string]string{
		"SKILL.md": "# Placeholder",
	})
	targetPath := sb.CreateTarget("claude")

	sourceRoot := filepath.Dir(sb.SourcePath)
	agentsSource := filepath.Join(sourceRoot, "extras", "agents")
	os.MkdirAll(filepath.Join(agentsSource, "curriculum"), 0755)
	os.MkdirAll(filepath.Join(agentsSource, "software"), 0755)
	os.WriteFile(filepath.Join(agentsSource, "curriculum", "tactician.md"), []byte("# Tactician"), 0644)
	os.WriteFile(filepath.Join(agentsSource, "software", "implementer.md"), []byte("# Implementer"), 0644)
	os.WriteFile(filepath.Join(agentsSource, "reviewer.md"), []byte("# Reviewer"), 0644)

	agentsTarget := filepath.Join(sb.Home, ".claude", "agents")
	os.MkdirAll(agentsTarget, 0755)

	sb.WriteConfig(`source: ` + sb.SourcePath + `
targets:
  claude:
    path: ` + targetPath + `
extras:
  - name: agents
    targets:
      - path: ` + agentsTarget + `
        flatten: true
`)

	result := sb.RunCLI("sync", "extras")
	result.AssertSuccess(t)
	result.AssertAnyOutputContains(t, "3 files")

	for _, name := range []string{"tactician.md", "implementer.md", "reviewer.md"} {
		if !sb.IsSymlink(filepath.Join(agentsTarget, name)) {
			t.Errorf("%s should be a flat symlink in target", name)
		}
	}
	if _, err := os.Stat(filepath.Join(agentsTarget, "curriculum")); !os.IsNotExist(err) {
		t.Error("curriculum/ should not exist in target")
	}
}

func TestSyncExtras_FlattenCollision(t *testing.T) {
	sb := testutil.NewSandbox(t)
	defer sb.Cleanup()

	sb.CreateSkill("placeholder", map[string]string{
		"SKILL.md": "# Placeholder",
	})
	targetPath := sb.CreateTarget("claude")

	sourceRoot := filepath.Dir(sb.SourcePath)
	agentsSource := filepath.Join(sourceRoot, "extras", "agents")
	os.MkdirAll(filepath.Join(agentsSource, "team-a"), 0755)
	os.MkdirAll(filepath.Join(agentsSource, "team-b"), 0755)
	os.WriteFile(filepath.Join(agentsSource, "team-a", "agent.md"), []byte("# Team A"), 0644)
	os.WriteFile(filepath.Join(agentsSource, "team-b", "agent.md"), []byte("# Team B"), 0644)

	agentsTarget := filepath.Join(sb.Home, ".claude", "agents")
	os.MkdirAll(agentsTarget, 0755)

	sb.WriteConfig(`source: ` + sb.SourcePath + `
targets:
  claude:
    path: ` + targetPath + `
extras:
  - name: agents
    targets:
      - path: ` + agentsTarget + `
        flatten: true
`)

	result := sb.RunCLI("sync", "extras")
	result.AssertSuccess(t)
	result.AssertAnyOutputContains(t, "flatten conflict")

	if !sb.IsSymlink(filepath.Join(agentsTarget, "agent.md")) {
		t.Error("agent.md should be a symlink")
	}
}

func TestSyncExtras_FlattenConfigRoundTrip(t *testing.T) {
	sb := testutil.NewSandbox(t)
	defer sb.Cleanup()

	sb.CreateSkill("placeholder", map[string]string{
		"SKILL.md": "# Placeholder",
	})
	targetPath := sb.CreateTarget("claude")

	agentsTarget := filepath.Join(sb.Home, ".claude", "agents")

	sb.WriteConfig(`source: ` + sb.SourcePath + `
targets:
  claude:
    path: ` + targetPath + `
extras:
  - name: agents
    targets:
      - path: ` + agentsTarget + `
        flatten: true
`)

	content := sb.ReadFile(sb.ConfigPath)
	if !strings.Contains(content, "flatten: true") {
		t.Error("config should contain flatten: true")
	}
}

// --- Extras "agents" overlap with agents sync ---

func TestSyncExtras_AgentsOverlap_Skipped(t *testing.T) {
	sb := testutil.NewSandbox(t)
	defer sb.Cleanup()

	sb.CreateSkill("placeholder", map[string]string{
		"SKILL.md": "# Placeholder",
	})
	targetPath := sb.CreateTarget("claude")

	// Create agents source with a real agent (the regular agents sync system)
	agentsDir := filepath.Join(filepath.Dir(sb.SourcePath), "agents")
	os.MkdirAll(agentsDir, 0755)
	os.WriteFile(filepath.Join(agentsDir, "helper.md"), []byte("# Helper Agent"), 0644)

	// Create extras source for "agents" with a file
	sourceRoot := filepath.Dir(sb.SourcePath)
	extrasAgentsSource := filepath.Join(sourceRoot, "extras", "agents")
	os.MkdirAll(extrasAgentsSource, 0755)
	os.WriteFile(filepath.Join(extrasAgentsSource, "extra-agent.md"), []byte("# Extra Agent"), 0644)

	claudeAgents := filepath.Join(sb.Home, ".claude", "agents")
	os.MkdirAll(claudeAgents, 0755)

	sb.WriteConfig(`source: ` + sb.SourcePath + `
targets:
  claude:
    skills:
      path: ` + targetPath + `
    agents:
      path: ` + claudeAgents + `
extras:
  - name: agents
    targets:
      - path: ` + claudeAgents + `
`)

	result := sb.RunCLI("sync", "extras")
	result.AssertSuccess(t)
	result.AssertAnyOutputContains(t, "Skipping extras")
	result.AssertAnyOutputContains(t, "already managed by agents sync")
}

func TestSyncExtras_AgentsOverlap_NoAgentsSource(t *testing.T) {
	sb := testutil.NewSandbox(t)
	defer sb.Cleanup()

	sb.CreateSkill("placeholder", map[string]string{
		"SKILL.md": "# Placeholder",
	})
	targetPath := sb.CreateTarget("claude")

	// NO agents source directory — extras "agents" should sync normally
	sourceRoot := filepath.Dir(sb.SourcePath)
	extrasAgentsSource := filepath.Join(sourceRoot, "extras", "agents")
	os.MkdirAll(extrasAgentsSource, 0755)
	os.WriteFile(filepath.Join(extrasAgentsSource, "extra-agent.md"), []byte("# Extra Agent"), 0644)

	claudeAgents := filepath.Join(sb.Home, ".claude", "agents")
	os.MkdirAll(claudeAgents, 0755)

	sb.WriteConfig(`source: ` + sb.SourcePath + `
targets:
  claude:
    skills:
      path: ` + targetPath + `
    agents:
      path: ` + claudeAgents + `
extras:
  - name: agents
    targets:
      - path: ` + claudeAgents + `
`)

	result := sb.RunCLI("sync", "extras")
	result.AssertSuccess(t)
	result.AssertOutputNotContains(t, "Skipping extras")

	// Extras agent file should be synced normally
	if !sb.FileExists(filepath.Join(claudeAgents, "extra-agent.md")) {
		t.Error("extra-agent.md should be synced when no agents source exists")
	}
}

func TestSyncExtras_AgentsOverlap_PartialSkip(t *testing.T) {
	sb := testutil.NewSandbox(t)
	defer sb.Cleanup()

	sb.CreateSkill("placeholder", map[string]string{
		"SKILL.md": "# Placeholder",
	})
	targetPath := sb.CreateTarget("claude")

	// Create agents source
	agentsDir := filepath.Join(filepath.Dir(sb.SourcePath), "agents")
	os.MkdirAll(agentsDir, 0755)
	os.WriteFile(filepath.Join(agentsDir, "helper.md"), []byte("# Helper Agent"), 0644)

	// Create extras "agents" source
	sourceRoot := filepath.Dir(sb.SourcePath)
	extrasAgentsSource := filepath.Join(sourceRoot, "extras", "agents")
	os.MkdirAll(extrasAgentsSource, 0755)
	os.WriteFile(filepath.Join(extrasAgentsSource, "extra-agent.md"), []byte("# Extra Agent"), 0644)

	claudeAgents := filepath.Join(sb.Home, ".claude", "agents")
	customTarget := filepath.Join(sb.Home, "custom-agents")
	os.MkdirAll(claudeAgents, 0755)
	os.MkdirAll(customTarget, 0755)

	// Two targets: one overlaps with agents, one doesn't
	sb.WriteConfig(`source: ` + sb.SourcePath + `
targets:
  claude:
    skills:
      path: ` + targetPath + `
    agents:
      path: ` + claudeAgents + `
extras:
  - name: agents
    targets:
      - path: ` + claudeAgents + `
      - path: ` + customTarget + `
`)

	result := sb.RunCLI("sync", "extras")
	result.AssertSuccess(t)

	// Overlapping target should be skipped
	result.AssertAnyOutputContains(t, "Skipping extras")

	// Non-overlapping target should be synced
	if !sb.FileExists(filepath.Join(customTarget, "extra-agent.md")) {
		t.Error("extra-agent.md should be synced to non-overlapping target")
	}
}

func TestSyncExtras_AgentsOverlap_NonAgentsNameNotSkipped(t *testing.T) {
	sb := testutil.NewSandbox(t)
	defer sb.Cleanup()

	sb.CreateSkill("placeholder", map[string]string{
		"SKILL.md": "# Placeholder",
	})
	targetPath := sb.CreateTarget("claude")

	// Create agents source (real agents system active)
	agentsDir := filepath.Join(filepath.Dir(sb.SourcePath), "agents")
	os.MkdirAll(agentsDir, 0755)
	os.WriteFile(filepath.Join(agentsDir, "helper.md"), []byte("# Helper Agent"), 0644)

	// Create extras "rules" that targets the agents directory
	sourceRoot := filepath.Dir(sb.SourcePath)
	rulesSource := filepath.Join(sourceRoot, "extras", "rules")
	os.MkdirAll(rulesSource, 0755)
	os.WriteFile(filepath.Join(rulesSource, "rule.md"), []byte("# Rule"), 0644)

	claudeAgents := filepath.Join(sb.Home, ".claude", "agents")
	os.MkdirAll(claudeAgents, 0755)

	// extras "rules" targets the same path as agents — should NOT be skipped (name != "agents")
	sb.WriteConfig(`source: ` + sb.SourcePath + `
targets:
  claude:
    skills:
      path: ` + targetPath + `
    agents:
      path: ` + claudeAgents + `
extras:
  - name: rules
    targets:
      - path: ` + claudeAgents + `
`)

	result := sb.RunCLI("sync", "extras")
	result.AssertSuccess(t)
	result.AssertOutputNotContains(t, "Skipping extras")

	if !sb.FileExists(filepath.Join(claudeAgents, "rule.md")) {
		t.Error("rule.md should be synced — extras named 'rules' should not be affected by agents overlap")
	}
}

func TestSyncExtras_AgentsOverlap_NoTargetOverlap(t *testing.T) {
	sb := testutil.NewSandbox(t)
	defer sb.Cleanup()

	sb.CreateSkill("placeholder", map[string]string{
		"SKILL.md": "# Placeholder",
	})
	targetPath := sb.CreateTarget("claude")

	// Create agents source
	agentsDir := filepath.Join(filepath.Dir(sb.SourcePath), "agents")
	os.MkdirAll(agentsDir, 0755)
	os.WriteFile(filepath.Join(agentsDir, "helper.md"), []byte("# Helper Agent"), 0644)

	// Create extras "agents" source targeting a DIFFERENT path
	sourceRoot := filepath.Dir(sb.SourcePath)
	extrasAgentsSource := filepath.Join(sourceRoot, "extras", "agents")
	os.MkdirAll(extrasAgentsSource, 0755)
	os.WriteFile(filepath.Join(extrasAgentsSource, "extra-agent.md"), []byte("# Extra Agent"), 0644)

	claudeAgents := filepath.Join(sb.Home, ".claude", "agents")
	customTarget := filepath.Join(sb.Home, "my-agents")
	os.MkdirAll(customTarget, 0755)

	sb.WriteConfig(`source: ` + sb.SourcePath + `
targets:
  claude:
    skills:
      path: ` + targetPath + `
    agents:
      path: ` + claudeAgents + `
extras:
  - name: agents
    targets:
      - path: ` + customTarget + `
`)

	result := sb.RunCLI("sync", "extras")
	result.AssertSuccess(t)
	result.AssertOutputNotContains(t, "Skipping extras")

	if !sb.FileExists(filepath.Join(customTarget, "extra-agent.md")) {
		t.Error("extra-agent.md should be synced to non-overlapping target")
	}
}

func TestSyncAll_AgentsExtrasOverlap_WarnsAndPreservesAgents_Global(t *testing.T) {
	sb := testutil.NewSandbox(t)
	defer sb.Cleanup()

	sb.CreateSkill("placeholder", map[string]string{
		"SKILL.md": "# Placeholder",
	})
	targetPath := sb.CreateTarget("claude")

	agentsDir := filepath.Join(filepath.Dir(sb.SourcePath), "agents")
	os.MkdirAll(agentsDir, 0755)
	os.WriteFile(filepath.Join(agentsDir, "demo.md"), []byte("# Demo Agent"), 0644)

	sourceRoot := filepath.Dir(sb.SourcePath)
	extrasAgentsSource := filepath.Join(sourceRoot, "extras", "agents")
	os.MkdirAll(extrasAgentsSource, 0755)
	os.WriteFile(filepath.Join(extrasAgentsSource, "extra-agent.md"), []byte("# Extra Agent"), 0644)

	claudeAgents := filepath.Join(sb.Home, ".claude", "agents")
	os.MkdirAll(claudeAgents, 0755)

	sb.WriteConfig(`source: ` + sb.SourcePath + `
targets:
  claude:
    path: ` + targetPath + `
extras:
  - name: agents
    targets:
      - path: ` + claudeAgents + `
`)

	result := sb.RunCLI("sync", "--all")
	result.AssertSuccess(t)
	result.AssertAnyOutputContains(t, "Skipping extras")
	result.AssertAnyOutputContains(t, "already managed by agents sync")

	if !sb.IsSymlink(filepath.Join(claudeAgents, "demo.md")) {
		t.Error("demo agent should remain synced in global mode")
	}
	if sb.FileExists(filepath.Join(claudeAgents, "extra-agent.md")) {
		t.Error("extras agent file should not be synced when target overlaps agents sync")
	}
}

func TestSyncExtras_ProjectUsesRelativeSymlinks(t *testing.T) {
	sb := testutil.NewSandbox(t)
	defer sb.Cleanup()

	projectRoot := sb.SetupProjectDir("claude")
	rulesSource := filepath.Join(projectRoot, ".skillshare", "extras", "rules")
	if err := os.MkdirAll(rulesSource, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(rulesSource, "rule.md"), []byte("# Rule"), 0644); err != nil {
		t.Fatal(err)
	}

	sb.WriteProjectConfig(projectRoot, `targets:
  - claude
extras:
  - name: rules
    targets:
      - path: .cursor/rules
`)

	result := sb.RunCLIInDir(projectRoot, "sync", "extras", "-p")
	result.AssertSuccess(t)

	link := filepath.Join(projectRoot, ".cursor", "rules", "rule.md")
	if !sb.IsSymlink(link) {
		t.Fatal("extra file should be a symlink")
	}
	if target := sb.SymlinkTarget(link); filepath.IsAbs(target) {
		t.Fatalf("project-mode extra symlink should be relative, got %q", target)
	}
}

func TestSyncAll_AgentsExtrasOverlap_WarnsAndPreservesAgents_Project(t *testing.T) {
	sb := testutil.NewSandbox(t)
	defer sb.Cleanup()

	projectRoot := sb.SetupProjectDir("claude", "cursor")

	projectAgents := filepath.Join(projectRoot, ".skillshare", "agents")
	os.MkdirAll(projectAgents, 0755)
	os.WriteFile(filepath.Join(projectAgents, "demo.md"), []byte("# Demo Agent"), 0644)

	projectExtrasAgents := filepath.Join(projectRoot, ".skillshare", "extras", "agents")
	os.MkdirAll(projectExtrasAgents, 0755)
	os.WriteFile(filepath.Join(projectExtrasAgents, "extra-agent.md"), []byte("# Extra Agent"), 0644)

	claudeAgents := filepath.Join(projectRoot, ".claude", "agents")
	cursorAgents := filepath.Join(projectRoot, ".cursor", "agents")
	os.MkdirAll(claudeAgents, 0755)
	os.MkdirAll(cursorAgents, 0755)

	sb.WriteProjectConfig(projectRoot, `targets:
  - claude
  - cursor
extras:
  - name: agents
    targets:
      - path: .claude/agents
      - path: .cursor/agents
`)

	result := sb.RunCLIInDir(projectRoot, "sync", "--all", "-p")
	result.AssertSuccess(t)
	result.AssertAnyOutputContains(t, "Skipping extras")
	result.AssertAnyOutputContains(t, "already managed by agents sync")

	if !sb.IsSymlink(filepath.Join(claudeAgents, "demo.md")) {
		t.Error("claude demo agent should remain synced in project mode")
	}
	if !sb.IsSymlink(filepath.Join(cursorAgents, "demo.md")) {
		t.Error("cursor demo agent should remain synced in project mode")
	}
	if sb.FileExists(filepath.Join(claudeAgents, "extra-agent.md")) {
		t.Error("project extras agent file should not be synced to claude when target overlaps agents sync")
	}
	if sb.FileExists(filepath.Join(cursorAgents, "extra-agent.md")) {
		t.Error("project extras agent file should not be synced to cursor when target overlaps agents sync")
	}
}
