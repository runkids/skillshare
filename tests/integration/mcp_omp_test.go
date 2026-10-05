//go:build !online

package integration

import (
	"encoding/json"
	"path/filepath"
	"strings"
	"testing"

	"skillshare/internal/testutil"
)

func TestMCPOMPIssue409DryRun(t *testing.T) {
	sb := testutil.NewSandbox(t)
	defer sb.Cleanup()
	sb.WriteConfig("targets: {}\n")
	before := sb.ReadFile(sb.ConfigPath)
	result := sb.RunCLI("mcp", "add", "omp-probe", "--url", "https://example.invalid/mcp", "--target", "omp", "-g", "--dry-run", "--json")
	result.AssertSuccess(t)
	var plan struct {
		Changes []struct{ Target, Path, Name, Action string }
	}
	if err := json.Unmarshal([]byte(result.Stdout), &plan); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(sb.Home, ".omp", "agent", "mcp.json")
	if len(plan.Changes) != 1 || plan.Changes[0].Target != "omp" || plan.Changes[0].Path != path || plan.Changes[0].Action != "add" {
		t.Fatalf("wrong native plan: %s", result.Stdout)
	}
	if sb.FileExists(path) || sb.ReadFile(sb.ConfigPath) != before {
		t.Fatal("dry-run changed files")
	}
}

func TestMCPOMPSkillsAndConnections(t *testing.T) {
	for _, project := range []bool{false, true} {
		t.Run(map[bool]string{false: "global", true: "project"}[project], func(t *testing.T) {
			sb := testutil.NewSandbox(t)
			defer sb.Cleanup()
			run := sb.RunCLI
			mode := "-g"
			skillRoot := filepath.Join(sb.Home, ".omp", "agent", "skills")
			mcpPath := filepath.Join(sb.Home, ".omp", "agent", "mcp.json")
			skill := map[string]string{"SKILL.md": "---\nname: omp-demo\ndescription: OMP discovery probe\n---\n# OMP demo\n"}
			if project {
				root := sb.SetupProjectDir("omp")
				sb.CreateProjectSkill(root, "omp-demo", skill)
				run = func(args ...string) *testutil.Result { return sb.RunCLIInDir(root, args...) }
				mode = "-p"
				skillRoot = filepath.Join(root, ".omp", "skills")
				mcpPath = filepath.Join(root, ".omp", "mcp.json")
			} else {
				sb.CreateSkill("omp-demo", skill)
				sb.WriteConfig("source: " + sb.SourcePath + "\ntargets:\n  omp: {}\n")
			}
			run("sync", mode).AssertSuccess(t)
			if !strings.Contains(sb.ReadFile(filepath.Join(skillRoot, "omp-demo", "SKILL.md")), "OMP discovery probe") {
				t.Fatal("skill not synced to OMP's one-level layout")
			}
			sb.WriteFile(mcpPath, `{"$schema":"https://example.com/schema","disabledServers":["external"],"enabledServers":["manual"],"mcpServers":{"manual":{"command":"manual"}}}`)
			run("mcp", "add", "docs", "--url", "https://example.com/mcp", "--target", "omp", "--sync", mode, "--no-tui").AssertSuccess(t)
			before := sb.ReadFile(mcpPath)
			run("sync", "mcp", mode).AssertSuccess(t)
			if sb.ReadFile(mcpPath) != before {
				t.Fatal("OMP sync is not idempotent")
			}
			run("mcp", "import", "docs", "--from", "omp", "--target", "omp", "--replace", "--dry-run", mode, "--no-tui").AssertSuccess(t)
			run("mcp", "edit", "docs", "--url", "https://example.com/updated", "--sync", mode, "--no-tui").AssertSuccess(t)
			if !strings.Contains(sb.ReadFile(mcpPath), "https://example.com/updated") {
				t.Fatal("OMP edit did not update native config")
			}
			run("mcp", "remove", "docs", "--sync", mode, "--no-tui").AssertSuccess(t)
			data := sb.ReadFile(mcpPath)
			if strings.Contains(data, `"docs"`) || !strings.Contains(data, `"manual"`) || !strings.Contains(data, `"disabledServers"`) || !strings.Contains(data, `"enabledServers"`) {
				t.Fatalf("OMP remove lost unrelated config: %s", data)
			}
		})
	}
}

func TestCompletion_OMP_AllShells(t *testing.T) {
	sb := testutil.NewSandbox(t)
	defer sb.Cleanup()
	for _, shell := range []string{"bash", "zsh", "fish", "powershell", "nushell"} {
		result := sb.RunCLI("completion", shell)
		result.AssertSuccess(t)
		result.AssertOutputContains(t, "omp")
	}
}
