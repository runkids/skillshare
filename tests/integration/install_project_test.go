//go:build !online

package integration

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"skillshare/internal/install"
	"skillshare/internal/testutil"
)

func TestInstallProject_LocalPath(t *testing.T) {
	sb := testutil.NewSandbox(t)
	defer sb.Cleanup()
	projectRoot := sb.SetupProjectDir("claude")

	// Create a source skill to install from
	sourceSkill := filepath.Join(sb.Root, "external-skill")
	os.MkdirAll(sourceSkill, 0755)
	os.WriteFile(filepath.Join(sourceSkill, "SKILL.md"), []byte("---\nname: external-skill\n---\n# External"), 0644)

	result := sb.RunCLIInDir(projectRoot, "install", sourceSkill, "-p")
	result.AssertSuccess(t)

	// Skill name comes from directory name, not frontmatter
	if !sb.FileExists(filepath.Join(projectRoot, ".skillshare", "skills", "external-skill", "SKILL.md")) {
		t.Error("skill should be installed to .skillshare/skills/")
	}
}

// TestInstallProject_GitignoresMetadata follows #759: after init -p and an
// install, Git must treat the per-machine .metadata.json as ignored.
func TestInstallProject_GitignoresMetadata(t *testing.T) {
	sb := testutil.NewSandbox(t)
	defer sb.Cleanup()
	projectRoot := filepath.Join(sb.Root, "project")
	os.MkdirAll(projectRoot, 0755)
	sb.WriteFile(filepath.Join(sb.Root, "external-skill", "SKILL.md"), "---\nname: external-skill\n---\n# External")

	sb.RunCLIInDir(projectRoot, "init", "-p", "--targets", "claude").AssertSuccess(t)
	sb.RunCLIInDir(projectRoot, "install", filepath.Join(sb.Root, "external-skill"), "-p").AssertSuccess(t)

	metadata := filepath.Join(".skillshare", "skills", install.MetadataFileName)
	if !sb.FileExists(filepath.Join(projectRoot, metadata)) {
		t.Fatalf("install should write %s", metadata)
	}
	testutil.RunGit(t, projectRoot, "init", "-q")
	if err := exec.Command("git", "-C", projectRoot, "check-ignore", "-q", metadata).Run(); err != nil {
		t.Errorf("%s should be gitignored (git check-ignore: %v)", metadata, err)
	}
}

func TestInstallProject_RelativeLocalPath_StaysRelative(t *testing.T) {
	sb := testutil.NewSandbox(t)
	defer sb.Cleanup()
	projectRoot := sb.SetupProjectDir("claude")
	sb.WriteFile(filepath.Join(projectRoot, "vendor", "team-skill", "SKILL.md"), "---\nname: team-skill\n---\n# Team")

	sb.RunCLIInDir(projectRoot, "install", "./vendor/team-skill", "-p").AssertSuccess(t)

	// The project config is committed and replayed on other machines, so a
	// relative source must not be rewritten to this machine's absolute path.
	cfg := sb.ReadFile(filepath.Join(projectRoot, ".skillshare", "config.yaml"))
	if !strings.Contains(cfg, "source: ./vendor/team-skill") {
		t.Errorf("project config should keep the relative source, got:\n%s", cfg)
	}
}

func TestInstallProject_CurrentDirectoryRejected(t *testing.T) {
	sb := testutil.NewSandbox(t)
	defer sb.Cleanup()
	projectRoot := sb.SetupProjectDir("claude")

	os.WriteFile(filepath.Join(projectRoot, "SKILL.md"), []byte("---\nname: project-root\n---\n# Project"), 0644)

	result := sb.RunCLIInDir(projectRoot, "install", "./", "--name", "project-root", "-p", "--skip-audit")
	result.AssertFailure(t)
	result.AssertAnyOutputContains(t, "cannot install the project root into itself")
	result.AssertAnyOutputContains(t, "skillshare install ./my-skill -p")

	if sb.FileExists(filepath.Join(projectRoot, ".skillshare", "skills", "project-root")) {
		t.Fatal("project root install should not create a skill")
	}
}

func TestInstallProject_CustomName(t *testing.T) {
	sb := testutil.NewSandbox(t)
	defer sb.Cleanup()
	projectRoot := sb.SetupProjectDir("claude")

	sourceSkill := filepath.Join(sb.Root, "my-source")
	os.MkdirAll(sourceSkill, 0755)
	os.WriteFile(filepath.Join(sourceSkill, "SKILL.md"), []byte("---\nname: original\n---\n# S"), 0644)

	result := sb.RunCLIInDir(projectRoot, "install", sourceSkill, "--name", "custom", "-p")
	result.AssertSuccess(t)

	if !sb.FileExists(filepath.Join(projectRoot, ".skillshare", "skills", "custom", "SKILL.md")) {
		t.Error("skill should be installed with custom name")
	}
}

func TestInstallProject_DryRun(t *testing.T) {
	sb := testutil.NewSandbox(t)
	defer sb.Cleanup()
	projectRoot := sb.SetupProjectDir("claude")

	sourceSkill := filepath.Join(sb.Root, "dry-skill")
	os.MkdirAll(sourceSkill, 0755)
	os.WriteFile(filepath.Join(sourceSkill, "SKILL.md"), []byte("---\nname: dry\n---\n# D"), 0644)

	result := sb.RunCLIInDir(projectRoot, "install", sourceSkill, "--dry-run", "-p")
	result.AssertSuccess(t)

	if sb.FileExists(filepath.Join(projectRoot, ".skillshare", "skills", "dry")) {
		t.Error("dry-run should not install skill")
	}
}

func TestInstallProject_TrackRequiresGitSource(t *testing.T) {
	sb := testutil.NewSandbox(t)
	defer sb.Cleanup()
	projectRoot := sb.SetupProjectDir("claude")

	// --track with a non-git source should fail with git-related error
	result := sb.RunCLIInDir(projectRoot, "install", "/some/path", "--track", "-p")
	result.AssertFailure(t)
	result.AssertAnyOutputContains(t, "git repository source")
}

func TestInstallProject_FromConfig_SkipsExisting(t *testing.T) {
	sb := testutil.NewSandbox(t)
	defer sb.Cleanup()
	projectRoot := sb.SetupProjectDir("claude")

	// Create an already-installed skill
	sb.CreateProjectSkill(projectRoot, "already-here", map[string]string{
		"SKILL.md": "# Already",
	})

	// Write metadata entry so config-based install sees it
	skillsDir := filepath.Join(projectRoot, ".skillshare", "skills")
	store, err := install.LoadMetadata(skillsDir)
	if err != nil {
		t.Fatalf("failed to load metadata: %v", err)
	}
	store.Set("already-here", &install.MetadataEntry{
		Source: "someone/skills/already-here",
		Type:   "github",
	})
	if err := store.Save(skillsDir); err != nil {
		t.Fatalf("failed to save metadata: %v", err)
	}

	sb.WriteProjectConfig(projectRoot, `targets:
  - claude
skills:
  - name: already-here
    source: someone/skills/already-here
`)

	// install (no args) → should skip existing
	result := sb.RunCLIInDir(projectRoot, "install", "-p")
	result.AssertSuccess(t)
	result.AssertAnyOutputContains(t, "skipped")
}

func TestInstallProject_FromConfig_EmptySkills(t *testing.T) {
	sb := testutil.NewSandbox(t)
	defer sb.Cleanup()
	projectRoot := sb.SetupProjectDir("claude")

	result := sb.RunCLIInDir(projectRoot, "install", "-p")
	result.AssertSuccess(t)
	result.AssertAnyOutputContains(t, "No remote skills")
}

func TestInstallProject_NoSource_WithName_Errors(t *testing.T) {
	sb := testutil.NewSandbox(t)
	defer sb.Cleanup()
	projectRoot := sb.SetupProjectDir("claude")

	result := sb.RunCLIInDir(projectRoot, "install", "-p", "--name", "custom")
	result.AssertFailure(t)
	result.AssertAnyOutputContains(t, "require a source argument")
}

func TestInstallProject_NoSource_WithExclude_Errors(t *testing.T) {
	sb := testutil.NewSandbox(t)
	defer sb.Cleanup()
	projectRoot := sb.SetupProjectDir("claude")

	result := sb.RunCLIInDir(projectRoot, "install", "-p", "--exclude", "skip-me")
	result.AssertFailure(t)
	result.AssertAnyOutputContains(t, "require a source argument")
}

func TestInstallProject_AuditThresholdShortFlag_BlocksHighFinding(t *testing.T) {
	sb := testutil.NewSandbox(t)
	defer sb.Cleanup()
	projectRoot := sb.SetupProjectDir("claude")

	highPath := filepath.Join(sb.Root, "project-flag-high")
	os.MkdirAll(highPath, 0755)
	os.WriteFile(filepath.Join(highPath, "SKILL.md"),
		[]byte("---\nname: project-flag-high\n---\n# CI helper\nsudo apt-get install -y jq"), 0644)

	result := sb.RunCLIInDir(projectRoot, "install", highPath, "-p", "-T", "high")
	result.AssertFailure(t)
	result.AssertAnyOutputContains(t, "at/above HIGH")

	projectSkillPath := filepath.Join(projectRoot, ".skillshare", "skills", "project-flag-high", "SKILL.md")
	if sb.FileExists(projectSkillPath) {
		t.Error("project install should be blocked when -T high is set")
	}
}

// Regression for #280: config-driven `install -p` must not prune the skills:
// entries it just installed from config.yaml. Uses a subdir source so the
// skill is a plain copy (no .git) — the case the stale store cannot see.
func TestInstallProject_FromConfig_KeepsConfigSkills(t *testing.T) {
	sb := testutil.NewSandbox(t)
	defer sb.Cleanup()
	projectRoot := sb.SetupProjectDir("claude")

	repoPath := filepath.Join(sb.Root, "cfg-src")
	os.MkdirAll(filepath.Join(repoPath, "skills", "cfg-skill"), 0755)
	os.WriteFile(filepath.Join(repoPath, "skills", "cfg-skill", "SKILL.md"), []byte("---\nname: cfg-skill\n---\n# Cfg"), 0644)
	initGitRepo(t, repoPath)

	sb.WriteProjectConfig(projectRoot, `targets:
  - claude
skills:
  - name: cfg-skill
    source: file://`+repoPath+`//skills/cfg-skill
`)

	sb.RunCLIInDir(projectRoot, "install", "-p").AssertSuccess(t)

	if !sb.FileExists(filepath.Join(projectRoot, ".skillshare", "skills", "cfg-skill", "SKILL.md")) {
		t.Fatal("skill should be installed from config")
	}
	cfg := sb.ReadFile(filepath.Join(projectRoot, ".skillshare", "config.yaml"))
	if !strings.Contains(cfg, "name: cfg-skill") {
		t.Errorf("config.yaml lost the skills entry after install -p:\n%s", cfg)
	}
}

// Issue #858: installing a skill again from another source, after an uninstall
// or over the existing copy, must leave the new source in config.yaml and the
// lockfile, since those are what teammates install from.
func TestInstallProject_NewSource_ReplacesConfigSource(t *testing.T) {
	for _, viaUninstall := range []bool{true, false} {
		sb := testutil.NewSandbox(t)
		projectRoot := sb.SetupProjectDir("claude")
		oldRepo := createMultiSkillGitRepo(t, sb, "old-skills", []string{"dev-testy"})
		newRepo := createMultiSkillGitRepo(t, sb, "new-skills", []string{"dev-testy"})
		configPath := filepath.Join(projectRoot, ".skillshare", "config.yaml")

		sb.RunCLIInDir(projectRoot, "install", "file://"+oldRepo+"//dev-testy", "-p").AssertSuccess(t)
		reinstall := []string{"install", "file://" + newRepo, "-s", "dev-testy", "-p"}
		if viaUninstall {
			sb.RunCLIInDir(projectRoot, "uninstall", "dev-testy", "-p", "--force").AssertSuccess(t)
			if cfg := sb.ReadFile(configPath); strings.Contains(cfg, "dev-testy") {
				t.Errorf("uninstall left the skill in config.yaml:\n%s", cfg)
			}
		} else {
			reinstall = append(reinstall, "--force")
		}
		sb.RunCLIInDir(projectRoot, reinstall...).AssertSuccess(t)

		for _, file := range []string{configPath, filepath.Join(projectRoot, ".skillshare", "skills.lock.json")} {
			content := sb.ReadFile(file)
			if strings.Contains(content, oldRepo) || !strings.Contains(content, newRepo) {
				t.Errorf("viaUninstall=%v: %s should name only the new source:\n%s", viaUninstall, filepath.Base(file), content)
			}
		}
		sb.Cleanup()
	}
}

// On a fresh clone the config lists skills that are not installed yet. An
// install from a named source must not drop them from config.yaml.
func TestInstallProject_NamedSource_KeepsSkillsNotInstalledYet(t *testing.T) {
	sb := testutil.NewSandbox(t)
	defer sb.Cleanup()
	projectRoot := sb.SetupProjectDir("claude")
	repo := createMultiSkillGitRepo(t, sb, "team-skills", []string{"pending", "added"})
	sb.WriteProjectConfig(projectRoot, `targets:
  - claude
skills:
  - name: pending
    source: file://`+repo+`//pending
`)

	sb.RunCLIInDir(projectRoot, "install", "file://"+repo+"//added", "-p").AssertSuccess(t)

	cfg := sb.ReadFile(filepath.Join(projectRoot, ".skillshare", "config.yaml"))
	if !strings.Contains(cfg, "name: pending") || !strings.Contains(cfg, "name: added") {
		t.Errorf("config.yaml should list both skills:\n%s", cfg)
	}
}
