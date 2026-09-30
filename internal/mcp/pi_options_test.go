package mcp

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// piOptions carries the Pi fields Skillshare has no setting for. Refs: #289.
func TestPiOptionsRenderedOnlyForPi(t *testing.T) {
	server := Server{Command: "echo", PiOptions: map[string]any{"excludeTools": []any{"*emulator*"}}}
	out, err := Render("pi", server)
	if err != nil {
		t.Fatal(err)
	}
	if list, ok := out["excludeTools"].([]any); !ok || len(list) != 1 {
		t.Fatalf("pi entry: %v", out)
	}
	out, err = Render("opencode", server)
	if err != nil || out["excludeTools"] != nil {
		t.Fatalf("piOptions leaked into another Agent: %v %v", out, err)
	}
}

func TestPiOptionsRejected(t *testing.T) {
	options := map[string]any{"excludeTools": []any{"a"}}
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
	if got := sync("      piOptions:\n        excludeTools: [\"*emulator*\"]\n        retries: 3\n", "update"); !strings.Contains(got, `"*emulator*"`) || !strings.Contains(got, `"retries": 3`) {
		t.Fatalf("options not written: %s", got)
	}
	sync("      piOptions:\n        excludeTools: [\"*emulator*\"]\n        retries: 3\n", "unchanged")
}
