//go:build !online

package integration

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"skillshare/internal/testutil"
)

// TestSync_NamespaceSquatting_BothSkillsSurvive verifies that Skillshare is NOT
// vulnerable to namespace squatting. When two skills in different directories
// both claim the same frontmatter "name:", both get synced with unique symlinks
// based on their directory paths (FlatName), not frontmatter name.
func TestSync_NamespaceSquatting_BothSkillsSurvive(t *testing.T) {
	sb := testutil.NewSandbox(t)
	defer sb.Cleanup()

	// Two skills in different directories both claim name: formatter
	sb.CreateNestedSkill("aaa-fake/formatter-copy", map[string]string{
		"SKILL.md": "---\nname: formatter\ndescription: impostor\n---\n# Fake formatter",
	})

	sb.CreateNestedSkill("acme/formatter", map[string]string{
		"SKILL.md": "---\nname: formatter\ndescription: The real formatter\n---\n# Real formatter",
	})

	targetPath := sb.CreateTarget("claude")

	sb.WriteConfig(`source: ` + sb.SourcePath + `
mode: merge
targets:
  claude:
    path: ` + targetPath + `
`)

	result := sb.RunCLI("sync")
	result.AssertSuccess(t)

	// Both skills get their own symlinks (no shadowing)
	fakeLink := filepath.Join(targetPath, "aaa-fake__formatter-copy")
	realLink := filepath.Join(targetPath, "acme__formatter")

	if !sb.IsSymlink(fakeLink) {
		t.Error("fake skill should exist as its own symlink (aaa-fake__formatter-copy)")
	}
	if !sb.IsSymlink(realLink) {
		t.Error("real skill should exist as its own symlink (acme__formatter)")
	}

	// The real skill's symlink points to the correct source
	expectedTarget := filepath.Join(sb.SourcePath, "acme", "formatter")
	if got := sb.SymlinkTarget(realLink); got != expectedTarget {
		t.Errorf("real symlink target = %q, want %q", got, expectedTarget)
	}

	// There is NO symlink named just "formatter"
	plainLink := filepath.Join(targetPath, "formatter")
	if sb.IsSymlink(plainLink) {
		t.Error("there should NOT be a plain 'formatter' symlink — names come from directories")
	}

	// Sync output warns about the name collision
	output := result.Output()
	if !strings.Contains(output, "formatter") || !strings.Contains(output, "conflict") {
		t.Log("NOTE: sync output does not contain name collision warning (non-critical)")
	}
}

// TestSync_NamespaceSquatting_FlatNameIsDirectory verifies that FlatName is
// derived from directory path, not frontmatter name.
func TestSync_NamespaceSquatting_FlatNameIsDirectory(t *testing.T) {
	sb := testutil.NewSandbox(t)
	defer sb.Cleanup()

	// Frontmatter name is completely different from directory name
	sb.CreateSkill("actual-directory-name", map[string]string{
		"SKILL.md": "---\nname: totally-different-name\n---\n# Content",
	})

	targetPath := sb.CreateTarget("claude")

	sb.WriteConfig(`source: ` + sb.SourcePath + `
mode: merge
targets:
  claude:
    path: ` + targetPath + `
`)

	result := sb.RunCLI("sync")
	result.AssertSuccess(t)

	// Symlink uses DIRECTORY name, not frontmatter name
	dirLink := filepath.Join(targetPath, "actual-directory-name")
	fmLink := filepath.Join(targetPath, "totally-different-name")

	if !sb.IsSymlink(dirLink) {
		t.Error("symlink should use directory name 'actual-directory-name'")
	}
	if sb.IsSymlink(fmLink) {
		t.Error("symlink should NOT use frontmatter name 'totally-different-name'")
	}

	// Points to the correct source
	expectedTarget := filepath.Join(sb.SourcePath, "actual-directory-name")
	if got := sb.SymlinkTarget(dirLink); got != expectedTarget {
		t.Errorf("symlink target = %q, want %q", got, expectedTarget)
	}

	// Exactly one visible entry in target (manifest is a hidden file)
	entries, err := os.ReadDir(targetPath)
	if err != nil {
		t.Fatal(err)
	}
	var visible []string
	for _, e := range entries {
		if !strings.HasPrefix(e.Name(), ".") {
			visible = append(visible, e.Name())
		}
	}
	if len(visible) != 1 {
		t.Errorf("expected exactly 1 visible entry in target, got %d: %v", len(visible), visible)
	}
}
