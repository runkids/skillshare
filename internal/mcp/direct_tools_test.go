package mcp

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// directTools was pi-mcp-adapter's. Pi's built-in MCP does not read it, so until it has a
// replacement it stays in the config and reaches no Agent.
func TestDirectToolsNotRendered(t *testing.T) {
	server := Server{Command: "echo", DirectTools: []any{"search", "fetch"}}
	for _, target := range []string{"pi", "opencode"} {
		out, err := Render(target, server)
		if err != nil || out["directTools"] != nil {
			t.Fatalf("%s: %v %v", target, out, err)
		}
	}
}

func TestDirectToolsRejected(t *testing.T) {
	for name, server := range map[string]Server{
		"number":          {Command: "echo", DirectTools: 3},
		"unknown keyword": {Command: "echo", DirectTools: "all"},
		"empty tool name": {Command: "echo", DirectTools: []any{""}},
		"switch only":     {Disabled: true, DirectTools: true},
	} {
		if err := server.Validate("docs"); err == nil || !strings.Contains(err.Error(), "directTools") {
			t.Errorf("%s: got %v", name, err)
		}
	}
	for name, value := range map[string]any{"true": true, "false": false, "search": "search", "names": []any{"a"}} {
		if err := (Server{Command: "echo", DirectTools: value}).Validate("docs"); err != nil {
			t.Errorf("%s rejected: %v", name, err)
		}
	}
}

func TestDirectToolsKeepsHandAddedValue(t *testing.T) {
	s := testService(t)
	path := filepath.Join(s.Home, ".pi", "agent", "mcp.json")
	sync := func(directTools, wantAction string) string {
		t.Helper()
		config := "mcp:\n  targets: [pi]\n  servers:\n    docs:\n      command: docs\n" + directTools
		if err := os.WriteFile(s.ConfigPath, []byte(config), 0600); err != nil {
			t.Fatal(err)
		}
		plan, err := s.Preview()
		if err != nil || plan.Blocked || plan.Changes[0].Action != wantAction {
			t.Fatalf("want %s, got %+v %v", wantAction, plan, err)
		}
		if _, err := s.Apply(plan.Revision); err != nil {
			t.Fatal(err)
		}
		data, _ := os.ReadFile(path)
		return string(data)
	}
	sync("", "add")
	// A value added by hand is Pi's own field, not a conflict.
	data, _ := os.ReadFile(path)
	if err := os.WriteFile(path, []byte(strings.Replace(string(data), `"command"`, `"directTools": true, "command"`, 1)), 0600); err != nil {
		t.Fatal(err)
	}
	if got := sync("", "unchanged"); !strings.Contains(got, `"directTools": true`) {
		t.Fatalf("hand-added value lost: %s", got)
	}
	if got := sync("      directTools: [search]\n", "unchanged"); !strings.Contains(got, `"directTools": true`) || strings.Contains(got, `"search"`) {
		t.Fatalf("config value synced: %s", got)
	}
}

func TestDirectToolsImportedFromPi(t *testing.T) {
	candidates, err := Import("pi", []byte(`{"mcpServers":{"docs":{"command":"docs","directTools":["search"]}}}`), "")
	if err != nil || len(candidates) != 1 {
		t.Fatalf("%+v %v", candidates, err)
	}
	if list, ok := candidates[0].Server.DirectTools.([]any); !ok || len(list) != 1 {
		t.Fatalf("not imported: %+v", candidates[0])
	}
	if !strings.Contains(strings.Join(candidates[0].Warnings, ";"), "not synced") {
		t.Fatalf("warnings: %v", candidates[0].Warnings)
	}
}

func TestDirectToolsLoadsWithNotice(t *testing.T) {
	s, tmp := projectsService(t, `mcp:
  targets: [pi]
  directTools: true
  servers:
    own:
      command: b
      directTools: [search]
  projects:
    $TMP/quiet:
      directTools: false
      servers:
        local:
          command: c
          directTools: true
`)
	plan, err := s.Preview()
	if err != nil || plan.Blocked {
		t.Fatalf("%+v %v", plan, err)
	}
	if len(plan.Notices) != 1 {
		t.Fatalf("notices: %v", plan.Notices)
	}
	for _, want := range []string{"mcp.directTools", "own", "local (" + filepath.Join(tmp, "quiet") + ")", "directTools (" + filepath.Join(tmp, "quiet") + ")"} {
		if !strings.Contains(plan.Notices[0], want) {
			t.Errorf("notice lacks %q: %s", want, plan.Notices[0])
		}
	}
}

func TestDirectToolsDefaultRejectsBadValue(t *testing.T) {
	s, _ := projectsService(t, "mcp:\n  directTools: all\n")
	if _, err := s.Preview(); err == nil || !strings.Contains(err.Error(), "directTools") {
		t.Fatalf("got %v", err)
	}
}
