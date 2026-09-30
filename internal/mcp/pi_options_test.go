package mcp

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// piOptions carries the Pi fields Skillshare has no setting for. Refs: #289.
func TestPiOptionsRenderedOnlyForPi(t *testing.T) {
	server := Server{Command: "echo", PiOptions: map[string]any{"retries": 3}}
	out, err := Render("pi", server)
	if err != nil {
		t.Fatal(err)
	}
	if out["retries"] != 3 {
		t.Fatalf("pi entry: %v", out)
	}
	out, err = Render("opencode", server)
	if err != nil || out["retries"] != nil {
		t.Fatalf("piOptions leaked into another Agent: %v %v", out, err)
	}
}

func TestPiOptionsRejected(t *testing.T) {
	options := map[string]any{"timeout": 30}
	for name, server := range map[string]Server{
		"switch only":               {Disabled: true, PiOptions: options},
		"a field Skillshare writes": {Command: "echo", PiOptions: map[string]any{"command": "other"}},
		"directTools":               {Command: "echo", PiOptions: map[string]any{"directTools": true}},
	} {
		if err := server.Validate("docs"); err == nil || !strings.Contains(err.Error(), "piOptions") {
			t.Errorf("%s: got %v", name, err)
		}
	}
}

func TestPiOptionsSyncFollowsConfig(t *testing.T) {
	s := testService(t)
	sync := func(options, wantAction string) string {
		t.Helper()
		config := "mcp:\n  targets: [pi]\n  servers:\n    docs:\n      command: docs\n" + options
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
		data, _ := os.ReadFile(filepath.Join(s.Home, ".pi", "agent", "mcp.json"))
		return string(data)
	}
	sync("", "add")
	if got := sync("      piOptions:\n        timeout: 30\n        retries: 3\n", "update"); !strings.Contains(got, `"timeout": 30`) || !strings.Contains(got, `"retries": 3`) {
		t.Fatalf("options not written: %s", got)
	}
	sync("      piOptions:\n        timeout: 30\n        retries: 3\n", "unchanged")
}

// pi-mcp-adapter options reached Pi's file through piOptions before 0.23.0. Pi's built-in
// MCP does not read them, so loading drops them with a notice; other keys still pass through.
func TestAdapterPiOptionsDroppedOnLoad(t *testing.T) {
	s, _ := projectsService(t, `mcp:
  targets: [pi]
  servers:
    docs:
      command: docs
      piOptions:
        lifecycle: eager
        idleTimeout: 5
        timeout: 30
        retries: 3
`)
	plan, err := s.Preview()
	if err != nil || plan.Blocked {
		t.Fatalf("%+v %v", plan, err)
	}
	if len(plan.Notices) != 2 || plan.Notices[0] != "Pi's built-in MCP does not read idleTimeout, lifecycle; the next sync removes them: docs" || plan.Notices[1] != PiBuiltinNotice {
		t.Fatalf("notices: %q", plan.Notices)
	}
	if _, err := s.Apply(plan.Revision); err != nil {
		t.Fatal(err)
	}
	data, _ := os.ReadFile(filepath.Join(s.Home, ".pi", "agent", "mcp.json"))
	if strings.Contains(string(data), "lifecycle") || strings.Contains(string(data), "idleTimeout") || !strings.Contains(string(data), `"retries": 3`) || !strings.Contains(string(data), `"timeout": 30`) {
		t.Fatalf("pi file: %s", data)
	}
}

func TestAdapterPiOptionsRejectedOnSave(t *testing.T) {
	err := Server{Command: "echo", PiOptions: map[string]any{"lifecycle": "eager"}}.Validate("docs")
	if err == nil || !strings.Contains(err.Error(), "piOptions.lifecycle is a pi-mcp-adapter setting") {
		t.Fatalf("got %v", err)
	}
}
