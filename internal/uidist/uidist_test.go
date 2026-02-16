package uidist

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestCacheDir_Default(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("XDG_CACHE_HOME", tmp)

	dir := CacheDir("1.2.3")
	expected := filepath.Join(tmp, "skillshare", "ui", "1.2.3")
	if dir != expected {
		t.Errorf("got %s, want %s", dir, expected)
	}
}

func TestIsCached_Missing(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("XDG_CACHE_HOME", tmp)

	_, ok := IsCached("1.0.0")
	if ok {
		t.Error("expected IsCached to return false for missing version")
	}
}

func TestIsCached_Present(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("XDG_CACHE_HOME", tmp)

	dir := filepath.Join(tmp, "skillshare", "ui", "1.0.0")
	if err := os.MkdirAll(dir, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "index.html"), []byte("<html>"), 0644); err != nil {
		t.Fatal(err)
	}

	gotDir, ok := IsCached("1.0.0")
	if !ok {
		t.Error("expected IsCached to return true")
	}
	if gotDir != dir {
		t.Errorf("got dir %s, want %s", gotDir, dir)
	}
}

func TestParseChecksum(t *testing.T) {
	input := "abc123  skillshare_1.0.0_linux_amd64.tar.gz\ndef456  skillshare-ui-dist.tar.gz\n"
	hash, err := parseChecksum(strings.NewReader(input), "skillshare-ui-dist.tar.gz")
	if err != nil {
		t.Fatal(err)
	}
	if hash != "def456" {
		t.Errorf("got %s, want def456", hash)
	}
}

func TestParseChecksum_NotFound(t *testing.T) {
	input := "abc123  other-file.tar.gz\n"
	_, err := parseChecksum(strings.NewReader(input), "skillshare-ui-dist.tar.gz")
	if err == nil {
		t.Error("expected error for missing checksum")
	}
}

func TestExtractTarGz(t *testing.T) {
	// Create an in-memory tar.gz with test files
	tarball := createTestTarGz(t, map[string]string{
		"index.html":       "<html>hello</html>",
		"assets/main.js":   "console.log('hi')",
		"assets/style.css": "body {}",
	})

	// Write to temp file
	tmpFile := filepath.Join(t.TempDir(), "test.tar.gz")
	if err := os.WriteFile(tmpFile, tarball, 0644); err != nil {
		t.Fatal(err)
	}

	// Extract
	destDir := filepath.Join(t.TempDir(), "extracted")
	if err := os.MkdirAll(destDir, 0755); err != nil {
		t.Fatal(err)
	}
	if err := extractTarGz(tmpFile, destDir); err != nil {
		t.Fatal(err)
	}

	// Verify files exist
	for _, name := range []string{"index.html", "assets/main.js", "assets/style.css"} {
		p := filepath.Join(destDir, name)
		if _, err := os.Stat(p); err != nil {
			t.Errorf("expected file %s to exist: %v", name, err)
		}
	}

	// Verify content
	data, err := os.ReadFile(filepath.Join(destDir, "index.html"))
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != "<html>hello</html>" {
		t.Errorf("unexpected content: %s", data)
	}
}

func TestExtractTarGz_RejectsPathTraversal(t *testing.T) {
	tarball := createTestTarGz(t, map[string]string{
		"../escape.txt": "nope",
	})

	tmpFile := filepath.Join(t.TempDir(), "evil.tar.gz")
	if err := os.WriteFile(tmpFile, tarball, 0644); err != nil {
		t.Fatal(err)
	}

	destDir := filepath.Join(t.TempDir(), "dest")
	if err := os.MkdirAll(destDir, 0755); err != nil {
		t.Fatal(err)
	}

	err := extractTarGz(tmpFile, destDir)
	if err == nil {
		t.Error("expected error for path traversal")
	}
}

func TestClearCache(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("XDG_CACHE_HOME", tmp)

	// Create fake cached version
	dir := filepath.Join(tmp, "skillshare", "ui", "1.0.0")
	if err := os.MkdirAll(dir, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "index.html"), []byte("hi"), 0644); err != nil {
		t.Fatal(err)
	}

	if err := ClearCache(); err != nil {
		t.Fatal(err)
	}

	// ui/ directory should be gone
	if _, err := os.Stat(filepath.Join(tmp, "skillshare", "ui")); !os.IsNotExist(err) {
		t.Error("expected ui cache dir to be removed")
	}
}

func TestClearCache_NoDir(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("XDG_CACHE_HOME", tmp)

	// Should not error when nothing exists
	if err := ClearCache(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestClearVersion(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("XDG_CACHE_HOME", tmp)

	// Create two versions
	for _, v := range []string{"1.0.0", "1.1.0"} {
		dir := filepath.Join(tmp, "skillshare", "ui", v)
		if err := os.MkdirAll(dir, 0755); err != nil {
			t.Fatal(err)
		}
		os.WriteFile(filepath.Join(dir, "index.html"), []byte("hi"), 0644)
	}

	if err := ClearVersion("1.0.0"); err != nil {
		t.Fatal(err)
	}

	// 1.0.0 gone, 1.1.0 still there
	if _, err := os.Stat(filepath.Join(tmp, "skillshare", "ui", "1.0.0")); !os.IsNotExist(err) {
		t.Error("expected 1.0.0 to be removed")
	}
	if _, err := os.Stat(filepath.Join(tmp, "skillshare", "ui", "1.1.0")); err != nil {
		t.Error("expected 1.1.0 to still exist")
	}
}

func TestCleanOldVersions(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("XDG_CACHE_HOME", tmp)

	// Create three versions
	for _, v := range []string{"0.9.0", "1.0.0", "1.1.0"} {
		dir := filepath.Join(tmp, "skillshare", "ui", v)
		os.MkdirAll(dir, 0755)
		os.WriteFile(filepath.Join(dir, "index.html"), []byte("hi"), 0644)
	}

	if err := cleanOldVersions("1.0.0"); err != nil {
		t.Fatal(err)
	}

	// Only 1.0.0 should remain
	entries, _ := os.ReadDir(filepath.Join(tmp, "skillshare", "ui"))
	if len(entries) != 1 || entries[0].Name() != "1.0.0" {
		names := make([]string, len(entries))
		for i, e := range entries {
			names[i] = e.Name()
		}
		t.Errorf("expected only [1.0.0], got %v", names)
	}
}

// createTestTarGz builds an in-memory tar.gz from a map of path→content.
func createTestTarGz(t *testing.T, files map[string]string) []byte {
	t.Helper()

	var buf bytes.Buffer
	gw := gzip.NewWriter(&buf)
	tw := tar.NewWriter(gw)

	for name, content := range files {
		hdr := &tar.Header{
			Name: name,
			Mode: 0644,
			Size: int64(len(content)),
		}
		if err := tw.WriteHeader(hdr); err != nil {
			t.Fatal(err)
		}
		if _, err := tw.Write([]byte(content)); err != nil {
			t.Fatal(err)
		}
	}

	if err := tw.Close(); err != nil {
		t.Fatal(err)
	}
	if err := gw.Close(); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}
