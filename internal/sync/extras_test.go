package sync

import (
	"os"
	"path/filepath"
	"testing"
)

// setupExtrasTest creates source and target directories under a temp root.
// It writes the given files (relative paths) into the source directory.
func setupExtrasTest(t *testing.T, files map[string]string) (srcDir, tgtDir string) {
	t.Helper()
	tmp := t.TempDir()
	srcDir = filepath.Join(tmp, "extras-source")
	tgtDir = filepath.Join(tmp, "extras-target")
	os.MkdirAll(srcDir, 0755)
	os.MkdirAll(tgtDir, 0755)
	for rel, content := range files {
		full := filepath.Join(srcDir, rel)
		os.MkdirAll(filepath.Dir(full), 0755)
		if err := os.WriteFile(full, []byte(content), 0644); err != nil {
			t.Fatalf("write %s: %v", rel, err)
		}
	}
	return
}

// --- DiscoverExtraFiles tests ---

func TestDiscoverExtraFiles(t *testing.T) {
	src, _ := setupExtrasTest(t, map[string]string{
		"flat.txt":          "hello",
		"nested/deep.md":    "# deep",
		".git/config":       "should be skipped",
		".git/hooks/pre-ci": "skip",
	})

	files, err := DiscoverExtraFiles(src)
	if err != nil {
		t.Fatal(err)
	}

	// Should find flat.txt and nested/deep.md, skip .git/**
	if len(files) != 2 {
		t.Fatalf("expected 2 files, got %d: %v", len(files), files)
	}

	expected := map[string]bool{
		"flat.txt":                         true,
		filepath.Join("nested", "deep.md"): true,
	}
	for _, f := range files {
		if !expected[f] {
			t.Errorf("unexpected file: %q", f)
		}
	}
}

// --- SyncExtra merge mode tests ---

func TestSyncExtra_MergeMode(t *testing.T) {
	src, tgt := setupExtrasTest(t, map[string]string{
		"rules.md":   "# Rules",
		"config.yml": "key: value",
	})

	result, err := SyncExtra(src, tgt, "merge", false, false)
	if err != nil {
		t.Fatal(err)
	}
	if result.Synced != 2 {
		t.Errorf("expected 2 synced, got %d", result.Synced)
	}

	// Verify symlinks exist and point to source
	for _, rel := range []string{"rules.md", "config.yml"} {
		tgtPath := filepath.Join(tgt, rel)
		info, lErr := os.Lstat(tgtPath)
		if lErr != nil {
			t.Errorf("expected file %s to exist: %v", rel, lErr)
			continue
		}
		if info.Mode()&os.ModeSymlink == 0 {
			t.Errorf("expected %s to be a symlink", rel)
		}
		dest, _ := os.Readlink(tgtPath)
		absDest, _ := filepath.Abs(dest)
		absSrc, _ := filepath.Abs(filepath.Join(src, rel))
		if absDest != absSrc {
			t.Errorf("symlink %s points to %s, expected %s", rel, absDest, absSrc)
		}
	}
}

// --- SyncExtra copy mode tests ---

func TestSyncExtra_CopyMode(t *testing.T) {
	src, tgt := setupExtrasTest(t, map[string]string{
		"readme.txt": "hello world",
	})

	result, err := SyncExtra(src, tgt, "copy", false, false)
	if err != nil {
		t.Fatal(err)
	}
	if result.Synced != 1 {
		t.Errorf("expected 1 synced, got %d", result.Synced)
	}

	// Verify it's a real file, not a symlink
	tgtPath := filepath.Join(tgt, "readme.txt")
	info, err := os.Lstat(tgtPath)
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode()&os.ModeSymlink != 0 {
		t.Error("expected a real file, got a symlink")
	}

	content, err := os.ReadFile(tgtPath)
	if err != nil {
		t.Fatal(err)
	}
	if string(content) != "hello world" {
		t.Errorf("expected 'hello world', got %q", string(content))
	}
}

// --- Conflict tests ---

func TestSyncExtra_ConflictSkipped(t *testing.T) {
	src, tgt := setupExtrasTest(t, map[string]string{
		"conflict.md": "from source",
	})

	// Pre-create a local file at the target
	os.WriteFile(filepath.Join(tgt, "conflict.md"), []byte("local version"), 0644)

	result, err := SyncExtra(src, tgt, "merge", false, false)
	if err != nil {
		t.Fatal(err)
	}
	if result.Skipped != 1 {
		t.Errorf("expected 1 skipped, got %d", result.Skipped)
	}
	if result.Synced != 0 {
		t.Errorf("expected 0 synced, got %d", result.Synced)
	}

	// Original local content should be preserved
	content, _ := os.ReadFile(filepath.Join(tgt, "conflict.md"))
	if string(content) != "local version" {
		t.Errorf("expected local content preserved, got %q", string(content))
	}
}

func TestSyncExtra_ConflictForce(t *testing.T) {
	src, tgt := setupExtrasTest(t, map[string]string{
		"conflict.md": "from source",
	})

	// Pre-create a local file at the target
	os.WriteFile(filepath.Join(tgt, "conflict.md"), []byte("local version"), 0644)

	result, err := SyncExtra(src, tgt, "merge", false, true)
	if err != nil {
		t.Fatal(err)
	}
	if result.Synced != 1 {
		t.Errorf("expected 1 synced (force replaced), got %d", result.Synced)
	}
	if result.Skipped != 0 {
		t.Errorf("expected 0 skipped, got %d", result.Skipped)
	}

	// Should now be a symlink
	info, _ := os.Lstat(filepath.Join(tgt, "conflict.md"))
	if info.Mode()&os.ModeSymlink == 0 {
		t.Error("expected symlink after force replace")
	}
}

// --- Nested directories test ---

func TestSyncExtra_NestedDirectories(t *testing.T) {
	src, tgt := setupExtrasTest(t, map[string]string{
		filepath.Join("a", "b", "deep.md"): "deep content",
	})

	result, err := SyncExtra(src, tgt, "merge", false, false)
	if err != nil {
		t.Fatal(err)
	}
	if result.Synced != 1 {
		t.Errorf("expected 1 synced, got %d", result.Synced)
	}

	deepTarget := filepath.Join(tgt, "a", "b", "deep.md")
	info, err := os.Lstat(deepTarget)
	if err != nil {
		t.Fatalf("expected nested file to exist: %v", err)
	}
	if info.Mode()&os.ModeSymlink == 0 {
		t.Error("expected symlink for nested file")
	}
}

// --- Empty source test ---

func TestSyncExtra_EmptySource(t *testing.T) {
	src, tgt := setupExtrasTest(t, map[string]string{})

	result, err := SyncExtra(src, tgt, "merge", false, false)
	if err != nil {
		t.Fatal(err)
	}
	if result.Synced != 0 {
		t.Errorf("expected 0 synced for empty source, got %d", result.Synced)
	}
}

// --- Source does not exist test ---

func TestSyncExtra_SourceNotExist(t *testing.T) {
	tgt := t.TempDir()
	_, err := SyncExtra("/nonexistent/extras/source", tgt, "merge", false, false)
	if err == nil {
		t.Error("expected error for non-existent source")
	}
}

// --- Dry run test ---

func TestSyncExtra_DryRun(t *testing.T) {
	src, tgt := setupExtrasTest(t, map[string]string{
		"alpha.md": "a",
		"beta.md":  "b",
	})

	result, err := SyncExtra(src, tgt, "merge", true, false)
	if err != nil {
		t.Fatal(err)
	}
	if result.Synced != 2 {
		t.Errorf("expected 2 synced in dry-run count, got %d", result.Synced)
	}

	// Verify no files were actually created
	entries, _ := os.ReadDir(tgt)
	if len(entries) != 0 {
		t.Errorf("expected empty target in dry-run, got %d entries", len(entries))
	}
}

// --- Idempotent test ---

func TestSyncExtra_Idempotent(t *testing.T) {
	src, tgt := setupExtrasTest(t, map[string]string{
		"stable.md": "content",
	})

	// First sync
	r1, err := SyncExtra(src, tgt, "merge", false, false)
	if err != nil {
		t.Fatal(err)
	}
	if r1.Synced != 1 {
		t.Fatalf("first sync: expected 1 synced, got %d", r1.Synced)
	}

	// Second sync — should still report synced (already correct)
	r2, err := SyncExtra(src, tgt, "merge", false, false)
	if err != nil {
		t.Fatal(err)
	}
	if r2.Synced != 1 {
		t.Errorf("second sync: expected 1 synced (idempotent), got %d", r2.Synced)
	}
	if r2.Skipped != 0 {
		t.Errorf("second sync: expected 0 skipped, got %d", r2.Skipped)
	}
}

// --- Prune orphans test ---

func TestSyncExtra_PrunesOrphans(t *testing.T) {
	src, tgt := setupExtrasTest(t, map[string]string{
		"keep.md":   "keep this",
		"remove.md": "will be removed from source",
	})

	// First sync both files
	_, err := SyncExtra(src, tgt, "merge", false, false)
	if err != nil {
		t.Fatal(err)
	}

	// Remove "remove.md" from source
	os.Remove(filepath.Join(src, "remove.md"))

	// Sync again — should prune orphan
	result, err := SyncExtra(src, tgt, "merge", false, false)
	if err != nil {
		t.Fatal(err)
	}
	if result.Pruned != 1 {
		t.Errorf("expected 1 pruned, got %d", result.Pruned)
	}

	// Verify orphan was removed
	if _, err := os.Lstat(filepath.Join(tgt, "remove.md")); !os.IsNotExist(err) {
		t.Error("expected orphan symlink to be removed")
	}

	// Verify kept file still exists
	if _, err := os.Lstat(filepath.Join(tgt, "keep.md")); err != nil {
		t.Error("expected keep.md to still exist")
	}
}

// --- CollectExtraFiles tests ---

func TestCollectExtraFiles(t *testing.T) {
	sourceDir := filepath.Join(t.TempDir(), "source")
	targetDir := filepath.Join(t.TempDir(), "target")

	// Create target with a local file
	os.MkdirAll(targetDir, 0755)
	os.WriteFile(filepath.Join(targetDir, "rule1.md"), []byte("# Rule 1"), 0644)

	result, err := CollectExtraFiles(sourceDir, targetDir, false)
	if err != nil {
		t.Fatalf("CollectExtraFiles: %v", err)
	}

	if result.Collected != 1 {
		t.Errorf("Collected = %d, want 1", result.Collected)
	}

	// Verify file moved to source
	data, err := os.ReadFile(filepath.Join(sourceDir, "rule1.md"))
	if err != nil {
		t.Fatalf("Read source file: %v", err)
	}
	if string(data) != "# Rule 1" {
		t.Errorf("Source content = %q, want %q", string(data), "# Rule 1")
	}

	// Verify target is now a symlink
	info, err := os.Lstat(filepath.Join(targetDir, "rule1.md"))
	if err != nil {
		t.Fatalf("Lstat target: %v", err)
	}
	if info.Mode()&os.ModeSymlink == 0 {
		t.Error("Target should be a symlink after collect")
	}
}

func TestCollectExtraFiles_DryRun(t *testing.T) {
	sourceDir := filepath.Join(t.TempDir(), "source")
	targetDir := filepath.Join(t.TempDir(), "target")

	os.MkdirAll(targetDir, 0755)
	os.WriteFile(filepath.Join(targetDir, "rule1.md"), []byte("# Rule 1"), 0644)

	result, err := CollectExtraFiles(sourceDir, targetDir, true)
	if err != nil {
		t.Fatalf("CollectExtraFiles dry run: %v", err)
	}

	if result.Collected != 1 {
		t.Errorf("Collected = %d, want 1", result.Collected)
	}

	// Verify file NOT moved (dry run)
	if _, err := os.Stat(filepath.Join(sourceDir, "rule1.md")); !os.IsNotExist(err) {
		t.Error("Source file should not exist in dry run")
	}

	// Verify target is still a regular file
	info, err := os.Lstat(filepath.Join(targetDir, "rule1.md"))
	if err != nil {
		t.Fatalf("Lstat target: %v", err)
	}
	if info.Mode()&os.ModeSymlink != 0 {
		t.Error("Target should NOT be a symlink in dry run")
	}
}

func TestCollectExtraFiles_SkipsExisting(t *testing.T) {
	sourceDir := filepath.Join(t.TempDir(), "source")
	targetDir := filepath.Join(t.TempDir(), "target")

	// Create both source and target with same file
	os.MkdirAll(sourceDir, 0755)
	os.MkdirAll(targetDir, 0755)
	os.WriteFile(filepath.Join(sourceDir, "rule1.md"), []byte("source version"), 0644)
	os.WriteFile(filepath.Join(targetDir, "rule1.md"), []byte("target version"), 0644)

	result, err := CollectExtraFiles(sourceDir, targetDir, false)
	if err != nil {
		t.Fatalf("CollectExtraFiles: %v", err)
	}

	if result.Skipped != 1 {
		t.Errorf("Skipped = %d, want 1", result.Skipped)
	}
	if result.Collected != 0 {
		t.Errorf("Collected = %d, want 0", result.Collected)
	}
}

// --- Symlink mode (entire directory) test ---

func TestSyncExtra_SymlinkMode(t *testing.T) {
	src, _ := setupExtrasTest(t, map[string]string{
		"file.txt": "content",
	})

	tmp := t.TempDir()
	tgt := filepath.Join(tmp, "extras-link")

	result, err := SyncExtra(src, tgt, "symlink", false, false)
	if err != nil {
		t.Fatal(err)
	}
	if result.Synced != 1 {
		t.Errorf("expected 1 synced, got %d", result.Synced)
	}

	// Verify target is a symlink to the source directory
	info, err := os.Lstat(tgt)
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode()&os.ModeSymlink == 0 {
		t.Error("expected target to be a symlink")
	}

	dest, _ := os.Readlink(tgt)
	absDest, _ := filepath.Abs(dest)
	absSrc, _ := filepath.Abs(src)
	if absDest != absSrc {
		t.Errorf("symlink points to %s, expected %s", absDest, absSrc)
	}

	// Verify files are accessible through the symlink
	content, err := os.ReadFile(filepath.Join(tgt, "file.txt"))
	if err != nil {
		t.Fatal(err)
	}
	if string(content) != "content" {
		t.Errorf("expected 'content', got %q", string(content))
	}
}
