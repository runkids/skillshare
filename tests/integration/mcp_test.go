//go:build !online

package integration

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"skillshare/internal/testutil"
)

// TestMCP_AddListRemove verifies the add → list → remove lifecycle.
func TestMCP_AddListRemove(t *testing.T) {
	sb := testutil.NewSandbox(t)
	defer sb.Cleanup()

	// Add a server
	result := sb.RunCLI("mcp", "add", "-g", "ctx7", "--command", "npx", "--args", "-y", "--args", "@upstash/context7-mcp")
	result.AssertSuccess(t)
	result.AssertAnyOutputContains(t, "ctx7")

	// List — should appear
	result = sb.RunCLI("mcp", "list", "-g")
	result.AssertSuccess(t)
	result.AssertOutputContains(t, "ctx7")

	// Remove
	result = sb.RunCLI("mcp", "remove", "-g", "ctx7")
	result.AssertSuccess(t)
	result.AssertAnyOutputContains(t, "ctx7")

	// List again — should be gone
	result = sb.RunCLI("mcp", "list", "-g")
	result.AssertSuccess(t)
	result.AssertOutputNotContains(t, "ctx7")
}

// TestMCP_SyncCreatesSymlinks verifies that sync mcp creates a symlink at the
// target's expected path when mcp_mode is "symlink".
func TestMCP_SyncCreatesSymlinks(t *testing.T) {
	sb := testutil.NewSandbox(t)
	defer sb.Cleanup()

	// Use symlink mode explicitly (default is now merge)
	sb.WriteConfig("source: " + sb.SourcePath + "\ntargets: {}\nmcp_mode: symlink\n")

	// Add a server
	result := sb.RunCLI("mcp", "add", "-g", "ctx7", "--command", "npx", "--args", "-y", "--args", "@upstash/context7-mcp")
	result.AssertSuccess(t)

	// Run sync mcp
	result = sb.RunCLI("sync", "mcp", "-g")
	result.AssertSuccess(t)
	result.AssertAnyOutputContains(t, "linked")

	// cursor is a global target: ~/.cursor/mcp.json
	cursorMCPPath := filepath.Join(sb.Home, ".cursor", "mcp.json")

	// Must be a symlink
	info, err := os.Lstat(cursorMCPPath)
	if err != nil {
		t.Fatalf("expected %s to exist, got: %v", cursorMCPPath, err)
	}
	if info.Mode()&os.ModeSymlink == 0 {
		t.Fatalf("expected %s to be a symlink", cursorMCPPath)
	}

	// Symlink must resolve and contain mcpServers with ctx7
	data, err := os.ReadFile(cursorMCPPath)
	if err != nil {
		t.Fatalf("failed to read through symlink %s: %v", cursorMCPPath, err)
	}

	var doc map[string]json.RawMessage
	if err := json.Unmarshal(data, &doc); err != nil {
		t.Fatalf("invalid JSON in %s: %v", cursorMCPPath, err)
	}

	if _, ok := doc["mcpServers"]; !ok {
		t.Fatalf("expected mcpServers key in %s, got keys: %v", cursorMCPPath, keysOf(doc))
	}

	var servers map[string]json.RawMessage
	if err := json.Unmarshal(doc["mcpServers"], &servers); err != nil {
		t.Fatalf("failed to unmarshal mcpServers: %v", err)
	}
	if _, ok := servers["ctx7"]; !ok {
		t.Fatalf("expected ctx7 server in mcpServers, got: %v", keysOf(servers))
	}
}

// TestMCP_SyncSkipsExistingFile verifies that sync mcp in symlink mode skips a
// target when a regular (non-managed) file already exists.
func TestMCP_SyncSkipsExistingFile(t *testing.T) {
	sb := testutil.NewSandbox(t)
	defer sb.Cleanup()

	// Use symlink mode explicitly (default is now merge)
	sb.WriteConfig("source: " + sb.SourcePath + "\ntargets: {}\nmcp_mode: symlink\n")

	// Pre-create cursor mcp.json as a regular file
	cursorMCPPath := filepath.Join(sb.Home, ".cursor", "mcp.json")
	if err := os.WriteFile(cursorMCPPath, []byte(`{"mcpServers":{}}`), 0644); err != nil {
		t.Fatalf("failed to write existing file: %v", err)
	}

	// Add a server then sync
	sb.RunCLI("mcp", "add", "-g", "ctx7", "--command", "npx").AssertSuccess(t)
	result := sb.RunCLI("sync", "mcp", "-g")
	result.AssertSuccess(t)
	result.AssertAnyOutputContains(t, "skipped")

	// File must still be a regular file (not replaced with symlink)
	info, err := os.Lstat(cursorMCPPath)
	if err != nil {
		t.Fatalf("file should still exist: %v", err)
	}
	if info.Mode()&os.ModeSymlink != 0 {
		t.Fatal("expected regular file, got symlink — sync should not overwrite existing files")
	}
}

// TestMCP_SyncDryRun verifies that --dry-run does not create any files.
func TestMCP_SyncDryRun(t *testing.T) {
	sb := testutil.NewSandbox(t)
	defer sb.Cleanup()

	// Add a server
	sb.RunCLI("mcp", "add", "-g", "ctx7", "--command", "npx").AssertSuccess(t)

	// Dry-run sync
	result := sb.RunCLI("sync", "mcp", "--dry-run", "-g")
	result.AssertSuccess(t)

	// cursor path must NOT be created
	cursorMCPPath := filepath.Join(sb.Home, ".cursor", "mcp.json")
	if _, err := os.Lstat(cursorMCPPath); err == nil {
		t.Fatal("dry-run must not create target files, but cursor mcp.json was created")
	}
}

// TestMCP_Status verifies that mcp status reports linked targets after sync in symlink mode.
func TestMCP_Status(t *testing.T) {
	sb := testutil.NewSandbox(t)
	defer sb.Cleanup()

	// Use symlink mode explicitly (default is now merge)
	sb.WriteConfig("source: " + sb.SourcePath + "\ntargets: {}\nmcp_mode: symlink\n")

	// Add and sync
	sb.RunCLI("mcp", "add", "-g", "ctx7", "--command", "npx").AssertSuccess(t)
	sb.RunCLI("sync", "mcp", "-g").AssertSuccess(t)

	// Plain status
	result := sb.RunCLI("mcp", "status", "-g")
	result.AssertSuccess(t)
	result.AssertAnyOutputContains(t, "cursor")

	// JSON status
	result = sb.RunCLI("mcp", "status", "-g", "--json")
	result.AssertSuccess(t)

	var entries []struct {
		Target string `json:"target"`
		Status string `json:"status"`
	}
	if err := json.Unmarshal([]byte(result.Stdout), &entries); err != nil {
		t.Fatalf("invalid JSON from mcp status --json: %v\noutput: %s", err, result.Stdout)
	}
	if len(entries) == 0 {
		t.Fatal("expected at least one status entry")
	}

	found := false
	for _, e := range entries {
		if e.Target == "cursor" && e.Status == "linked" {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("expected cursor to be linked, got entries: %+v", entries)
	}
}

// TestMCP_AddDuplicate verifies that adding a server with an existing name fails.
func TestMCP_AddDuplicate(t *testing.T) {
	sb := testutil.NewSandbox(t)
	defer sb.Cleanup()

	sb.RunCLI("mcp", "add", "-g", "ctx7", "--command", "npx").AssertSuccess(t)

	// Add same name again → must fail
	result := sb.RunCLI("mcp", "add", "-g", "ctx7", "--command", "npx")
	result.AssertFailure(t)
	result.AssertAnyOutputContains(t, "already exists")
}

// TestMCP_AddRemoteServer verifies adding a server with --url instead of --command.
func TestMCP_AddRemoteServer(t *testing.T) {
	sb := testutil.NewSandbox(t)
	defer sb.Cleanup()

	result := sb.RunCLI("mcp", "add", "-g", "remote-svc", "--url", "https://example.com/mcp")
	result.AssertSuccess(t)
	result.AssertAnyOutputContains(t, "remote-svc")

	// List — should show the URL
	result = sb.RunCLI("mcp", "list", "-g")
	result.AssertSuccess(t)
	result.AssertOutputContains(t, "remote-svc")
	result.AssertOutputContains(t, "https://example.com/mcp")
}

// TestMCP_SyncMergeMode_PreservesUserEntries verifies that merge mode preserves
// user-managed entries that were not added by skillshare.
func TestMCP_SyncMergeMode_PreservesUserEntries(t *testing.T) {
	sb := testutil.NewSandbox(t)
	defer sb.Cleanup()

	// Pre-create cursor mcp.json with a user-managed server
	cursorMCPPath := filepath.Join(sb.Home, ".cursor", "mcp.json")
	if err := os.WriteFile(cursorMCPPath, []byte(`{"mcpServers":{"user-server":{"command":"my-cmd"}}}`), 0644); err != nil {
		t.Fatalf("failed to write existing file: %v", err)
	}

	// Add a skillshare server and sync (merge is default)
	sb.RunCLI("mcp", "add", "-g", "ctx7", "--command", "npx").AssertSuccess(t)
	sb.RunCLI("sync", "mcp", "-g").AssertSuccess(t)

	// Read the resulting file and verify both servers are present
	data, err := os.ReadFile(cursorMCPPath)
	if err != nil {
		t.Fatalf("failed to read %s: %v", cursorMCPPath, err)
	}

	var doc map[string]json.RawMessage
	if err := json.Unmarshal(data, &doc); err != nil {
		t.Fatalf("invalid JSON in %s: %v", cursorMCPPath, err)
	}

	var servers map[string]json.RawMessage
	if err := json.Unmarshal(doc["mcpServers"], &servers); err != nil {
		t.Fatalf("failed to unmarshal mcpServers: %v", err)
	}

	if _, ok := servers["user-server"]; !ok {
		t.Fatalf("merge should preserve user-server, got servers: %v", keysOf(servers))
	}
	if _, ok := servers["ctx7"]; !ok {
		t.Fatalf("merge should add ctx7, got servers: %v", keysOf(servers))
	}
}

// TestMCP_SyncMergeMode_CleansUpRemovedServers verifies that merge mode removes
// skillshare-managed servers that were subsequently removed from config.
func TestMCP_SyncMergeMode_CleansUpRemovedServers(t *testing.T) {
	sb := testutil.NewSandbox(t)
	defer sb.Cleanup()

	// Add a server and sync
	sb.RunCLI("mcp", "add", "-g", "ctx7", "--command", "npx").AssertSuccess(t)
	sb.RunCLI("sync", "mcp", "-g").AssertSuccess(t)

	// Remove the server and sync again
	sb.RunCLI("mcp", "remove", "-g", "ctx7").AssertSuccess(t)
	sb.RunCLI("sync", "mcp", "-g").AssertSuccess(t)

	// The target file should still exist
	cursorMCPPath := filepath.Join(sb.Home, ".cursor", "mcp.json")
	info, err := os.Lstat(cursorMCPPath)
	if err != nil {
		t.Fatalf("target file should still exist after merge cleanup: %v", err)
	}
	// Must be a regular file (merge mode writes files, not symlinks)
	if info.Mode()&os.ModeSymlink != 0 {
		t.Fatal("expected regular file, not symlink")
	}

	// ctx7 server must be gone
	data, err := os.ReadFile(cursorMCPPath)
	if err != nil {
		t.Fatalf("failed to read %s: %v", cursorMCPPath, err)
	}

	var doc map[string]json.RawMessage
	if err := json.Unmarshal(data, &doc); err != nil {
		t.Fatalf("invalid JSON in %s: %v", cursorMCPPath, err)
	}

	var servers map[string]json.RawMessage
	if err := json.Unmarshal(doc["mcpServers"], &servers); err != nil {
		t.Fatalf("failed to unmarshal mcpServers: %v", err)
	}

	if _, ok := servers["ctx7"]; ok {
		t.Fatalf("ctx7 should have been removed from target, got servers: %v", keysOf(servers))
	}
}

// TestMCP_SyncMergeMode_PreservesOtherKeys verifies that merge mode leaves
// non-mcpServers keys in the target file intact.
func TestMCP_SyncMergeMode_PreservesOtherKeys(t *testing.T) {
	sb := testutil.NewSandbox(t)
	defer sb.Cleanup()

	// Pre-create cursor mcp.json with an extra key
	cursorMCPPath := filepath.Join(sb.Home, ".cursor", "mcp.json")
	initialContent := `{"mcpServers":{},"permissions":{"allow":["Read"]}}`
	if err := os.WriteFile(cursorMCPPath, []byte(initialContent), 0644); err != nil {
		t.Fatalf("failed to write existing file: %v", err)
	}

	// Add a server and sync (merge is default)
	sb.RunCLI("mcp", "add", "-g", "ctx7", "--command", "npx").AssertSuccess(t)
	sb.RunCLI("sync", "mcp", "-g").AssertSuccess(t)

	// The permissions key must still be present
	data, err := os.ReadFile(cursorMCPPath)
	if err != nil {
		t.Fatalf("failed to read %s: %v", cursorMCPPath, err)
	}

	var doc map[string]json.RawMessage
	if err := json.Unmarshal(data, &doc); err != nil {
		t.Fatalf("invalid JSON in %s: %v", cursorMCPPath, err)
	}

	if _, ok := doc["permissions"]; !ok {
		t.Fatalf("merge should preserve 'permissions' key, got top-level keys: %v", keysOf(doc))
	}
	if _, ok := doc["mcpServers"]; !ok {
		t.Fatalf("mcpServers key missing after merge, got keys: %v", keysOf(doc))
	}
}

// TestMCP_SyncCopyMode verifies that copy mode writes a regular file (not a symlink)
// with the correct JSON content.
func TestMCP_SyncCopyMode(t *testing.T) {
	sb := testutil.NewSandbox(t)
	defer sb.Cleanup()

	// Set mcp_mode to copy via config
	sb.WriteConfig("source: " + sb.SourcePath + "\ntargets: {}\nmcp_mode: copy\n")

	// Add a server and sync
	sb.RunCLI("mcp", "add", "-g", "ctx7", "--command", "npx").AssertSuccess(t)
	result := sb.RunCLI("sync", "mcp", "-g")
	result.AssertSuccess(t)

	cursorMCPPath := filepath.Join(sb.Home, ".cursor", "mcp.json")

	// Must be a regular file, not a symlink
	info, err := os.Lstat(cursorMCPPath)
	if err != nil {
		t.Fatalf("expected %s to exist: %v", cursorMCPPath, err)
	}
	if info.Mode()&os.ModeSymlink != 0 {
		t.Fatalf("copy mode must write a regular file, got symlink at %s", cursorMCPPath)
	}

	// Must contain ctx7 in mcpServers
	data, err := os.ReadFile(cursorMCPPath)
	if err != nil {
		t.Fatalf("failed to read %s: %v", cursorMCPPath, err)
	}

	var doc map[string]json.RawMessage
	if err := json.Unmarshal(data, &doc); err != nil {
		t.Fatalf("invalid JSON in %s: %v", cursorMCPPath, err)
	}

	var servers map[string]json.RawMessage
	if err := json.Unmarshal(doc["mcpServers"], &servers); err != nil {
		t.Fatalf("failed to unmarshal mcpServers: %v", err)
	}

	if _, ok := servers["ctx7"]; !ok {
		t.Fatalf("expected ctx7 server in mcpServers, got: %v", keysOf(servers))
	}
}

// keysOf returns the string keys of a map for diagnostic messages.
func keysOf[V any](m map[string]V) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	return keys
}
