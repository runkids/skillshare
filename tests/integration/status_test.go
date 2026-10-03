//go:build !online

package integration

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"skillshare/internal/testutil"
)

func TestStatus_ShowsSourceInfo(t *testing.T) {
	sb := testutil.NewSandbox(t)
	defer sb.Cleanup()

	sb.CreateSkill("skill1", map[string]string{"SKILL.md": "# Skill 1"})
	sb.CreateSkill("skill2", map[string]string{"SKILL.md": "# Skill 2"})

	sb.WriteConfig(`source: ` + sb.SourcePath + `
targets: {}
`)

	result := sb.RunCLI("status")

	result.AssertSuccess(t)
	result.AssertOutputContains(t, "Source")
	result.AssertRowContains(t, "skills", "~/.config/skillshare/skills  2 skills")
}

func TestStatus_ShowsTargetStatus(t *testing.T) {
	sb := testutil.NewSandbox(t)
	defer sb.Cleanup()

	sb.CreateSkill("skill1", map[string]string{"SKILL.md": "# Skill 1"})
	targetPath := sb.CreateTarget("claude")

	sb.WriteConfig(`source: ` + sb.SourcePath + `
targets:
  claude:
    path: ` + targetPath + `
`)

	result := sb.RunCLI("status")

	result.AssertSuccess(t)
	result.AssertOutputContains(t, "Targets")
	result.AssertOutputContains(t, "claude")
}

func TestStatus_LinkedTarget_ShowsLinked(t *testing.T) {
	sb := testutil.NewSandbox(t)
	defer sb.Cleanup()

	sb.CreateSkill("skill1", map[string]string{"SKILL.md": "# Skill 1"})

	targetPath := filepath.Join(sb.Home, ".claude", "skills")
	os.MkdirAll(filepath.Dir(targetPath), 0755)
	os.Symlink(sb.SourcePath, targetPath)

	sb.WriteConfig(`source: ` + sb.SourcePath + `
targets:
  claude:
    path: ` + targetPath + `
    mode: symlink
`)

	result := sb.RunCLI("status")

	result.AssertSuccess(t)
	result.AssertOutputContains(t, "linked")
}

func TestStatus_MergedTarget_ShowsMerged(t *testing.T) {
	sb := testutil.NewSandbox(t)
	defer sb.Cleanup()

	sb.CreateSkill("skill1", map[string]string{"SKILL.md": "# Skill 1"})
	targetPath := sb.CreateTarget("claude")

	// Create symlink to skill (merge mode)
	skillLink := filepath.Join(targetPath, "skill1")
	os.Symlink(filepath.Join(sb.SourcePath, "skill1"), skillLink)

	sb.WriteConfig(`source: ` + sb.SourcePath + `
mode: merge
targets:
  claude:
    path: ` + targetPath + `
`)

	result := sb.RunCLI("status")

	result.AssertSuccess(t)
	result.AssertOutputContains(t, "✓ 1 linked")
}

func TestStatus_NoConfig_ReturnsError(t *testing.T) {
	sb := testutil.NewSandbox(t)
	defer sb.Cleanup()

	os.Remove(sb.ConfigPath)

	result := sb.RunCLI("status")

	result.AssertFailure(t)
	result.AssertAnyOutputContains(t, "init")
}

func TestStatus_EmptySource_ShowsZeroSkills(t *testing.T) {
	sb := testutil.NewSandbox(t)
	defer sb.Cleanup()

	sb.WriteConfig(`source: ` + sb.SourcePath + `
targets: {}
`)

	result := sb.RunCLI("status")

	result.AssertSuccess(t)
	result.AssertOutputContains(t, "0 skills")
}

func TestStatus_SkillignoreShown(t *testing.T) {
	sb := testutil.NewSandbox(t)
	defer sb.Cleanup()

	sb.CreateSkill("keep-me", map[string]string{"SKILL.md": "# Keep"})
	sb.CreateSkill("test-draft", map[string]string{"SKILL.md": "# Draft"})

	// Create .skillignore that excludes test-draft
	sb.WriteFile(filepath.Join(sb.SourcePath, ".skillignore"), "test-*\n")

	sb.WriteConfig(`source: ` + sb.SourcePath + `
targets: {}
`)

	result := sb.RunCLI("status")

	result.AssertSuccess(t)
	result.AssertOutputContains(t, ".skillignore")
	result.AssertOutputContains(t, "1 patterns")
	result.AssertOutputContains(t, "1 skills ignored")
}

func TestStatus_SkillignoreHiddenWhenAbsent(t *testing.T) {
	sb := testutil.NewSandbox(t)
	defer sb.Cleanup()

	sb.CreateSkill("skill1", map[string]string{"SKILL.md": "# Skill 1"})

	sb.WriteConfig(`source: ` + sb.SourcePath + `
targets: {}
`)

	result := sb.RunCLI("status")

	result.AssertSuccess(t)
	result.AssertOutputNotContains(t, ".skillignore")
}

func TestStatus_JSON_SkillignoreField(t *testing.T) {
	sb := testutil.NewSandbox(t)
	defer sb.Cleanup()

	sb.CreateSkill("keep-me", map[string]string{"SKILL.md": "# Keep"})
	sb.CreateSkill("test-draft", map[string]string{"SKILL.md": "# Draft"})

	// Create .skillignore that excludes test-draft
	sb.WriteFile(filepath.Join(sb.SourcePath, ".skillignore"), "test-*\n")

	sb.WriteConfig(`source: ` + sb.SourcePath + `
targets: {}
`)

	result := sb.RunCLI("status", "--json")

	result.AssertSuccess(t)

	var output struct {
		Source struct {
			Skillignore struct {
				Active       bool     `json:"active"`
				IgnoredCount int      `json:"ignored_count"`
				Patterns     []string `json:"patterns"`
			} `json:"skillignore"`
		} `json:"source"`
	}
	if err := json.Unmarshal([]byte(result.Stdout), &output); err != nil {
		t.Fatalf("failed to parse JSON output: %v\nstdout: %s", err, result.Stdout)
	}

	if !output.Source.Skillignore.Active {
		t.Error("expected skillignore.active to be true")
	}
	if output.Source.Skillignore.IgnoredCount != 1 {
		t.Errorf("expected ignored_count=1, got %d", output.Source.Skillignore.IgnoredCount)
	}
	if len(output.Source.Skillignore.Patterns) != 1 || output.Source.Skillignore.Patterns[0] != "test-*" {
		t.Errorf("expected patterns=[test-*], got %v", output.Source.Skillignore.Patterns)
	}
}

// makeTrackedRepoWithBrokenStatus creates a committed tracked repo under
// sourceDir whose `git status` fails because its index is corrupt.
func makeTrackedRepoWithBrokenStatus(t *testing.T, sourceDir, name string) {
	t.Helper()
	repoPath := filepath.Join(sourceDir, name)
	if err := os.MkdirAll(filepath.Join(repoPath, "my-skill"), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(repoPath, "my-skill", "SKILL.md"), []byte("---\nname: my-skill\n---\n# My Skill"), 0644); err != nil {
		t.Fatal(err)
	}
	run(t, repoPath, "git", "init")
	run(t, repoPath, "git", "add", "-A")
	run(t, repoPath, "git", "commit", "-m", "init")
	corruptGitIndex(t, repoPath)
}

func TestStatus_TrackedRepoGitStatusError_ShowsUnknown(t *testing.T) {
	sb := testutil.NewSandbox(t)
	defer sb.Cleanup()
	makeTrackedRepoWithBrokenStatus(t, sb.SourcePath, "_broken-repo")
	sb.WriteConfig(`source: ` + sb.SourcePath + "\ntargets: {}\n")

	result := sb.RunCLI("status")
	result.AssertSuccess(t)
	result.AssertRowContains(t, "_broken-repo", "1 skill · failed to check git status")
}

func TestStatus_JSON_TrackedRepoGitStatusError_ShowsUnknown(t *testing.T) {
	sb := testutil.NewSandbox(t)
	defer sb.Cleanup()
	makeTrackedRepoWithBrokenStatus(t, sb.SourcePath, "_broken-repo")
	sb.WriteConfig(`source: ` + sb.SourcePath + "\ntargets: {}\n")

	result := sb.RunCLI("status", "--json")
	result.AssertSuccess(t)

	var output struct {
		TrackedRepos []struct {
			Name    string `json:"name"`
			Status  string `json:"status"`
			Message string `json:"message"`
		} `json:"tracked_repos"`
	}
	if err := json.Unmarshal([]byte(result.Stdout), &output); err != nil {
		t.Fatalf("invalid JSON: %v\n%s", err, result.Stdout)
	}
	if len(output.TrackedRepos) != 1 || output.TrackedRepos[0].Status != "unknown" {
		t.Fatalf("expected one tracked repo with status unknown, got %+v", output.TrackedRepos)
	}
}
