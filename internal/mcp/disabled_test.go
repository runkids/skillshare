package mcp

import (
	"encoding/json"
	"maps"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

// A project turns off a server that the Agent's global config defines by writing an
// entry holding only the switch; the Agent merges it over the global one by field.
func TestDisabledTurnsOffGlobalServerInProject(t *testing.T) {
	s := testService(t)
	s.ProjectRoot = filepath.Join(s.Home, "project")
	source := "mcp:\n  servers:\n    docs:\n      disabled: true\n      targets: [opencode, kilocode]\n"
	if err := os.WriteFile(s.ConfigPath, []byte(source), 0600); err != nil {
		t.Fatal(err)
	}
	plan, err := s.Preview()
	if err != nil {
		t.Fatal(err)
	}
	if _, err = s.Apply(plan.Revision); err != nil {
		t.Fatal(err)
	}
	for file, want := range map[string]string{
		"opencode.json": `{"mcp":{"docs":{"enabled":false}}}`,
		"kilo.jsonc":    `{"mcp":{"docs":{"enabled":false}}}`,
	} {
		data, err := os.ReadFile(filepath.Join(s.ProjectRoot, file))
		if err != nil {
			t.Fatal(err)
		}
		var got, expected any
		if json.Unmarshal(data, &got) != nil || json.Unmarshal([]byte(want), &expected) != nil {
			t.Fatalf("%s: %s", file, data)
		}
		a, _ := json.Marshal(got)
		b, _ := json.Marshal(expected)
		if string(a) != string(b) {
			t.Fatalf("%s: got %s want %s", file, a, b)
		}
	}
	if plan, err = s.Preview(); err != nil || len(plan.Changes) != 2 || plan.Changes[0].Action != "unchanged" {
		t.Fatalf("not idempotent: %+v %v", plan, err)
	}
	if err := os.WriteFile(s.ConfigPath, []byte("mcp:\n  servers: {}\n"), 0600); err != nil {
		t.Fatal(err)
	}
	if plan, err = s.Preview(); err != nil || len(plan.Changes) != 2 || plan.Changes[0].Action != "remove" {
		t.Fatalf("switch not removed with its source: %+v %v", plan, err)
	}
}

func TestDisabledRejected(t *testing.T) {
	for name, tc := range map[string]struct {
		project bool
		server  string
		want    string
	}{
		"global mode":       {false, "disabled: true\n      targets: [opencode]", "project"},
		"whole-entry agent": {true, "disabled: true\n      targets: [cursor]", "cursor"},
		"codex":             {true, "disabled: true\n      targets: [codex]", "whole config"},
		"pi":                {true, "disabled: true\n      targets: [pi]", "pi cannot turn off"},
		"with a command":    {true, "disabled: true\n      command: tool\n      targets: [opencode]", "leave out"},
	} {
		t.Run(name, func(t *testing.T) {
			s := testService(t)
			if tc.project {
				s.ProjectRoot = filepath.Join(s.Home, "project")
			}
			if err := os.WriteFile(s.ConfigPath, []byte("mcp:\n  servers:\n    docs:\n      "+tc.server+"\n"), 0600); err != nil {
				t.Fatal(err)
			}
			if _, err := s.Preview(); err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("want %q, got %v", tc.want, err)
			}
		})
	}
}

// The dashboard's preview must refuse what saving would refuse.
func TestPreviewRefusesDisabledInGlobalMode(t *testing.T) {
	s := testService(t)
	got := s.RenderNative("docs", Server{Disabled: true, Targets: []string{"opencode"}})
	if len(got) != 1 || got[0].Content != "" || !strings.Contains(got[0].Error, "project mode") {
		t.Fatalf("preview %+v", got)
	}
}

// Claude Code replaces a whole entry across scopes, so a lone switch in .mcp.json would
// break the server. It keeps a per-project off list in ~/.claude.json instead, the one
// /mcp edits. The global config may manage a server of the same name in that same file.
func TestClaudeDisabledUsesProjectOffList(t *testing.T) {
	global := testService(t)
	plan, err := global.Preview()
	if err != nil {
		t.Fatal(err)
	}
	if _, err = global.Apply(plan.Revision); err != nil {
		t.Fatal(err)
	}
	s := *global
	s.ProjectRoot = filepath.Join(s.Home, "project")
	s.ConfigPath = filepath.Join(s.ProjectRoot, ".skillshare", "config.yaml")
	if err := os.MkdirAll(filepath.Dir(s.ConfigPath), 0755); err != nil {
		t.Fatal(err)
	}
	file := filepath.Join(s.Home, ".claude.json")
	var document map[string]any
	data, _ := os.ReadFile(file)
	if err := json.Unmarshal(data, &document); err != nil {
		t.Fatal(err)
	}
	document["projects"] = map[string]any{s.ProjectRoot: map[string]any{"disabledMcpServers": []string{"mine"}}}
	data, _ = json.Marshal(document)
	if err := os.WriteFile(file, data, 0600); err != nil {
		t.Fatal(err)
	}
	offList := func() string {
		t.Helper()
		var got struct {
			McpServers map[string]any
			Projects   map[string]struct{ DisabledMcpServers []string }
		}
		data, _ := os.ReadFile(file)
		if err := json.Unmarshal(data, &got); err != nil || got.McpServers["docs"] == nil {
			t.Fatalf("user-scope docs lost: %s", data)
		}
		return strings.Join(got.Projects[s.ProjectRoot].DisabledMcpServers, ",")
	}
	sync := func(source, action string) {
		t.Helper()
		if err := os.WriteFile(s.ConfigPath, []byte(source), 0600); err != nil {
			t.Fatal(err)
		}
		plan, err := s.Preview()
		if err != nil || plan.Blocked || len(plan.Changes) != 1 || plan.Changes[0].Action != action || plan.Changes[0].Target != "claude" {
			t.Fatalf("want %s: %+v %v", action, plan, err)
		}
		if _, err = s.Apply(plan.Revision); err != nil {
			t.Fatal(err)
		}
	}
	off := "mcp:\n  servers:\n    docs:\n      disabled: true\n      targets: [claude]\n"
	sync(off, "add")
	if got := offList(); got != "mine,docs" {
		t.Fatalf("off list %q", got)
	}
	if _, err := os.Stat(filepath.Join(s.ProjectRoot, ".mcp.json")); !os.IsNotExist(err) {
		t.Fatal("the switch belongs in ~/.claude.json, not .mcp.json")
	}
	sync(off, "unchanged")
	sync("mcp:\n  servers: {}\n", "remove")
	if got := offList(); got != "mine" {
		t.Fatalf("off list after removal %q", got)
	}
	// A name the person turned off in /mcp is theirs: never claimed, never removed.
	sync("mcp:\n  servers:\n    mine:\n      disabled: true\n      targets: [claude]\n", "unchanged")
	if err := os.WriteFile(s.ConfigPath, []byte("mcp:\n  servers: {}\n"), 0600); err != nil {
		t.Fatal(err)
	}
	if plan, err = s.Preview(); err != nil || len(plan.Changes) != 0 {
		t.Fatalf("claimed the person's own switch: %+v %v", plan, err)
	}
	if got := s.RenderNative("docs", Server{Disabled: true, Targets: []string{"claude"}}); got[0].Error != "" || got[0].Path != file || !strings.Contains(got[0].Content, "disabledMcpServers") {
		t.Fatalf("preview %+v", got)
	}
}

// One plan writes ~/.claude.json twice: the user-scope servers, then the off list of a
// root under mcp.projects that turns one of them off. Both live in that one file.
func TestClaudeProjectsOffListSharesTheGlobalFile(t *testing.T) {
	s := testService(t)
	root := filepath.Join(s.Home, "project")
	file := filepath.Join(s.Home, ".claude.json")
	write := func(project string) {
		t.Helper()
		source := "mcp:\n  targets: [claude]\n  servers:\n    docs:\n      url: https://example.com/mcp\n  projects:\n    " + root + ":\n      servers:\n" + project
		if err := os.WriteFile(s.ConfigPath, []byte(source), 0600); err != nil {
			t.Fatal(err)
		}
		plan, err := s.Preview()
		if err != nil || plan.Blocked {
			t.Fatalf("preview %+v %v", plan, err)
		}
		// Both changes name ~/.claude.json as claude, so only Root tells the off switch from
		// the server. The dashboard needs it to keep the switch out of the global list.
		var roots []string
		for _, c := range plan.Changes {
			if c.Path == file && c.Target == "claude" && c.Name == "docs" {
				roots = append(roots, c.Root)
			}
		}
		slices.Sort(roots)
		if want := []string{"", root}; !slices.Equal(roots, want) {
			t.Fatalf("roots of the docs changes are %q, want %q", roots, want)
		}
		if _, err := s.Apply(plan.Revision); err != nil {
			t.Fatal(err)
		}
	}
	read := func() (map[string]any, []string) {
		t.Helper()
		var got struct {
			McpServers map[string]any
			Projects   map[string]struct{ DisabledMcpServers []string }
		}
		data, _ := os.ReadFile(file)
		if err := json.Unmarshal(data, &got); err != nil {
			t.Fatalf("%v: %s", err, data)
		}
		return got.McpServers, got.Projects[root].DisabledMcpServers
	}

	write("        docs:\n          disabled: true\n          targets: [claude]\n")
	servers, off := read()
	if servers["docs"] == nil {
		t.Fatal("the off list overwrote the user-scope server")
	}
	if strings.Join(off, ",") != "docs" {
		t.Fatalf("off list %q", off)
	}
	if _, err := os.Stat(filepath.Join(root, ".mcp.json")); !os.IsNotExist(err) {
		t.Fatal("the switch belongs in ~/.claude.json, not .mcp.json")
	}

	// Turning it back on removes only the name, leaving the server it named.
	write("        {}\n")
	if servers, off = read(); servers["docs"] == nil || len(off) != 0 {
		t.Fatalf("back on: %v %q", servers["docs"], off)
	}
}

// Claude Code lets a local-scope server win over .mcp.json and the user scope, whole.
func TestClaudeLocalScopeShadowIsReported(t *testing.T) {
	s := testService(t)
	s.ProjectRoot = filepath.Join(s.Home, "project")
	local, _ := json.Marshal(map[string]any{"projects": map[string]any{s.ProjectRoot: map[string]any{"mcpServers": map[string]any{"docs": map[string]any{"command": "other"}}}}})
	if err := os.WriteFile(filepath.Join(s.Home, ".claude.json"), local, 0600); err != nil {
		t.Fatal(err)
	}
	plan, err := s.Preview()
	if err != nil || plan.Blocked {
		t.Fatalf("%+v %v", plan, err)
	}
	for _, c := range plan.Changes {
		if shadowed := strings.Contains(c.Message, "local scope"); shadowed != (c.Target == "claude") {
			t.Fatalf("%s: %q", c.Target, c.Message)
		}
	}
}

// The plan says which changes only flip a switch, so a review can say "turned off here"
// rather than "new server entry". A removal has no source entry left to ask.
func TestPlanMarksSwitchOnlyChanges(t *testing.T) {
	s := testService(t)
	s.ProjectRoot = filepath.Join(s.Home, "project")
	marks := func(source, action string) map[string]bool {
		t.Helper()
		if err := os.WriteFile(s.ConfigPath, []byte(source), 0600); err != nil {
			t.Fatal(err)
		}
		plan, err := s.Preview()
		if err != nil {
			t.Fatal(err)
		}
		got := map[string]bool{}
		for _, c := range plan.Changes {
			if c.Action != action {
				t.Fatalf("%s %s: %s, want %s", c.Target, c.Name, c.Action, action)
			}
			got[c.Target+"/"+c.Name] = c.Switch
		}
		if _, err = s.Apply(plan.Revision); err != nil {
			t.Fatal(err)
		}
		return got
	}
	want := map[string]bool{"opencode/docs": true, "claude/docs": true, "opencode/own": false}
	added := marks("mcp:\n  servers:\n    docs:\n      disabled: true\n      targets: [opencode, claude]\n    own:\n      command: tool\n      targets: [opencode]\n", "add")
	if !maps.Equal(added, want) {
		t.Fatalf("add: got %v want %v", added, want)
	}
	if removed := marks("mcp:\n  servers: {}\n", "remove"); !maps.Equal(removed, want) {
		t.Fatalf("remove: got %v want %v", removed, want)
	}
}

// A switch that names no targets follows the project: it goes to the Agents the project uses,
// that the global server reaches and that have a switch. Storing the list instead left it
// stale as soon as the project's targets changed.
func TestSwitchWithoutTargetsFollowsTheProject(t *testing.T) {
	for name, tc := range map[string]struct {
		global, project string
		want            []string
	}{
		"only what the project uses":    {"claude, opencode, kilocode, pi", "opencode, kilocode", []string{"kilocode", "opencode"}},
		"only what the server reaches":  {"claude, codex, pi", "claude, opencode", []string{"claude"}},
		"skips an Agent with no switch": {"cursor, opencode, pi", "cursor, opencode, pi", []string{"opencode"}},
	} {
		t.Run(name, func(t *testing.T) {
			s, tmp := projectsService(t, "mcp:\n  servers:\n    docs:\n      command: tool\n      targets: ["+tc.global+"]\n  projects:\n    $TMP/p1:\n      targets: ["+tc.project+"]\n      servers:\n        docs:\n          disabled: true\n")
			plan, err := s.Preview()
			if err != nil {
				t.Fatal(err)
			}
			var got []string
			for _, c := range plan.Changes {
				if c.Root == filepath.Join(tmp, "p1") {
					got = append(got, c.Target)
				}
			}
			slices.Sort(got)
			if !slices.Equal(got, tc.want) {
				t.Fatalf("got %v want %v", got, tc.want)
			}
		})
	}
}

// Pi's built-in MCP has no per-project switch, so a switch that names no targets leaves Pi
// alone even where the project and the global server both reach it.
func TestSwitchLeavesPiAlone(t *testing.T) {
	s, tmp := projectsService(t, "mcp:\n  servers:\n    docs:\n      command: tool\n      targets: [opencode, pi]\n  projects:\n    $TMP/p1:\n      targets: [opencode, pi]\n      servers:\n        mine:\n          command: tool\n          targets: [pi]\n        docs:\n          disabled: true\n")
	plan, err := s.Preview()
	if err != nil {
		t.Fatal(err)
	}
	var got []string
	for _, c := range plan.Changes {
		if c.Root == filepath.Join(tmp, "p1") && c.Name == "docs" {
			got = append(got, c.Target)
		}
	}
	if !slices.Equal(got, []string{"opencode"}) {
		t.Fatalf("got %v", got)
	}
}
