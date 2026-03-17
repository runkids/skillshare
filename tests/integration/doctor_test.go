//go:build !online

package integration

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"skillshare/internal/testutil"
)

func TestDoctor_AllGood_PassesAll(t *testing.T) {
	sb := testutil.NewSandbox(t)
	defer sb.Cleanup()

	sb.CreateSkill("skill1", map[string]string{
		"SKILL.md": "# Skill 1",
		// Include meta with correct file hash so integrity check passes
		".skillshare-meta.json": `{"source":"test","type":"local","installed_at":"2026-01-01T00:00:00Z","file_hashes":{"SKILL.md":"sha256:c90671f17f3b99f87d8fe1a542ee2d6829d2b2cfb7684d298e44c7591d8b0712"}}`,
	})
	targetPath := sb.CreateTarget("claude")

	// Initialize git and commit to avoid warnings
	cmd := exec.Command("git", "init")
	cmd.Dir = sb.SourcePath
	if err := cmd.Run(); err != nil {
		t.Skip("git not available")
	}

	cmd = exec.Command("git", "config", "user.email", "test@test.com")
	cmd.Dir = sb.SourcePath
	cmd.Run()

	cmd = exec.Command("git", "config", "user.name", "Test")
	cmd.Dir = sb.SourcePath
	cmd.Run()

	cmd = exec.Command("git", "add", "-A")
	cmd.Dir = sb.SourcePath
	cmd.Run()

	cmd = exec.Command("git", "commit", "-m", "initial")
	cmd.Dir = sb.SourcePath
	cmd.Run()

	// Create synced state
	os.Symlink(filepath.Join(sb.SourcePath, "skill1"), filepath.Join(targetPath, "skill1"))

	sb.WriteConfig(`source: ` + sb.SourcePath + `
targets:
  claude:
    path: ` + targetPath + `
`)

	result := sb.RunCLI("doctor")

	result.AssertSuccess(t)
	result.AssertOutputContains(t, "All checks passed")
}

func TestDoctor_NoConfig_ShowsError(t *testing.T) {
	sb := testutil.NewSandbox(t)
	defer sb.Cleanup()

	os.Remove(sb.ConfigPath)

	result := sb.RunCLI("doctor")

	result.AssertSuccess(t) // doctor doesn't return error, it reports issues
	result.AssertOutputContains(t, "not found")
}

func TestDoctor_NoSource_ShowsError(t *testing.T) {
	sb := testutil.NewSandbox(t)
	defer sb.Cleanup()

	// Remove source directory
	os.RemoveAll(sb.SourcePath)

	sb.WriteConfig(`source: ` + sb.SourcePath + `
targets: {}
`)

	result := sb.RunCLI("doctor")

	result.AssertSuccess(t)
	result.AssertOutputContains(t, "Source not found")
}

func TestDoctor_ChecksSymlinkSupport(t *testing.T) {
	sb := testutil.NewSandbox(t)
	defer sb.Cleanup()

	sb.WriteConfig(`source: ` + sb.SourcePath + `
targets: {}
`)

	result := sb.RunCLI("doctor")

	result.AssertSuccess(t)
	result.AssertOutputContains(t, "Symlink")
}

func TestDoctor_ShowsSymlinkCompatHint(t *testing.T) {
	sb := testutil.NewSandbox(t)
	defer sb.Cleanup()

	claudePath := sb.CreateTarget("claude")
	cursorPath := sb.CreateTarget("cursor")

	sb.WriteConfig(`source: ` + sb.SourcePath + `
mode: merge
targets:
  claude:
    path: ` + claudePath + `
  cursor:
    path: ` + cursorPath + `
`)

	result := sb.RunCLI("doctor")

	result.AssertSuccess(t)
	result.AssertOutputContains(t, "Symlink compatibility")
	result.AssertOutputContains(t, "--mode copy")
}

func TestDoctor_NoSymlinkCompatHint_WhenAllCopy(t *testing.T) {
	sb := testutil.NewSandbox(t)
	defer sb.Cleanup()

	claudePath := sb.CreateTarget("claude")
	codexPath := sb.CreateTarget("codex")

	sb.WriteConfig(`source: ` + sb.SourcePath + `
targets:
  claude:
    path: ` + claudePath + `
    mode: copy
  codex:
    path: ` + codexPath + `
    mode: copy
`)

	result := sb.RunCLI("doctor")

	result.AssertSuccess(t)
	result.AssertOutputNotContains(t, "Symlink compatibility")
}

func TestDoctor_TargetIssues_ShowsProblems(t *testing.T) {
	sb := testutil.NewSandbox(t)
	defer sb.Cleanup()

	// Point to non-existent directory with non-existent parent
	sb.WriteConfig(`source: ` + sb.SourcePath + `
targets:
  broken:
    path: /nonexistent/path/skills
`)

	result := sb.RunCLI("doctor")

	result.AssertSuccess(t)
	result.AssertAnyOutputContains(t, "parent directory not found")
}

func TestDoctor_WrongSymlink_ShowsWarning(t *testing.T) {
	sb := testutil.NewSandbox(t)
	defer sb.Cleanup()

	// Create wrong symlink
	wrongSource := filepath.Join(sb.Home, "wrong-source")
	os.MkdirAll(wrongSource, 0755)

	targetPath := filepath.Join(sb.Home, ".claude", "skills")
	os.MkdirAll(filepath.Dir(targetPath), 0755)
	os.Symlink(wrongSource, targetPath)

	sb.WriteConfig(`source: ` + sb.SourcePath + `
targets:
  claude:
    path: ` + targetPath + `
    mode: symlink
`)

	result := sb.RunCLI("doctor")

	result.AssertSuccess(t)
	result.AssertAnyOutputContains(t, "wrong location")
}

func TestDoctor_ShowsSkillCount(t *testing.T) {
	sb := testutil.NewSandbox(t)
	defer sb.Cleanup()

	sb.CreateSkill("skill1", map[string]string{"SKILL.md": "# Skill 1"})
	sb.CreateSkill("skill2", map[string]string{"SKILL.md": "# Skill 2"})
	sb.CreateSkill("skill3", map[string]string{"SKILL.md": "# Skill 3"})

	sb.WriteConfig(`source: ` + sb.SourcePath + `
targets: {}
`)

	result := sb.RunCLI("doctor")

	result.AssertSuccess(t)
	result.AssertOutputContains(t, "3 skills")
}

func TestDoctor_GitNotInitialized_ShowsWarning(t *testing.T) {
	sb := testutil.NewSandbox(t)
	defer sb.Cleanup()

	sb.WriteConfig(`source: ` + sb.SourcePath + `
targets: {}
`)

	result := sb.RunCLI("doctor")

	result.AssertSuccess(t)
	result.AssertOutputContains(t, "Git")
	result.AssertOutputContains(t, "not initialized")
}

func TestDoctor_GitInitialized_ShowsSuccess(t *testing.T) {
	sb := testutil.NewSandbox(t)
	defer sb.Cleanup()

	// Initialize git in source
	cmd := exec.Command("git", "init")
	cmd.Dir = sb.SourcePath
	if err := cmd.Run(); err != nil {
		t.Skip("git not available")
	}

	sb.WriteConfig(`source: ` + sb.SourcePath + `
targets: {}
`)

	result := sb.RunCLI("doctor")

	result.AssertSuccess(t)
	result.AssertOutputContains(t, "Git")
	result.AssertOutputContains(t, "initialized")
}

func TestDoctor_GitUncommittedChanges_ShowsWarning(t *testing.T) {
	sb := testutil.NewSandbox(t)
	defer sb.Cleanup()

	// Initialize git
	cmd := exec.Command("git", "init")
	cmd.Dir = sb.SourcePath
	if err := cmd.Run(); err != nil {
		t.Skip("git not available")
	}

	cmd = exec.Command("git", "config", "user.email", "test@test.com")
	cmd.Dir = sb.SourcePath
	cmd.Run()

	cmd = exec.Command("git", "config", "user.name", "Test")
	cmd.Dir = sb.SourcePath
	cmd.Run()

	// Create a skill (uncommitted)
	sb.CreateSkill("uncommitted", map[string]string{"SKILL.md": "# Uncommitted"})

	sb.WriteConfig(`source: ` + sb.SourcePath + `
targets: {}
`)

	result := sb.RunCLI("doctor")

	result.AssertSuccess(t)
	result.AssertOutputContains(t, "uncommitted")
}

func TestDoctor_SkillWithoutSKILLmd_ShowsWarning(t *testing.T) {
	sb := testutil.NewSandbox(t)
	defer sb.Cleanup()

	// Create a skill with SKILL.md
	sb.CreateSkill("valid-skill", map[string]string{"SKILL.md": "# Valid"})

	// Create a directory without SKILL.md
	invalidSkill := filepath.Join(sb.SourcePath, "invalid-skill")
	os.MkdirAll(invalidSkill, 0755)
	os.WriteFile(filepath.Join(invalidSkill, "README.md"), []byte("# No SKILL.md"), 0644)

	sb.WriteConfig(`source: ` + sb.SourcePath + `
targets: {}
`)

	result := sb.RunCLI("doctor")

	result.AssertSuccess(t)
	result.AssertOutputContains(t, "without SKILL.md")
	result.AssertOutputContains(t, "invalid-skill")
}

func TestDoctor_GroupContainerWithoutSKILLmd_DoesNotWarn(t *testing.T) {
	sb := testutil.NewSandbox(t)
	defer sb.Cleanup()

	// Group container directories (e.g. devops/, security/) may hold nested skills
	// and should not be treated as invalid top-level skills.
	sb.CreateNestedSkill("devops/deploy", map[string]string{"SKILL.md": "# Deploy"})
	sb.CreateNestedSkill("security/audit", map[string]string{"SKILL.md": "# Audit"})

	sb.WriteConfig(`source: ` + sb.SourcePath + `
targets: {}
`)

	result := sb.RunCLI("doctor")

	result.AssertSuccess(t)
	result.AssertOutputNotContains(t, "Skills without SKILL.md")
}

func TestDoctor_BrokenSymlink_ShowsError(t *testing.T) {
	sb := testutil.NewSandbox(t)
	defer sb.Cleanup()

	targetPath := sb.CreateTarget("claude")

	// Create a broken symlink
	brokenLink := filepath.Join(targetPath, "broken-skill")
	os.Symlink("/nonexistent/path", brokenLink)

	sb.WriteConfig(`source: ` + sb.SourcePath + `
targets:
  claude:
    path: ` + targetPath + `
`)

	result := sb.RunCLI("doctor")

	result.AssertSuccess(t)
	result.AssertAnyOutputContains(t, "broken symlink")
}

func TestDoctor_DuplicateSkills_ShowsWarning(t *testing.T) {
	sb := testutil.NewSandbox(t)
	defer sb.Cleanup()

	// Create skill in source
	sb.CreateSkill("duplicate-skill", map[string]string{"SKILL.md": "# Source"})

	// Create target with local skill of same name (not symlink)
	// Use symlink mode - merge mode allows local skills by design
	targetPath := sb.CreateTarget("claude")
	localSkill := filepath.Join(targetPath, "duplicate-skill")
	os.MkdirAll(localSkill, 0755)
	os.WriteFile(filepath.Join(localSkill, "SKILL.md"), []byte("# Local"), 0644)

	sb.WriteConfig(`source: ` + sb.SourcePath + `
targets:
  claude:
    path: ` + targetPath + `
    mode: symlink
`)

	result := sb.RunCLI("doctor")

	result.AssertSuccess(t)
	result.AssertOutputContains(t, "Duplicate")
	result.AssertOutputContains(t, "duplicate-skill")
}

func TestDoctor_CopyModeManagedSkills_NoDuplicateWarning(t *testing.T) {
	sb := testutil.NewSandbox(t)
	defer sb.Cleanup()

	sb.CreateSkill("duplicate-skill", map[string]string{"SKILL.md": "# Source"})
	targetPath := sb.CreateTarget("copilot")

	sb.WriteConfig(`source: ` + sb.SourcePath + `
targets:
  copilot:
    path: ` + targetPath + `
    mode: copy
`)

	syncResult := sb.RunCLI("sync")
	syncResult.AssertSuccess(t)

	result := sb.RunCLI("doctor")
	result.AssertSuccess(t)
	result.AssertOutputNotContains(t, "Duplicate skills")
}

func TestDoctor_BackupExists_ShowsInfo(t *testing.T) {
	sb := testutil.NewSandbox(t)
	defer sb.Cleanup()

	// Create backup directory
	backupDir := filepath.Join(filepath.Dir(sb.ConfigPath), "backups", "2026-01-16_12-00-00")
	os.MkdirAll(backupDir, 0755)
	os.WriteFile(filepath.Join(backupDir, "test"), []byte("backup"), 0644)

	sb.WriteConfig(`source: ` + sb.SourcePath + `
targets: {}
`)

	result := sb.RunCLI("doctor")

	result.AssertSuccess(t)
	result.AssertOutputContains(t, "Backup")
}

func TestDoctor_NoBackups_ShowsNone(t *testing.T) {
	sb := testutil.NewSandbox(t)
	defer sb.Cleanup()

	sb.WriteConfig(`source: ` + sb.SourcePath + `
targets: {}
`)

	result := sb.RunCLI("doctor")

	result.AssertSuccess(t)
	result.AssertOutputContains(t, "Backup")
	result.AssertOutputContains(t, "none")
}

func TestDoctor_ProjectMode_AutoDetectsProjectConfig(t *testing.T) {
	sb := testutil.NewSandbox(t)
	defer sb.Cleanup()

	projectRoot := sb.SetupProjectDir("claude")
	sb.CreateProjectSkill(projectRoot, "project-skill", map[string]string{"SKILL.md": "# Project Skill"})

	result := sb.RunCLIInDir(projectRoot, "doctor")

	result.AssertSuccess(t)
	result.AssertOutputContains(t, "(project)")
	result.AssertOutputContains(t, ".skillshare/config.yaml")
	result.AssertOutputContains(t, ".skillshare/skills")
	result.AssertOutputContains(t, "Backups: not used in project mode")
}

func TestDoctor_ProjectMode_WithFlagUsesProjectConfig(t *testing.T) {
	sb := testutil.NewSandbox(t)
	defer sb.Cleanup()

	projectRoot := sb.SetupProjectDir("claude")
	sb.CreateProjectSkill(projectRoot, "project-skill", map[string]string{"SKILL.md": "# Project Skill"})

	result := sb.RunCLIInDir(projectRoot, "doctor", "-p")

	result.AssertSuccess(t)
	result.AssertOutputContains(t, "(project)")
	result.AssertOutputContains(t, ".skillshare/config.yaml")
	result.AssertOutputContains(t, ".skillshare/skills")
}

func TestDoctor_CustomTarget_NoUnknownWarning(t *testing.T) {
	sb := testutil.NewSandbox(t)
	defer sb.Cleanup()

	customPath := filepath.Join(sb.Home, ".custom-tool", "skills")
	os.MkdirAll(customPath, 0755)

	sb.CreateSkill("my-skill", map[string]string{
		"SKILL.md": "---\nname: my-skill\ntargets: [claude, custom-tool]\n---\n# My Skill",
	})

	sb.WriteConfig(`source: ` + sb.SourcePath + `
targets:
  claude:
    path: ` + sb.CreateTarget("claude") + `
  custom-tool:
    path: ` + customPath + `
`)

	result := sb.RunCLI("doctor")

	result.AssertSuccess(t)
	result.AssertOutputNotContains(t, "unknown target")
}

func TestDoctor_CustomTarget_ProjectMode_NoUnknownWarning(t *testing.T) {
	sb := testutil.NewSandbox(t)
	defer sb.Cleanup()

	customPath := filepath.Join(sb.Home, ".custom-tool", "skills")
	os.MkdirAll(customPath, 0755)

	projectRoot := sb.SetupProjectDir("claude")
	sb.CreateProjectSkill(projectRoot, "my-skill", map[string]string{
		"SKILL.md": "---\nname: my-skill\ntargets: [claude, custom-tool]\n---\n# My Skill",
	})
	sb.WriteProjectConfig(projectRoot, `targets:
  - claude
  - name: custom-tool
    path: `+customPath+`
`)

	result := sb.RunCLIInDir(projectRoot, "doctor", "-p")

	result.AssertSuccess(t)
	result.AssertOutputNotContains(t, "unknown target")
}

// --- JSON output tests ---

// doctorJSON mirrors the JSON structure emitted by `doctor --json`.
type doctorJSON struct {
	Checks  []doctorJSONCheck  `json:"checks"`
	Summary doctorJSONSummary  `json:"summary"`
	Version *doctorJSONVersion `json:"version,omitempty"`
}

type doctorJSONCheck struct {
	Name    string   `json:"name"`
	Status  string   `json:"status"`
	Message string   `json:"message"`
	Details []string `json:"details,omitempty"`
}

type doctorJSONSummary struct {
	Total    int `json:"total"`
	Pass     int `json:"pass"`
	Warnings int `json:"warnings"`
	Errors   int `json:"errors"`
}

type doctorJSONVersion struct {
	Current         string `json:"current"`
	Latest          string `json:"latest,omitempty"`
	UpdateAvailable bool   `json:"update_available"`
}

func parseDoctorJSON(t *testing.T, stdout string) doctorJSON {
	t.Helper()
	var out doctorJSON
	if err := json.Unmarshal([]byte(stdout), &out); err != nil {
		t.Fatalf("failed to parse doctor JSON: %v\nraw output:\n%s", err, stdout)
	}
	return out
}

func TestDoctor_JSON_AllGood(t *testing.T) {
	sb := testutil.NewSandbox(t)
	defer sb.Cleanup()

	sb.CreateSkill("skill1", map[string]string{
		"SKILL.md": "# Skill 1",
		".skillshare-meta.json": `{"source":"test","type":"local","installed_at":"2026-01-01T00:00:00Z","file_hashes":{"SKILL.md":"sha256:c90671f17f3b99f87d8fe1a542ee2d6829d2b2cfb7684d298e44c7591d8b0712"}}`,
	})
	targetPath := sb.CreateTarget("claude")

	// Initialize git and commit to avoid warnings
	cmd := exec.Command("git", "init")
	cmd.Dir = sb.SourcePath
	if err := cmd.Run(); err != nil {
		t.Skip("git not available")
	}

	cmd = exec.Command("git", "config", "user.email", "test@test.com")
	cmd.Dir = sb.SourcePath
	cmd.Run()

	cmd = exec.Command("git", "config", "user.name", "Test")
	cmd.Dir = sb.SourcePath
	cmd.Run()

	cmd = exec.Command("git", "add", "-A")
	cmd.Dir = sb.SourcePath
	cmd.Run()

	cmd = exec.Command("git", "commit", "-m", "initial")
	cmd.Dir = sb.SourcePath
	cmd.Run()

	// Create synced symlink
	os.Symlink(filepath.Join(sb.SourcePath, "skill1"), filepath.Join(targetPath, "skill1"))

	sb.WriteConfig(`source: ` + sb.SourcePath + `
targets:
  claude:
    path: ` + targetPath + `
`)

	result := sb.RunCLI("doctor", "--json")

	result.AssertSuccess(t)
	out := parseDoctorJSON(t, result.Stdout)
	if out.Summary.Errors != 0 {
		t.Errorf("expected 0 errors, got %d", out.Summary.Errors)
	}
	if out.Summary.Pass == 0 {
		t.Error("expected at least one passing check")
	}
	if out.Version == nil || out.Version.Current == "" {
		t.Error("expected non-empty version.current")
	}
}

func TestDoctor_JSON_WithErrors(t *testing.T) {
	sb := testutil.NewSandbox(t)
	defer sb.Cleanup()

	sb.CreateSkill("skill1", map[string]string{"SKILL.md": "# Skill 1"})

	// Point target to non-existent path with non-existent parent
	sb.WriteConfig(`source: ` + sb.SourcePath + `
targets:
  broken:
    path: /nonexistent/path/skills
`)

	result := sb.RunCLI("doctor", "--json")

	if result.ExitCode == 0 {
		t.Error("expected non-zero exit code when errors are present")
	}
	out := parseDoctorJSON(t, result.Stdout)
	if out.Summary.Errors == 0 {
		t.Error("expected summary.errors > 0")
	}
}

func TestDoctor_JSON_WithWarnings(t *testing.T) {
	sb := testutil.NewSandbox(t)
	defer sb.Cleanup()

	// Create a directory without SKILL.md to trigger skills_validity warning
	invalidSkill := filepath.Join(sb.SourcePath, "no-skillmd")
	os.MkdirAll(invalidSkill, 0755)
	os.WriteFile(filepath.Join(invalidSkill, "README.md"), []byte("# No SKILL.md"), 0644)

	sb.WriteConfig(`source: ` + sb.SourcePath + `
targets: {}
`)

	result := sb.RunCLI("doctor", "--json")

	// Warnings don't cause failure
	result.AssertSuccess(t)
	out := parseDoctorJSON(t, result.Stdout)
	if out.Summary.Warnings == 0 {
		t.Error("expected summary.warnings > 0")
	}
}

func TestDoctor_JSON_ProjectMode(t *testing.T) {
	sb := testutil.NewSandbox(t)
	defer sb.Cleanup()

	projectRoot := sb.SetupProjectDir("claude")
	sb.CreateProjectSkill(projectRoot, "project-skill", map[string]string{"SKILL.md": "# Project Skill"})

	result := sb.RunCLIInDir(projectRoot, "doctor", "--json")

	result.AssertSuccess(t)
	out := parseDoctorJSON(t, result.Stdout)

	// Project mode should not include git_status check
	for _, c := range out.Checks {
		if c.Name == "git_status" {
			t.Error("expected no git_status check in project mode")
		}
	}
}

func TestDoctor_JSON_HasVersionField(t *testing.T) {
	sb := testutil.NewSandbox(t)
	defer sb.Cleanup()

	sb.CreateSkill("skill1", map[string]string{"SKILL.md": "# Skill 1"})
	targetPath := sb.CreateTarget("claude")

	sb.WriteConfig(`source: ` + sb.SourcePath + `
targets:
  claude:
    path: ` + targetPath + `
`)

	result := sb.RunCLI("doctor", "--json")

	result.AssertSuccess(t)
	out := parseDoctorJSON(t, result.Stdout)
	if out.Version == nil {
		t.Fatal("expected version field to be present")
	}
	if out.Version.Current == "" {
		t.Error("expected version.current to be non-empty")
	}
}

func TestDoctor_JSON_HasBackupTrash(t *testing.T) {
	sb := testutil.NewSandbox(t)
	defer sb.Cleanup()

	sb.CreateSkill("skill1", map[string]string{"SKILL.md": "# Skill 1"})
	targetPath := sb.CreateTarget("claude")

	sb.WriteConfig(`source: ` + sb.SourcePath + `
targets:
  claude:
    path: ` + targetPath + `
`)

	result := sb.RunCLI("doctor", "--json")

	result.AssertSuccess(t)
	out := parseDoctorJSON(t, result.Stdout)

	hasBackup := false
	hasTrash := false
	for _, c := range out.Checks {
		if c.Name == "backup" {
			hasBackup = true
		}
		if c.Name == "trash" {
			hasTrash = true
		}
	}
	if !hasBackup {
		t.Error("expected checks to contain an entry with name \"backup\"")
	}
	if !hasTrash {
		t.Error("expected checks to contain an entry with name \"trash\"")
	}
}
