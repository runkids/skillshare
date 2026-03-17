package skillignore

import (
	"os"
	"path/filepath"
	"testing"
)

func TestMatch_ExactMatch(t *testing.T) {
	patterns := []string{"debug-tool"}
	if !Match("debug-tool", patterns) {
		t.Error("expected exact match")
	}
}

func TestMatch_GroupPrefix(t *testing.T) {
	patterns := []string{"experimental"}
	if !Match("experimental/sub-skill", patterns) {
		t.Error("expected group prefix match")
	}
}

func TestMatch_WildcardSuffix(t *testing.T) {
	patterns := []string{"test-*"}
	if !Match("test-alpha", patterns) {
		t.Error("expected wildcard match for test-alpha")
	}
	if !Match("test-beta/sub", patterns) {
		t.Error("expected wildcard match for test-beta/sub")
	}
}

func TestMatch_NoMatch(t *testing.T) {
	patterns := []string{"debug-tool", "test-*"}
	if Match("production-skill", patterns) {
		t.Error("expected no match")
	}
}

func TestReadPatterns_ParsesFile(t *testing.T) {
	dir := t.TempDir()
	content := "# Comment line\n\ndebug-tool\ntest-*\nexperimental\n"
	if err := os.WriteFile(filepath.Join(dir, ".skillignore"), []byte(content), 0644); err != nil {
		t.Fatal(err)
	}

	patterns := ReadPatterns(dir)
	if len(patterns) != 3 {
		t.Fatalf("expected 3 patterns, got %d: %v", len(patterns), patterns)
	}
	expected := []string{"debug-tool", "test-*", "experimental"}
	for i, p := range patterns {
		if p != expected[i] {
			t.Errorf("pattern[%d] = %q, want %q", i, p, expected[i])
		}
	}
}

func TestReadPatterns_NoFile(t *testing.T) {
	dir := t.TempDir()
	patterns := ReadPatterns(dir)
	if patterns != nil {
		t.Errorf("expected nil, got %v", patterns)
	}
}
