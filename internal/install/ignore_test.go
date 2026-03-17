package install

import (
	"os"
	"path/filepath"
	"testing"
)

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
