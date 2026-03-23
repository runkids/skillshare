package mcp_test

import (
	"os"
	"path/filepath"
	"testing"

	"skillshare/internal/mcp"
)

// TestSyncTarget_CreatesSymlink: target doesn't exist → creates symlink, status="linked"
func TestSyncTarget_CreatesSymlink(t *testing.T) {
	dir := t.TempDir()
	generatedDir := filepath.Join(dir, "generated")
	if err := os.MkdirAll(generatedDir, 0755); err != nil {
		t.Fatal(err)
	}

	generatedPath := filepath.Join(generatedDir, "claude.json")
	if err := os.WriteFile(generatedPath, []byte(`{}`), 0644); err != nil {
		t.Fatal(err)
	}

	targetPath := filepath.Join(dir, "target", "mcp.json")

	result := mcp.SyncTarget("claude", generatedPath, targetPath, false)

	if result.Status != "linked" {
		t.Errorf("Status = %q, want %q", result.Status, "linked")
	}
	if result.Target != "claude" {
		t.Errorf("Target = %q, want %q", result.Target, "claude")
	}
	if result.Path != targetPath {
		t.Errorf("Path = %q, want %q", result.Path, targetPath)
	}
	if result.Error != "" {
		t.Errorf("unexpected error: %q", result.Error)
	}

	// Verify symlink was actually created
	info, err := os.Lstat(targetPath)
	if err != nil {
		t.Fatalf("symlink not created: %v", err)
	}
	if info.Mode()&os.ModeSymlink == 0 {
		t.Error("expected symlink, got regular file")
	}

	link, err := os.Readlink(targetPath)
	if err != nil {
		t.Fatal(err)
	}
	if link != generatedPath {
		t.Errorf("symlink target = %q, want %q", link, generatedPath)
	}
}

// TestSyncTarget_SkipsExistingNonSymlink: regular file exists → status="skipped"
func TestSyncTarget_SkipsExistingNonSymlink(t *testing.T) {
	dir := t.TempDir()
	generatedDir := filepath.Join(dir, "generated")
	if err := os.MkdirAll(generatedDir, 0755); err != nil {
		t.Fatal(err)
	}

	generatedPath := filepath.Join(generatedDir, "claude.json")
	if err := os.WriteFile(generatedPath, []byte(`{}`), 0644); err != nil {
		t.Fatal(err)
	}

	// Place a regular file at target path
	targetDir := filepath.Join(dir, "target")
	if err := os.MkdirAll(targetDir, 0755); err != nil {
		t.Fatal(err)
	}
	targetPath := filepath.Join(targetDir, "mcp.json")
	if err := os.WriteFile(targetPath, []byte(`{"existing": true}`), 0644); err != nil {
		t.Fatal(err)
	}

	result := mcp.SyncTarget("claude", generatedPath, targetPath, false)

	if result.Status != "skipped" {
		t.Errorf("Status = %q, want %q", result.Status, "skipped")
	}

	// Verify the original file is unchanged
	data, err := os.ReadFile(targetPath)
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != `{"existing": true}` {
		t.Errorf("file was modified, got: %q", string(data))
	}
}

// TestSyncTarget_UpdatesExistingSymlink: our symlink pointing to old file → updates, status="updated"
func TestSyncTarget_UpdatesExistingSymlink(t *testing.T) {
	dir := t.TempDir()
	generatedDir := filepath.Join(dir, "generated")
	if err := os.MkdirAll(generatedDir, 0755); err != nil {
		t.Fatal(err)
	}

	// Old generated file (old symlink target)
	oldGeneratedPath := filepath.Join(generatedDir, "claude-old.json")
	if err := os.WriteFile(oldGeneratedPath, []byte(`{"old": true}`), 0644); err != nil {
		t.Fatal(err)
	}

	// New generated file
	newGeneratedPath := filepath.Join(generatedDir, "claude.json")
	if err := os.WriteFile(newGeneratedPath, []byte(`{"new": true}`), 0644); err != nil {
		t.Fatal(err)
	}

	// Create a symlink pointing to the OLD generated file
	targetDir := filepath.Join(dir, "target")
	if err := os.MkdirAll(targetDir, 0755); err != nil {
		t.Fatal(err)
	}
	targetPath := filepath.Join(targetDir, "mcp.json")
	if err := os.Symlink(oldGeneratedPath, targetPath); err != nil {
		t.Fatal(err)
	}

	result := mcp.SyncTarget("claude", newGeneratedPath, targetPath, false)

	if result.Status != "updated" {
		t.Errorf("Status = %q, want %q", result.Status, "updated")
	}

	// Verify symlink now points to new generated path
	link, err := os.Readlink(targetPath)
	if err != nil {
		t.Fatal(err)
	}
	if link != newGeneratedPath {
		t.Errorf("symlink target = %q, want %q", link, newGeneratedPath)
	}
}

// TestSyncTarget_AlreadyLinked: symlink already correct → status="ok"
func TestSyncTarget_AlreadyLinked(t *testing.T) {
	dir := t.TempDir()
	generatedDir := filepath.Join(dir, "generated")
	if err := os.MkdirAll(generatedDir, 0755); err != nil {
		t.Fatal(err)
	}

	generatedPath := filepath.Join(generatedDir, "claude.json")
	if err := os.WriteFile(generatedPath, []byte(`{}`), 0644); err != nil {
		t.Fatal(err)
	}

	// Create symlink already pointing to correct file
	targetDir := filepath.Join(dir, "target")
	if err := os.MkdirAll(targetDir, 0755); err != nil {
		t.Fatal(err)
	}
	targetPath := filepath.Join(targetDir, "mcp.json")
	if err := os.Symlink(generatedPath, targetPath); err != nil {
		t.Fatal(err)
	}

	result := mcp.SyncTarget("claude", generatedPath, targetPath, false)

	if result.Status != "ok" {
		t.Errorf("Status = %q, want %q", result.Status, "ok")
	}
}

// TestSyncTarget_DryRun: doesn't create file, but returns correct status
func TestSyncTarget_DryRun(t *testing.T) {
	dir := t.TempDir()
	generatedDir := filepath.Join(dir, "generated")
	if err := os.MkdirAll(generatedDir, 0755); err != nil {
		t.Fatal(err)
	}

	generatedPath := filepath.Join(generatedDir, "claude.json")
	if err := os.WriteFile(generatedPath, []byte(`{}`), 0644); err != nil {
		t.Fatal(err)
	}

	targetPath := filepath.Join(dir, "target", "mcp.json")

	result := mcp.SyncTarget("claude", generatedPath, targetPath, true)

	if result.Status != "linked" {
		t.Errorf("Status = %q, want %q (dry run should return what would happen)", result.Status, "linked")
	}

	// Verify nothing was created
	if _, err := os.Lstat(targetPath); !os.IsNotExist(err) {
		t.Error("dry run should not create the symlink")
	}
}

// TestUnsyncTarget_RemovesSymlink: removes our symlink, returns true
func TestUnsyncTarget_RemovesSymlink(t *testing.T) {
	dir := t.TempDir()
	generatedDir := filepath.Join(dir, "generated")
	if err := os.MkdirAll(generatedDir, 0755); err != nil {
		t.Fatal(err)
	}

	generatedPath := filepath.Join(generatedDir, "claude.json")
	if err := os.WriteFile(generatedPath, []byte(`{}`), 0644); err != nil {
		t.Fatal(err)
	}

	targetPath := filepath.Join(dir, "mcp.json")
	if err := os.Symlink(generatedPath, targetPath); err != nil {
		t.Fatal(err)
	}

	removed := mcp.UnsyncTarget(targetPath, generatedDir)

	if !removed {
		t.Error("UnsyncTarget() = false, want true")
	}

	if _, err := os.Lstat(targetPath); !os.IsNotExist(err) {
		t.Error("symlink should have been removed")
	}
}

// TestUnsyncTarget_SkipsNonSymlink: regular file → returns false
func TestUnsyncTarget_SkipsNonSymlink(t *testing.T) {
	dir := t.TempDir()
	targetPath := filepath.Join(dir, "mcp.json")
	if err := os.WriteFile(targetPath, []byte(`{"existing": true}`), 0644); err != nil {
		t.Fatal(err)
	}

	generatedDir := filepath.Join(dir, "generated")

	removed := mcp.UnsyncTarget(targetPath, generatedDir)

	if removed {
		t.Error("UnsyncTarget() = true, want false for regular file")
	}

	// Verify file still exists
	if _, err := os.Stat(targetPath); err != nil {
		t.Error("regular file should not have been removed")
	}
}

// TestUnsyncTarget_SkipsExternalSymlink: symlink pointing outside generatedDir → returns false
func TestUnsyncTarget_SkipsExternalSymlink(t *testing.T) {
	dir := t.TempDir()
	generatedDir := filepath.Join(dir, "generated")

	// External file (not in generatedDir)
	externalPath := filepath.Join(dir, "external.json")
	if err := os.WriteFile(externalPath, []byte(`{}`), 0644); err != nil {
		t.Fatal(err)
	}

	targetPath := filepath.Join(dir, "mcp.json")
	if err := os.Symlink(externalPath, targetPath); err != nil {
		t.Fatal(err)
	}

	removed := mcp.UnsyncTarget(targetPath, generatedDir)

	if removed {
		t.Error("UnsyncTarget() = true, want false for symlink outside generatedDir")
	}

	// Verify symlink still exists
	if _, err := os.Lstat(targetPath); err != nil {
		t.Error("external symlink should not have been removed")
	}
}

// TestCleanupStaleLinks: removes symlinks for targets no longer in generatedFiles
func TestCleanupStaleLinks(t *testing.T) {
	dir := t.TempDir()
	generatedDir := filepath.Join(dir, "generated")
	if err := os.MkdirAll(generatedDir, 0755); err != nil {
		t.Fatal(err)
	}

	// Create two generated files
	for _, name := range []string{"claude.json", "cursor.json"} {
		path := filepath.Join(generatedDir, name)
		if err := os.WriteFile(path, []byte(`{}`), 0644); err != nil {
			t.Fatal(err)
		}
	}

	targetDir := filepath.Join(dir, "targets")
	if err := os.MkdirAll(targetDir, 0755); err != nil {
		t.Fatal(err)
	}

	// Create symlinks for both targets
	claudeTarget := filepath.Join(targetDir, "claude-mcp.json")
	cursorTarget := filepath.Join(targetDir, "cursor-mcp.json")
	if err := os.Symlink(filepath.Join(generatedDir, "claude.json"), claudeTarget); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(filepath.Join(generatedDir, "cursor.json"), cursorTarget); err != nil {
		t.Fatal(err)
	}

	targets := []mcp.MCPTargetSpec{
		{Name: "claude"},
		{Name: "cursor"},
	}

	resolveTargetPath := func(spec mcp.MCPTargetSpec) string {
		if spec.Name == "claude" {
			return claudeTarget
		}
		return cursorTarget
	}

	// Only claude has a generated file (cursor is stale)
	generatedFiles := map[string]string{
		"claude": filepath.Join(generatedDir, "claude.json"),
	}

	removed := mcp.CleanupStaleLinks(targets, generatedDir, resolveTargetPath, generatedFiles)

	if len(removed) != 1 || removed[0] != "cursor" {
		t.Errorf("CleanupStaleLinks() = %v, want [cursor]", removed)
	}

	// cursor symlink should be gone
	if _, err := os.Lstat(cursorTarget); !os.IsNotExist(err) {
		t.Error("cursor symlink should have been removed")
	}

	// claude symlink should still exist
	if _, err := os.Lstat(claudeTarget); err != nil {
		t.Error("claude symlink should still exist")
	}
}
