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

// setupSkillsOffGlobal writes a global config with gemini synced in merge mode
// and returns its skills folder after a first sync.
func setupSkillsOffGlobal(t *testing.T, sb *testutil.Sandbox) string {
	t.Helper()
	sb.CreateSkill("alpha", map[string]string{"SKILL.md": "---\nname: alpha\n---\n# Alpha"})
	sb.CreateSkill("beta", map[string]string{"SKILL.md": "---\nname: beta\n---\n# Beta"})
	gemini := sb.CreateTarget("gemini")
	sb.WriteConfig(`source: ` + sb.SourcePath + `
mode: merge
targets:
  gemini:
    skills:
      path: ` + gemini + `
`)
	sb.RunCLI("sync").AssertSuccess(t)
	if !sb.IsSymlink(filepath.Join(gemini, "alpha")) {
		t.Fatal("setup: alpha should be linked")
	}
	return gemini
}

func TestTargetSkillsOff_RemovesLinksKeepsLocal(t *testing.T) {
	sb := testutil.NewSandbox(t)
	defer sb.Cleanup()
	gemini := setupSkillsOffGlobal(t, sb)
	sb.WriteFile(filepath.Join(gemini, "mine", "SKILL.md"), "# mine")

	result := sb.RunCLI("target", "gemini", "--skills=false")
	result.AssertSuccess(t)
	result.AssertOutputContains(t, "Skills off for gemini")
	result.AssertOutputContains(t, "Removed   2 links  alpha, beta")
	result.AssertOutputContains(t, "Kept      1 local skill  mine")

	if sb.FileExists(filepath.Join(gemini, "alpha")) {
		t.Error("alpha link should be removed")
	}
	if !sb.FileExists(filepath.Join(gemini, "mine", "SKILL.md")) {
		t.Error("local skill must be kept")
	}
	if !strings.Contains(sb.ReadFile(sb.ConfigPath), "enabled: false") {
		t.Error("config should record skills.enabled: false")
	}

	// Sync leaves the folder alone while skills are off.
	sync := sb.RunCLI("sync")
	sync.AssertSuccess(t)
	sync.AssertRowContains(t, "gemini", "skills off")
	if sb.FileExists(filepath.Join(gemini, "alpha")) {
		t.Error("sync must not relink skills while off")
	}
}

func TestTargetSkillsOff_CopyModeListsCopiesApart(t *testing.T) {
	sb := testutil.NewSandbox(t)
	defer sb.Cleanup()
	sb.CreateSkill("alpha", map[string]string{"SKILL.md": "---\nname: alpha\n---\n# Alpha"})
	gemini := sb.CreateTarget("gemini")
	sb.WriteConfig(`source: ` + sb.SourcePath + `
mode: copy
targets:
  gemini:
    skills:
      path: ` + gemini + `
`)
	sb.RunCLI("sync").AssertSuccess(t)

	result := sb.RunCLI("target", "gemini", "--skills=false")
	result.AssertSuccess(t)
	result.AssertAnyOutputContains(t, "Kept      1 copied skill  alpha")
	result.AssertAnyOutputContains(t, "The tool still loads these copies")
	if !sb.FileExists(filepath.Join(gemini, "alpha", "SKILL.md")) {
		t.Error("the copy must be kept")
	}
}

func TestTargetSkillsOff_RefusesFilterFlags(t *testing.T) {
	sb := testutil.NewSandbox(t)
	defer sb.Cleanup()
	gemini := setupSkillsOffGlobal(t, sb)

	result := sb.RunCLI("target", "gemini", "--add-exclude", "alpha", "--skills=false")
	result.AssertFailure(t)
	result.AssertAnyOutputContains(t, "cannot be combined with include/exclude flags")
	if !sb.IsSymlink(filepath.Join(gemini, "alpha")) {
		t.Error("nothing should change when the flags are refused")
	}
}

func TestTargetSkillsOff_DryRunChangesNothing(t *testing.T) {
	sb := testutil.NewSandbox(t)
	defer sb.Cleanup()
	gemini := setupSkillsOffGlobal(t, sb)

	result := sb.RunCLI("target", "gemini", "--skills=false", "--dry-run")
	result.AssertSuccess(t)
	result.AssertOutputContains(t, "Would remove  2 links")
	if !sb.IsSymlink(filepath.Join(gemini, "alpha")) {
		t.Error("dry run must keep links")
	}
	if strings.Contains(sb.ReadFile(sb.ConfigPath), "enabled: false") {
		t.Error("dry run must not change config")
	}
}

func TestTargetSkillsOn_ResumesOnNextSync(t *testing.T) {
	sb := testutil.NewSandbox(t)
	defer sb.Cleanup()
	gemini := setupSkillsOffGlobal(t, sb)
	sb.RunCLI("target", "gemini", "--skills=false").AssertSuccess(t)

	result := sb.RunCLI("target", "gemini", "--skills=true")
	result.AssertSuccess(t)
	result.AssertOutputContains(t, "Skills on for gemini")
	if sb.FileExists(filepath.Join(gemini, "alpha")) {
		t.Error("turning on only changes config")
	}

	sb.RunCLI("sync").AssertSuccess(t)
	if !sb.IsSymlink(filepath.Join(gemini, "alpha")) {
		t.Error("sync should relink after turning skills on")
	}
}

func TestTargetList_JSONReportsSkillsOff(t *testing.T) {
	sb := testutil.NewSandbox(t)
	defer sb.Cleanup()
	setupSkillsOffGlobal(t, sb)
	sb.RunCLI("target", "gemini", "--skills=false").AssertSuccess(t)

	result := sb.RunCLI("target", "list", "--json")
	result.AssertSuccess(t)
	var out struct {
		Targets []struct {
			Name          string `json:"name"`
			SkillsEnabled bool   `json:"skillsEnabled"`
		} `json:"targets"`
	}
	if err := json.Unmarshal([]byte(result.Stdout), &out); err != nil {
		t.Fatalf("parse: %v\n%s", err, result.Stdout)
	}
	if len(out.Targets) != 1 || out.Targets[0].SkillsEnabled {
		t.Errorf("targets = %+v", out.Targets)
	}
}

func TestTargetAdd_NoSkills(t *testing.T) {
	sb := testutil.NewSandbox(t)
	defer sb.Cleanup()
	sb.CreateSkill("alpha", map[string]string{"SKILL.md": "---\nname: alpha\n---\n# Alpha"})
	sb.WriteConfig(`source: ` + sb.SourcePath + `
targets: {}
`)
	gemini := filepath.Join(sb.Home, ".gemini", "skills")

	result := sb.RunCLI("target", "add", "gemini", gemini, "--no-skills")
	result.AssertSuccess(t)
	result.AssertOutputContains(t, "skills off")
	if !strings.Contains(sb.ReadFile(sb.ConfigPath), "enabled: false") {
		t.Error("config should record skills.enabled: false")
	}

	sb.RunCLI("sync").AssertSuccess(t)
	if sb.FileExists(gemini) {
		t.Error("sync must not create the skills folder of a target with skills off")
	}
}

func TestTargetRemove_SkillsOffLeavesFolder(t *testing.T) {
	sb := testutil.NewSandbox(t)
	defer sb.Cleanup()
	gemini := setupSkillsOffGlobal(t, sb)
	sb.RunCLI("target", "gemini", "--skills=false").AssertSuccess(t)
	sb.WriteFile(filepath.Join(gemini, "mine", "SKILL.md"), "# mine")

	sb.RunCLI("target", "remove", "gemini").AssertSuccess(t)
	if !sb.FileExists(filepath.Join(gemini, "mine", "SKILL.md")) {
		t.Error("removing a target with skills off must not touch its folder")
	}
}

func TestTargetSkillsOff_Project(t *testing.T) {
	sb := testutil.NewSandbox(t)
	defer sb.Cleanup()
	projectRoot := sb.SetupProjectDir("claude")
	sb.CreateProjectSkill(projectRoot, "alpha", map[string]string{"SKILL.md": "---\nname: alpha\n---\n# Alpha"})
	sb.RunCLIInDir(projectRoot, "sync", "-p").AssertSuccess(t)
	link := filepath.Join(projectRoot, ".claude", "skills", "alpha")
	if !sb.IsSymlink(link) {
		t.Fatal("setup: alpha should be linked")
	}

	result := sb.RunCLIInDir(projectRoot, "target", "claude", "--skills=false", "-p")
	result.AssertSuccess(t)
	result.AssertOutputContains(t, "Skills off for claude")
	if _, err := os.Lstat(link); err == nil {
		t.Error("project link should be removed")
	}
	cfg := sb.ReadFile(filepath.Join(projectRoot, ".skillshare", "config.yaml"))
	if !strings.Contains(cfg, "enabled: false") {
		t.Errorf("project config should record enabled: false:\n%s", cfg)
	}

	sb.RunCLIInDir(projectRoot, "sync", "-p").AssertSuccess(t)
	if _, err := os.Lstat(link); err == nil {
		t.Error("project sync must not relink while off")
	}
}

func TestTargetProject_AddNoSkills(t *testing.T) {
	sb := testutil.NewSandbox(t)
	defer sb.Cleanup()
	projectRoot := sb.SetupProjectDir("claude")

	result := sb.RunCLIInDir(projectRoot, "target", "add", "gemini", "--no-skills", "-p")
	result.AssertSuccess(t)
	if sb.FileExists(filepath.Join(projectRoot, ".gemini", "skills")) {
		t.Error("add --no-skills must not create the skills folder")
	}
	cfg := sb.ReadFile(filepath.Join(projectRoot, ".skillshare", "config.yaml"))
	if !strings.Contains(cfg, "enabled: false") {
		t.Errorf("project config should record enabled: false:\n%s", cfg)
	}
}

func TestTargetSkillsOn_RefusesPrefixedNamingOutsideCopyMode(t *testing.T) {
	sb := testutil.NewSandbox(t)
	defer sb.Cleanup()
	targetPath := sb.CreateTarget("claude")
	sb.WriteConfig(`source: ` + sb.SourcePath + `
mode: merge
target_naming: prefixed
targets:
  claude:
    skills:
      path: ` + targetPath + `
      enabled: false
`)

	result := sb.RunCLI("target", "claude", "--skills=true")
	result.AssertFailure(t)
	result.AssertAnyOutputContains(t, "requires copy mode")
	result.AssertAnyOutputContains(t, "set --mode copy first")
}
