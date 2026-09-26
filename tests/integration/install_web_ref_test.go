//go:build !online

package integration

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"skillshare/internal/install"
	"skillshare/internal/testutil"
)

// setupWebRefRemote serves a bare repo as https://github.com/acme/skills.git.
// main has skills/foo, tag v1.0 marks its first version, and branch feature/x
// adds skills/bar.
func setupWebRefRemote(t *testing.T, sb *testutil.Sandbox) (workDir string) {
	t.Helper()
	remoteRepo := filepath.Join(sb.Root, "skills.git")
	gitInit(t, remoteRepo, true)
	workDir = filepath.Join(sb.Root, "work")
	gitClone(t, remoteRepo, workDir)

	writeSkill(t, workDir, "skills/foo", "foo", "V1")
	gitAddCommit(t, workDir, "v1")
	gitPush(t, workDir)
	run(t, workDir, "git", "tag", "v1.0")
	run(t, workDir, "git", "push", "origin", "v1.0")

	run(t, workDir, "git", "checkout", "-q", "-b", "feature/x")
	writeSkill(t, workDir, "skills/bar", "bar", "FX")
	gitAddCommit(t, workDir, "bar")
	run(t, workDir, "git", "push", "-q", "origin", "feature/x")
	run(t, workDir, "git", "checkout", "-q", "-")

	configureGitURLRewriteOrSkip(t, sb.Home, remoteRepo, "https://github.com/acme/skills.git")
	return workDir
}

func writeSkill(t *testing.T, workDir, dir, name, body string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Join(workDir, dir), 0755); err != nil {
		t.Fatal(err)
	}
	content := "---\nname: " + name + "\n---\n# " + body
	if err := os.WriteFile(filepath.Join(workDir, dir, "SKILL.md"), []byte(content), 0644); err != nil {
		t.Fatal(err)
	}
}

func assertSkillBody(t *testing.T, sb *testutil.Sandbox, name, want string) {
	t.Helper()
	content, err := os.ReadFile(filepath.Join(sb.SourcePath, name, "SKILL.md"))
	if err != nil {
		t.Fatalf("read %s: %v", name, err)
	}
	if !strings.Contains(string(content), "# "+want) {
		t.Errorf("%s content = %q, want %q", name, content, want)
	}
}

func TestInstallWebURLRef_UpdateKeepsPin(t *testing.T) {
	sb := testutil.NewSandbox(t)
	defer sb.Cleanup()
	sb.WriteConfig("source: " + sb.SourcePath + "\ntargets: {}\n")
	workDir := setupWebRefRemote(t, sb)

	sb.RunCLI("install", "github.com/acme/skills/tree/v1.0/skills/foo", "--skip-audit").AssertSuccess(t)
	assertSkillBody(t, sb, "foo", "V1")

	store, err := install.LoadMetadata(sb.SourcePath)
	if err != nil {
		t.Fatalf("load metadata: %v", err)
	}
	if entry := store.Get("foo"); entry == nil || entry.Branch != "v1.0" {
		t.Fatalf("metadata entry = %+v, want branch v1.0", entry)
	}

	writeSkill(t, workDir, "skills/foo", "foo", "V2")
	gitAddCommit(t, workDir, "v2")
	gitPush(t, workDir)

	sb.RunCLI("update", "foo", "--skip-audit").AssertSuccess(t)
	assertSkillBody(t, sb, "foo", "V1")
}

func TestInstallWebURLRef_BranchWithSlash(t *testing.T) {
	sb := testutil.NewSandbox(t)
	defer sb.Cleanup()
	sb.WriteConfig("source: " + sb.SourcePath + "\ntargets: {}\n")
	workDir := setupWebRefRemote(t, sb)

	sb.RunCLI("install", "github.com/acme/skills/tree/feature/x/skills/bar", "--skip-audit").AssertSuccess(t)
	assertSkillBody(t, sb, "bar", "FX")

	run(t, workDir, "git", "checkout", "-q", "feature/x")
	writeSkill(t, workDir, "skills/bar", "bar", "FX2")
	gitAddCommit(t, workDir, "bar v2")
	run(t, workDir, "git", "push", "-q", "origin", "feature/x")

	sb.RunCLI("update", "bar", "--skip-audit").AssertSuccess(t)
	assertSkillBody(t, sb, "bar", "FX2")
}

func TestInstallWebURLRef_UnknownRefFails(t *testing.T) {
	sb := testutil.NewSandbox(t)
	defer sb.Cleanup()
	sb.WriteConfig("source: " + sb.SourcePath + "\ntargets: {}\n")
	setupWebRefRemote(t, sb)

	result := sb.RunCLI("install", "github.com/acme/skills/tree/nope/skills/foo", "--skip-audit")
	result.AssertFailure(t)
	result.AssertAnyOutputContains(t, "--branch")
	if _, err := os.Stat(filepath.Join(sb.SourcePath, "foo")); err == nil {
		t.Error("foo should not be installed from an unknown ref")
	}
}
