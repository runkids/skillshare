package mcp

import (
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"slices"
	"strings"
	"testing"
)

func renderJSON(t *testing.T, target string, server Server, keys ...string) string {
	t.Helper()
	entry, err := Render(target, server)
	if err != nil {
		t.Fatal(err)
	}
	picked := map[string]any{}
	for _, key := range keys {
		if value, ok := entry[key]; ok {
			picked[key] = value
		}
	}
	data, _ := json.Marshal(picked)
	return string(data)
}

// Pi takes an exact name over any pattern and the first matching pattern, so denied tools
// come first and "*": "hidden" last; an allowed name a denied pattern matches is left out.
func TestPiToolPolicyOrder(t *testing.T) {
	for name, tc := range map[string]struct {
		tools    ToolPolicy
		exposure string
		want     string
	}{
		"allow and deny": {ToolPolicy{Allow: []string{"get_*", "delete_all", "search"}, Deny: []string{"delete_*"}}, "",
			`{"toolExposure":{"delete_*":"hidden","get_*":"codemode","search":"codemode","*":"hidden"}}`},
		"allowed tools take piOptions.exposure": {ToolPolicy{Allow: []string{"get_*"}}, "codemode-deferred",
			`{"exposure":"codemode-deferred","toolExposure":{"get_*":"codemode-deferred","*":"hidden"}}`},
		"hidden server, allowed tools at Pi's default": {ToolPolicy{Allow: []string{"get_*"}}, "hidden",
			`{"exposure":"hidden","toolExposure":{"get_*":"codemode","*":"hidden"}}`},
		"deny only": {ToolPolicy{Deny: []string{"delete_*"}}, "", `{"toolExposure":{"delete_*":"hidden"}}`},
		"no policy": {ToolPolicy{}, "", `{}`},
	} {
		server := Server{Command: "x", Tools: tc.tools}
		if tc.exposure != "" {
			server.PiOptions = PiOptions{"exposure": tc.exposure}
		}
		if got := renderJSON(t, "pi", server, "exposure", "toolExposure"); got != tc.want {
			t.Errorf("%s: got %s want %s", name, got, tc.want)
		}
	}
}

func TestCopilotToolPolicy(t *testing.T) {
	for name, tc := range map[string]struct {
		tools ToolPolicy
		want  string
	}{
		"allowed names":            {ToolPolicy{Allow: []string{"get_issue", "delete_issue"}, Deny: []string{"delete_*"}}, `{"tools":["get_issue"]}`},
		"no allow":                 {ToolPolicy{Deny: []string{"delete_*"}}, `{"tools":["*"]}`},
		"patterns are not applied": {ToolPolicy{Allow: []string{"get_*"}}, `{"tools":["*"]}`},
	} {
		if got := renderJSON(t, "copilot", Server{Command: "x", Tools: tc.tools}, "tools"); got != tc.want {
			t.Errorf("%s: got %s want %s", name, got, tc.want)
		}
	}
}

func TestCodexToolPolicy(t *testing.T) {
	for name, tc := range map[string]struct {
		tools ToolPolicy
		want  string
	}{
		"names":                             {ToolPolicy{Allow: []string{"open", "screenshot"}, Deny: []string{"screenshot"}}, `{"disabled_tools":["screenshot"],"enabled_tools":["open","screenshot"]}`},
		"denied patterns leave the allowed": {ToolPolicy{Allow: []string{"open", "delete_all"}, Deny: []string{"delete_*"}}, `{"enabled_tools":["open"]}`},
		"allowed patterns are not applied":  {ToolPolicy{Allow: []string{"get_*"}, Deny: []string{"delete_all"}}, `{"disabled_tools":["delete_all"]}`},
	} {
		if got := renderJSON(t, "codex", Server{Command: "x", Tools: tc.tools}, "enabled_tools", "disabled_tools"); got != tc.want {
			t.Errorf("%s: got %s want %s", name, got, tc.want)
		}
	}
}

// Codex's tool lists are managed fields, so clearing the policy removes them on the next sync.
func TestCodexToolPolicyRemovedWithThePolicy(t *testing.T) {
	s := testService(t)
	sync := func(tools string) string {
		t.Helper()
		config := "mcp:\n  targets: [codex]\n  servers:\n    docs:\n      command: docs\n" + tools
		if err := os.WriteFile(s.ConfigPath, []byte(config), 0600); err != nil {
			t.Fatal(err)
		}
		plan, err := s.Preview()
		if err != nil || plan.Blocked {
			t.Fatalf("%+v %v", plan, err)
		}
		if _, err := s.Apply(plan.Revision); err != nil {
			t.Fatal(err)
		}
		data, _ := os.ReadFile(filepath.Join(s.Home, ".codex", "config.toml"))
		return string(data)
	}
	if got := sync("      tools:\n        allow: [open]\n"); !strings.Contains(got, "enabled_tools") {
		t.Fatalf("not written: %s", got)
	}
	if got := sync(""); strings.Contains(got, "enabled_tools") {
		t.Fatalf("not removed: %s", got)
	}
}

// A part of a policy an Agent cannot hold is named in the plan and by check, never dropped silently.
func TestToolPolicyNotAppliedIsReported(t *testing.T) {
	s, _ := projectsService(t, `mcp:
  targets: [pi, cursor, copilot, codex]
  servers:
    github:
      url: https://example.com/mcp
      tools:
        allow: ["get_*"]
        deny: [delete_repo]
    plain:
      url: https://example.com/plain
`)
	plan, err := s.Preview()
	if err != nil {
		t.Fatal(err)
	}
	want := []string{
		"tool policy not applied for codex: allow patterns (github)",
		"tool policy not applied for copilot: allow patterns, deny (github)",
		"tool policy not applied for cursor: allow, deny (github)",
	}
	if !reflect.DeepEqual(plan.Notices, want) {
		t.Fatalf("notices:\n%q", plan.Notices)
	}
	report, err := s.Check(CheckOptions{SkipDNS: true, Names: []string{"github"}})
	if err != nil {
		t.Fatal(err)
	}
	var found []string
	for _, f := range report.Servers[0].Findings {
		if f.Check == "tools" {
			found = append(found, f.Message+" ("+report.Servers[0].Name+")")
		}
	}
	slices.Sort(found)
	if !reflect.DeepEqual(found, want) {
		t.Fatalf("check findings: %q", found)
	}
}

func TestToolPolicyRejected(t *testing.T) {
	for name, tc := range map[string]struct {
		server Server
		want   string
	}{
		"another wildcard":      {Server{Command: "x", Tools: ToolPolicy{Allow: []string{"get_?"}}}, "not a tool name"},
		"listed twice":          {Server{Command: "x", Tools: ToolPolicy{Deny: []string{"a", "a"}}}, "twice"},
		"every tool denied":     {Server{Command: "x", Tools: ToolPolicy{Allow: []string{"delete_a"}, Deny: []string{"delete_*"}}}, "removes every tool"},
		"switch-only entry":     {Server{Disabled: true, Tools: ToolPolicy{Deny: []string{"a"}}}, "disabled"},
		"Pi exposure elsewhere": {Server{Command: "x", Tools: ToolPolicy{Deny: []string{"a"}}, PiOptions: PiOptions{"toolExposure": map[string]any{"b": "hidden"}}}, "keep it in tools"},
		"adapter tool list":     {Server{Command: "x", PiOptions: PiOptions{"excludeTools": []any{"a"}}}, "use tools.deny"},
	} {
		if err := tc.server.Validate("docs"); err == nil || !strings.Contains(err.Error(), tc.want) {
			t.Errorf("%s: want %q, got %v", name, tc.want, err)
		}
	}
}

// Pi's exposure mode lives in piOptions and sits next to a tool policy.
func TestToolPolicyKeepsPiExposure(t *testing.T) {
	server := Server{Command: "x", Tools: ToolPolicy{Deny: []string{"a"}}, PiOptions: PiOptions{"exposure": "direct"}}
	if err := server.Validate("docs"); err != nil {
		t.Fatal(err)
	}
}

// What import reads back into tools renders to the same native fields.
func TestToolPolicyImportRoundTrip(t *testing.T) {
	for target, tools := range map[string]ToolPolicy{
		"pi":      {Allow: []string{"get_*", "search"}, Deny: []string{"delete_*"}},
		"copilot": {Allow: []string{"get_issue", "list_issues"}},
		"codex":   {Allow: []string{"open", "screenshot"}, Deny: []string{"screenshot"}},
	} {
		s := testService(t)
		if err := os.WriteFile(s.ConfigPath, []byte("mcp:\n  servers: {}\n"), 0600); err != nil {
			t.Fatal(err)
		}
		rendered := s.RenderNative("docs", Server{Command: "docs", Targets: TargetList{target}, Tools: tools})
		if rendered[0].Error != "" {
			t.Fatalf("%s: %s", target, rendered[0].Error)
		}
		candidates, err := Import(target, []byte(rendered[0].Content), "")
		if err != nil || len(candidates) != 1 {
			t.Fatalf("%s: %+v %v", target, candidates, err)
		}
		if got := candidates[0].Server; !reflect.DeepEqual(got.Tools, tools) || got.PiOptions != nil {
			t.Errorf("%s: got %+v %v, warnings %v", target, got.Tools, got.PiOptions, candidates[0].Warnings)
		}
	}
}

func TestRenderNativeNamesTheToolPolicyPartsEachAgentCannotHold(t *testing.T) {
	service := testService(t)
	got := map[string][]string{}
	for _, r := range service.RenderNative("docs", Server{Command: "docs", Targets: TargetList{"pi", "copilot", "opencode"}, Tools: ToolPolicy{Allow: []string{"get_*"}, Deny: []string{"delete_issue"}}}) {
		got[r.Target] = r.ToolGaps
	}
	want := map[string][]string{"pi": nil, "copilot": {"allow patterns", "deny"}, "opencode": {"allow", "deny"}}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got %v, want %v", got, want)
	}
}

// Pi exposure a policy cannot express exactly stays in piOptions, for Pi only.
func TestPiExposureKeptWhenNotExpressible(t *testing.T) {
	candidates, err := Import("pi", []byte(`{"mcpServers":{"docs":{"command":"docs","exposure":"codemode-deferred","toolExposure":{"a":"direct"}}}}`), "")
	if err != nil || len(candidates) != 1 {
		t.Fatalf("%+v %v", candidates, err)
	}
	c := candidates[0]
	if !c.Server.Tools.IsZero() || c.Server.PiOptions["exposure"] != "codemode-deferred" || c.Server.PiOptions["toolExposure"] == nil || !strings.Contains(strings.Join(c.Warnings, ";"), "stays in piOptions") {
		t.Fatalf("%+v %v", c.Server, c.Warnings)
	}
}
