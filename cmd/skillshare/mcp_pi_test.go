package main

import (
	"skillshare/internal/mcp"
	"testing"
)

func TestMCPPiCLI(t *testing.T) {
	s := mcpTUIService(t)
	o, err := parseMCPOptions([]string{"docs", "--target", "pi", "--pi-extension", "pi-mcp-extension", "--url", "https://example.com/mcp", "--no-tui"})
	if err != nil {
		t.Fatal(err)
	}
	if err = runMCPAdd(s, o); err != nil {
		t.Fatal(err)
	}
	source, err := mcp.LoadSource(s.ConfigPath)
	if err != nil || source.Servers["docs"].PiExtension != "pi-mcp-extension" {
		t.Fatalf("%+v %v", source, err)
	}
	o, err = parseMCPOptions([]string{"docs", "--pi-extension", "pi-mcp-adapter", "--no-tui"})
	if err != nil {
		t.Fatal(err)
	}
	if err = runMCPEdit(s, o); err != nil {
		t.Fatal(err)
	}
	source, err = mcp.LoadSource(s.ConfigPath)
	if err != nil || source.Servers["docs"].PiExtension != "pi-mcp-adapter" {
		t.Fatalf("%+v %v", source, err)
	}
}

type piPrompts struct{ scriptedMCPPrompts }

func (p *piPrompts) choose(c checklistConfig) ([]int, error) {
	for i, item := range c.items {
		if item.label == "pi" || item.label == "pi-mcp-extension" {
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
	for _, server := range servers {
		if server.PiExtension != "pi-mcp-extension" {
			t.Fatal("extension selection lost")
		}
	}
}
