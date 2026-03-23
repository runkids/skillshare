package mcp_test

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"skillshare/internal/mcp"
)

func TestLoadMCPState_FileNotExist(t *testing.T) {
	path := filepath.Join(t.TempDir(), "mcp_state.yaml")

	state, err := mcp.LoadMCPState(path)
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}
	if state == nil {
		t.Fatal("expected non-nil state")
	}
	if state.Targets == nil {
		t.Fatal("expected non-nil Targets map")
	}
	if len(state.Targets) != 0 {
		t.Fatalf("expected empty Targets, got: %v", state.Targets)
	}
}

func TestMCPStateSaveAndLoad(t *testing.T) {
	dir := t.TempDir()
	path := mcp.MCPStatePath(dir)

	state := &mcp.MCPState{
		Targets: map[string]mcp.MCPTargetState{
			"claude": {
				Servers:    []string{"fetch", "git"},
				ConfigPath: "/home/user/.claude/mcp.json",
				LastSync:   "2024-01-01T00:00:00Z",
			},
		},
	}

	if err := state.Save(path); err != nil {
		t.Fatalf("Save failed: %v", err)
	}

	if _, err := os.Stat(path); err != nil {
		t.Fatalf("file not created: %v", err)
	}

	loaded, err := mcp.LoadMCPState(path)
	if err != nil {
		t.Fatalf("LoadMCPState failed: %v", err)
	}

	got, ok := loaded.Targets["claude"]
	if !ok {
		t.Fatal("expected 'claude' target in loaded state")
	}
	if got.ConfigPath != "/home/user/.claude/mcp.json" {
		t.Errorf("ConfigPath: got %q, want %q", got.ConfigPath, "/home/user/.claude/mcp.json")
	}
	if got.LastSync != "2024-01-01T00:00:00Z" {
		t.Errorf("LastSync: got %q, want %q", got.LastSync, "2024-01-01T00:00:00Z")
	}
	if len(got.Servers) != 2 || got.Servers[0] != "fetch" || got.Servers[1] != "git" {
		t.Errorf("Servers: got %v, want [fetch git]", got.Servers)
	}
}

func TestMCPState_PreviousServers(t *testing.T) {
	state := &mcp.MCPState{
		Targets: map[string]mcp.MCPTargetState{
			"claude": {
				Servers: []string{"fetch", "git"},
			},
		},
	}

	servers := state.PreviousServers("claude")
	if len(servers) != 2 || servers[0] != "fetch" || servers[1] != "git" {
		t.Errorf("PreviousServers(claude): got %v, want [fetch git]", servers)
	}

	unknown := state.PreviousServers("unknown-target")
	if unknown != nil && len(unknown) != 0 {
		t.Errorf("PreviousServers(unknown): expected nil or empty, got %v", unknown)
	}
}

func TestMCPState_UpdateTarget(t *testing.T) {
	state := &mcp.MCPState{
		Targets: map[string]mcp.MCPTargetState{},
	}

	before := time.Now().UTC().Truncate(time.Second)
	state.UpdateTarget("claude", []string{"git", "fetch"}, "/path/to/config.json")
	after := time.Now().UTC().Add(time.Second)

	got, ok := state.Targets["claude"]
	if !ok {
		t.Fatal("expected 'claude' target after UpdateTarget")
	}

	// servers should be sorted
	if len(got.Servers) != 2 || got.Servers[0] != "fetch" || got.Servers[1] != "git" {
		t.Errorf("Servers should be sorted: got %v, want [fetch git]", got.Servers)
	}

	if got.ConfigPath != "/path/to/config.json" {
		t.Errorf("ConfigPath: got %q, want %q", got.ConfigPath, "/path/to/config.json")
	}

	// LastSync should be a valid RFC3339 time within range
	ts, err := time.Parse(time.RFC3339, got.LastSync)
	if err != nil {
		t.Fatalf("LastSync not valid RFC3339: %q, err: %v", got.LastSync, err)
	}
	if ts.Before(before) || ts.After(after) {
		t.Errorf("LastSync %v not in expected range [%v, %v]", ts, before, after)
	}
}
