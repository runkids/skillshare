package mcp_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"skillshare/internal/mcp"
)

func TestGenerateTargetJSON_Stdio(t *testing.T) {
	servers := map[string]mcp.MCPServer{
		"fetch": {
			Command: "uvx",
			Args:    []string{"mcp-server-fetch"},
			Env:     map[string]string{"API_KEY": "secret"},
		},
	}
	target := mcp.MCPTargetSpec{
		Name: "cursor",
		Key:  "mcpServers",
	}

	data, err := mcp.GenerateTargetJSON(servers, target)
	if err != nil {
		t.Fatal(err)
	}

	var doc map[string]any
	if err := json.Unmarshal(data, &doc); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}

	mcpServers, ok := doc["mcpServers"].(map[string]any)
	if !ok {
		t.Fatalf("expected top-level key 'mcpServers', got: %v", doc)
	}

	fetch, ok := mcpServers["fetch"].(map[string]any)
	if !ok {
		t.Fatalf("expected 'fetch' entry, got: %v", mcpServers)
	}

	if fetch["command"] != "uvx" {
		t.Errorf("fetch.command = %v, want 'uvx'", fetch["command"])
	}

	args, ok := fetch["args"].([]any)
	if !ok || len(args) != 1 || args[0] != "mcp-server-fetch" {
		t.Errorf("fetch.args = %v, want [mcp-server-fetch]", fetch["args"])
	}

	env, ok := fetch["env"].(map[string]any)
	if !ok {
		t.Fatalf("expected 'env' in fetch, got: %v", fetch)
	}
	if env["API_KEY"] != "secret" {
		t.Errorf("fetch.env.API_KEY = %v, want 'secret'", env["API_KEY"])
	}

	// Verify JSON ends with a trailing newline
	if len(data) == 0 || data[len(data)-1] != '\n' {
		t.Error("expected JSON output to end with a trailing newline")
	}
}

func TestGenerateTargetJSON_URLKeyOverride(t *testing.T) {
	// windsurf uses "serverUrl" instead of "url"
	servers := map[string]mcp.MCPServer{
		"remote-fs": {
			URL:     "https://mcp.example.com/fs",
			Headers: map[string]string{"Authorization": "Bearer token123"},
		},
	}
	target := mcp.MCPTargetSpec{
		Name:   "windsurf",
		Key:    "mcpServers",
		URLKey: "serverUrl",
	}

	data, err := mcp.GenerateTargetJSON(servers, target)
	if err != nil {
		t.Fatal(err)
	}

	var doc map[string]any
	if err := json.Unmarshal(data, &doc); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}

	mcpServers, ok := doc["mcpServers"].(map[string]any)
	if !ok {
		t.Fatalf("expected top-level key 'mcpServers', got: %v", doc)
	}

	srv, ok := mcpServers["remote-fs"].(map[string]any)
	if !ok {
		t.Fatalf("expected 'remote-fs' entry")
	}

	// Should use "serverUrl" NOT "url"
	if _, hasURL := srv["url"]; hasURL {
		t.Error("windsurf should NOT have 'url' key, should have 'serverUrl'")
	}
	if srv["serverUrl"] != "https://mcp.example.com/fs" {
		t.Errorf("remote-fs.serverUrl = %v, want 'https://mcp.example.com/fs'", srv["serverUrl"])
	}

	headers, ok := srv["headers"].(map[string]any)
	if !ok {
		t.Fatalf("expected 'headers' in remote-fs, got: %v", srv)
	}
	if headers["Authorization"] != "Bearer token123" {
		t.Errorf("headers.Authorization = %v, want 'Bearer token123'", headers["Authorization"])
	}
}

func TestGenerateTargetJSON_VSCodeServersKey(t *testing.T) {
	// copilot uses "servers" as top-level key, NOT "mcpServers"
	servers := map[string]mcp.MCPServer{
		"my-tool": {
			Command: "npx",
			Args:    []string{"-y", "some-mcp-server"},
		},
	}
	target := mcp.MCPTargetSpec{
		Name: "copilot",
		Key:  "servers",
	}

	data, err := mcp.GenerateTargetJSON(servers, target)
	if err != nil {
		t.Fatal(err)
	}

	var doc map[string]any
	if err := json.Unmarshal(data, &doc); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}

	// Should NOT have "mcpServers"
	if _, hasMCP := doc["mcpServers"]; hasMCP {
		t.Error("copilot should NOT have 'mcpServers' key, should have 'servers'")
	}

	serversMap, ok := doc["servers"].(map[string]any)
	if !ok {
		t.Fatalf("expected top-level key 'servers', got keys: %v", keys(doc))
	}

	if _, ok := serversMap["my-tool"]; !ok {
		t.Error("expected 'my-tool' in servers map")
	}
}

func TestGenerateAllTargetFiles(t *testing.T) {
	outDir := t.TempDir()

	cfg := &mcp.MCPConfig{
		Servers: map[string]mcp.MCPServer{
			"fetch": {
				Command: "uvx",
				Args:    []string{"mcp-server-fetch"},
			},
			"remote": {
				URL: "https://mcp.example.com",
			},
		},
	}

	targets := []mcp.MCPTargetSpec{
		{Name: "cursor", Key: "mcpServers"},
		{Name: "copilot", Key: "servers"},
	}

	result, err := mcp.GenerateAllTargetFiles(cfg, targets, outDir)
	if err != nil {
		t.Fatalf("GenerateAllTargetFiles error: %v", err)
	}

	if len(result) != 2 {
		t.Fatalf("expected 2 files generated, got %d", len(result))
	}

	// Verify cursor file was created
	cursorPath, ok := result["cursor"]
	if !ok {
		t.Fatal("expected 'cursor' in result map")
	}
	if _, err := os.Stat(cursorPath); err != nil {
		t.Fatalf("cursor file not created at %q: %v", cursorPath, err)
	}

	// Verify cursor file has correct content
	cursorData, err := os.ReadFile(cursorPath)
	if err != nil {
		t.Fatal(err)
	}
	var cursorDoc map[string]any
	if err := json.Unmarshal(cursorData, &cursorDoc); err != nil {
		t.Fatalf("cursor file invalid JSON: %v", err)
	}
	if _, ok := cursorDoc["mcpServers"]; !ok {
		t.Error("cursor file should have 'mcpServers' key")
	}

	// Verify copilot file was created
	copilotPath, ok := result["copilot"]
	if !ok {
		t.Fatal("expected 'copilot' in result map")
	}
	if _, err := os.Stat(copilotPath); err != nil {
		t.Fatalf("copilot file not created at %q: %v", copilotPath, err)
	}

	// Verify copilot file has "servers" key
	copilotData, err := os.ReadFile(copilotPath)
	if err != nil {
		t.Fatal(err)
	}
	var copilotDoc map[string]any
	if err := json.Unmarshal(copilotData, &copilotDoc); err != nil {
		t.Fatalf("copilot file invalid JSON: %v", err)
	}
	if _, ok := copilotDoc["servers"]; !ok {
		t.Error("copilot file should have 'servers' key")
	}

	// Files should be inside outDir
	for name, path := range result {
		rel, err := filepath.Rel(outDir, path)
		if err != nil || rel == ".." || len(rel) > 0 && rel[0] == '.' {
			t.Errorf("file for target %q (%q) is not inside outDir %q", name, path, outDir)
		}
	}
}

func TestGenerateAllTargetFiles_SkipsEmptyServers(t *testing.T) {
	outDir := t.TempDir()

	// Only claude-only server
	cfg := &mcp.MCPConfig{
		Servers: map[string]mcp.MCPServer{
			"claude-only": {
				Command: "npx",
				Args:    []string{"-y", "some-server"},
				Targets: []string{"claude"},
			},
		},
	}

	// Two targets: claude (matches) and cursor (no matching servers)
	targets := []mcp.MCPTargetSpec{
		{Name: "claude", Key: "mcpServers"},
		{Name: "cursor", Key: "mcpServers"},
	}

	result, err := mcp.GenerateAllTargetFiles(cfg, targets, outDir)
	if err != nil {
		t.Fatalf("GenerateAllTargetFiles error: %v", err)
	}

	if _, ok := result["claude"]; !ok {
		t.Error("expected 'claude' to be generated (has matching servers)")
	}

	if _, ok := result["cursor"]; ok {
		t.Error("cursor should be skipped (no matching servers)")
	}

	// Verify cursor file was NOT created
	cursorFile := filepath.Join(outDir, "cursor.json")
	if _, err := os.Stat(cursorFile); !os.IsNotExist(err) {
		t.Error("cursor.json should not have been created")
	}
}

// keys is a test helper to list map keys for error messages.
func keys(m map[string]any) []string {
	result := make([]string, 0, len(m))
	for k := range m {
		result = append(result, k)
	}
	return result
}
