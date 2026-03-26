//go:build !online

package integration

import (
	"encoding/json"
	"fmt"
	"strings"
	"testing"

	"skillshare/internal/testutil"
)

func TestAnalyze_TableOutput(t *testing.T) {
	sb := testutil.NewSandbox(t)
	defer sb.Cleanup()

	sb.CreateSkill("skill1", map[string]string{
		"SKILL.md": "---\nname: skill1\ndescription: First skill description\n---\n# Skill 1\nBody content",
	})
	sb.CreateSkill("skill2", map[string]string{
		"SKILL.md": "---\nname: skill2\ndescription: Second skill\n---\n# Skill 2\nMore body",
	})

	target1 := sb.CreateTarget("claude")
	target2 := sb.CreateTarget("cursor")

	sb.WriteConfig(`source: ` + sb.SourcePath + `
targets:
  claude:
    path: ` + target1 + `
  cursor:
    path: ` + target2 + `
`)

	result := sb.RunCLI("analyze")
	result.AssertSuccess(t)
	result.AssertOutputContains(t, "Context Analysis")
	result.AssertOutputContains(t, "Always loaded:")
	result.AssertOutputContains(t, "On-demand max:")
	result.AssertOutputContains(t, "claude")
	result.AssertOutputContains(t, "cursor")
}

func TestAnalyze_Verbose(t *testing.T) {
	sb := testutil.NewSandbox(t)
	defer sb.Cleanup()

	for i := 1; i <= 12; i++ {
		name := fmt.Sprintf("skill%d", i)
		desc := strings.Repeat("x", i*50)
		content := fmt.Sprintf("---\nname: %s\ndescription: %s\n---\n# %s\nBody", name, desc, name)
		sb.CreateSkill(name, map[string]string{"SKILL.md": content})
	}

	target := sb.CreateTarget("claude")
	sb.WriteConfig(`source: ` + sb.SourcePath + `
targets:
  claude:
    path: ` + target + `
`)

	result := sb.RunCLI("analyze", "--verbose")
	result.AssertSuccess(t)
	result.AssertOutputContains(t, "claude")
	result.AssertOutputContains(t, "12 skills")
	result.AssertOutputContains(t, "Always loaded:")
	result.AssertOutputContains(t, "Largest descriptions:")
	result.AssertOutputContains(t, "... 2 more")
}

func TestAnalyze_SingleTarget(t *testing.T) {
	sb := testutil.NewSandbox(t)
	defer sb.Cleanup()

	sb.CreateSkill("skill1", map[string]string{
		"SKILL.md": "---\nname: skill1\ndescription: test\n---\n# S1",
	})
	target1 := sb.CreateTarget("claude")
	target2 := sb.CreateTarget("cursor")

	sb.WriteConfig(`source: ` + sb.SourcePath + `
targets:
  claude:
    path: ` + target1 + `
  cursor:
    path: ` + target2 + `
`)

	result := sb.RunCLI("analyze", "claude")
	result.AssertSuccess(t)
	result.AssertOutputContains(t, "claude")
	result.AssertOutputNotContains(t, "cursor")
}

func TestAnalyze_JSON(t *testing.T) {
	sb := testutil.NewSandbox(t)
	defer sb.Cleanup()

	sb.CreateSkill("skill1", map[string]string{
		"SKILL.md": "---\nname: skill1\ndescription: A test skill\n---\n# Skill 1\nBody here",
	})
	target := sb.CreateTarget("claude")

	sb.WriteConfig(`source: ` + sb.SourcePath + `
targets:
  claude:
    path: ` + target + `
`)

	result := sb.RunCLI("analyze", "--json")
	result.AssertSuccess(t)

	var output struct {
		Targets []struct {
			Name         string `json:"name"`
			SkillCount   int    `json:"skill_count"`
			AlwaysLoaded struct {
				Chars           int `json:"chars"`
				EstimatedTokens int `json:"estimated_tokens"`
			} `json:"always_loaded"`
			OnDemandMax struct {
				Chars int `json:"chars"`
			} `json:"on_demand_max"`
			Skills []struct {
				Name             string `json:"name"`
				DescriptionChars int    `json:"description_chars"`
				BodyChars        int    `json:"body_chars"`
			} `json:"skills"`
		} `json:"targets"`
	}
	if err := json.Unmarshal([]byte(result.Stdout), &output); err != nil {
		t.Fatalf("failed to parse JSON: %v\noutput: %s", err, result.Stdout)
	}
	if len(output.Targets) != 1 {
		t.Fatalf("expected 1 target, got %d", len(output.Targets))
	}
	tgt := output.Targets[0]
	if tgt.Name != "claude" {
		t.Errorf("expected target name 'claude', got %q", tgt.Name)
	}
	if tgt.SkillCount != 1 {
		t.Errorf("expected 1 skill, got %d", tgt.SkillCount)
	}
	if tgt.AlwaysLoaded.Chars <= 0 {
		t.Errorf("expected positive always_loaded.chars, got %d", tgt.AlwaysLoaded.Chars)
	}
	if tgt.OnDemandMax.Chars <= 0 {
		t.Errorf("expected positive on_demand_max.chars, got %d", tgt.OnDemandMax.Chars)
	}
	if len(tgt.Skills) != 1 {
		t.Fatalf("expected 1 skill in list, got %d", len(tgt.Skills))
	}
	if tgt.Skills[0].DescriptionChars <= 0 {
		t.Errorf("expected positive description_chars, got %d", tgt.Skills[0].DescriptionChars)
	}
}

func TestAnalyze_EmptySource(t *testing.T) {
	sb := testutil.NewSandbox(t)
	defer sb.Cleanup()

	target := sb.CreateTarget("claude")
	sb.WriteConfig(`source: ` + sb.SourcePath + `
targets:
  claude:
    path: ` + target + `
`)

	result := sb.RunCLI("analyze")
	result.AssertSuccess(t)
	result.AssertOutputContains(t, "No skills found")
}

func TestAnalyze_UnknownTarget(t *testing.T) {
	sb := testutil.NewSandbox(t)
	defer sb.Cleanup()

	sb.CreateSkill("skill1", map[string]string{"SKILL.md": "---\nname: s\n---\n# S"})
	target := sb.CreateTarget("claude")

	sb.WriteConfig(`source: ` + sb.SourcePath + `
targets:
  claude:
    path: ` + target + `
`)

	result := sb.RunCLI("analyze", "nonexistent")
	result.AssertFailure(t)
	result.AssertAnyOutputContains(t, "not configured")
}

func TestAnalyze_IncludeExcludeFilter(t *testing.T) {
	sb := testutil.NewSandbox(t)
	defer sb.Cleanup()

	sb.CreateSkill("keep-me", map[string]string{
		"SKILL.md": "---\nname: keep-me\ndescription: kept\n---\n# Keep",
	})
	sb.CreateSkill("skip-me", map[string]string{
		"SKILL.md": "---\nname: skip-me\ndescription: skipped\n---\n# Skip",
	})

	target := sb.CreateTarget("claude")
	sb.WriteConfig(`source: ` + sb.SourcePath + `
targets:
  claude:
    path: ` + target + `
    exclude:
      - "skip-*"
`)

	result := sb.RunCLI("analyze", "--json")
	result.AssertSuccess(t)

	var output struct {
		Targets []struct {
			SkillCount int `json:"skill_count"`
		} `json:"targets"`
	}
	if err := json.Unmarshal([]byte(result.Stdout), &output); err != nil {
		t.Fatalf("failed to parse JSON: %v\noutput: %s", err, result.Stdout)
	}
	if len(output.Targets) != 1 {
		t.Fatalf("expected 1 target, got %d", len(output.Targets))
	}
	if output.Targets[0].SkillCount != 1 {
		t.Errorf("expected 1 skill after exclude, got %d", output.Targets[0].SkillCount)
	}
}

func TestAnalyze_SkillTargetRestriction(t *testing.T) {
	sb := testutil.NewSandbox(t)
	defer sb.Cleanup()

	sb.CreateSkill("claude-only", map[string]string{
		"SKILL.md": "---\nname: claude-only\ntargets:\n  - claude\ndescription: only claude\n---\n# Claude",
	})
	sb.CreateSkill("universal", map[string]string{
		"SKILL.md": "---\nname: universal\ndescription: everywhere\n---\n# Universal",
	})

	target1 := sb.CreateTarget("claude")
	target2 := sb.CreateTarget("cursor")

	sb.WriteConfig(`source: ` + sb.SourcePath + `
targets:
  claude:
    path: ` + target1 + `
  cursor:
    path: ` + target2 + `
`)

	result := sb.RunCLI("analyze", "--json")
	result.AssertSuccess(t)

	var output struct {
		Targets []struct {
			Name       string `json:"name"`
			SkillCount int    `json:"skill_count"`
		} `json:"targets"`
	}
	if err := json.Unmarshal([]byte(result.Stdout), &output); err != nil {
		t.Fatalf("failed to parse JSON: %v\noutput: %s", err, result.Stdout)
	}

	for _, tgt := range output.Targets {
		switch tgt.Name {
		case "claude":
			if tgt.SkillCount != 2 {
				t.Errorf("claude: expected 2 skills, got %d", tgt.SkillCount)
			}
		case "cursor":
			if tgt.SkillCount != 1 {
				t.Errorf("cursor: expected 1 skill (universal only), got %d", tgt.SkillCount)
			}
		}
	}
}

func TestAnalyze_ProjectMode(t *testing.T) {
	sb := testutil.NewSandbox(t)
	defer sb.Cleanup()

	projectDir := sb.SetupProjectDir("claude")
	sb.CreateProjectSkill(projectDir, "proj-skill", map[string]string{
		"SKILL.md": "---\nname: proj-skill\ndescription: project skill\n---\n# Project",
	})

	result := sb.RunCLIInDir(projectDir, "analyze", "-p")
	result.AssertSuccess(t)
}

func TestAnalyze_NoTUI(t *testing.T) {
	sb := testutil.NewSandbox(t)
	defer sb.Cleanup()

	sb.CreateSkill("skill1", map[string]string{
		"SKILL.md": "---\nname: skill1\ndescription: First skill\n---\n# Body",
	})
	target := sb.CreateTarget("claude")
	sb.WriteConfig(`source: ` + sb.SourcePath + "\ntargets:\n  claude:\n    path: " + target)

	result := sb.RunCLI("analyze", "--no-tui")
	result.AssertSuccess(t)
	result.AssertOutputContains(t, "Context Analysis")
	result.AssertOutputContains(t, "Always loaded:")
}

func TestAnalyze_HelpShowsNoTUI(t *testing.T) {
	sb := testutil.NewSandbox(t)
	defer sb.Cleanup()

	result := sb.RunCLI("analyze", "--help")
	result.AssertSuccess(t)
	result.AssertOutputContains(t, "--no-tui")
}

func TestAnalyze_JSONLintIssues(t *testing.T) {
	sb := testutil.NewSandbox(t)
	defer sb.Cleanup()

	// Skill with lint issues: no description, empty body
	sb.CreateSkill("bad-skill", map[string]string{
		"SKILL.md": "---\nname: bad-skill\n---\n",
	})
	// Clean skill
	sb.CreateSkill("good-skill", map[string]string{
		"SKILL.md": "---\nname: good-skill\ndescription: Use this skill when you need to do good things and help users accomplish tasks efficiently\n---\n# Good Skill\nThis is the body content.",
	})

	target := sb.CreateTarget("claude")
	sb.WriteConfig(`source: ` + sb.SourcePath + `
targets:
  claude:
    path: ` + target + `
`)

	result := sb.RunCLI("analyze", "--json")
	result.AssertSuccess(t)

	var output struct {
		Targets []struct {
			Name   string `json:"name"`
			Skills []struct {
				Name       string `json:"name"`
				LintIssues []struct {
					Rule     string `json:"rule"`
					Severity string `json:"severity"`
					Category string `json:"category"`
					Message  string `json:"message"`
				} `json:"lint_issues"`
			} `json:"skills"`
		} `json:"targets"`
	}
	if err := json.Unmarshal([]byte(result.Stdout), &output); err != nil {
		t.Fatalf("failed to parse JSON: %v", err)
	}

	if len(output.Targets) == 0 {
		t.Fatal("expected at least one target")
	}

	// Find bad-skill and verify it has lint issues
	var badSkillIssues int
	var goodSkillIssues int
	for _, target := range output.Targets {
		for _, skill := range target.Skills {
			switch skill.Name {
			case "bad-skill":
				badSkillIssues = len(skill.LintIssues)
				hasDesc := false
				hasBody := false
				for _, issue := range skill.LintIssues {
					if issue.Rule == "missing-description" {
						hasDesc = true
						if issue.Severity != "error" {
							t.Errorf("missing-description should be error, got %s", issue.Severity)
						}
						if issue.Category != "structure" {
							t.Errorf("missing-description category should be structure, got %s", issue.Category)
						}
					}
					if issue.Rule == "empty-body" {
						hasBody = true
					}
				}
				if !hasDesc {
					t.Error("bad-skill should have missing-description issue")
				}
				if !hasBody {
					t.Error("bad-skill should have empty-body issue")
				}
			case "good-skill":
				goodSkillIssues = len(skill.LintIssues)
			}
		}
	}

	if badSkillIssues == 0 {
		t.Error("bad-skill should have lint issues")
	}
	if goodSkillIssues != 0 {
		t.Errorf("good-skill should have no lint issues, got %d", goodSkillIssues)
	}
}
