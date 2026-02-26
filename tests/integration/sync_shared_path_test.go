//go:build !online

package integration

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"skillshare/internal/testutil"
)

// TestSync_SharedPath_NoRaceCondition verifies that multiple targets sharing
// the same resolved directory path do not cause "file exists" errors during
// parallel sync. This was a regression introduced when sync moved from
// sequential to parallel execution — concurrent goroutines would race on
// os.Symlink for the same destination path.
//
// The fix groups targets by resolved path so that same-path targets run
// sequentially within one goroutine.
func TestSync_SharedPath_NoRaceCondition(t *testing.T) {
	sb := testutil.NewSandbox(t)
	defer sb.Cleanup()

	// Create multiple skills to increase race window.
	for i := 0; i < 10; i++ {
		sb.CreateSkill(fmt.Sprintf("skill-%02d", i), map[string]string{
			"SKILL.md": fmt.Sprintf("---\nname: skill-%02d\n---\n# Skill %d", i, i),
		})
	}

	// All 4 targets share the same physical directory — mimics the real-world
	// universal/amp/kimi/replit pattern in targets.yaml.
	sharedDir := filepath.Join(sb.Home, ".config", "agents", "skills")
	if err := os.MkdirAll(sharedDir, 0755); err != nil {
		t.Fatalf("create shared dir: %v", err)
	}

	sb.WriteConfig(fmt.Sprintf(`source: %s
mode: merge
targets:
  target-a:
    path: %s
  target-b:
    path: %s
  target-c:
    path: %s
  target-d:
    path: %s
`, sb.SourcePath, sharedDir, sharedDir, sharedDir, sharedDir))

	result := sb.RunCLI("sync")
	result.AssertSuccess(t)

	// Verify all 10 skills exist as correct symlinks.
	for i := 0; i < 10; i++ {
		name := fmt.Sprintf("skill-%02d", i)
		link := filepath.Join(sharedDir, name)
		if !sb.IsSymlink(link) {
			t.Errorf("symlink not created for %s", name)
			continue
		}
		expected := filepath.Join(sb.SourcePath, name)
		if got := sb.SymlinkTarget(link); got != expected {
			t.Errorf("skill %s: symlink target = %q, want %q", name, got, expected)
		}
	}
}

// TestSync_SharedPath_WithSameFilters verifies that shared-path targets
// with identical include filters both succeed without race conditions.
func TestSync_SharedPath_WithSameFilters(t *testing.T) {
	sb := testutil.NewSandbox(t)
	defer sb.Cleanup()

	sb.CreateSkill("alpha", map[string]string{
		"SKILL.md": "---\nname: alpha\n---\n# Alpha",
	})
	sb.CreateSkill("beta", map[string]string{
		"SKILL.md": "---\nname: beta\n---\n# Beta",
	})

	sharedDir := filepath.Join(sb.Home, ".config", "shared-skills")
	if err := os.MkdirAll(sharedDir, 0755); err != nil {
		t.Fatalf("create shared dir: %v", err)
	}

	// Both targets share the same path and same filter — no prune conflict.
	sb.WriteConfig(fmt.Sprintf(`source: %s
mode: merge
targets:
  target-a:
    path: %s
    include: [alpha]
  target-b:
    path: %s
    include: [alpha]
`, sb.SourcePath, sharedDir, sharedDir))

	result := sb.RunCLI("sync")
	result.AssertSuccess(t)

	// alpha should be symlinked; beta excluded by both filters.
	if !sb.IsSymlink(filepath.Join(sharedDir, "alpha")) {
		t.Error("symlink not created for alpha")
	}
	if sb.IsSymlink(filepath.Join(sharedDir, "beta")) {
		t.Error("beta should not be symlinked (excluded by both targets)")
	}
}

// TestSync_SharedPath_RepeatedRuns verifies that running sync multiple times
// with shared-path targets is idempotent and never produces errors.
func TestSync_SharedPath_RepeatedRuns(t *testing.T) {
	sb := testutil.NewSandbox(t)
	defer sb.Cleanup()

	for i := 0; i < 5; i++ {
		sb.CreateSkill(fmt.Sprintf("skill-%d", i), map[string]string{
			"SKILL.md": fmt.Sprintf("---\nname: skill-%d\n---\n# Skill", i),
		})
	}

	sharedDir := filepath.Join(sb.Home, ".config", "agents", "skills")
	if err := os.MkdirAll(sharedDir, 0755); err != nil {
		t.Fatalf("create shared dir: %v", err)
	}

	sb.WriteConfig(fmt.Sprintf(`source: %s
mode: merge
targets:
  alpha:
    path: %s
  beta:
    path: %s
  gamma:
    path: %s
`, sb.SourcePath, sharedDir, sharedDir, sharedDir))

	// Run sync 3 times — must be idempotent.
	for run := 1; run <= 3; run++ {
		result := sb.RunCLI("sync")
		result.AssertSuccess(t)
	}

	// Verify final state.
	for i := 0; i < 5; i++ {
		link := filepath.Join(sharedDir, fmt.Sprintf("skill-%d", i))
		if !sb.IsSymlink(link) {
			t.Errorf("run 3: symlink not created for skill-%d", i)
		}
	}
}

// TestSync_SharedPath_ForceMode verifies that --force with shared-path targets
// does not fail when replacing local copies concurrently.
func TestSync_SharedPath_ForceMode(t *testing.T) {
	sb := testutil.NewSandbox(t)
	defer sb.Cleanup()

	sb.CreateSkill("my-skill", map[string]string{
		"SKILL.md": "---\nname: my-skill\n---\n# My Skill",
	})

	sharedDir := filepath.Join(sb.Home, ".config", "agents", "skills")
	if err := os.MkdirAll(sharedDir, 0755); err != nil {
		t.Fatalf("create shared dir: %v", err)
	}

	// Pre-create a local copy (directory, not symlink) to trigger --force path.
	localSkill := filepath.Join(sharedDir, "my-skill")
	if err := os.MkdirAll(localSkill, 0755); err != nil {
		t.Fatalf("create local skill dir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(localSkill, "SKILL.md"), []byte("local"), 0644); err != nil {
		t.Fatalf("write local SKILL.md: %v", err)
	}

	sb.WriteConfig(fmt.Sprintf(`source: %s
mode: merge
targets:
  target-a:
    path: %s
  target-b:
    path: %s
`, sb.SourcePath, sharedDir, sharedDir))

	result := sb.RunCLI("sync", "--force")
	result.AssertSuccess(t)

	// Should be a symlink now, not a directory.
	if !sb.IsSymlink(filepath.Join(sharedDir, "my-skill")) {
		t.Error("my-skill should be a symlink after --force sync")
	}
}

// TestSync_SharedPath_MixedWithUniquePaths ensures that targets with unique
// paths still run in parallel alongside shared-path target groups.
func TestSync_SharedPath_MixedWithUniquePaths(t *testing.T) {
	sb := testutil.NewSandbox(t)
	defer sb.Cleanup()

	sb.CreateSkill("the-skill", map[string]string{
		"SKILL.md": "---\nname: the-skill\n---\n# The Skill",
	})

	sharedDir := filepath.Join(sb.Home, ".config", "agents", "skills")
	uniqueDir1 := filepath.Join(sb.Home, ".cursor", "skills")
	uniqueDir2 := filepath.Join(sb.Home, ".claude", "skills")

	for _, d := range []string{sharedDir, uniqueDir1, uniqueDir2} {
		if err := os.MkdirAll(d, 0755); err != nil {
			t.Fatalf("create dir %s: %v", d, err)
		}
	}

	sb.WriteConfig(fmt.Sprintf(`source: %s
mode: merge
targets:
  shared-a:
    path: %s
  shared-b:
    path: %s
  unique-cursor:
    path: %s
  unique-claude:
    path: %s
`, sb.SourcePath, sharedDir, sharedDir, uniqueDir1, uniqueDir2))

	result := sb.RunCLI("sync")
	result.AssertSuccess(t)

	// Verify all three directories got the symlink.
	for _, dir := range []string{sharedDir, uniqueDir1, uniqueDir2} {
		link := filepath.Join(dir, "the-skill")
		if !sb.IsSymlink(link) {
			t.Errorf("symlink not created in %s", dir)
		}
	}
}

// TestSync_SharedPath_OutputShowsAllTargets verifies that sync output includes
// status lines for each target name, even when they share a physical path.
func TestSync_SharedPath_OutputShowsAllTargets(t *testing.T) {
	sb := testutil.NewSandbox(t)
	defer sb.Cleanup()

	sb.CreateSkill("test-skill", map[string]string{
		"SKILL.md": "---\nname: test-skill\n---\n# Test",
	})

	sharedDir := filepath.Join(sb.Home, ".config", "agents", "skills")
	if err := os.MkdirAll(sharedDir, 0755); err != nil {
		t.Fatalf("create shared dir: %v", err)
	}

	sb.WriteConfig(fmt.Sprintf(`source: %s
mode: merge
targets:
  universal:
    path: %s
  replit:
    path: %s
  kimi:
    path: %s
`, sb.SourcePath, sharedDir, sharedDir, sharedDir))

	result := sb.RunCLI("sync")
	result.AssertSuccess(t)

	// All target names should appear in output, even though they share a path.
	output := strings.ToLower(result.Output())
	for _, name := range []string{"universal", "replit", "kimi"} {
		if !strings.Contains(output, name) {
			t.Errorf("output should mention target %q, got:\n%s", name, result.Output())
		}
	}
}
