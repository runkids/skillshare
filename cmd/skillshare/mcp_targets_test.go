package main

import (
	"os"
	"path/filepath"
	"skillshare/internal/mcp"
	"skillshare/internal/testutil"
	"slices"
	"strings"
	"testing"
)

// --target none keeps a server in Skillshare without writing it to any Agent. Refs: #289.
func TestMCPTargetNone(t *testing.T) {
	s := mcpTUIService(t)
	targets := func(handler func(*mcp.Service, mcpOptions) error, args ...string) mcp.TargetList {
		t.Helper()
		o, err := parseMCPOptions(append(args, "--no-tui"))
		if err != nil {
			t.Fatal(err)
		}
		if err = handler(s, o); err != nil {
			t.Fatal(err)
		}
		source, err := mcp.LoadSource(s.ConfigPath)
		if err != nil {
			t.Fatal(err)
		}
		return source.Servers[args[0]].Targets
	}
	if got := targets(runMCPAdd, "parked", "--target", "none", "--url", "https://example.com/mcp"); got == nil || len(got) != 0 {
		t.Fatalf("add: %#v", got)
	}
	if got := targets(runMCPAdd, "live", "--target", "claude", "--url", "https://example.com/mcp"); len(got) != 1 {
		t.Fatalf("add: %#v", got)
	}
	if got := targets(runMCPEdit, "live", "--target", "none"); got == nil || len(got) != 0 {
		t.Fatalf("edit: %#v", got)
	}
	if _, err := parseMCPOptions([]string{"mixed", "--target", "none", "--target", "claude"}); err == nil {
		t.Fatal("none was accepted next to a client")
	}
}

// The agy CLI reads the file the antigravity client writes, so the names other commands
// use for it select that client in --target and --from.
func TestMCPAntigravityAliases(t *testing.T) {
	for _, alias := range []string{"antigravity-cli", "agy"} {
		o, err := parseMCPOptions([]string{"docs", "--target", alias, "--from", alias, "--no-tui"})
		if err != nil {
			t.Fatalf("%s: %v", alias, err)
		}
		if !slices.Equal(o.targets, []string{"antigravity"}) || o.from != "antigravity" {
			t.Fatalf("%s: targets %#v, from %q", alias, o.targets, o.from)
		}
	}
	o, err := parseMCPOptions([]string{"docs", "--target", "agy", "--target", "claude", "--no-tui"})
	if err != nil || !slices.Equal(o.targets, []string{"antigravity", "claude"}) {
		t.Fatalf("%#v %v", o.targets, err)
	}
}

type confirmEmptyPrompts struct{ terminalMCPPrompts }

func (confirmEmptyPrompts) choose(checklistConfig) ([]int, error) { return []int{}, nil }

// Confirming the Agent list with nothing selected means none; only Esc cancels.
func TestMCPTargetPickerAcceptsNone(t *testing.T) {
	targets, err := chooseMCPTargets(mcpTUIService(t), []mcp.Server{{Command: "echo"}}, nil, confirmEmptyPrompts{})
	if err != nil || targets == nil || len(targets) != 0 {
		t.Fatalf("%#v %v", targets, err)
	}
}

// A sync has nothing to do for a server no Agent receives, so the plan alone would hide it.
func TestMCPListNamesServersWithoutTargets(t *testing.T) {
	source := &mcp.Source{
		Servers:  map[string]mcp.Server{"parked": {Targets: mcp.TargetList{}}, "inherits": {}, "leaving": {Targets: mcp.TargetList{}}},
		Projects: map[string]mcp.Project{"/repo": {Servers: map[string]mcp.Server{"local": {Targets: mcp.TargetList{}}}}},
	}
	plan := &mcp.Plan{Changes: []mcp.Change{{Name: "leaving", Target: "claude", Action: "remove"}}}
	got := mcpServersWithoutTargets(source, plan)
	if len(got) != 2 || got[0] != [2]string{"parked", "no targets"} || got[1] != [2]string{"local", "no targets (/repo)"} {
		t.Fatalf("%v", got)
	}
}

func TestMCPListShowsWhereASwitchGoes(t *testing.T) {
	source := &mcp.Source{Targets: []string{"cursor", "opencode"}, Servers: map[string]mcp.Server{"docs": {Disabled: true}}}
	got := mcpListItems(source, nil)[0].(mcpListItem).description
	if !strings.Contains(got, "· opencode ·") {
		t.Fatalf("a switch goes only to the Agents that have one: %s", got)
	}
}

func TestMCPListShowsTheTargetsOfTheProjectsThatUseASharedServer(t *testing.T) {
	off := false
	source := &mcp.Source{
		Targets:  []string{"cursor"},
		Servers:  map[string]mcp.Server{"docs": {URL: "https://docs.example/mcp", Global: &off}},
		Projects: map[string]mcp.Project{"/repo": {Targets: []string{"claude"}, Use: []string{"docs"}}},
	}
	plan := &mcp.Plan{Changes: []mcp.Change{{Target: "claude", Name: "docs", Root: "/repo", Action: "add"}}}
	item := mcpListItems(source, plan)[0].(mcpListItem)
	if !strings.Contains(item.description, "· claude ·") {
		t.Fatalf("a shared server goes where its projects send it: %s", item.description)
	}
	// The detail pane is what the screen shows: which projects, and which project each change is in.
	if !strings.Contains(item.detail, "/repo\n") || !strings.Contains(item.detail, "claude: add (/repo)") {
		t.Fatalf("the detail does not name the project:\n%s", item.detail)
	}
}

// A target that is another account of an Agent is an MCP target too.
func TestMCPAccountTarget(t *testing.T) {
	s := mcpTUIService(t)
	work := filepath.Join(s.Home, ".claude-work")
	config := "targets:\n  claude-work:\n    agent: claude\n    config_dir: " + work + "\nmcp: {targets: [claude], servers: {}}\n"
	if err := os.WriteFile(s.ConfigPath, []byte(config), 0600); err != nil {
		t.Fatal(err)
	}
	labels := []string{}
	for _, item := range mcpTargetItems(s, &mcp.Server{URL: "https://example.com/mcp"}) {
		labels = append(labels, item.label)
	}
	if !slices.Contains(labels, "claude-work") {
		t.Fatalf("the picker does not offer the account: %v", labels)
	}
	o, err := parseMCPOptions([]string{"docs", "--target", "claude-work", "--url", "https://example.com/mcp", "--sync", "--no-tui"})
	if err != nil {
		t.Fatal(err)
	}
	if err := runMCPAdd(s, o); err != nil {
		t.Fatal(err)
	}
	if data, _ := os.ReadFile(filepath.Join(work, ".claude.json")); !strings.Contains(string(data), "example.com") {
		t.Fatalf("the account's file: %s", data)
	}
}

// mcp import --from <account> reads the account's own file, not the Agent's default one.
func TestMCPImportFromAccount(t *testing.T) {
	s := mcpTUIService(t)
	work := filepath.Join(s.Home, ".claude-work")
	config := "targets:\n  claude-work:\n    agent: claude\n    config_dir: " + work + "\nmcp: {targets: [claude-work], servers: {}}\n"
	if err := os.WriteFile(s.ConfigPath, []byte(config), 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(work, 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(work, ".claude.json"), []byte(`{"mcpServers":{"docs":{"url":"https://example.com/mcp"}}}`), 0600); err != nil {
		t.Fatal(err)
	}
	o, err := parseMCPOptions([]string{"docs", "--from", "claude-work", "--no-tui"})
	if err != nil {
		t.Fatal(err)
	}
	if err := runMCPImport(s, o); err != nil {
		t.Fatal(err)
	}
	source, err := mcp.LoadSource(s.ConfigPath)
	if err != nil {
		t.Fatal(err)
	}
	if source.Servers["docs"].URL != "https://example.com/mcp" {
		t.Fatalf("imported: %+v", source.Servers)
	}
}

// --file says what to read, --from which client wrote it; an account's file is its Agent's.
func TestMCPImportFileFromAccount(t *testing.T) {
	s := mcpTUIService(t)
	work := filepath.Join(s.Home, ".claude-work")
	config := "targets:\n  claude-work:\n    agent: claude\n    config_dir: " + work + "\nmcp: {targets: [claude-work], servers: {}}\n"
	if err := os.WriteFile(s.ConfigPath, []byte(config), 0600); err != nil {
		t.Fatal(err)
	}
	file := filepath.Join(s.Home, "exported.json")
	if err := os.WriteFile(file, []byte(`{"mcpServers":{"docs":{"url":"https://example.com/mcp"}}}`), 0600); err != nil {
		t.Fatal(err)
	}
	o, err := parseMCPOptions([]string{"docs", "--file", file, "--from", "claude-work", "--no-tui"})
	if err != nil {
		t.Fatal(err)
	}
	if err := runMCPImport(s, o); err != nil {
		t.Fatal(err)
	}
	source, err := mcp.LoadSource(s.ConfigPath)
	if err != nil {
		t.Fatal(err)
	}
	if source.Servers["docs"].URL != "https://example.com/mcp" {
		t.Fatalf("imported: %+v", source.Servers)
	}
}

// Removing an account the MCP config still selects is allowed, but says where the name
// is left behind: the next mcp sync would refuse a target it can no longer resolve.
func TestTargetRemoveWarnsWhenMCPStillNamesTheAccount(t *testing.T) {
	sb := testutil.NewSandbox(t)
	defer sb.Cleanup()

	work := filepath.Join(sb.Home, ".claude-work")
	if err := os.MkdirAll(filepath.Join(work, "skills"), 0755); err != nil {
		t.Fatal(err)
	}
	sb.WriteConfig("source: " + sb.SourcePath + "\nmode: merge\ntargets:\n  claude-work:\n    agent: claude\n    config_dir: " + work +
		"\nmcp:\n  targets: [claude-work]\n  servers:\n    docs:\n      url: https://example.com/mcp\n      targets: [claude-work]\n")

	output := stripANSIWarnings(captureStdout(t, func() {
		if err := targetRemove([]string{"claude-work"}); err != nil {
			t.Fatalf("target remove: %v", err)
		}
	}))
	if !strings.Contains(output, "mcp.targets, mcp.servers.docs still name claude-work") {
		t.Fatalf("no warning about the MCP config:\n%s", output)
	}
	saved, err := os.ReadFile(sb.ConfigPath)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(saved), "agent: claude") {
		t.Fatalf("the removal was blocked: %s", saved)
	}
}
