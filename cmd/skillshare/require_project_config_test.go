package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func writeSkill(t *testing.T, dir, name string) {
	t.Helper()
	skillDir := filepath.Join(dir, name)
	if err := os.MkdirAll(skillDir, 0755); err != nil {
		t.Fatal(err)
	}
	body := "---\nname: " + name + "\ndescription: Fixture skill.\n---\n\n# " + name + "\n"
	if err := os.WriteFile(filepath.Join(skillDir, "SKILL.md"), []byte(body), 0644); err != nil {
		t.Fatal(err)
	}
}

func TestEnsureProjectConfig_AlreadyInitialized(t *testing.T) {
	root := t.TempDir()
	dir := filepath.Join(root, ".skillshare")
	if err := os.MkdirAll(dir, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "config.yaml"), []byte("targets: []\n"), 0644); err != nil {
		t.Fatal(err)
	}

	if err := ensureProjectConfig(root); err != nil {
		t.Errorf("ensureProjectConfig() = %v, want nil", err)
	}
}

// A repository with no project directory at all still bootstraps, which is the
// documented behaviour of the project commands.
func TestEnsureProjectConfig_NoProjectStillInitializes(t *testing.T) {
	root := t.TempDir()

	if err := ensureProjectConfig(root); err != nil {
		t.Fatalf("ensureProjectConfig() = %v, want nil", err)
	}
	if _, err := os.Stat(filepath.Join(root, ".skillshare", "config.yaml")); err != nil {
		t.Errorf("no config created for a fresh repository: %v", err)
	}
}

// The --config local shared skills repo gitignores config.yaml, so a fresh
// clone legitimately has content but no config and must still regenerate one.
func TestEnsureProjectConfig_SharedRepoStillRepairs(t *testing.T) {
	root := t.TempDir()
	dir := filepath.Join(root, ".skillshare")
	writeSkill(t, filepath.Join(dir, "skills"), "shared-skill")
	if err := os.WriteFile(filepath.Join(dir, ".gitignore"), []byte("config.yaml\n"), 0644); err != nil {
		t.Fatal(err)
	}

	if err := ensureProjectConfig(root); err != nil {
		t.Fatalf("ensureProjectConfig() = %v, want nil for the shared-repo flow", err)
	}
	if _, err := os.Stat(filepath.Join(dir, "config.yaml")); err != nil {
		t.Errorf("shared-repo repair did not write a config: %v", err)
	}
	if _, err := os.Stat(filepath.Join(dir, "skills", "shared-skill", "SKILL.md")); err != nil {
		t.Errorf("shared skill was disturbed: %v", err)
	}
}

// The guarded case: a project directory that already holds content, whose
// config.yaml is simply missing and is not gitignored. Re-initializing would
// replace it with an empty config and silently drop every configured target.
func TestEnsureProjectConfig_RefusesToDiscardExistingProject(t *testing.T) {
	root := t.TempDir()
	dir := filepath.Join(root, ".skillshare")
	writeSkill(t, filepath.Join(dir, "skills"), "demo")

	err := ensureProjectConfig(root)
	if err == nil {
		t.Fatal("ensureProjectConfig() = nil, want an error for a content-bearing project without a config")
	}
	if !strings.Contains(err.Error(), ".skillshare") {
		t.Errorf("error %q does not name the project directory", err)
	}
	if !strings.Contains(err.Error(), "init -p") {
		t.Errorf("error %q does not point at 'init -p'", err)
	}
	if _, statErr := os.Stat(filepath.Join(dir, "config.yaml")); !os.IsNotExist(statErr) {
		t.Error("a config was written over an existing project")
	}
	if _, statErr := os.Stat(filepath.Join(dir, "skills", "demo", "SKILL.md")); statErr != nil {
		t.Errorf("existing skill was disturbed: %v", statErr)
	}
}

// Agents alone are enough to make a project worth protecting.
func TestEnsureProjectConfig_RefusesWhenOnlyAgentsExist(t *testing.T) {
	root := t.TempDir()
	dir := filepath.Join(root, ".skillshare")
	writeSkill(t, filepath.Join(dir, "agents"), "demo-agent")

	if err := ensureProjectConfig(root); err == nil {
		t.Fatal("ensureProjectConfig() = nil, want an error for a project holding only agents")
	}
	if _, err := os.Stat(filepath.Join(dir, "config.yaml")); !os.IsNotExist(err) {
		t.Error("a config was written over an existing project")
	}
}

// An empty project directory has nothing to lose, so it repairs silently as
// before.
func TestEnsureProjectConfig_EmptyProjectDirRepairs(t *testing.T) {
	root := t.TempDir()
	dir := filepath.Join(root, ".skillshare")
	if err := os.MkdirAll(filepath.Join(dir, "skills"), 0755); err != nil {
		t.Fatal(err)
	}

	if err := ensureProjectConfig(root); err != nil {
		t.Fatalf("ensureProjectConfig() = %v, want nil for an empty project directory", err)
	}
	if _, err := os.Stat(filepath.Join(dir, "config.yaml")); err != nil {
		t.Errorf("empty project directory was not repaired: %v", err)
	}
}

// Regression guard for the reported data loss, at the command level.
func TestCmdStatusProject_DoesNotDiscardExistingProject(t *testing.T) {
	root := t.TempDir()
	dir := filepath.Join(root, ".skillshare")
	writeSkill(t, filepath.Join(dir, "skills"), "demo")

	if err := cmdStatusProject(root); err == nil {
		t.Fatal("cmdStatusProject() = nil, want an error for a content-bearing project without a config")
	}
	if _, err := os.Stat(filepath.Join(dir, "config.yaml")); !os.IsNotExist(err) {
		t.Error("status wrote a fresh config over an existing project")
	}
	if _, err := os.Stat(filepath.Join(dir, "skills", "demo", "SKILL.md")); err != nil {
		t.Errorf("existing skill was disturbed: %v", err)
	}
}
