//go:build !online

package integration

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"skillshare/internal/testutil"
)

func TestGitignoreProject_InstallAddsEntry(t *testing.T) {
	sb := testutil.NewSandbox(t)
	defer sb.Cleanup()
	projectRoot := sb.SetupProjectDir("claude")

	sourceSkill := filepath.Join(sb.Root, "ext")
	os.MkdirAll(sourceSkill, 0755)
	os.WriteFile(filepath.Join(sourceSkill, "SKILL.md"), []byte("---\nname: ext\n---\n# E"), 0644)

	sb.RunCLIInDir(projectRoot, "install", sourceSkill, "-p").AssertSuccess(t)

	gitignore := sb.ReadFile(filepath.Join(projectRoot, ".skillshare", ".gitignore"))
	if !strings.Contains(gitignore, "skills/ext/") {
		t.Errorf("gitignore should contain skills/ext/, got:\n%s", gitignore)
	}
	if !strings.Contains(gitignore, "BEGIN SKILLSHARE MANAGED") {
		t.Error("gitignore should contain marker block")
	}
}

func TestGitignoreProject_UninstallRemovesEntry(t *testing.T) {
	sb := testutil.NewSandbox(t)
	defer sb.Cleanup()
	projectRoot := sb.SetupProjectDir("claude")

	// Create remote skill with meta
	skillDir := sb.CreateProjectSkill(projectRoot, "removable", map[string]string{
		"SKILL.md": "# Removable",
	})
	meta := map[string]interface{}{"source": "org/removable", "type": "github"}
	metaJSON, _ := json.Marshal(meta)
	os.WriteFile(filepath.Join(skillDir, ".skillshare-meta.json"), metaJSON, 0644)

	// Write gitignore with the entry
	sb.WriteFile(filepath.Join(projectRoot, ".skillshare", ".gitignore"),
		"# BEGIN SKILLSHARE MANAGED - DO NOT EDIT\nskills/removable/\n# END SKILLSHARE MANAGED\n")

	sb.WriteProjectConfig(projectRoot, `targets:
  - claude
skills:
  - name: removable
    source: org/removable
`)

	sb.RunCLIInDir(projectRoot, "uninstall", "removable", "--force", "-p").AssertSuccess(t)

	gitignore := sb.ReadFile(filepath.Join(projectRoot, ".skillshare", ".gitignore"))
	if strings.Contains(gitignore, "skills/removable/") {
		t.Errorf("gitignore should not contain removed entry, got:\n%s", gitignore)
	}
}

func TestGitignoreProject_PreservesUserEntries(t *testing.T) {
	sb := testutil.NewSandbox(t)
	defer sb.Cleanup()
	projectRoot := sb.SetupProjectDir("claude")

	// Write gitignore with user entries
	sb.WriteFile(filepath.Join(projectRoot, ".skillshare", ".gitignore"),
		"# my custom rule\n*.tmp\n")

	sourceSkill := filepath.Join(sb.Root, "ext2")
	os.MkdirAll(sourceSkill, 0755)
	os.WriteFile(filepath.Join(sourceSkill, "SKILL.md"), []byte("---\nname: ext2\n---\n# E"), 0644)

	sb.RunCLIInDir(projectRoot, "install", sourceSkill, "-p").AssertSuccess(t)

	gitignore := sb.ReadFile(filepath.Join(projectRoot, ".skillshare", ".gitignore"))
	if !strings.Contains(gitignore, "*.tmp") {
		t.Error("user entries should be preserved")
	}
	if !strings.Contains(gitignore, "skills/ext2/") {
		t.Error("new entry should be added")
	}
}
