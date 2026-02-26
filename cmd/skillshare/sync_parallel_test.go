package main

import (
	"os"
	"path/filepath"
	"testing"
)

func TestCanonicalPath_TrailingSlash(t *testing.T) {
	dir := t.TempDir()
	// With and without trailing slash should resolve to the same key.
	a := canonicalPath(dir)
	b := canonicalPath(dir + "/")
	if a != b {
		t.Errorf("trailing slash produced different keys: %q vs %q", a, b)
	}
}

func TestCanonicalPath_RelativeSegments(t *testing.T) {
	dir := t.TempDir()
	sub := filepath.Join(dir, "sub")
	os.MkdirAll(sub, 0o755)

	a := canonicalPath(sub)
	b := canonicalPath(filepath.Join(dir, "sub", "..", "sub"))
	if a != b {
		t.Errorf("relative segments produced different keys: %q vs %q", a, b)
	}
}

func TestCanonicalPath_Symlink(t *testing.T) {
	dir := t.TempDir()
	real := filepath.Join(dir, "real")
	link := filepath.Join(dir, "link")
	os.MkdirAll(real, 0o755)
	if err := os.Symlink(real, link); err != nil {
		t.Skip("symlinks not supported")
	}

	a := canonicalPath(real)
	b := canonicalPath(link)
	if a != b {
		t.Errorf("symlink produced different keys: %q vs %q", a, b)
	}
}

func TestCanonicalPath_SymlinkParentNonExistentTail(t *testing.T) {
	// /link -> /real exists, but /link/skills and /real/skills do NOT exist.
	// Both must resolve to the same canonical key.
	dir := t.TempDir()
	real := filepath.Join(dir, "real")
	link := filepath.Join(dir, "link")
	os.MkdirAll(real, 0o755)
	if err := os.Symlink(real, link); err != nil {
		t.Skip("symlinks not supported")
	}

	a := canonicalPath(filepath.Join(real, "skills"))
	b := canonicalPath(filepath.Join(link, "skills"))
	if a != b {
		t.Errorf("symlink parent with non-existent tail produced different keys: %q vs %q", a, b)
	}
}

func TestCanonicalPath_NonExistent(t *testing.T) {
	// Should not panic on non-existent paths; falls back to filepath.Abs.
	p := canonicalPath("/nonexistent/path/that/does/not/exist")
	if p == "" {
		t.Error("canonicalPath returned empty string for non-existent path")
	}
	if !filepath.IsAbs(p) {
		t.Errorf("expected absolute path, got %q", p)
	}
}
