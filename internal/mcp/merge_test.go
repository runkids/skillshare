package mcp_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"skillshare/internal/mcp"
)

// targetSpec is a test helper for cursor-style target (mcpServers key).
func cursorTarget() mcp.MCPTargetSpec {
	return mcp.MCPTargetSpec{
		Name: "cursor",
		Key:  "mcpServers",
	}
}

// vsCodeTarget returns a VS Code / copilot-style target (servers key).
func vsCodeTarget() mcp.MCPTargetSpec {
	return mcp.MCPTargetSpec{
		Name: "copilot",
		Key:  "servers",
	}
}

// readJSON is a test helper that unmarshals a file or fails the test.
func readJSON(t *testing.T, path string) map[string]any {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("readJSON: %v", err)
	}
	var doc map[string]any
	if err := json.Unmarshal(data, &doc); err != nil {
		t.Fatalf("readJSON unmarshal: %v", err)
	}
	return doc
}

// getMCPSection returns the mcp section map under key, or fails the test.
func getMCPSection(t *testing.T, doc map[string]any, key string) map[string]any {
	t.Helper()
	raw, ok := doc[key]
	if !ok {
		t.Fatalf("expected key %q in document, got keys: %v", key, keys(doc))
	}
	section, ok := raw.(map[string]any)
	if !ok {
		t.Fatalf("key %q is not an object, got: %T", key, raw)
	}
	return section
}

// TestMergeToTarget_NewFile: target JSON doesn't exist → created with added entries.
func TestMergeToTarget_NewFile(t *testing.T) {
	dir := t.TempDir()
	targetFile := filepath.Join(dir, "mcp.json")

	servers := map[string]mcp.MCPServer{
		"fetch": {Command: "uvx", Args: []string{"mcp-server-fetch"}},
	}

	result, err := mcp.MergeToTarget(targetFile, servers, nil, cursorTarget(), false)
	if err != nil {
		t.Fatalf("MergeToTarget error: %v", err)
	}

	if result.Target != "cursor" {
		t.Errorf("Target = %q, want %q", result.Target, "cursor")
	}
	if result.Error != "" {
		t.Errorf("unexpected error in result: %q", result.Error)
	}
	if len(result.Added) != 1 || result.Added[0] != "fetch" {
		t.Errorf("Added = %v, want [fetch]", result.Added)
	}
	if len(result.Updated) != 0 {
		t.Errorf("Updated = %v, want []", result.Updated)
	}
	if len(result.Removed) != 0 {
		t.Errorf("Removed = %v, want []", result.Removed)
	}

	// Verify file was created on disk.
	doc := readJSON(t, targetFile)
	section := getMCPSection(t, doc, "mcpServers")
	if _, ok := section["fetch"]; !ok {
		t.Error("expected 'fetch' in mcpServers section")
	}
}

// TestMergeToTarget_PreservesUserEntries: existing user-managed entries are not removed.
func TestMergeToTarget_PreservesUserEntries(t *testing.T) {
	dir := t.TempDir()
	targetFile := filepath.Join(dir, "mcp.json")

	// Pre-existing file with a user-managed entry.
	existing := map[string]any{
		"mcpServers": map[string]any{
			"user-tool": map[string]any{"command": "my-tool"},
		},
	}
	data, _ := json.MarshalIndent(existing, "", "  ")
	if err := os.WriteFile(targetFile, append(data, '\n'), 0644); err != nil {
		t.Fatal(err)
	}

	servers := map[string]mcp.MCPServer{
		"fetch": {Command: "uvx", Args: []string{"mcp-server-fetch"}},
	}

	_, err := mcp.MergeToTarget(targetFile, servers, nil, cursorTarget(), false)
	if err != nil {
		t.Fatalf("MergeToTarget error: %v", err)
	}

	doc := readJSON(t, targetFile)
	section := getMCPSection(t, doc, "mcpServers")

	// User entry must be preserved.
	if _, ok := section["user-tool"]; !ok {
		t.Error("user-tool should be preserved")
	}
	// Skillshare entry must be added.
	if _, ok := section["fetch"]; !ok {
		t.Error("fetch should have been added")
	}
}

// TestMergeToTarget_PreservesOtherTopLevelKeys: non-mcp keys like "permissions" survive.
func TestMergeToTarget_PreservesOtherTopLevelKeys(t *testing.T) {
	dir := t.TempDir()
	targetFile := filepath.Join(dir, "mcp.json")

	existing := map[string]any{
		"permissions": map[string]any{"allow": []any{"Bash"}},
		"mcpServers":  map[string]any{},
	}
	data, _ := json.MarshalIndent(existing, "", "  ")
	if err := os.WriteFile(targetFile, append(data, '\n'), 0644); err != nil {
		t.Fatal(err)
	}

	servers := map[string]mcp.MCPServer{
		"fetch": {Command: "uvx"},
	}

	_, err := mcp.MergeToTarget(targetFile, servers, nil, cursorTarget(), false)
	if err != nil {
		t.Fatalf("MergeToTarget error: %v", err)
	}

	doc := readJSON(t, targetFile)
	if _, ok := doc["permissions"]; !ok {
		t.Error("'permissions' key should be preserved")
	}
}

// TestMergeToTarget_RemovesPreviouslySynced: entries in previousServers but not in current servers are removed.
func TestMergeToTarget_RemovesPreviouslySynced(t *testing.T) {
	dir := t.TempDir()
	targetFile := filepath.Join(dir, "mcp.json")

	// Pre-existing file with two skillshare-managed entries and one user entry.
	existing := map[string]any{
		"mcpServers": map[string]any{
			"fetch":     map[string]any{"command": "uvx"},
			"old-tool":  map[string]any{"command": "old"},
			"user-tool": map[string]any{"command": "my-tool"},
		},
	}
	data, _ := json.MarshalIndent(existing, "", "  ")
	if err := os.WriteFile(targetFile, append(data, '\n'), 0644); err != nil {
		t.Fatal(err)
	}

	// Current servers: only "fetch" remains (old-tool dropped).
	servers := map[string]mcp.MCPServer{
		"fetch": {Command: "uvx", Args: []string{"mcp-server-fetch"}},
	}
	// previousServers records what was previously managed.
	previousServers := []string{"fetch", "old-tool"}

	result, err := mcp.MergeToTarget(targetFile, servers, previousServers, cursorTarget(), false)
	if err != nil {
		t.Fatalf("MergeToTarget error: %v", err)
	}

	if len(result.Removed) != 1 || result.Removed[0] != "old-tool" {
		t.Errorf("Removed = %v, want [old-tool]", result.Removed)
	}

	doc := readJSON(t, targetFile)
	section := getMCPSection(t, doc, "mcpServers")

	if _, ok := section["old-tool"]; ok {
		t.Error("old-tool should have been removed")
	}
	if _, ok := section["fetch"]; !ok {
		t.Error("fetch should still be present")
	}
	// user-tool must NOT be removed (not in previousServers).
	if _, ok := section["user-tool"]; !ok {
		t.Error("user-tool should be preserved (not a managed entry)")
	}
}

// TestMergeToTarget_DryRun: reports changes but does not write.
func TestMergeToTarget_DryRun(t *testing.T) {
	dir := t.TempDir()
	targetFile := filepath.Join(dir, "mcp.json")

	servers := map[string]mcp.MCPServer{
		"fetch": {Command: "uvx"},
	}

	result, err := mcp.MergeToTarget(targetFile, servers, nil, cursorTarget(), true)
	if err != nil {
		t.Fatalf("MergeToTarget error: %v", err)
	}

	if len(result.Added) != 1 || result.Added[0] != "fetch" {
		t.Errorf("Added = %v, want [fetch]", result.Added)
	}

	// File must NOT have been created.
	if _, err := os.Stat(targetFile); !os.IsNotExist(err) {
		t.Error("dry run should not create the target file")
	}
}

// TestMergeToTarget_VSCodeServersKey: uses "servers" key instead of "mcpServers".
func TestMergeToTarget_VSCodeServersKey(t *testing.T) {
	dir := t.TempDir()
	targetFile := filepath.Join(dir, "mcp.json")

	servers := map[string]mcp.MCPServer{
		"my-tool": {Command: "npx", Args: []string{"-y", "some-mcp-server"}},
	}

	result, err := mcp.MergeToTarget(targetFile, servers, nil, vsCodeTarget(), false)
	if err != nil {
		t.Fatalf("MergeToTarget error: %v", err)
	}

	if result.Target != "copilot" {
		t.Errorf("Target = %q, want %q", result.Target, "copilot")
	}

	doc := readJSON(t, targetFile)

	// Must use "servers", NOT "mcpServers".
	if _, hasMCP := doc["mcpServers"]; hasMCP {
		t.Error("copilot should NOT have 'mcpServers' key")
	}
	section := getMCPSection(t, doc, "servers")
	if _, ok := section["my-tool"]; !ok {
		t.Error("expected 'my-tool' in servers section")
	}
}

// TestMergeToTarget_UpdatedEntry: existing entry gets new config → reported as Updated.
func TestMergeToTarget_UpdatedEntry(t *testing.T) {
	dir := t.TempDir()
	targetFile := filepath.Join(dir, "mcp.json")

	// Pre-existing file with old fetch command.
	existing := map[string]any{
		"mcpServers": map[string]any{
			"fetch": map[string]any{"command": "old-cmd"},
		},
	}
	data, _ := json.MarshalIndent(existing, "", "  ")
	if err := os.WriteFile(targetFile, append(data, '\n'), 0644); err != nil {
		t.Fatal(err)
	}

	servers := map[string]mcp.MCPServer{
		"fetch": {Command: "uvx", Args: []string{"mcp-server-fetch"}},
	}
	previousServers := []string{"fetch"}

	result, err := mcp.MergeToTarget(targetFile, servers, previousServers, cursorTarget(), false)
	if err != nil {
		t.Fatalf("MergeToTarget error: %v", err)
	}

	if len(result.Updated) != 1 || result.Updated[0] != "fetch" {
		t.Errorf("Updated = %v, want [fetch]", result.Updated)
	}
	if len(result.Added) != 0 {
		t.Errorf("Added = %v, want []", result.Added)
	}
}
