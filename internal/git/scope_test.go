package git

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func readGitignore(t *testing.T, dir string) string {
	t.Helper()
	b, err := os.ReadFile(filepath.Join(dir, ".gitignore"))
	if err != nil {
		t.Fatalf("read .gitignore: %v", err)
	}
	return string(b)
}

func TestWriteScopeGitignore_FreshRootExcludesConfig(t *testing.T) {
	dir := t.TempDir()
	if err := WriteScopeGitignore(dir, "root"); err != nil {
		t.Fatal(err)
	}
	got := readGitignore(t, dir)
	if !strings.Contains(got, "config.yaml") {
		t.Errorf("root .gitignore must exclude config.yaml, got:\n%s", got)
	}
}

func TestWriteScopeGitignore_FreshNonRootOmitsConfig(t *testing.T) {
	dir := t.TempDir()
	if err := WriteScopeGitignore(dir, "skills"); err != nil {
		t.Fatal(err)
	}
	got := readGitignore(t, dir)
	if strings.Contains(got, "config.yaml") {
		t.Errorf("non-root .gitignore must not mention config.yaml, got:\n%s", got)
	}
}

func TestWriteScopeGitignore_AppendsToExistingAtRoot(t *testing.T) {
	dir := t.TempDir()
	gi := filepath.Join(dir, ".gitignore")
	if err := os.WriteFile(gi, []byte("node_modules/\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := WriteScopeGitignore(dir, "root"); err != nil {
		t.Fatal(err)
	}
	got := readGitignore(t, dir)
	if !strings.Contains(got, "config.yaml") {
		t.Errorf("existing root .gitignore must gain config.yaml, got:\n%s", got)
	}
	if !strings.Contains(got, "node_modules/") {
		t.Errorf("existing entries must be preserved, got:\n%s", got)
	}
}

func TestWriteScopeGitignore_IdempotentAtRoot(t *testing.T) {
	dir := t.TempDir()
	if err := WriteScopeGitignore(dir, "root"); err != nil {
		t.Fatal(err)
	}
	if err := WriteScopeGitignore(dir, "root"); err != nil {
		t.Fatal(err)
	}
	got := readGitignore(t, dir)
	if n := strings.Count(got, "config.yaml"); n != 1 {
		t.Errorf("config.yaml must appear exactly once, got %d:\n%s", n, got)
	}
}
