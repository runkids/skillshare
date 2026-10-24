package textdiff

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestFiles(t *testing.T) {
	dir := t.TempDir()
	oldPath := filepath.Join(dir, "old.txt")
	newPath := filepath.Join(dir, "new.txt")

	if err := os.WriteFile(oldPath, []byte("line1\nline2\nline3\n"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(newPath, []byte("line1\nmodified\nline3\nextra\n"), 0644); err != nil {
		t.Fatal(err)
	}

	result := Files(oldPath, newPath)

	if !strings.Contains(result, "+") && !strings.Contains(result, "-") {
		t.Errorf("expected diff output to contain '+' and '-' lines, got:\n%s", result)
	}
	if !strings.Contains(result, "modified") {
		t.Errorf("expected diff to mention 'modified', got:\n%s", result)
	}
	if !strings.Contains(result, "extra") {
		t.Errorf("expected diff to mention 'extra', got:\n%s", result)
	}
}

func TestIsBinary(t *testing.T) {
	if !isBinary("hello\x00world") {
		t.Error("expected isBinary to return true for string with null byte")
	}
	if isBinary("hello world, this is normal text") {
		t.Error("expected isBinary to return false for normal text")
	}
	if isBinary("") {
		t.Error("expected isBinary to return false for empty string")
	}
}
