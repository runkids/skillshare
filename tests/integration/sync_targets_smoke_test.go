//go:build !online

package integration

import (
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	"skillshare/internal/config"
	"skillshare/internal/testutil"
)

// TestSync_AllGlobalTargets_SmokeTest verifies that sync creates correct
// symlinks for every target defined in targets.yaml. This catches path
// typos, misconfigured directory structures, and regressions when adding
// new targets.
func TestSync_AllGlobalTargets_SmokeTest(t *testing.T) {
	sb := testutil.NewSandbox(t)
	defer sb.Cleanup()

	sb.CreateSkill("smoke-skill", map[string]string{
		"SKILL.md": "# Smoke Test Skill",
	})

	// Load all global targets — ~ expands to sb.Home via $HOME
	targets := config.DefaultTargets()
	if len(targets) < 40 {
		t.Fatalf("expected 40+ targets from targets.yaml, got %d", len(targets))
	}

	// Sort for deterministic config and subtest order
	names := make([]string, 0, len(targets))
	for name := range targets {
		names = append(names, name)
	}
	sort.Strings(names)

	// Build config with every target
	var b strings.Builder
	b.WriteString("source: " + sb.SourcePath + "\nmode: merge\ntargets:\n")
	for _, name := range names {
		tc := targets[name]
		if err := os.MkdirAll(tc.Path, 0755); err != nil {
			t.Fatalf("create dir for %s (%s): %v", name, tc.Path, err)
		}
		b.WriteString("  " + name + ":\n    path: " + tc.Path + "\n")
	}
	sb.WriteConfig(b.String())

	result := sb.RunCLI("sync")
	result.AssertSuccess(t)

	// Verify symlink in every target directory
	for _, name := range names {
		tc := targets[name]
		t.Run(name, func(t *testing.T) {
			skillLink := filepath.Join(tc.Path, "smoke-skill")
			if !sb.IsSymlink(skillLink) {
				t.Errorf("symlink not created at %s", skillLink)
			}
			expected := filepath.Join(sb.SourcePath, "smoke-skill")
			if got := sb.SymlinkTarget(skillLink); got != expected {
				t.Errorf("symlink target = %q, want %q", got, expected)
			}
		})
	}

	t.Logf("smoke test verified %d targets", len(names))
}
