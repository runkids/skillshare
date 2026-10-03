//go:build !online

package integration

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"skillshare/internal/install"
	"skillshare/internal/oplog"
	"skillshare/internal/testutil"
)

// These tests pin the observable differences between global and project
// uninstall so that both modes can share one flow without drifting.

const uninstallModesGlobalConfig = "targets: {}\n"

func setupGlobalUninstall(sb *testutil.Sandbox) {
	sb.WriteConfig("source: " + sb.SourcePath + "\n" + uninstallModesGlobalConfig)
}

func lastUninstallOp(t *testing.T, configPath string) *oplog.Entry {
	t.Helper()
	entries, err := oplog.Read(configPath, oplog.OpsFile, 0)
	if err != nil {
		t.Fatalf("read oplog: %v", err)
	}
	for i := range entries {
		if entries[i].Command == "uninstall" {
			return &entries[i]
		}
	}
	return nil
}

func projectConfigPath(projectRoot string) string {
	return filepath.Join(projectRoot, ".skillshare", "config.yaml")
}

func TestUninstallModes_SingleConfirmPrompt(t *testing.T) {
	sb := testutil.NewSandbox(t)
	defer sb.Cleanup()
	setupGlobalUninstall(sb)
	sb.CreateSkill("keep", map[string]string{"SKILL.md": "# Keep"})
	projectRoot := sb.SetupProjectDir("claude")
	sb.CreateProjectSkill(projectRoot, "keep", map[string]string{"SKILL.md": "# Keep"})

	global := sb.RunCLIWithInput("n\n", "uninstall", "keep", "-g")
	global.AssertSuccess(t)
	global.AssertAnyOutputContains(t, "Uninstall keep?")
	global.AssertAnyOutputContains(t, "Cancelled. Nothing was removed.")

	project := sb.RunCLIInDirWithInput(projectRoot, "n\n", "uninstall", "keep", "-p")
	project.AssertSuccess(t)
	project.AssertAnyOutputContains(t, "Uninstall keep from the project?")
	project.AssertAnyOutputContains(t, "Cancelled. Nothing was removed.")
}

func TestUninstallModes_BatchConfirmPrompt(t *testing.T) {
	sb := testutil.NewSandbox(t)
	defer sb.Cleanup()
	setupGlobalUninstall(sb)
	projectRoot := sb.SetupProjectDir("claude")
	for _, name := range []string{"a", "b"} {
		sb.CreateSkill(name, map[string]string{"SKILL.md": "# " + name})
		sb.CreateProjectSkill(projectRoot, name, map[string]string{"SKILL.md": "# " + name})
	}

	global := sb.RunCLIWithInput("n\n", "uninstall", "a", "b", "-g")
	global.AssertSuccess(t)
	global.AssertAnyOutputContains(t, "Uninstall 2 skills?")

	project := sb.RunCLIInDirWithInput(projectRoot, "n\n", "uninstall", "a", "b", "-p")
	project.AssertSuccess(t)
	project.AssertAnyOutputContains(t, "Uninstall 2 skills from the project?")
}

func TestUninstallModes_ReinstallHint(t *testing.T) {
	sb := testutil.NewSandbox(t)
	defer sb.Cleanup()
	setupGlobalUninstall(sb)
	projectRoot := sb.SetupProjectDir("claude")
	projectSkills := filepath.Join(projectRoot, ".skillshare", "skills")
	for _, dir := range []string{sb.SourcePath, projectSkills} {
		store := install.NewMetadataStore()
		store.Set("remote", &install.MetadataEntry{Source: "github.com/org/repo/remote", Type: "github"})
		if err := store.Save(dir); err != nil {
			t.Fatal(err)
		}
	}
	sb.CreateSkill("remote", map[string]string{"SKILL.md": "# R"})
	sb.CreateProjectSkill(projectRoot, "remote", map[string]string{"SKILL.md": "# R"})

	globalDry := sb.RunCLI("uninstall", "remote", "--dry-run", "-g")
	globalDry.AssertSuccess(t)
	globalDry.AssertAnyOutputContains(t, "reinstall with skillshare install github.com/org/repo/remote\n")

	projectDry := sb.RunCLIInDir(projectRoot, "uninstall", "remote", "--dry-run", "-p")
	projectDry.AssertSuccess(t)
	projectDry.AssertAnyOutputContains(t, "reinstall with skillshare install github.com/org/repo/remote --project\n")

	global := sb.RunCLI("uninstall", "remote", "--force", "-g")
	global.AssertSuccess(t)
	global.AssertAnyOutputContains(t, "skillshare install github.com/org/repo/remote  ")

	project := sb.RunCLIInDir(projectRoot, "uninstall", "remote", "--force", "-p")
	project.AssertSuccess(t)
	project.AssertAnyOutputContains(t, "skillshare install github.com/org/repo/remote --project  ")
}

func TestUninstallModes_DryRunGitignoreLines(t *testing.T) {
	sb := testutil.NewSandbox(t)
	defer sb.Cleanup()
	setupGlobalUninstall(sb)
	sb.CreateSkill("plain", map[string]string{"SKILL.md": "# P"})
	repo := filepath.Join(sb.SourcePath, "_repo")
	os.MkdirAll(repo, 0755)
	os.WriteFile(filepath.Join(repo, "SKILL.md"), []byte("# Repo"), 0644)
	initGitRepo(t, repo)
	projectRoot := sb.SetupProjectDir("claude")
	sb.CreateProjectSkill(projectRoot, "plain", map[string]string{"SKILL.md": "# P"})

	globalPlain := sb.RunCLI("uninstall", "plain", "--dry-run", "-g")
	globalPlain.AssertSuccess(t)
	globalPlain.AssertOutputNotContains(t, ".gitignore")

	globalRepo := sb.RunCLI("uninstall", "_repo", "--dry-run", "-g")
	globalRepo.AssertSuccess(t)
	globalRepo.AssertAnyOutputContains(t, "would remove _repo from .gitignore")

	project := sb.RunCLIInDir(projectRoot, "uninstall", "plain", "--dry-run", "-p")
	project.AssertSuccess(t)
	project.AssertAnyOutputContains(t, "would update .skillshare/.gitignore")
}

func TestUninstallModes_TrashLocation(t *testing.T) {
	sb := testutil.NewSandbox(t)
	defer sb.Cleanup()
	projectRoot := sb.SetupProjectDir("claude")
	sb.CreateProjectSkill(projectRoot, "gone", map[string]string{"SKILL.md": "# Gone"})

	sb.RunCLIInDir(projectRoot, "uninstall", "gone", "--force", "-p").AssertSuccess(t)

	entries, err := os.ReadDir(filepath.Join(projectRoot, ".skillshare", "trash"))
	if err != nil || len(entries) != 1 || !strings.HasPrefix(entries[0].Name(), "gone_") {
		t.Fatalf("project trash should hold one gone_* entry, got %v (err %v)", entries, err)
	}
	global, _ := os.ReadDir(filepath.Join(sb.Home, ".local", "share", "skillshare", "trash"))
	if len(global) != 0 {
		t.Errorf("project uninstall must not use the global trash, got %v", global)
	}
}

func TestUninstallModes_GitignoreEntries(t *testing.T) {
	sb := testutil.NewSandbox(t)
	defer sb.Cleanup()
	setupGlobalUninstall(sb)
	sb.CreateSkill("plain", map[string]string{"SKILL.md": "# P"})
	repo := filepath.Join(sb.SourcePath, "_repo")
	os.MkdirAll(repo, 0755)
	os.WriteFile(filepath.Join(repo, "SKILL.md"), []byte("# Repo"), 0644)
	initGitRepo(t, repo)
	gitignore := filepath.Join(sb.SourcePath, ".gitignore")
	sb.WriteFile(gitignore, "# BEGIN SKILLSHARE MANAGED - DO NOT EDIT\n_repo/\nplain/\n# END SKILLSHARE MANAGED\n")

	sb.RunCLI("uninstall", "plain", "_repo", "--force", "-g").AssertSuccess(t)

	got := sb.ReadFile(gitignore)
	if strings.Contains(got, "_repo/") {
		t.Errorf("global uninstall should remove the tracked repo entry, got:\n%s", got)
	}
	if !strings.Contains(got, "plain/") {
		t.Errorf("global uninstall only edits tracked repo entries, got:\n%s", got)
	}
}

func TestUninstallModes_ProjectIgnoresJSONFlag(t *testing.T) {
	sb := testutil.NewSandbox(t)
	defer sb.Cleanup()
	projectRoot := sb.SetupProjectDir("claude")
	sb.CreateProjectSkill(projectRoot, "keep", map[string]string{"SKILL.md": "# Keep"})

	// Project mode neither emits JSON nor treats --json as --force.
	result := sb.RunCLIInDirWithInput(projectRoot, "n\n", "uninstall", "keep", "--json", "-p")
	result.AssertSuccess(t)
	result.AssertAnyOutputContains(t, "Uninstall keep from the project?")
	var out map[string]any
	if json.Unmarshal([]byte(result.Stdout), &out) == nil {
		t.Errorf("project uninstall should not print JSON, got %s", result.Stdout)
	}
	if !sb.FileExists(filepath.Join(projectRoot, ".skillshare", "skills", "keep")) {
		t.Error("declined prompt must keep the skill")
	}
}

func TestUninstallModes_GlobPattern(t *testing.T) {
	sb := testutil.NewSandbox(t)
	defer sb.Cleanup()
	setupGlobalUninstall(sb)
	projectRoot := sb.SetupProjectDir("claude")
	for _, name := range []string{"core-a", "core-b"} {
		sb.CreateSkill(name, map[string]string{"SKILL.md": "# " + name})
		sb.CreateProjectSkill(projectRoot, name, map[string]string{"SKILL.md": "# " + name})
	}

	global := sb.RunCLI("uninstall", "core-*", "--force", "-g")
	global.AssertSuccess(t)
	global.AssertAnyOutputContains(t, "Pattern 'core-*' matched 2 item(s)")

	project := sb.RunCLIInDir(projectRoot, "uninstall", "core-*", "--force", "-p")
	project.AssertFailure(t)
	project.AssertAnyOutputContains(t, "skill 'core-*' not found in .skillshare/skills")
}

func TestUninstallModes_AmbiguousNestedName(t *testing.T) {
	sb := testutil.NewSandbox(t)
	defer sb.Cleanup()
	setupGlobalUninstall(sb)
	projectRoot := sb.SetupProjectDir("claude")
	for _, rel := range []string{"x/dup", "y/dup"} {
		sb.CreateNestedSkill(rel, map[string]string{"SKILL.md": "# dup"})
		sb.CreateProjectSkill(projectRoot, rel, map[string]string{"SKILL.md": "# dup"})
	}

	global := sb.RunCLI("uninstall", "dup", "--force", "-g")
	global.AssertFailure(t)
	global.AssertAnyOutputContains(t, "'dup' matches multiple skills")

	project := sb.RunCLIInDir(projectRoot, "uninstall", "dup", "--force", "-p")
	project.AssertFailure(t)
	project.AssertAnyOutputContains(t, "'dup' matches multiple skills")
}

func TestUninstallModes_FileNameIsNotADirectory(t *testing.T) {
	sb := testutil.NewSandbox(t)
	defer sb.Cleanup()
	setupGlobalUninstall(sb)
	projectRoot := sb.SetupProjectDir("claude")
	sb.WriteFile(filepath.Join(sb.SourcePath, "notes.txt"), "x")
	sb.WriteFile(filepath.Join(projectRoot, ".skillshare", "skills", "notes.txt"), "x")

	global := sb.RunCLI("uninstall", "notes.txt", "--force", "-g")
	global.AssertFailure(t)
	global.AssertAnyOutputContains(t, "'notes.txt' is not a directory")

	project := sb.RunCLIInDir(projectRoot, "uninstall", "notes.txt", "--force", "-p")
	project.AssertFailure(t)
	project.AssertAnyOutputContains(t, "'notes.txt' is not a directory")
}

func TestUninstallModes_AllWithEmptySource(t *testing.T) {
	sb := testutil.NewSandbox(t)
	defer sb.Cleanup()
	setupGlobalUninstall(sb)
	projectRoot := sb.SetupProjectDir("claude")

	global := sb.RunCLI("uninstall", "--all", "--force", "-g")
	global.AssertFailure(t)
	global.AssertAnyOutputContains(t, "no skills found in source")

	project := sb.RunCLIInDir(projectRoot, "uninstall", "--all", "--force", "-p")
	project.AssertFailure(t)
	project.AssertAnyOutputContains(t, "no skills found in project source")
}

// makeDirtyRepo creates a tracked repo with an untracked file.
func makeDirtyRepo(t *testing.T, dir string) {
	t.Helper()
	os.MkdirAll(dir, 0755)
	os.WriteFile(filepath.Join(dir, "SKILL.md"), []byte("# Dirty"), 0644)
	initGitRepo(t, dir)
	os.WriteFile(filepath.Join(dir, "uncommitted.txt"), []byte("dirty"), 0644)
}

func TestUninstallModes_BatchDirtyRepoMessages(t *testing.T) {
	sb := testutil.NewSandbox(t)
	defer sb.Cleanup()
	setupGlobalUninstall(sb)
	projectRoot := sb.SetupProjectDir("claude")
	projectSkills := filepath.Join(projectRoot, ".skillshare", "skills")
	makeDirtyRepo(t, filepath.Join(sb.SourcePath, "_dirty"))
	makeDirtyRepo(t, filepath.Join(projectSkills, "_dirty"))
	for _, name := range []string{"clean-a", "clean-b"} {
		sb.CreateSkill(name, map[string]string{"SKILL.md": "# C"})
		sb.CreateProjectSkill(projectRoot, name, map[string]string{"SKILL.md": "# C"})
	}

	global := sb.RunCLIWithInput("y\n", "uninstall", "_dirty", "clean-a", "clean-b", "-g")
	global.AssertSuccess(t)
	global.AssertAnyOutputContains(t, "uncommitted changes, use --force")
	global.AssertAnyOutputContains(t, "1 tracked repo skipped, 2 remaining")
	global.AssertAnyOutputContains(t, "Uninstalled 2 skills, 1 skipped")
	global.AssertAnyOutputContains(t, "skillshare trash list  ")

	project := sb.RunCLIInDirWithInput(projectRoot, "y\n", "uninstall", "_dirty", "clean-a", "clean-b", "-p")
	project.AssertSuccess(t)
	project.AssertAnyOutputContains(t, "Skipping _dirty: uncommitted changes detected, use --force to override")
	project.AssertAnyOutputContains(t, "1 tracked repo skipped, 2 remaining")
	project.AssertAnyOutputContains(t, "Uninstalled 2 skills, 1 skipped")
	project.AssertAnyOutputContains(t, "skillshare trash list --project")
}

func TestUninstallModes_BatchAllDirtyError(t *testing.T) {
	sb := testutil.NewSandbox(t)
	defer sb.Cleanup()
	setupGlobalUninstall(sb)
	projectRoot := sb.SetupProjectDir("claude")
	projectSkills := filepath.Join(projectRoot, ".skillshare", "skills")
	for _, name := range []string{"_d1", "_d2"} {
		makeDirtyRepo(t, filepath.Join(sb.SourcePath, name))
		makeDirtyRepo(t, filepath.Join(projectSkills, name))
	}

	global := sb.RunCLI("uninstall", "_d1", "_d2", "-g")
	global.AssertFailure(t)
	global.AssertAnyOutputContains(t, "2 tracked repos skipped due to uncommitted changes; use --force to override")

	project := sb.RunCLIInDir(projectRoot, "uninstall", "_d1", "_d2", "-p")
	project.AssertFailure(t)
	project.AssertAnyOutputContains(t, "no skills to uninstall after pre-flight checks")
}

func TestUninstallModes_ForceDirtyWarning(t *testing.T) {
	sb := testutil.NewSandbox(t)
	defer sb.Cleanup()
	setupGlobalUninstall(sb)
	projectRoot := sb.SetupProjectDir("claude")
	makeDirtyRepo(t, filepath.Join(sb.SourcePath, "_dirty"))
	makeDirtyRepo(t, filepath.Join(projectRoot, ".skillshare", "skills", "_dirty"))

	global := sb.RunCLI("uninstall", "_dirty", "--force", "-g")
	global.AssertSuccess(t)
	global.AssertAnyOutputContains(t, "Repository _dirty has uncommitted changes (proceeding with --force)")

	project := sb.RunCLIInDir(projectRoot, "uninstall", "_dirty", "--force", "-p")
	project.AssertSuccess(t)
	project.AssertAnyOutputContains(t, "Repository has uncommitted changes (proceeding with --force)")
}

func TestUninstallModes_ForceGitStatusErrorWarning(t *testing.T) {
	sb := testutil.NewSandbox(t)
	defer sb.Cleanup()
	setupGlobalUninstall(sb)
	makeUnreadableTrackedRepo(t, sb, "_broken")
	projectRoot := sb.SetupProjectDir("claude")
	projectRepo := sb.CreateProjectSkill(projectRoot, "_broken", map[string]string{"SKILL.md": "# B"})
	os.MkdirAll(filepath.Join(projectRepo, ".git"), 0755)

	global := sb.RunCLI("uninstall", "_broken", "--force", "-g")
	global.AssertSuccess(t)
	global.AssertAnyOutputContains(t, "Could not check git status for _broken (proceeding with --force)")

	project := sb.RunCLIInDir(projectRoot, "uninstall", "_broken", "--force", "-p")
	project.AssertSuccess(t)
	project.AssertAnyOutputContains(t, "Could not check git status (proceeding with --force)")
}

func TestUninstallModes_ProjectBatchGitStatusErrorKeepsCleanSkills(t *testing.T) {
	sb := testutil.NewSandbox(t)
	defer sb.Cleanup()
	projectRoot := sb.SetupProjectDir("claude")
	repo := sb.CreateProjectSkill(projectRoot, "_broken", map[string]string{"SKILL.md": "# B"})
	os.MkdirAll(filepath.Join(repo, ".git"), 0755)
	sb.CreateProjectSkill(projectRoot, "clean", map[string]string{"SKILL.md": "# C"})

	result := sb.RunCLIInDirWithInput(projectRoot, "y\n", "uninstall", "_broken", "clean", "-p")
	result.AssertSuccess(t)
	result.AssertAnyOutputContains(t, "failed to check git status")
	if !sb.FileExists(repo) {
		t.Error("tracked repo must stay in place when git status cannot be read")
	}
	if sb.FileExists(filepath.Join(projectRoot, ".skillshare", "skills", "clean")) {
		t.Error("clean should still be removed")
	}
}

func TestUninstallModes_SingleNextSteps(t *testing.T) {
	sb := testutil.NewSandbox(t)
	defer sb.Cleanup()
	setupGlobalUninstall(sb)
	sb.CreateSkill("one", map[string]string{"SKILL.md": "# One"})
	projectRoot := sb.SetupProjectDir("claude")
	sb.CreateProjectSkill(projectRoot, "one", map[string]string{"SKILL.md": "# One"})

	global := sb.RunCLI("uninstall", "one", "--force", "-g")
	global.AssertSuccess(t)
	global.AssertAnyOutputContains(t, "skillshare trash restore one  ")

	project := sb.RunCLIInDir(projectRoot, "uninstall", "one", "--force", "-p")
	project.AssertSuccess(t)
	project.AssertAnyOutputContains(t, "skillshare trash restore one --project")
}

func TestUninstallModes_OplogSkipsUnexecutedRuns(t *testing.T) {
	sb := testutil.NewSandbox(t)
	defer sb.Cleanup()
	setupGlobalUninstall(sb)
	projectRoot := sb.SetupProjectDir("claude")
	sb.CreateSkill("keep", map[string]string{"SKILL.md": "# K"})
	sb.CreateProjectSkill(projectRoot, "keep", map[string]string{"SKILL.md": "# K"})

	// Resolve failure, text dry-run, declined prompt and help write no entry.
	sb.RunCLI("uninstall", "missing", "--force", "-g").AssertFailure(t)
	sb.RunCLI("uninstall", "keep", "--dry-run", "-g").AssertSuccess(t)
	sb.RunCLIWithInput("n\n", "uninstall", "keep", "-g").AssertSuccess(t)
	sb.RunCLI("uninstall", "--help", "-g").AssertSuccess(t)
	if e := lastUninstallOp(t, sb.ConfigPath); e != nil {
		t.Errorf("global unexecuted runs should not be logged, got %+v", e)
	}

	sb.RunCLIInDir(projectRoot, "uninstall", "missing", "--force", "-p").AssertFailure(t)
	sb.RunCLIInDir(projectRoot, "uninstall", "keep", "--dry-run", "-p").AssertSuccess(t)
	sb.RunCLIInDirWithInput(projectRoot, "n\n", "uninstall", "keep", "-p").AssertSuccess(t)
	sb.RunCLIInDir(projectRoot, "uninstall", "--help", "-p").AssertSuccess(t)
	if e := lastUninstallOp(t, projectConfigPath(projectRoot)); e != nil {
		t.Errorf("project unexecuted runs should not be logged, got %+v", e)
	}
}

func TestUninstallModes_OplogJSONDryRun(t *testing.T) {
	sb := testutil.NewSandbox(t)
	defer sb.Cleanup()
	setupGlobalUninstall(sb)
	sb.CreateSkill("dry", map[string]string{"SKILL.md": "# D"})

	sb.RunCLI("uninstall", "dry", "--dry-run", "--json", "-g").AssertSuccess(t)
	if e := lastUninstallOp(t, sb.ConfigPath); e == nil || e.Status != "ok" {
		t.Errorf("global JSON dry-run should log an ok entry, got %+v", e)
	}
}

func TestUninstallModes_OplogRecordsPartial(t *testing.T) {
	sb := testutil.NewSandbox(t)
	defer sb.Cleanup()
	setupGlobalUninstall(sb)
	projectRoot := sb.SetupProjectDir("claude")
	sb.CreateSkill("real", map[string]string{"SKILL.md": "# R"})
	sb.CreateProjectSkill(projectRoot, "real", map[string]string{"SKILL.md": "# R"})

	sb.RunCLI("uninstall", "real", "ghost", "--force", "-g").AssertSuccess(t)
	if e := lastUninstallOp(t, sb.ConfigPath); e == nil || e.Status != "partial" || e.Args["succeeded"] != float64(1) {
		t.Errorf("global partial uninstall should log partial with succeeded=1, got %+v", e)
	}

	sb.RunCLIInDir(projectRoot, "uninstall", "real", "ghost", "--force", "-p").AssertSuccess(t)
	if e := lastUninstallOp(t, projectConfigPath(projectRoot)); e == nil || e.Status != "partial" || e.Args["succeeded"] != float64(1) {
		t.Errorf("project partial uninstall should log partial with succeeded=1, got %+v", e)
	}
}

// Project uninstall backfills the operational entries for projects created
// before v0.17.3; they belong in .skillshare/.gitignore, never the project root.
func TestUninstallModes_ProjectEnsuresOperationalGitignore(t *testing.T) {
	sb := testutil.NewSandbox(t)
	defer sb.Cleanup()
	projectRoot := sb.SetupProjectDir("claude")
	sb.CreateProjectSkill(projectRoot, "x", map[string]string{"SKILL.md": "# X"})

	sb.RunCLIInDir(projectRoot, "uninstall", "x", "--dry-run", "-p").AssertSuccess(t)

	got := sb.ReadFile(filepath.Join(projectRoot, ".skillshare", ".gitignore"))
	for _, entry := range []string{"logs/", "trash/", "backups/"} {
		if !strings.Contains(got, entry) {
			t.Errorf("project uninstall should gitignore %s, got:\n%s", entry, got)
		}
	}
	if sb.FileExists(filepath.Join(projectRoot, ".gitignore")) {
		t.Errorf("project uninstall must not write the root .gitignore, got:\n%s",
			sb.ReadFile(filepath.Join(projectRoot, ".gitignore")))
	}
}
