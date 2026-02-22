package install

import (
	"os"
	"path/filepath"
	"testing"
)

func TestMatchSkillIgnore_ExactMatch(t *testing.T) {
	patterns := []string{"debug-tool"}
	if !matchSkillIgnore("debug-tool", patterns) {
		t.Error("expected exact match to return true")
	}
}

func TestMatchSkillIgnore_GroupPrefix(t *testing.T) {
	patterns := []string{"experimental"}
	// "experimental" matches "experimental/sub-skill" as a group prefix
	if !matchSkillIgnore("experimental/sub-skill", patterns) {
		t.Error("expected group prefix match to return true")
	}
}

func TestMatchSkillIgnore_WildcardSuffix(t *testing.T) {
	patterns := []string{"test-*"}
	if !matchSkillIgnore("test-alpha", patterns) {
		t.Error("expected wildcard suffix match for 'test-alpha'")
	}
	if !matchSkillIgnore("test-beta/sub", patterns) {
		t.Error("expected wildcard suffix match for 'test-beta/sub'")
	}
}

func TestMatchSkillIgnore_NoMatch(t *testing.T) {
	patterns := []string{"debug-tool", "test-*"}
	if matchSkillIgnore("production-skill", patterns) {
		t.Error("expected no match for 'production-skill'")
	}
}

func TestReadSkillIgnore_ParsesFile(t *testing.T) {
	dir := t.TempDir()
	content := "# Comment line\n\ndebug-tool\ntest-*\nexperimental\n"
	if err := os.WriteFile(filepath.Join(dir, ".skillignore"), []byte(content), 0644); err != nil {
		t.Fatal(err)
	}

	patterns := readSkillIgnore(dir)
	if len(patterns) != 3 {
		t.Fatalf("expected 3 patterns (skipping comment and blank), got %d: %v", len(patterns), patterns)
	}
	expected := []string{"debug-tool", "test-*", "experimental"}
	for i, p := range patterns {
		if p != expected[i] {
			t.Errorf("pattern[%d] = %q, want %q", i, p, expected[i])
		}
	}
}

func TestReadSkillIgnore_NoFile(t *testing.T) {
	dir := t.TempDir()
	patterns := readSkillIgnore(dir)
	if patterns != nil {
		t.Errorf("expected nil patterns for missing .skillignore, got %v", patterns)
	}
}

func TestDiscoverSkills_WithSkillIgnore(t *testing.T) {
	dir := t.TempDir()

	// Create skills
	for _, name := range []string{"alpha", "beta", "test-debug"} {
		skillDir := filepath.Join(dir, name)
		os.MkdirAll(skillDir, 0755)
		os.WriteFile(filepath.Join(skillDir, "SKILL.md"), []byte("---\nname: "+name+"\n---\n"), 0644)
	}

	// Create .skillignore to exclude test-*
	os.WriteFile(filepath.Join(dir, ".skillignore"), []byte("test-*\n"), 0644)

	skills := discoverSkills(dir, false)
	for _, s := range skills {
		if s.Name == "test-debug" {
			t.Error("expected 'test-debug' to be filtered by .skillignore")
		}
	}
}
