package main

import (
	"os"
	"path/filepath"
	"skillshare/internal/mcp"
	"strings"
	"testing"
)

// The flags that chose Pi's MCP mode and opted in to pruning were removed in 0.23.0; naming
// one says what happens now instead of calling it unknown.
func TestMCPRemovedPiFlags(t *testing.T) {
	for _, args := range [][]string{
		{"docs", "--pi-extension", "builtin"},
		{"docs", "--pi-extension=pi-mcp-adapter"},
		{"docs", "--pi-options-prune"},
		{"docs", "--pi-options-prune=false"},
	} {
		flag, _, _ := strings.Cut(args[1], "=")
		if _, err := parseMCPOptions(args); err == nil || !strings.Contains(err.Error(), flag+" was removed in 0.23.0") {
			t.Errorf("%v: %v", args, err)
		}
	}
	if o, err := parseMCPOptions([]string{"docs", "--", "server", "--pi-extension"}); err != nil || len(o.command) != 2 {
		t.Fatalf("a command argument is not a flag: %+v %v", o, err)
	}
}

func TestMCPPiImportFile(t *testing.T) {
	s := mcpTUIService(t)
	path := filepath.Join(s.Home, "export.json")
	if err := os.WriteFile(path, []byte(`{"mcpServers":{"a":{"command":"c","exposure":"direct"}}}`), 0600); err != nil {
		t.Fatal(err)
	}
	o, err := parseMCPOptions([]string{"a", "--file", path, "--target", "claude", "--no-tui"})
	if err != nil {
		t.Fatal(err)
	}
	if err := runMCPImport(s, o); err != nil {
		t.Fatal(err)
	}
	source, err := mcp.LoadSource(s.ConfigPath)
	if err != nil || source.Servers["a"].PiOptions["exposure"] != "direct" {
		t.Fatalf("%+v %v", source, err)
	}
}

// Servers written by hand into pi-mcp-adapter's file can still be imported.
func TestMCPPiImportReadsAdapterFile(t *testing.T) {
	s := mcpTUIService(t)
	dir := filepath.Join(s.Home, ".pi", "agent")
	if err := os.MkdirAll(dir, 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "mcp-adapter.json"), []byte(`{"mcpServers":{"docs":{"command":"adapter","excludeTools":["delete_*"]}}}`), 0600); err != nil {
		t.Fatal(err)
	}
	o, err := parseMCPOptions([]string{"docs", "--from", "pi", "--target", "claude", "--no-tui"})
	if err != nil {
		t.Fatal(err)
	}
	if err := runMCPImport(s, o); err != nil {
		t.Fatal(err)
	}
	source, err := mcp.LoadSource(s.ConfigPath)
	if err != nil || source.Servers["docs"].Command != "adapter" || source.Servers["docs"].PiOptions["excludeTools"] == nil {
		t.Fatalf("%+v %v", source, err)
	}
	if data, _ := os.ReadFile(filepath.Join(dir, "mcp-adapter.json")); !strings.Contains(string(data), `"adapter"`) {
		t.Fatalf("adapter file changed: %s", data)
	}
}

func TestMCPImportIntoPi(t *testing.T) {
	s := mcpTUIService(t)
	if err := os.WriteFile(filepath.Join(s.Home, ".claude.json"), []byte(`{"mcpServers":{"docs":{"command":"docs"}}}`), 0600); err != nil {
		t.Fatal(err)
	}
	o, err := parseMCPOptions([]string{"docs", "--from", "claude", "--target", "pi", "--no-tui"})
	if err != nil {
		t.Fatal(err)
	}
	if err = runMCPImport(s, o); err != nil {
		t.Fatal(err)
	}
	if data, err := os.ReadFile(s.ConfigPath); err != nil || strings.Contains(string(data), "piExtension") {
		t.Fatalf("%s %v", data, err)
	}
}

type piPrompts struct{ scriptedMCPPrompts }

func (p *piPrompts) choose(c checklistConfig) ([]int, error) {
	for i, item := range c.items {
		if item.label == "pi" {
			return []int{i}, nil
		}
	}
	return nil, errMCPCancelled
}
func TestMCPPiTUI(t *testing.T) {
	s := mcpTUIService(t)
	servers := []mcp.Server{{Command: "echo"}, {URL: "https://example.com/mcp"}}
	targets, err := chooseMCPTargets(s, servers, nil, &piPrompts{})
	if err != nil || len(targets) != 1 || targets[0] != "pi" {
		t.Fatalf("%v %v", targets, err)
	}
}

func TestMCPDirectToolsFlag(t *testing.T) {
	s := mcpTUIService(t)
	run := func(handler func(*mcp.Service, mcpOptions) error, args ...string) any {
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
		return source.Servers["tools"].DirectTools
	}
	if got := run(runMCPAdd, "tools", "--target", "pi", "--direct-tools", "true", "--url", "https://example.com/mcp"); got != true {
		t.Fatalf("add: %v", got)
	}
	if got, ok := run(runMCPEdit, "tools", "--direct-tools", "search_docs,fetch").([]any); !ok || len(got) != 2 || got[1] != "fetch" {
		t.Fatalf("edit to a list: %v", got)
	}
	if got := run(runMCPEdit, "tools", "--direct-tools", "search"); got != "search" {
		t.Fatalf("edit to search: %v", got)
	}
}

func TestMCPPiOptionsFlag(t *testing.T) {
	s := mcpTUIService(t)
	run := func(handler func(*mcp.Service, mcpOptions) error, args ...string) map[string]any {
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
		return source.Servers["tools"].PiOptions
	}
	if got := run(runMCPAdd, "tools", "--target", "pi", "--pi-options", `{"excludeTools":["*emulator*"]}`, "--url", "https://example.com/mcp"); len(got) != 1 {
		t.Fatalf("add: %v", got)
	}
	if got := run(runMCPEdit, "tools", "--pi-options", `{"approveTools":["delete_*"]}`); len(got) != 1 || got["approveTools"] == nil {
		t.Fatalf("edit replaces the options: %v", got)
	}
	if got := run(runMCPEdit, "tools", "--pi-options", `{}`); len(got) != 0 {
		t.Fatalf("an empty object clears them: %v", got)
	}
	if _, err := parseMCPOptions([]string{"tools", "--pi-options", `["a"]`}); err == nil {
		t.Fatal("a JSON array was accepted")
	}
}
