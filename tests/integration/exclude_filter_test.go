//go:build !online

package integration

import (
	"os"
	"path/filepath"
	"testing"

	"skillshare/internal/testutil"
)

func TestExclude_GlobalMerge_SkipsExcludedOnFirstSync(t *testing.T) {
	sb := testutil.NewSandbox(t)
	defer sb.Cleanup()

	sb.CreateSkill("keep-me", map[string]string{"SKILL.md": "# Keep"})
	sb.CreateSkill("exclude-me", map[string]string{"SKILL.md": "# Exclude"})
	targetPath := sb.CreateTarget("claude")

	sb.WriteConfig(`source: ` + sb.SourcePath + `
mode: merge
targets:
  claude:
    path: ` + targetPath + `
    exclude: [exclude-*]
`)

	result := sb.RunCLI("sync")
	result.AssertSuccess(t)

	if !sb.IsSymlink(filepath.Join(targetPath, "keep-me")) {
		t.Error("non-excluded skill should be linked")
	}
	if sb.FileExists(filepath.Join(targetPath, "exclude-me")) {
		t.Error("excluded skill should not be synced")
	}
}

func TestExclude_GlobalMerge_RemovesExistingSourceLink(t *testing.T) {
	sb := testutil.NewSandbox(t)
	defer sb.Cleanup()

	sb.CreateSkill("keep-me", map[string]string{"SKILL.md": "# Keep"})
	sb.CreateSkill("exclude-me", map[string]string{"SKILL.md": "# Exclude"})
	targetPath := sb.CreateTarget("claude")

	// First sync without filters creates links for both skills.
	sb.WriteConfig(`source: ` + sb.SourcePath + `
mode: merge
targets:
  claude:
    path: ` + targetPath + `
`)
	sb.RunCLI("sync").AssertSuccess(t)

	// Add exclude and sync again.
	sb.WriteConfig(`source: ` + sb.SourcePath + `
mode: merge
targets:
  claude:
    path: ` + targetPath + `
    exclude: [exclude-*]
`)
	result := sb.RunCLI("sync")
	result.AssertSuccess(t)

	if !sb.IsSymlink(filepath.Join(targetPath, "keep-me")) {
		t.Error("non-excluded skill should remain linked")
	}
	if sb.FileExists(filepath.Join(targetPath, "exclude-me")) {
		t.Error("excluded existing source-linked entry should be removed")
	}
}

func TestExclude_GlobalMerge_PreservesLocalDirectory(t *testing.T) {
	sb := testutil.NewSandbox(t)
	defer sb.Cleanup()

	sb.CreateSkill("exclude-me", map[string]string{"SKILL.md": "# Source"})
	targetPath := sb.CreateTarget("claude")

	localDir := filepath.Join(targetPath, "exclude-me")
	if err := os.MkdirAll(localDir, 0755); err != nil {
		t.Fatalf("mkdir local dir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(localDir, "SKILL.md"), []byte("# Local"), 0644); err != nil {
		t.Fatalf("write local SKILL.md: %v", err)
	}

	sb.WriteConfig(`source: ` + sb.SourcePath + `
mode: merge
targets:
  claude:
    path: ` + targetPath + `
    exclude: [exclude-*]
`)

	result := sb.RunCLI("sync")
	result.AssertSuccess(t)

	if !sb.FileExists(localDir) {
		t.Error("local directory should be preserved")
	}
	if sb.IsSymlink(localDir) {
		t.Error("local directory should not be converted to symlink")
	}
}

func TestExclude_GlobalSymlinkMode_Ignored(t *testing.T) {
	sb := testutil.NewSandbox(t)
	defer sb.Cleanup()

	sb.CreateSkill("exclude-me", map[string]string{"SKILL.md": "# Source"})
	targetPath := filepath.Join(sb.Home, ".claude", "skills")
	os.RemoveAll(targetPath)

	sb.WriteConfig(`source: ` + sb.SourcePath + `
targets:
  claude:
    path: ` + targetPath + `
    mode: symlink
    exclude: [exclude-*]
`)

	result := sb.RunCLI("sync")
	result.AssertSuccess(t)

	if !sb.IsSymlink(targetPath) {
		t.Error("target should be symlink in symlink mode")
	}
	if !sb.FileExists(filepath.Join(targetPath, "exclude-me", "SKILL.md")) {
		t.Error("exclude should be ignored in symlink mode")
	}
}

func TestExclude_GlobalMerge_InvalidExcludePatternFails(t *testing.T) {
	sb := testutil.NewSandbox(t)
	defer sb.Cleanup()

	sb.CreateSkill("skill-a", map[string]string{"SKILL.md": "# Skill"})
	targetPath := sb.CreateTarget("claude")

	sb.WriteConfig(`source: ` + sb.SourcePath + `
mode: merge
targets:
  claude:
    path: ` + targetPath + `
    exclude: ["["]
`)

	result := sb.RunCLI("sync")
	result.AssertFailure(t)
	result.AssertAnyOutputContains(t, "invalid exclude pattern")
}

func TestExclude_ProjectMerge_SkipsExcludedOnFirstSync(t *testing.T) {
	sb := testutil.NewSandbox(t)
	defer sb.Cleanup()

	projectRoot := sb.SetupProjectDir("claude-code")
	sb.CreateProjectSkill(projectRoot, "keep-me", map[string]string{"SKILL.md": "# Keep"})
	sb.CreateProjectSkill(projectRoot, "exclude-me", map[string]string{"SKILL.md": "# Exclude"})
	sb.WriteProjectConfig(projectRoot, `targets:
  - name: claude-code
    exclude: [exclude-*]
`)

	result := sb.RunCLIInDir(projectRoot, "sync", "-p")
	result.AssertSuccess(t)

	targetPath := filepath.Join(projectRoot, ".claude", "skills")
	if !sb.IsSymlink(filepath.Join(targetPath, "keep-me")) {
		t.Error("non-excluded project skill should be linked")
	}
	if sb.FileExists(filepath.Join(targetPath, "exclude-me")) {
		t.Error("excluded project skill should not be synced")
	}
}

func TestExclude_ProjectMerge_RemovesExistingSourceLink(t *testing.T) {
	sb := testutil.NewSandbox(t)
	defer sb.Cleanup()

	projectRoot := sb.SetupProjectDir("claude-code")
	sb.CreateProjectSkill(projectRoot, "keep-me", map[string]string{"SKILL.md": "# Keep"})
	sb.CreateProjectSkill(projectRoot, "exclude-me", map[string]string{"SKILL.md": "# Exclude"})

	// First sync without filters.
	sb.WriteProjectConfig(projectRoot, `targets:
  - claude-code
`)
	sb.RunCLIInDir(projectRoot, "sync", "-p").AssertSuccess(t)

	// Add exclude and sync again.
	sb.WriteProjectConfig(projectRoot, `targets:
  - name: claude-code
    exclude: [exclude-*]
`)
	result := sb.RunCLIInDir(projectRoot, "sync", "-p")
	result.AssertSuccess(t)

	targetPath := filepath.Join(projectRoot, ".claude", "skills")
	if !sb.IsSymlink(filepath.Join(targetPath, "keep-me")) {
		t.Error("non-excluded project skill should remain linked")
	}
	if sb.FileExists(filepath.Join(targetPath, "exclude-me")) {
		t.Error("excluded existing project source-linked entry should be removed")
	}
}

func TestExclude_ProjectMerge_PreservesLocalDirectory(t *testing.T) {
	sb := testutil.NewSandbox(t)
	defer sb.Cleanup()

	projectRoot := sb.SetupProjectDir("claude-code")
	sb.CreateProjectSkill(projectRoot, "exclude-me", map[string]string{"SKILL.md": "# Source"})
	sb.WriteProjectConfig(projectRoot, `targets:
  - name: claude-code
    exclude: [exclude-*]
`)

	targetPath := filepath.Join(projectRoot, ".claude", "skills")
	localDir := filepath.Join(targetPath, "exclude-me")
	if err := os.MkdirAll(localDir, 0755); err != nil {
		t.Fatalf("mkdir local dir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(localDir, "SKILL.md"), []byte("# Local"), 0644); err != nil {
		t.Fatalf("write local SKILL.md: %v", err)
	}

	result := sb.RunCLIInDir(projectRoot, "sync", "-p")
	result.AssertSuccess(t)

	if !sb.FileExists(localDir) {
		t.Error("local project directory should be preserved")
	}
	if sb.IsSymlink(localDir) {
		t.Error("local project directory should not become symlink")
	}
}

func TestExclude_ProjectMerge_InvalidExcludePatternFails(t *testing.T) {
	sb := testutil.NewSandbox(t)
	defer sb.Cleanup()

	projectRoot := sb.SetupProjectDir("claude-code")
	sb.CreateProjectSkill(projectRoot, "skill-a", map[string]string{"SKILL.md": "# Skill"})
	sb.WriteProjectConfig(projectRoot, `targets:
  - name: claude-code
    exclude: ["["]
`)

	result := sb.RunCLIInDir(projectRoot, "sync", "-p")
	result.AssertFailure(t)
	result.AssertAnyOutputContains(t, "invalid exclude pattern")
}

func TestInclude_GlobalMerge_SkipsNonIncludedOnFirstSync(t *testing.T) {
	sb := testutil.NewSandbox(t)
	defer sb.Cleanup()

	sb.CreateSkill("include-me", map[string]string{"SKILL.md": "# Include"})
	sb.CreateSkill("other-skill", map[string]string{"SKILL.md": "# Other"})
	targetPath := sb.CreateTarget("claude")

	sb.WriteConfig(`source: ` + sb.SourcePath + `
mode: merge
targets:
  claude:
    path: ` + targetPath + `
    include: [include-*]
`)

	result := sb.RunCLI("sync")
	result.AssertSuccess(t)

	if !sb.IsSymlink(filepath.Join(targetPath, "include-me")) {
		t.Error("included skill should be linked")
	}
	if sb.FileExists(filepath.Join(targetPath, "other-skill")) {
		t.Error("non-included skill should not be synced")
	}
}

func TestInclude_GlobalMerge_RemovesExistingSourceLinkOutsideInclude(t *testing.T) {
	sb := testutil.NewSandbox(t)
	defer sb.Cleanup()

	sb.CreateSkill("include-me", map[string]string{"SKILL.md": "# Include"})
	sb.CreateSkill("other-skill", map[string]string{"SKILL.md": "# Other"})
	targetPath := sb.CreateTarget("claude")

	sb.WriteConfig(`source: ` + sb.SourcePath + `
mode: merge
targets:
  claude:
    path: ` + targetPath + `
`)
	sb.RunCLI("sync").AssertSuccess(t)

	sb.WriteConfig(`source: ` + sb.SourcePath + `
mode: merge
targets:
  claude:
    path: ` + targetPath + `
    include: [include-*]
`)
	result := sb.RunCLI("sync")
	result.AssertSuccess(t)

	if !sb.IsSymlink(filepath.Join(targetPath, "include-me")) {
		t.Error("included skill should remain linked")
	}
	if sb.FileExists(filepath.Join(targetPath, "other-skill")) {
		t.Error("source-linked skill outside include should be removed")
	}
}

func TestInclude_GlobalMerge_PreservesLocalDirectoryOutsideInclude(t *testing.T) {
	sb := testutil.NewSandbox(t)
	defer sb.Cleanup()

	sb.CreateSkill("include-me", map[string]string{"SKILL.md": "# Include"})
	sb.CreateSkill("other-skill", map[string]string{"SKILL.md": "# Source"})
	targetPath := sb.CreateTarget("claude")

	localDir := filepath.Join(targetPath, "other-skill")
	if err := os.MkdirAll(localDir, 0755); err != nil {
		t.Fatalf("mkdir local dir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(localDir, "SKILL.md"), []byte("# Local"), 0644); err != nil {
		t.Fatalf("write local SKILL.md: %v", err)
	}

	sb.WriteConfig(`source: ` + sb.SourcePath + `
mode: merge
targets:
  claude:
    path: ` + targetPath + `
    include: [include-*]
`)

	result := sb.RunCLI("sync")
	result.AssertSuccess(t)

	if !sb.IsSymlink(filepath.Join(targetPath, "include-me")) {
		t.Error("included skill should be linked")
	}
	if !sb.FileExists(localDir) {
		t.Error("local directory outside include should be preserved")
	}
	if sb.IsSymlink(localDir) {
		t.Error("local directory should not be converted to symlink")
	}
}

func TestIncludeExclude_GlobalMerge_PrecedenceAndRemoval(t *testing.T) {
	sb := testutil.NewSandbox(t)
	defer sb.Cleanup()

	sb.CreateSkill("app-main", map[string]string{"SKILL.md": "# Main"})
	sb.CreateSkill("app-beta", map[string]string{"SKILL.md": "# Beta"})
	sb.CreateSkill("tool-main", map[string]string{"SKILL.md": "# Tool"})
	targetPath := sb.CreateTarget("claude")

	// First sync without filters links all source skills.
	sb.WriteConfig(`source: ` + sb.SourcePath + `
mode: merge
targets:
  claude:
    path: ` + targetPath + `
`)
	sb.RunCLI("sync").AssertSuccess(t)

	// include first, then exclude.
	sb.WriteConfig(`source: ` + sb.SourcePath + `
mode: merge
targets:
  claude:
    path: ` + targetPath + `
    include: [app-*]
    exclude: ["*-beta"]
`)
	result := sb.RunCLI("sync")
	result.AssertSuccess(t)

	if !sb.IsSymlink(filepath.Join(targetPath, "app-main")) {
		t.Error("app-main should remain linked")
	}
	if sb.FileExists(filepath.Join(targetPath, "app-beta")) {
		t.Error("app-beta should be excluded after include")
	}
	if sb.FileExists(filepath.Join(targetPath, "tool-main")) {
		t.Error("tool-main outside include should be removed")
	}
}

func TestInclude_ProjectMerge_RemovesExistingSourceLinkOutsideInclude(t *testing.T) {
	sb := testutil.NewSandbox(t)
	defer sb.Cleanup()

	projectRoot := sb.SetupProjectDir("claude-code")
	sb.CreateProjectSkill(projectRoot, "include-me", map[string]string{"SKILL.md": "# Include"})
	sb.CreateProjectSkill(projectRoot, "other-skill", map[string]string{"SKILL.md": "# Other"})

	sb.WriteProjectConfig(projectRoot, `targets:
  - claude-code
`)
	sb.RunCLIInDir(projectRoot, "sync", "-p").AssertSuccess(t)

	sb.WriteProjectConfig(projectRoot, `targets:
  - name: claude-code
    include: [include-*]
`)
	result := sb.RunCLIInDir(projectRoot, "sync", "-p")
	result.AssertSuccess(t)

	targetPath := filepath.Join(projectRoot, ".claude", "skills")
	if !sb.IsSymlink(filepath.Join(targetPath, "include-me")) {
		t.Error("included project skill should remain linked")
	}
	if sb.FileExists(filepath.Join(targetPath, "other-skill")) {
		t.Error("project source-linked skill outside include should be removed")
	}
}

func TestInclude_ProjectMerge_PreservesLocalDirectoryOutsideInclude(t *testing.T) {
	sb := testutil.NewSandbox(t)
	defer sb.Cleanup()

	projectRoot := sb.SetupProjectDir("claude-code")
	sb.CreateProjectSkill(projectRoot, "include-me", map[string]string{"SKILL.md": "# Include"})
	sb.CreateProjectSkill(projectRoot, "other-skill", map[string]string{"SKILL.md": "# Source"})
	sb.WriteProjectConfig(projectRoot, `targets:
  - name: claude-code
    include: [include-*]
`)

	targetPath := filepath.Join(projectRoot, ".claude", "skills")
	localDir := filepath.Join(targetPath, "other-skill")
	if err := os.MkdirAll(localDir, 0755); err != nil {
		t.Fatalf("mkdir local dir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(localDir, "SKILL.md"), []byte("# Local"), 0644); err != nil {
		t.Fatalf("write local SKILL.md: %v", err)
	}

	result := sb.RunCLIInDir(projectRoot, "sync", "-p")
	result.AssertSuccess(t)

	if !sb.IsSymlink(filepath.Join(targetPath, "include-me")) {
		t.Error("included project skill should be linked")
	}
	if !sb.FileExists(localDir) {
		t.Error("local project directory outside include should be preserved")
	}
	if sb.IsSymlink(localDir) {
		t.Error("local project directory should not become symlink")
	}
}

func TestIncludeExclude_ProjectMerge_PrecedenceAndRemoval(t *testing.T) {
	sb := testutil.NewSandbox(t)
	defer sb.Cleanup()

	projectRoot := sb.SetupProjectDir("claude-code")
	sb.CreateProjectSkill(projectRoot, "app-main", map[string]string{"SKILL.md": "# Main"})
	sb.CreateProjectSkill(projectRoot, "app-beta", map[string]string{"SKILL.md": "# Beta"})
	sb.CreateProjectSkill(projectRoot, "tool-main", map[string]string{"SKILL.md": "# Tool"})

	sb.WriteProjectConfig(projectRoot, `targets:
  - claude-code
`)
	sb.RunCLIInDir(projectRoot, "sync", "-p").AssertSuccess(t)

	sb.WriteProjectConfig(projectRoot, `targets:
  - name: claude-code
    include: [app-*]
    exclude: ["*-beta"]
`)
	result := sb.RunCLIInDir(projectRoot, "sync", "-p")
	result.AssertSuccess(t)

	targetPath := filepath.Join(projectRoot, ".claude", "skills")
	if !sb.IsSymlink(filepath.Join(targetPath, "app-main")) {
		t.Error("app-main should remain linked")
	}
	if sb.FileExists(filepath.Join(targetPath, "app-beta")) {
		t.Error("app-beta should be excluded after include")
	}
	if sb.FileExists(filepath.Join(targetPath, "tool-main")) {
		t.Error("tool-main outside include should be removed")
	}
}
