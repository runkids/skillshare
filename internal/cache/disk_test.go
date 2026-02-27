package cache

import (
	"os"
	"path/filepath"
	"testing"
)

func TestDiskCache_RoundTrip(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "discovery.gob")

	original := &DiskCache{
		Version: diskCacheVersion,
		RootDir: "/some/source",
		Entries: []DiskCacheEntry{
			{RelPath: "my-skill", FlatName: "my-skill", IsInRepo: false, Targets: nil, Mtime: 100, Size: 42},
			{RelPath: "_team/coding", FlatName: "_team__coding", IsInRepo: true, Targets: []string{"claude", "cursor"}, Mtime: 200, Size: 99},
		},
	}

	if err := saveDiskCache(path, original); err != nil {
		t.Fatalf("save: %v", err)
	}

	loaded, err := loadDiskCache(path)
	if err != nil {
		t.Fatalf("load: %v", err)
	}

	if loaded.Version != original.Version {
		t.Errorf("version: got %d, want %d", loaded.Version, original.Version)
	}
	if loaded.RootDir != original.RootDir {
		t.Errorf("rootDir: got %q, want %q", loaded.RootDir, original.RootDir)
	}
	if len(loaded.Entries) != 2 {
		t.Fatalf("entries: got %d, want 2", len(loaded.Entries))
	}
	if loaded.Entries[1].FlatName != "_team__coding" {
		t.Errorf("entry[1].FlatName: got %q", loaded.Entries[1].FlatName)
	}
	if len(loaded.Entries[1].Targets) != 2 {
		t.Errorf("entry[1].Targets: got %v", loaded.Entries[1].Targets)
	}
}

func TestDiskCachePath_PerSourceHash(t *testing.T) {
	dir := t.TempDir()
	pathA := diskCachePath(dir, "/source/a")
	pathB := diskCachePath(dir, "/source/b")

	if pathA == pathB {
		t.Error("expected different cache paths for different source dirs")
	}

	pathA2 := diskCachePath(dir, "/source/a")
	if pathA != pathA2 {
		t.Error("expected same cache path for same source dir")
	}
}

func TestDiskCache_VersionMismatch(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "discovery.gob")

	old := &DiskCache{Version: 0, RootDir: "/old"}
	if err := saveDiskCache(path, old); err != nil {
		t.Fatalf("save: %v", err)
	}

	_, err := loadDiskCache(path)
	if err == nil {
		t.Fatal("expected error for version mismatch")
	}
}

func TestDiskCache_MissingFile(t *testing.T) {
	_, err := loadDiskCache("/nonexistent/discovery.gob")
	if err == nil {
		t.Fatal("expected error for missing file")
	}
}

func TestDeleteDiskCache(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "discovery.gob")

	if err := os.WriteFile(path, []byte("dummy"), 0644); err != nil {
		t.Fatal(err)
	}

	deleteDiskCache(path)

	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Error("expected file to be deleted")
	}
}
