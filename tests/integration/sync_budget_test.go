//go:build !online

package integration

import (
	"encoding/json"
	"testing"

	"skillshare/internal/testutil"
)

func TestSync_TokenSummary(t *testing.T) {
	sb := testutil.NewSandbox(t)
	defer sb.Cleanup()

	sb.CreateSkill("my-skill", map[string]string{
		"SKILL.md": "---\nname: my-skill\ndescription: A test skill for token counting\n---\n# Body content",
	})
	targetPath := sb.CreateTarget("claude")
	sb.WriteConfig(`source: ` + sb.SourcePath + `
mode: merge
targets:
  claude:
    skills:
      path: ` + targetPath + `
`)

	result := sb.RunCLI("sync")
	result.AssertSuccess(t)
	result.AssertAnyOutputContains(t, "Context:")
	result.AssertAnyOutputContains(t, "always-loaded")
	result.AssertAnyOutputContains(t, "on-demand")
}

func TestSync_TokenSummary_Quiet(t *testing.T) {
	sb := testutil.NewSandbox(t)
	defer sb.Cleanup()

	sb.CreateSkill("my-skill", map[string]string{
		"SKILL.md": "---\nname: my-skill\ndescription: A test skill\n---\n# Body",
	})
	targetPath := sb.CreateTarget("claude")
	sb.WriteConfig(`source: ` + sb.SourcePath + `
mode: merge
targets:
  claude:
    skills:
      path: ` + targetPath + `
`)

	result := sb.RunCLI("sync", "--quiet")
	result.AssertSuccess(t)
	result.AssertOutputNotContains(t, "Context:")
}

func TestSync_BudgetWarning(t *testing.T) {
	sb := testutil.NewSandbox(t)
	defer sb.Cleanup()

	sb.CreateSkill("my-skill", map[string]string{
		"SKILL.md": "---\nname: my-skill\ndescription: This is a skill with a sufficiently long description text for testing budget warnings\n---\n# Body content here",
	})
	targetPath := sb.CreateTarget("claude")
	sb.WriteConfig(`source: ` + sb.SourcePath + `
mode: merge
targets:
  claude:
    skills:
      path: ` + targetPath + `
context_budget:
  warn_always_loaded_tokens: 1
`)

	result := sb.RunCLI("sync")
	result.AssertSuccess(t)
	result.AssertAnyOutputContains(t, "Always-loaded context is")
	result.AssertAnyOutputContains(t, "budget:")
}

func TestSync_JSON_ContextCost(t *testing.T) {
	sb := testutil.NewSandbox(t)
	defer sb.Cleanup()

	sb.CreateSkill("my-skill", map[string]string{
		"SKILL.md": "---\nname: my-skill\ndescription: A test skill\n---\n# Body",
	})
	targetPath := sb.CreateTarget("claude")
	sb.WriteConfig(`source: ` + sb.SourcePath + `
mode: merge
targets:
  claude:
    skills:
      path: ` + targetPath + `
`)

	result := sb.RunCLI("sync", "--json")
	result.AssertSuccess(t)

	var output map[string]json.RawMessage
	if err := json.Unmarshal([]byte(result.Stdout), &output); err != nil {
		t.Fatalf("failed to parse JSON output: %v", err)
	}
	if _, ok := output["context_cost"]; !ok {
		t.Error("expected context_cost field in JSON output")
	}

	var cost struct {
		Groups []struct {
			Targets            []string `json:"targets"`
			AlwaysLoadedTokens int      `json:"always_loaded_tokens"`
			OnDemandTokens     int      `json:"on_demand_tokens"`
		} `json:"groups"`
	}
	if err := json.Unmarshal(output["context_cost"], &cost); err != nil {
		t.Fatalf("failed to parse context_cost: %v", err)
	}
	if len(cost.Groups) == 0 {
		t.Error("expected at least one context cost group")
	}
}

func TestSync_BudgetWarning_Quiet_Suppresses_Text(t *testing.T) {
	sb := testutil.NewSandbox(t)
	defer sb.Cleanup()

	sb.CreateSkill("my-skill", map[string]string{
		"SKILL.md": "---\nname: my-skill\ndescription: Some description\n---\n# Body",
	})
	targetPath := sb.CreateTarget("claude")
	sb.WriteConfig(`source: ` + sb.SourcePath + `
mode: merge
targets:
  claude:
    skills:
      path: ` + targetPath + `
context_budget:
  warn_always_loaded_tokens: 1
`)

	result := sb.RunCLI("sync", "--quiet")
	result.AssertSuccess(t)
	result.AssertOutputNotContains(t, "Always-loaded context is")
}
