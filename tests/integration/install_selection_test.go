//go:build !online

package integration

import (
	"os"
	"path/filepath"
	"testing"

	"skillshare/internal/testutil"
)

func TestInstall_SkillFlag_SelectsSpecific(t *testing.T) {
	sb := testutil.NewSandbox(t)
	defer sb.Cleanup()

	sb.WriteConfig(`source: ` + sb.SourcePath + "\ntargets: {}\n")

	gitRepoPath := createMultiSkillGitRepo(t, sb, "skill-flag-repo", []string{"skill-one", "skill-two", "skill-three"})

	result := sb.RunCLI("install", "file://"+gitRepoPath, "-s", "skill-one")
	result.AssertSuccess(t)
	result.AssertAnyOutputContains(t, "skill-one")

	if !sb.FileExists(filepath.Join(sb.SourcePath, "skill-one", "SKILL.md")) {
		t.Error("skill-one should be installed")
	}
	if sb.FileExists(filepath.Join(sb.SourcePath, "skill-two")) {
		t.Error("skill-two should NOT be installed")
	}
	if sb.FileExists(filepath.Join(sb.SourcePath, "skill-three")) {
		t.Error("skill-three should NOT be installed")
	}
}

func TestInstall_SkillFlag_CommaSeparated(t *testing.T) {
	sb := testutil.NewSandbox(t)
	defer sb.Cleanup()

	sb.WriteConfig(`source: ` + sb.SourcePath + "\ntargets: {}\n")

	gitRepoPath := createMultiSkillGitRepo(t, sb, "comma-repo", []string{"skill-one", "skill-two", "skill-three"})

	result := sb.RunCLI("install", "file://"+gitRepoPath, "-s", "skill-one,skill-two")
	result.AssertSuccess(t)

	if !sb.FileExists(filepath.Join(sb.SourcePath, "skill-one", "SKILL.md")) {
		t.Error("skill-one should be installed")
	}
	if !sb.FileExists(filepath.Join(sb.SourcePath, "skill-two", "SKILL.md")) {
		t.Error("skill-two should be installed")
	}
	if sb.FileExists(filepath.Join(sb.SourcePath, "skill-three")) {
		t.Error("skill-three should NOT be installed")
	}
}

func TestInstall_SkillFlag_NotFound(t *testing.T) {
	sb := testutil.NewSandbox(t)
	defer sb.Cleanup()

	sb.WriteConfig(`source: ` + sb.SourcePath + "\ntargets: {}\n")

	gitRepoPath := createMultiSkillGitRepo(t, sb, "notfound-repo", []string{"skill-one", "skill-two"})

	result := sb.RunCLI("install", "file://"+gitRepoPath, "-s", "nonexistent")
	result.AssertFailure(t)
	result.AssertAnyOutputContains(t, "skills not found: nonexistent")
	result.AssertAnyOutputContains(t, "Available:")
}

func TestInstall_AllFlag_InstallsAll(t *testing.T) {
	sb := testutil.NewSandbox(t)
	defer sb.Cleanup()

	sb.WriteConfig(`source: ` + sb.SourcePath + "\ntargets: {}\n")

	gitRepoPath := createMultiSkillGitRepo(t, sb, "all-flag-repo", []string{"skill-one", "skill-two", "skill-three"})

	result := sb.RunCLI("install", "file://"+gitRepoPath, "--all")
	result.AssertSuccess(t)

	if !sb.FileExists(filepath.Join(sb.SourcePath, "skill-one", "SKILL.md")) {
		t.Error("skill-one should be installed")
	}
	if !sb.FileExists(filepath.Join(sb.SourcePath, "skill-two", "SKILL.md")) {
		t.Error("skill-two should be installed")
	}
	if !sb.FileExists(filepath.Join(sb.SourcePath, "skill-three", "SKILL.md")) {
		t.Error("skill-three should be installed")
	}
}

func TestInstall_AllFlag_ShowsAuditWarningsForFindingsBelowThreshold(t *testing.T) {
	sb := testutil.NewSandbox(t)
	defer sb.Cleanup()

	sb.WriteConfig(`source: ` + sb.SourcePath + "\ntargets: {}\n")

	gitRepoPath := filepath.Join(sb.Root, "all-flag-audit-warning-repo")
	safeSkill := filepath.Join(gitRepoPath, "safe-skill")
	highSkill := filepath.Join(gitRepoPath, "high-skill")
	if err := os.MkdirAll(safeSkill, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(highSkill, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(safeSkill, "SKILL.md"), []byte("# safe-skill"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(highSkill, "SKILL.md"), []byte("# high-skill\nsudo apt-get install -y jq"), 0644); err != nil {
		t.Fatal(err)
	}
	initGitRepo(t, gitRepoPath)

	result := sb.RunCLI("install", "file://"+gitRepoPath, "--all")
	result.AssertSuccess(t)
	result.AssertAnyOutputContains(t, "HIGH=1")
	result.AssertAnyOutputContains(t, "findings below the active block threshold")
}

func TestInstall_YesFlag_InstallsAll(t *testing.T) {
	sb := testutil.NewSandbox(t)
	defer sb.Cleanup()

	sb.WriteConfig(`source: ` + sb.SourcePath + "\ntargets: {}\n")

	gitRepoPath := createMultiSkillGitRepo(t, sb, "yes-flag-repo", []string{"skill-one", "skill-two", "skill-three"})

	result := sb.RunCLI("install", "file://"+gitRepoPath, "-y")
	result.AssertSuccess(t)

	if !sb.FileExists(filepath.Join(sb.SourcePath, "skill-one", "SKILL.md")) {
		t.Error("skill-one should be installed")
	}
	if !sb.FileExists(filepath.Join(sb.SourcePath, "skill-two", "SKILL.md")) {
		t.Error("skill-two should be installed")
	}
	if !sb.FileExists(filepath.Join(sb.SourcePath, "skill-three", "SKILL.md")) {
		t.Error("skill-three should be installed")
	}
}

func TestInstall_SkillFlag_DryRun(t *testing.T) {
	sb := testutil.NewSandbox(t)
	defer sb.Cleanup()

	sb.WriteConfig(`source: ` + sb.SourcePath + "\ntargets: {}\n")

	gitRepoPath := createMultiSkillGitRepo(t, sb, "dryrun-skill-repo", []string{"skill-one", "skill-two", "skill-three"})

	result := sb.RunCLI("install", "file://"+gitRepoPath, "-s", "skill-one", "--dry-run")
	result.AssertSuccess(t)
	result.AssertOutputContains(t, "skill-one")
	result.AssertOutputContains(t, "dry-run")
	result.AssertOutputContains(t, "1 skill(s)")

	// skill-two and skill-three should NOT appear in output
	result.AssertOutputNotContains(t, "skill-two")
	result.AssertOutputNotContains(t, "skill-three")

	// Nothing should be installed
	if sb.FileExists(filepath.Join(sb.SourcePath, "skill-one")) {
		t.Error("skill should not be installed in dry-run mode")
	}
}

func TestInstall_SkillAndAllConflict(t *testing.T) {
	sb := testutil.NewSandbox(t)
	defer sb.Cleanup()

	sb.WriteConfig(`source: ` + sb.SourcePath + "\ntargets: {}\n")

	result := sb.RunCLI("install", "file:///some/repo", "-s", "x", "--all")
	result.AssertFailure(t)
	result.AssertAnyOutputContains(t, "--skill and --all cannot be used together")
}

func TestInstall_SkillAndTrackConflict(t *testing.T) {
	sb := testutil.NewSandbox(t)
	defer sb.Cleanup()

	sb.WriteConfig(`source: ` + sb.SourcePath + "\ntargets: {}\n")

	result := sb.RunCLI("install", "file:///some/repo", "-s", "x", "--track")
	result.AssertFailure(t)
	result.AssertAnyOutputContains(t, "--skill cannot be used with --track")
}

// TestInstall_SubdirFuzzyResolve tests that when a subdir doesn't exist at the
// exact path, the installer scans the repo for a matching skill by basename.
// e.g. "owner/repo/pdf" where "pdf" lives at "skills/pdf/SKILL.md".
