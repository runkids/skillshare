package mcp

import (
	"maps"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"skillshare/internal/tokens"
)

func TestLiveCheckCountsToolTokens(t *testing.T) {
	s := checkService(t, liveConfig("pinned", nil))
	probePinned(t, s, pinnedTools, false)
	got := s.readToolCosts().Servers[""]["fake"]
	want := tokens.Estimate("echo\nEcho text\n"+`{"type":"object","properties":{"text":{"type":"string"}}}`) + tokens.Estimate("ls\nList files\n")
	if got.Tools != 2 || got.Tokens != want || got.Checked.IsZero() {
		t.Fatalf("cost = %+v, want 2 tools and %d tokens", got, want)
	}
}

func TestToolTokensIgnoreJSONLayout(t *testing.T) {
	s := checkService(t, liveConfig("pinned", nil))
	probePinned(t, s, pinnedTools, false)
	first := s.readToolCosts().Servers[""]["fake"].Tokens
	probePinned(t, s, pinnedToolsReordered, false)
	if again := s.readToolCosts().Servers[""]["fake"].Tokens; again != first {
		t.Fatalf("tokens = %d after reordering, want %d", again, first)
	}
}

func TestToolCostsGroupsByTargetAndListsUncounted(t *testing.T) {
	config := strings.Replace(liveConfig("pinned", map[string]string{"MCP_LIVE_TOOLS": pinnedTools}), "targets: [claude]", "targets: [claude, cursor]", 1) +
		"    remote:\n      url: https://mcp.example.invalid/mcp\n"
	s := checkService(t, config)
	opts := liveCheck(10*time.Second, tokenEnv)
	opts.Names = []string{"fake"}
	if _, err := s.Check(opts); err != nil {
		t.Fatal(err)
	}
	report, err := s.ToolCosts()
	if err != nil {
		t.Fatal(err)
	}
	counted := s.readToolCosts().Servers[""]["fake"].Tokens
	if len(report.Groups) != 2 || report.Groups[0].Target != "claude" || report.Groups[1].Target != "cursor" {
		t.Fatalf("groups = %+v", report.Groups)
	}
	for _, g := range report.Groups {
		if g.Tokens != counted || g.Measured != 1 || len(g.Servers) != 2 {
			t.Fatalf("group %s = %+v, want %d tokens from 1 of 2 servers", g.Target, g, counted)
		}
		if remote := g.Servers[1]; remote.Name != "remote" || remote.Checked != nil {
			t.Fatalf("uncounted server = %+v", remote)
		}
	}
}

func TestToolCostsReadsBrokenCacheAsEmpty(t *testing.T) {
	s := checkService(t, liveConfig("pinned", nil))
	if err := os.MkdirAll(s.StateDir+"/mcp", 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(s.toolCostsPath(), []byte("{"), 0600); err != nil {
		t.Fatal(err)
	}
	report, err := s.ToolCosts()
	if err != nil {
		t.Fatal(err)
	}
	if len(report.Groups) != 1 || report.Groups[0].Measured != 0 {
		t.Fatalf("groups = %+v", report.Groups)
	}
}

// An Agent in a project also loads the global servers the project has not turned off, so a
// project's total starts from them: counting only what the project adds hid a project over budget.
func TestToolCostsProjectGroupIncludesGlobalServersItKeeps(t *testing.T) {
	s, tmp := projectsService(t, `mcp:
  targets: [claude]
  servers:
    plain:
      command: plain-server
    noisy:
      command: noisy-server
    docs:
      global: false
      url: https://docs.example.com/mcp
  projects:
    $TMP/web:
      use: [docs]
      servers:
        local:
          command: local-server
        noisy:
          disabled: true
`)
	web := filepath.Join(tmp, "web")
	counted := func(n int) ToolCost { return ToolCost{Tools: 1, Tokens: n, Checked: time.Now()} }
	if err := writeJSONFile(s.toolCostsPath(), toolCostFile{Version: 1, Servers: map[string]map[string]ToolCost{
		"":  {"plain": counted(100), "noisy": counted(1000), "docs": counted(10)},
		web: {"local": counted(1)},
	}}); err != nil {
		t.Fatal(err)
	}
	report, err := s.ToolCosts()
	if err != nil {
		t.Fatal(err)
	}
	totals := map[string]int{}
	for _, g := range report.Groups {
		totals[g.Root+"@"+g.Target] = g.Tokens
	}
	if want := map[string]int{"@claude": 1100, web + "@claude": 111}; !maps.Equal(totals, want) {
		t.Fatalf("totals = %v, want %v", totals, want)
	}
}

// A project that only turns a global server off adds nothing of its own, yet its Agent loads a
// different set there, so it still gets a group: the global servers it keeps on.
func TestToolCostsProjectThatOnlyTurnsAServerOff(t *testing.T) {
	s, tmp := projectsService(t, `mcp:
  targets: [claude]
  servers:
    big:
      command: big-server
    small:
      command: small-server
  projects:
    $TMP/web:
      servers:
        small:
          disabled: true
`)
	counted := func(n int) ToolCost { return ToolCost{Tools: 1, Tokens: n, Checked: time.Now()} }
	if err := writeJSONFile(s.toolCostsPath(), toolCostFile{Version: 1, Servers: map[string]map[string]ToolCost{
		"": {"big": counted(40000), "small": counted(1000)},
	}}); err != nil {
		t.Fatal(err)
	}
	report, err := s.ToolCosts()
	if err != nil {
		t.Fatal(err)
	}
	totals := map[string]int{}
	for _, g := range report.Groups {
		totals[g.Root+"@"+g.Target] = g.Tokens
	}
	if want := map[string]int{"@claude": 41000, filepath.Join(tmp, "web") + "@claude": 40000}; !maps.Equal(totals, want) {
		t.Fatalf("totals = %v, want %v", totals, want)
	}
}
