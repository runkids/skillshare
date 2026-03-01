package cache

import (
	"encoding/gob"
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

func TestListDiskCaches_Empty(t *testing.T) {
	dir := t.TempDir()
	items := ListDiskCaches(dir)
	if len(items) != 0 {
		t.Fatalf("expected 0 items, got %d", len(items))
	}
}

func TestListDiskCaches_ValidFile(t *testing.T) {
	dir := t.TempDir()
	gobPath := filepath.Join(dir, "discovery-abcdef.gob")

	// Create a valid source dir
	sourceDir := t.TempDir()

	dc := &DiskCache{
		Version: diskCacheVersion,
		RootDir: sourceDir,
		Entries: []DiskCacheEntry{
			{RelPath: "skill-a", FlatName: "skill-a"},
			{RelPath: "skill-b", FlatName: "skill-b"},
		},
	}
	if err := saveDiskCache(gobPath, dc); err != nil {
		t.Fatalf("saveDiskCache: %v", err)
	}

	items := ListDiskCaches(dir)
	if len(items) != 1 {
		t.Fatalf("expected 1 item, got %d", len(items))
	}

	item := items[0]
	if item.Path != gobPath {
		t.Errorf("path: got %q, want %q", item.Path, gobPath)
	}
	if item.RootDir != sourceDir {
		t.Errorf("rootDir: got %q, want %q", item.RootDir, sourceDir)
	}
	if item.EntryCount != 2 {
		t.Errorf("entryCount: got %d, want 2", item.EntryCount)
	}
	if item.Orphan {
		t.Error("expected valid (not orphan)")
	}
	if item.Error != "" {
		t.Errorf("unexpected error: %s", item.Error)
	}
	if item.Size <= 0 {
		t.Errorf("size should be positive, got %d", item.Size)
	}
}

func TestListDiskCaches_OrphanDetection(t *testing.T) {
	dir := t.TempDir()
	gobPath := filepath.Join(dir, "discovery-orphan1.gob")

	dc := &DiskCache{
		Version: diskCacheVersion,
		RootDir: "/nonexistent/path/that/does/not/exist",
		Entries: []DiskCacheEntry{
			{RelPath: "skill-x", FlatName: "skill-x"},
		},
	}
	if err := saveDiskCache(gobPath, dc); err != nil {
		t.Fatalf("saveDiskCache: %v", err)
	}

	items := ListDiskCaches(dir)
	if len(items) != 1 {
		t.Fatalf("expected 1 item, got %d", len(items))
	}

	if !items[0].Orphan {
		t.Error("expected orphan=true for nonexistent RootDir")
	}
	if items[0].RootDir != "/nonexistent/path/that/does/not/exist" {
		t.Errorf("rootDir mismatch: %s", items[0].RootDir)
	}
}

func TestListDiskCaches_CorruptFile(t *testing.T) {
	dir := t.TempDir()
	gobPath := filepath.Join(dir, "discovery-corrupt.gob")

	// Write non-gob data
	if err := os.WriteFile(gobPath, []byte("not a gob file"), 0644); err != nil {
		t.Fatalf("write: %v", err)
	}

	items := ListDiskCaches(dir)
	if len(items) != 1 {
		t.Fatalf("expected 1 item, got %d", len(items))
	}

	if items[0].Error == "" {
		t.Error("expected Error to be set for corrupt file")
	}
	if items[0].EntryCount != 0 {
		t.Errorf("expected 0 entryCount for corrupt, got %d", items[0].EntryCount)
	}
}

func TestRemoveDiskCache_Success(t *testing.T) {
	dir := t.TempDir()
	gobPath := filepath.Join(dir, "discovery-remove.gob")
	if err := os.WriteFile(gobPath, []byte("data"), 0644); err != nil {
		t.Fatal(err)
	}

	if err := RemoveDiskCache(gobPath); err != nil {
		t.Fatalf("RemoveDiskCache: %v", err)
	}

	if _, err := os.Stat(gobPath); !os.IsNotExist(err) {
		t.Error("file should be deleted")
	}
}

func TestRemoveDiskCache_RejectsNonGob(t *testing.T) {
	err := RemoveDiskCache("/tmp/random-file.txt")
	if err == nil {
		t.Error("expected error for non-gob path")
	}
}

func TestClearAllDiskCaches(t *testing.T) {
	dir := t.TempDir()

	// Create 3 gob files
	for _, name := range []string{"discovery-aaa.gob", "discovery-bbb.gob", "discovery-ccc.gob"} {
		if err := os.WriteFile(filepath.Join(dir, name), []byte("data"), 0644); err != nil {
			t.Fatal(err)
		}
	}
	// Create a non-gob file (should not be removed)
	if err := os.WriteFile(filepath.Join(dir, "other.txt"), []byte("keep"), 0644); err != nil {
		t.Fatal(err)
	}

	removed, err := ClearAllDiskCaches(dir)
	if err != nil {
		t.Fatalf("ClearAllDiskCaches: %v", err)
	}
	if removed != 3 {
		t.Errorf("expected 3 removed, got %d", removed)
	}

	// Verify gob files are gone
	matches, _ := filepath.Glob(filepath.Join(dir, "discovery-*.gob"))
	if len(matches) != 0 {
		t.Errorf("expected 0 gob files remaining, got %d", len(matches))
	}

	// Verify non-gob file still exists
	if _, err := os.Stat(filepath.Join(dir, "other.txt")); os.IsNotExist(err) {
		t.Error("other.txt should not be deleted")
	}
}

func TestClearAllDiskCaches_EmptyDir(t *testing.T) {
	dir := t.TempDir()
	removed, err := ClearAllDiskCaches(dir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if removed != 0 {
		t.Errorf("expected 0 removed, got %d", removed)
	}
}

// --- New edge-case tests ---

func TestListDiskCaches_VersionMismatch(t *testing.T) {
	dir := t.TempDir()
	gobPath := filepath.Join(dir, "discovery-ver999.gob")

	// Encode a DiskCache with wrong version directly via gob (bypass saveDiskCache)
	f, err := os.Create(gobPath)
	if err != nil {
		t.Fatal(err)
	}
	dc := DiskCache{
		Version: 999,
		RootDir: "/some/path",
		Entries: []DiskCacheEntry{{RelPath: "a", FlatName: "a"}},
	}
	if err := gob.NewEncoder(f).Encode(&dc); err != nil {
		f.Close()
		t.Fatal(err)
	}
	f.Close()

	items := ListDiskCaches(dir)
	if len(items) != 1 {
		t.Fatalf("expected 1 item, got %d", len(items))
	}
	if items[0].Error == "" {
		t.Error("expected Error for version mismatch gob")
	}
	if items[0].EntryCount != 0 {
		t.Errorf("expected 0 entryCount, got %d", items[0].EntryCount)
	}
}

func TestListDiskCaches_ZeroByteFile(t *testing.T) {
	dir := t.TempDir()
	gobPath := filepath.Join(dir, "discovery-empty.gob")

	if err := os.WriteFile(gobPath, nil, 0644); err != nil {
		t.Fatal(err)
	}

	items := ListDiskCaches(dir)
	if len(items) != 1 {
		t.Fatalf("expected 1 item, got %d", len(items))
	}
	if items[0].Error == "" {
		t.Error("expected Error for 0-byte gob")
	}
	if items[0].Size != 0 {
		t.Errorf("expected size=0, got %d", items[0].Size)
	}
}

func TestListDiskCaches_MultipleFiles(t *testing.T) {
	dir := t.TempDir()
	sourceDir := t.TempDir()

	// 1. Valid file
	validPath := filepath.Join(dir, "discovery-aaa.gob")
	if err := saveDiskCache(validPath, &DiskCache{
		Version: diskCacheVersion,
		RootDir: sourceDir,
		Entries: []DiskCacheEntry{
			{RelPath: "s1", FlatName: "s1"},
			{RelPath: "s2", FlatName: "s2"},
		},
	}); err != nil {
		t.Fatal(err)
	}

	// 2. Orphan file (source doesn't exist)
	orphanPath := filepath.Join(dir, "discovery-bbb.gob")
	if err := saveDiskCache(orphanPath, &DiskCache{
		Version: diskCacheVersion,
		RootDir: "/no/such/dir",
		Entries: []DiskCacheEntry{{RelPath: "x", FlatName: "x"}},
	}); err != nil {
		t.Fatal(err)
	}

	// 3. Corrupt file
	corruptPath := filepath.Join(dir, "discovery-ccc.gob")
	if err := os.WriteFile(corruptPath, []byte("garbage"), 0644); err != nil {
		t.Fatal(err)
	}

	items := ListDiskCaches(dir)
	if len(items) != 3 {
		t.Fatalf("expected 3 items, got %d", len(items))
	}

	// Verify sorted by path (aaa < bbb < ccc)
	if filepath.Base(items[0].Path) != "discovery-aaa.gob" {
		t.Errorf("items[0] should be aaa, got %s", filepath.Base(items[0].Path))
	}
	if filepath.Base(items[1].Path) != "discovery-bbb.gob" {
		t.Errorf("items[1] should be bbb, got %s", filepath.Base(items[1].Path))
	}
	if filepath.Base(items[2].Path) != "discovery-ccc.gob" {
		t.Errorf("items[2] should be ccc, got %s", filepath.Base(items[2].Path))
	}

	// Verify states
	if items[0].Orphan || items[0].Error != "" {
		t.Error("items[0] (valid) should be valid, not orphan/corrupt")
	}
	if items[0].EntryCount != 2 {
		t.Errorf("items[0] entryCount: got %d, want 2", items[0].EntryCount)
	}

	if !items[1].Orphan {
		t.Error("items[1] (orphan) should be orphan")
	}
	if items[1].Error != "" {
		t.Error("items[1] (orphan) should not have decode error")
	}

	if items[2].Error == "" {
		t.Error("items[2] (corrupt) should have decode error")
	}
}

func TestListDiskCaches_NonexistentDir(t *testing.T) {
	items := ListDiskCaches("/nonexistent/cache/dir/that/does/not/exist")
	if items != nil {
		t.Errorf("expected nil for nonexistent dir, got %d items", len(items))
	}
}

func TestListDiskCaches_OrphanStatError(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("permission-based orphan test not reliable on Windows")
	}

	dir := t.TempDir()
	gobPath := filepath.Join(dir, "discovery-perms.gob")

	// Create a directory that will cause stat to fail with permission denied
	restrictedDir := filepath.Join(t.TempDir(), "restricted")
	os.MkdirAll(restrictedDir, 0755)
	// Create the gob pointing to a sub-path under restricted dir
	targetDir := filepath.Join(restrictedDir, "subdir")
	os.MkdirAll(targetDir, 0755)

	if err := saveDiskCache(gobPath, &DiskCache{
		Version: diskCacheVersion,
		RootDir: targetDir,
		Entries: []DiskCacheEntry{{RelPath: "s", FlatName: "s"}},
	}); err != nil {
		t.Fatal(err)
	}

	// Remove permissions on the parent so stat on targetDir fails
	os.Chmod(restrictedDir, 0000)
	defer os.Chmod(restrictedDir, 0755) // restore for cleanup

	items := ListDiskCaches(dir)
	if len(items) != 1 {
		t.Fatalf("expected 1 item, got %d", len(items))
	}
	if !items[0].Orphan {
		t.Error("expected orphan=true when stat returns permission error")
	}
	if items[0].Error != "" {
		t.Errorf("should not have decode error, got: %s", items[0].Error)
	}
}

func TestListDiskCaches_TruncatedGob(t *testing.T) {
	dir := t.TempDir()
	gobPath := filepath.Join(dir, "discovery-trunc.gob")

	// Create a valid gob, then truncate it
	validPath := filepath.Join(dir, "discovery-valid.gob")
	if err := saveDiskCache(validPath, &DiskCache{
		Version: diskCacheVersion,
		RootDir: "/some/path",
		Entries: []DiskCacheEntry{
			{RelPath: "s1", FlatName: "s1"},
			{RelPath: "s2", FlatName: "s2"},
		},
	}); err != nil {
		t.Fatal(err)
	}

	data, err := os.ReadFile(validPath)
	if err != nil {
		t.Fatal(err)
	}
	// Write only the first half — simulates disk-full interrupted write
	if err := os.WriteFile(gobPath, data[:len(data)/2], 0644); err != nil {
		t.Fatal(err)
	}
	os.Remove(validPath) // remove so only truncated one is found

	items := ListDiskCaches(dir)
	if len(items) != 1 {
		t.Fatalf("expected 1 item, got %d", len(items))
	}
	if items[0].Error == "" {
		t.Error("expected Error for truncated gob")
	}
	if items[0].Size <= 0 {
		t.Error("truncated file should still have positive Size")
	}
}

func TestRemoveDiskCache_AlreadyDeleted(t *testing.T) {
	dir := t.TempDir()
	// Path that looks valid but file doesn't exist
	gobPath := filepath.Join(dir, "discovery-gone.gob")
	if err := RemoveDiskCache(gobPath); err != nil {
		t.Errorf("expected no error for already-deleted file, got: %v", err)
	}
}

func TestRemoveDiskCache_EmptyPath(t *testing.T) {
	err := RemoveDiskCache("")
	if err == nil {
		t.Error("expected error for empty path")
	}
}

func TestRemoveDiskCache_ReadOnlyDir(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("read-only dir test not reliable on Windows")
	}

	dir := t.TempDir()
	gobPath := filepath.Join(dir, "discovery-readonly.gob")
	if err := os.WriteFile(gobPath, []byte("data"), 0644); err != nil {
		t.Fatal(err)
	}

	// Make directory read-only so remove fails with permission error
	os.Chmod(dir, 0555)
	defer os.Chmod(dir, 0755) // restore for cleanup

	err := RemoveDiskCache(gobPath)
	if err == nil {
		t.Error("expected error when directory is read-only")
	}
}

func TestClearAllDiskCaches_SkipsAlreadyDeleted(t *testing.T) {
	dir := t.TempDir()

	// Create 2 gob files, then delete one before calling ClearAll
	for _, name := range []string{"discovery-aaa.gob", "discovery-bbb.gob"} {
		if err := os.WriteFile(filepath.Join(dir, name), []byte("data"), 0644); err != nil {
			t.Fatal(err)
		}
	}

	// Simulate race: remove one file before ClearAll gets to it
	os.Remove(filepath.Join(dir, "discovery-aaa.gob"))

	removed, err := ClearAllDiskCaches(dir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// Only bbb should be counted as "removed" (aaa was already gone)
	if removed != 1 {
		t.Errorf("expected 1 removed, got %d", removed)
	}
}

func TestClearAllDiskCaches_NonexistentDir(t *testing.T) {
	removed, err := ClearAllDiskCaches("/nonexistent/dir/that/does/not/exist")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if removed != 0 {
		t.Errorf("expected 0 removed, got %d", removed)
	}
}

func TestClearAllDiskCaches_PreservesNonGobFiles(t *testing.T) {
	dir := t.TempDir()

	// Create gob files and non-gob files
	if err := os.WriteFile(filepath.Join(dir, "discovery-aaa.gob"), []byte("data"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, ".discovery-temp.tmp"), []byte("tmp"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "config.yaml"), []byte("cfg"), 0644); err != nil {
		t.Fatal(err)
	}

	removed, err := ClearAllDiskCaches(dir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if removed != 1 {
		t.Errorf("expected 1 removed, got %d", removed)
	}

	// Non-gob files should still exist
	if _, err := os.Stat(filepath.Join(dir, ".discovery-temp.tmp")); os.IsNotExist(err) {
		t.Error("tmp file should not be deleted")
	}
	if _, err := os.Stat(filepath.Join(dir, "config.yaml")); os.IsNotExist(err) {
		t.Error("config.yaml should not be deleted")
	}
}
