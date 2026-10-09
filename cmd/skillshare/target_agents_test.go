package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"

	"skillshare/internal/config"
	"skillshare/internal/targetsummary"
	"skillshare/internal/testutil"
)

func TestShowTargetInfo_ShowsAgentsSectionForBuiltinTarget(t *testing.T) {
	sb := testutil.NewSandbox(t)
	defer sb.Cleanup()

	targetPath := sb.CreateTarget("claude")
	writeGlobalTargetConfig(t, sb, "claude", targetPath, "")

	cfg, err := config.Load()
	if err != nil {
		t.Fatalf("load config: %v", err)
	}

	agentSource := cfg.EffectiveAgentsSource()
	agentFile := writeAgentFile(t, agentSource, "reviewer.md")
	agentTarget := filepath.Join(sb.Home, ".claude", "agents")
	linkAgentFile(t, agentTarget, "reviewer.md", agentFile)

	output := stripANSIWarnings(captureStdout(t, func() {
		if err := showTargetInfo(cfg, "claude", cfg.Targets["claude"]); err != nil {
			t.Fatalf("showTargetInfo: %v", err)
		}
	}))

	if !strings.Contains(output, "\nAgents\n") {
		t.Fatalf("expected agents section in output:\n%s", output)
	}
	if !strings.Contains(output, shortenPath(agentTarget)) {
		t.Fatalf("expected agent path %q in output:\n%s", agentTarget, output)
	}
	if !strings.Contains(output, "1/1 linked") {
		t.Fatalf("expected linked summary in output:\n%s", output)
	}
}

func TestShowTargetInfo_OmitsAgentsSectionWhenTargetHasNoAgentsPath(t *testing.T) {
	sb := testutil.NewSandbox(t)
	defer sb.Cleanup()

	targetPath := filepath.Join(sb.Root, "custom-skills")
	if err := os.MkdirAll(targetPath, 0755); err != nil {
		t.Fatalf("mkdir target: %v", err)
	}
	writeGlobalTargetConfig(t, sb, "custom-tool", targetPath, "")

	cfg, err := config.Load()
	if err != nil {
		t.Fatalf("load config: %v", err)
	}

	output := stripANSIWarnings(captureStdout(t, func() {
		if err := showTargetInfo(cfg, "custom-tool", cfg.Targets["custom-tool"]); err != nil {
			t.Fatalf("showTargetInfo: %v", err)
		}
	}))

	if strings.Contains(output, "Agents:") {
		t.Fatalf("did not expect agents section:\n%s", output)
	}
}

func TestTargetListJSON_IncludesAgentMetadata(t *testing.T) {
	sb := testutil.NewSandbox(t)
	defer sb.Cleanup()

	targetPath := sb.CreateTarget("claude")
	writeGlobalTargetConfig(t, sb, "claude", targetPath, "")

	cfg, err := config.Load()
	if err != nil {
		t.Fatalf("load config: %v", err)
	}

	agentSource := cfg.EffectiveAgentsSource()
	agentFile := writeAgentFile(t, agentSource, "reviewer.md")
	agentTarget := filepath.Join(sb.Home, ".claude", "agents")
	linkAgentFile(t, agentTarget, "reviewer.md", agentFile)

	output := captureStdout(t, func() {
		if err := targetListJSON(cfg); err != nil {
			t.Fatalf("targetListJSON: %v", err)
		}
	})

	var resp struct {
		Targets []targetListJSONItem `json:"targets"`
	}
	if err := json.Unmarshal([]byte(output), &resp); err != nil {
		t.Fatalf("decode json: %v\n%s", err, output)
	}
	if len(resp.Targets) != 1 {
		t.Fatalf("expected 1 target, got %d", len(resp.Targets))
	}

	target := resp.Targets[0]
	if target.Path != targetPath {
		t.Fatalf("target path = %q, want %q", target.Path, targetPath)
	}
	if target.TargetNaming != "flat" {
		t.Fatalf("target naming = %q, want flat", target.TargetNaming)
	}
	if target.Sync == "" {
		t.Fatal("expected non-empty skill sync summary")
	}
	if target.AgentPath != agentTarget {
		t.Fatalf("agent path = %q, want %q", target.AgentPath, agentTarget)
	}
	if target.AgentMode != "merge" {
		t.Fatalf("agent mode = %q, want merge", target.AgentMode)
	}
	if target.AgentSync == "" {
		t.Fatal("expected non-empty agent sync summary")
	}
	if target.AgentLinkedCount == nil || *target.AgentLinkedCount != 1 {
		t.Fatalf("agent linked = %v, want 1", target.AgentLinkedCount)
	}
	if target.AgentLocalCount == nil || *target.AgentLocalCount != 0 {
		t.Fatalf("agent local = %v, want 0", target.AgentLocalCount)
	}
	if target.AgentExpectedCount == nil || *target.AgentExpectedCount != 1 {
		t.Fatalf("agent expected = %v, want 1", target.AgentExpectedCount)
	}
}

func TestTargetListJSON_IncludesLocalOnlyAgentCount(t *testing.T) {
	sb := testutil.NewSandbox(t)
	defer sb.Cleanup()

	targetPath := sb.CreateTarget("claude")
	writeGlobalTargetConfig(t, sb, "claude", targetPath, "")

	cfg, err := config.Load()
	if err != nil {
		t.Fatalf("load config: %v", err)
	}

	agentTarget := filepath.Join(sb.Home, ".claude", "agents")
	writeAgentFile(t, agentTarget, "local-only.md")

	output := captureStdout(t, func() {
		if err := targetListJSON(cfg); err != nil {
			t.Fatalf("targetListJSON: %v", err)
		}
	})

	var resp struct {
		Targets []targetListJSONItem `json:"targets"`
	}
	if err := json.Unmarshal([]byte(output), &resp); err != nil {
		t.Fatalf("decode json: %v\n%s", err, output)
	}
	if len(resp.Targets) != 1 {
		t.Fatalf("expected 1 target, got %d", len(resp.Targets))
	}

	target := resp.Targets[0]
	if target.AgentLocalCount == nil || *target.AgentLocalCount != 1 {
		t.Fatalf("agent local = %v, want 1", target.AgentLocalCount)
	}
	if target.AgentLinkedCount == nil || *target.AgentLinkedCount != 0 {
		t.Fatalf("agent linked = %v, want 0", target.AgentLinkedCount)
	}
	if target.AgentExpectedCount == nil || *target.AgentExpectedCount != 0 {
		t.Fatalf("agent expected = %v, want 0", target.AgentExpectedCount)
	}
}

func TestTargetList_TextOutputShowsSkillsAndAgentsSections(t *testing.T) {
	sb := testutil.NewSandbox(t)
	defer sb.Cleanup()

	targetPath := sb.CreateTarget("claude")
	writeGlobalTargetConfig(t, sb, "claude", targetPath, "")

	cfg, err := config.Load()
	if err != nil {
		t.Fatalf("load config: %v", err)
	}

	agentSource := cfg.EffectiveAgentsSource()
	agentFile := writeAgentFile(t, agentSource, "reviewer.md")
	agentTarget := filepath.Join(sb.Home, ".claude", "agents")
	linkAgentFile(t, agentTarget, "reviewer.md", agentFile)

	output := stripANSIWarnings(captureStdout(t, func() {
		if err := targetList(false); err != nil {
			t.Fatalf("targetList: %v", err)
		}
	}))

	for _, want := range []string{
		"claude",
		"Skills",
		shortenPath(targetPath),
		"Agents",
		shortenPath(agentTarget),
		"1/1 linked",
		"1 target",
	} {
		if !strings.Contains(output, want) {
			t.Fatalf("expected %q in target list output:\n%s", want, output)
		}
	}
}

func TestRenderTargetDetail_AgentSection(t *testing.T) {
	keepPathsUnfolded(t)
	cases := []struct {
		name        string
		item        targetTUIItem
		want        []string
		notContains []string
	}{
		{
			name: "merge target shows builtin-like agents section",
			item: targetTUIItem{
				name:        "cursor",
				displayPath: ".cursor/skills",
				skillSync:   "merged (4 shared, 1 local)",
				target: config.TargetConfig{
					Skills: &config.ResourceTargetConfig{
						Path:         "/tmp/cursor/skills",
						Mode:         "merge",
						TargetNaming: "flat",
					},
				},
				agentSummary: &targetsummary.AgentSummary{
					DisplayPath:   ".cursor/agents",
					Path:          "/tmp/cursor/agents",
					Mode:          "merge",
					ManagedCount:  2,
					ExpectedCount: 3,
					Include:       []string{"team-*"},
				},
			},
			want: []string{"Skills", ".cursor/skills", "Sync", "merged (4 shared, 1 local)", "Agents", ".cursor/agents", "2/3 linked", "Include", "team-*"},
		},
		{
			name: "copy target shows custom agents section",
			item: targetTUIItem{
				name:        "custom",
				displayPath: "/tmp/custom/skills",
				skillSync:   "copied (2 managed, 0 local)",
				target: config.TargetConfig{
					Skills: &config.ResourceTargetConfig{
						Path:         "/tmp/custom/skills",
						Mode:         "copy",
						TargetNaming: "flat",
					},
				},
				agentSummary: &targetsummary.AgentSummary{
					DisplayPath:   "/tmp/custom/agents",
					Path:          "/tmp/custom/agents",
					Mode:          "copy",
					ManagedCount:  2,
					ExpectedCount: 2,
				},
			},
			want: []string{"Skills", "/tmp/custom/skills", "Sync", "copied (2 managed, 0 local)", "Agents", "/tmp/custom/agents", "2/2 managed"},
		},
		{
			name: "local-only agents are shown when source has none",
			item: targetTUIItem{
				name:        "claude",
				displayPath: ".claude/skills",
				skillSync:   "merged (0 shared, 0 local)",
				target: config.TargetConfig{
					Skills: &config.ResourceTargetConfig{
						Path:         "/tmp/claude/skills",
						Mode:         "merge",
						TargetNaming: "flat",
					},
				},
				agentSummary: &targetsummary.AgentSummary{
					DisplayPath: ".claude/agents",
					Path:        "/tmp/claude/agents",
					Mode:        "merge",
					LocalCount:  1,
				},
			},
			want: []string{"Agents", ".claude/agents", "no source agents yet (1 local)"},
		},
		{
			name: "symlink agent target shows filters ignored warning",
			item: targetTUIItem{
				name:        "claude",
				displayPath: ".claude/skills",
				skillSync:   "merged (7 shared, 0 local)",
				target: config.TargetConfig{
					Skills: &config.ResourceTargetConfig{
						Path:         "/tmp/claude/skills",
						Mode:         "merge",
						TargetNaming: "flat",
					},
				},
				agentSummary: &targetsummary.AgentSummary{
					DisplayPath:   ".claude/agents",
					Path:          "/tmp/claude/agents",
					Mode:          "symlink",
					ManagedCount:  5,
					ExpectedCount: 5,
					Include:       []string{"team-*"},
					Exclude:       []string{"draft-*"},
				},
			},
			want:        []string{"Agents", ".claude/agents", "5/5 linked (directory symlink)", "ignored in symlink mode"},
			notContains: []string{"team-*", "draft-*"},
		},
		{
			name: "naming the mode cannot honour shows the fix",
			item: targetTUIItem{
				name:      "claude",
				skillSync: "merged (7 shared, 0 local)",
				target: config.TargetConfig{
					Skills: &config.ResourceTargetConfig{Path: "/tmp/claude/skills", Mode: "merge", TargetNaming: "prefixed"},
				},
				namingErr: config.TargetNamingModeConfigError("prefixed", "merge", ""),
			},
			want: []string{"set mode: copy on the target"},
		},
		{
			name: "unsupported target omits agents section",
			item: targetTUIItem{
				name:        "custom-tool",
				displayPath: "/tmp/custom-tool/skills",
				skillSync:   "not exist (0 shared, 0 local)",
				target: config.TargetConfig{
					Skills: &config.ResourceTargetConfig{
						Path:         "/tmp/custom-tool/skills",
						Mode:         "merge",
						TargetNaming: "flat",
					},
				},
			},
			want:        []string{"Skills", "/tmp/custom-tool/skills", "Sync", "not exist (0 shared, 0 local)"},
			notContains: []string{"Agents"},
		},
	}

	model := targetListTUIModel{}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			rendered := stripANSIWarnings(model.renderTargetDetail(tc.item))
			for _, want := range tc.want {
				if !strings.Contains(rendered, want) {
					t.Fatalf("expected %q in output:\n%s", want, rendered)
				}
			}
			for _, unwanted := range tc.notContains {
				if strings.Contains(rendered, unwanted) {
					t.Fatalf("did not expect %q in output:\n%s", unwanted, rendered)
				}
			}
		})
	}
}

func TestFormatTargetAgentSyncSummary_IncludesLocalAgents(t *testing.T) {
	cases := []struct {
		name    string
		summary *targetsummary.AgentSummary
		want    string
	}{
		{
			name: "local only without source agents",
			summary: &targetsummary.AgentSummary{
				Mode:       "merge",
				LocalCount: 1,
			},
			want: "no source agents yet (1 local)",
		},
		{
			name: "managed and local without source agents",
			summary: &targetsummary.AgentSummary{
				Mode:         "copy",
				ManagedCount: 2,
				LocalCount:   1,
			},
			want: "no source agents yet (2 managed, 1 local)",
		},
		{
			name: "expected source agents with local suffix",
			summary: &targetsummary.AgentSummary{
				Mode:          "merge",
				ManagedCount:  2,
				LocalCount:    1,
				ExpectedCount: 3,
			},
			want: "2/3 linked, 1 local",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := formatTargetAgentSyncSummary(tc.summary); got != tc.want {
				t.Fatalf("formatTargetAgentSyncSummary() = %q, want %q", got, tc.want)
			}
		})
	}
}

func TestTargetEditOptions_SkipsAgentFiltersInSymlinkMode(t *testing.T) {
	item := targetTUIItem{
		name:         "claude",
		agentSummary: &targetsummary.AgentSummary{Mode: "symlink"},
	}

	options := targetEditOptions(item)
	agentsMode := slices.IndexFunc(options, func(o targetEditOption) bool { return o.label() == "Agents mode" })
	if agentsMode < 0 {
		t.Fatalf("expected an Agents mode option, got %+v", options)
	}
	if got := moveEditMenuCursor(options, agentsMode, 1); got != agentsMode {
		t.Fatalf("cursor should stay on Agents mode when the agent filters do nothing, got %d (%+v)", got, options[got])
	}
}

func TestTargetRemoveAsksOnTheKeyLine(t *testing.T) {
	m := newTargetListTUIModel("global", nil, nil, "")
	m.loading = false
	m.termWidth = 120
	m.allItems = []targetTUIItem{{name: "cursor"}}
	m.applyTargetFilter()

	next, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("d")})
	got := stripANSIWarnings(next.(targetListTUIModel).renderBottom())
	if !strings.Contains(got, "Remove target cursor?") || !strings.Contains(got, "skillshare target remove -g cursor") {
		t.Fatalf("d should ask on the key line and show the command, got %q", got)
	}
}

func TestDoSetTargetMode_Agents_GlobalAndProject(t *testing.T) {
	t.Run("global", func(t *testing.T) {
		sb := testutil.NewSandbox(t)
		defer sb.Cleanup()

		targetPath := sb.CreateTarget("claude")
		writeGlobalTargetConfig(t, sb, "claude", targetPath, "")

		model := targetListTUIModel{}
		if _, err := model.doSetTargetMode("claude", "agents", "copy"); err != nil {
			t.Fatalf("doSetTargetMode: %v", err)
		}

		cfg, err := config.Load()
		if err != nil {
			t.Fatalf("load config: %v", err)
		}
		target := cfg.Targets["claude"]
		if got := target.AgentsConfig().Mode; got != "copy" {
			t.Fatalf("agent mode = %q, want copy", got)
		}
	})

	t.Run("project", func(t *testing.T) {
		root := writeProjectTargetConfig(t, []config.ProjectTargetEntry{{Name: "claude"}})

		model := targetListTUIModel{
			projCfg: &config.ProjectConfig{},
			cwd:     root,
		}
		if _, err := model.doSetTargetMode("claude", "agents", "copy"); err != nil {
			t.Fatalf("doSetTargetMode: %v", err)
		}

		cfg, err := config.LoadProject(root)
		if err != nil {
			t.Fatalf("load project config: %v", err)
		}
		if got := cfg.Targets[0].AgentsConfig().Mode; got != "copy" {
			t.Fatalf("project agent mode = %q, want copy", got)
		}
	})
}

func TestDoAddAndRemovePattern_Agents_GlobalAndProject(t *testing.T) {
	t.Run("global", func(t *testing.T) {
		sb := testutil.NewSandbox(t)
		defer sb.Cleanup()

		targetPath := sb.CreateTarget("claude")
		writeGlobalTargetConfig(t, sb, "claude", targetPath, "")

		model := targetListTUIModel{}
		if _, err := model.doAddPattern("claude", "agents", "include", "team-*"); err != nil {
			t.Fatalf("doAddPattern: %v", err)
		}
		if _, err := model.doRemovePattern("claude", "agents", "include", "team-*"); err != nil {
			t.Fatalf("doRemovePattern: %v", err)
		}

		cfg, err := config.Load()
		if err != nil {
			t.Fatalf("load config: %v", err)
		}
		target := cfg.Targets["claude"]
		if got := target.AgentsConfig().Include; len(got) != 0 {
			t.Fatalf("agent include = %v, want empty", got)
		}
	})

	t.Run("project", func(t *testing.T) {
		root := writeProjectTargetConfig(t, []config.ProjectTargetEntry{{Name: "claude"}})

		model := targetListTUIModel{
			projCfg: &config.ProjectConfig{},
			cwd:     root,
		}
		if _, err := model.doAddPattern("claude", "agents", "exclude", "draft-*"); err != nil {
			t.Fatalf("doAddPattern: %v", err)
		}
		if _, err := model.doRemovePattern("claude", "agents", "exclude", "draft-*"); err != nil {
			t.Fatalf("doRemovePattern: %v", err)
		}

		cfg, err := config.LoadProject(root)
		if err != nil {
			t.Fatalf("load project config: %v", err)
		}
		if got := cfg.Targets[0].AgentsConfig().Exclude; len(got) != 0 {
			t.Fatalf("project agent exclude = %v, want empty", got)
		}
	})
}

func writeGlobalTargetConfig(t *testing.T, sb *testutil.Sandbox, name, skillsPath, agentPath string) {
	t.Helper()

	var b strings.Builder
	b.WriteString("source: " + sb.SourcePath + "\n")
	b.WriteString("mode: merge\n")
	b.WriteString("targets:\n")
	b.WriteString("  " + name + ":\n")
	b.WriteString("    skills:\n")
	b.WriteString("      path: " + skillsPath + "\n")
	if agentPath != "" {
		b.WriteString("    agents:\n")
		b.WriteString("      path: " + agentPath + "\n")
	}
	sb.WriteConfig(b.String())
}

func writeAgentFile(t *testing.T, dir, name string) string {
	t.Helper()
	if err := os.MkdirAll(dir, 0755); err != nil {
		t.Fatalf("mkdir agent source: %v", err)
	}
	path := filepath.Join(dir, name)
	if err := os.WriteFile(path, []byte("# "+name), 0644); err != nil {
		t.Fatalf("write agent file: %v", err)
	}
	return path
}

func linkAgentFile(t *testing.T, dir, name, source string) string {
	t.Helper()
	if err := os.MkdirAll(dir, 0755); err != nil {
		t.Fatalf("mkdir agent target: %v", err)
	}
	linkPath := filepath.Join(dir, name)
	if err := os.Symlink(source, linkPath); err != nil {
		t.Fatalf("symlink agent: %v", err)
	}
	return linkPath
}

func writeProjectTargetConfig(t *testing.T, targets []config.ProjectTargetEntry) string {
	t.Helper()

	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, ".skillshare", "skills"), 0755); err != nil {
		t.Fatalf("mkdir project skills: %v", err)
	}
	if err := os.MkdirAll(filepath.Join(root, ".skillshare", "agents"), 0755); err != nil {
		t.Fatalf("mkdir project agents: %v", err)
	}

	cfg := &config.ProjectConfig{Targets: targets}
	if err := cfg.Save(root); err != nil {
		t.Fatalf("save project config: %v", err)
	}
	return root
}

func TestBuildTargetSkillSyncSummary_PlainTextOmitsZeroCounts(t *testing.T) {
	root := t.TempDir()
	source := filepath.Join(root, "source")
	target := filepath.Join(root, "target")
	for _, dir := range []string{filepath.Join(source, "shared"), filepath.Join(target, "mine")} {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.Symlink(filepath.Join(source, "shared"), filepath.Join(target, "shared")); err != nil {
		t.Fatal(err)
	}

	summary, text := buildTargetSkillSyncSummary(target, source, "merge")
	if summary != "merged (1 shared, 1 local)" || text != "merged · 1 shared, 1 local" {
		t.Fatalf("got summary %q, text %q", summary, text)
	}

	summary, text = buildTargetSkillSyncSummary(filepath.Join(root, "missing"), source, "merge")
	if summary != "not exist (0 shared, 0 local)" || text != "not exist" {
		t.Fatalf("got summary %q, text %q", summary, text)
	}
}
