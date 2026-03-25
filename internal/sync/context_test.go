package sync

import (
	"os"
	"path/filepath"
	"testing"
)

func TestCalcSkillContext_WithDescription(t *testing.T) {
	dir := t.TempDir()
	content := "---\nname: my-skill\ndescription: A helpful skill for testing\n---\n# My Skill\nBody content here"
	os.WriteFile(filepath.Join(dir, "SKILL.md"), []byte(content), 0644)

	descChars, bodyChars, err := CalcSkillContext(dir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// "my-skill A helpful skill for testing" = 36 chars
	if descChars != 36 {
		t.Errorf("descChars: expected 36, got %d", descChars)
	}
	// "# My Skill\nBody content here" = 28 chars
	if bodyChars != 28 {
		t.Errorf("bodyChars: expected 28, got %d", bodyChars)
	}
}

func TestCalcSkillContext_NoDescription(t *testing.T) {
	dir := t.TempDir()
	content := "---\nname: minimal\n---\n# Minimal\nJust body"
	os.WriteFile(filepath.Join(dir, "SKILL.md"), []byte(content), 0644)

	descChars, bodyChars, err := CalcSkillContext(dir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if descChars != 7 {
		t.Errorf("descChars: expected 7 (name only), got %d", descChars)
	}
	if bodyChars == 0 {
		t.Errorf("bodyChars: expected > 0, got 0")
	}
}

func TestCalcSkillContext_NoFrontmatter(t *testing.T) {
	dir := t.TempDir()
	content := "# No Frontmatter\nJust plain content"
	os.WriteFile(filepath.Join(dir, "SKILL.md"), []byte(content), 0644)

	descChars, bodyChars, err := CalcSkillContext(dir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if descChars != 0 {
		t.Errorf("descChars: expected 0, got %d", descChars)
	}
	if bodyChars != 35 {
		t.Errorf("bodyChars: expected 35, got %d", bodyChars)
	}
}

func TestCalcSkillContext_EmptyFile(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "SKILL.md"), []byte(""), 0644)

	descChars, bodyChars, err := CalcSkillContext(dir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if descChars != 0 {
		t.Errorf("descChars: expected 0, got %d", descChars)
	}
	if bodyChars != 0 {
		t.Errorf("bodyChars: expected 0, got %d", bodyChars)
	}
}

func TestCalcSkillContext_MultilineDescription(t *testing.T) {
	dir := t.TempDir()
	content := "---\nname: multi\ndescription: |\n  Line one\n  Line two\n---\n# Body"
	os.WriteFile(filepath.Join(dir, "SKILL.md"), []byte(content), 0644)

	descChars, bodyChars, err := CalcSkillContext(dir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// Must be > 5 ("multi") since description adds chars
	if descChars <= 5 {
		t.Errorf("descChars: expected > 5 for multiline description, got %d", descChars)
	}
	if bodyChars == 0 {
		t.Errorf("bodyChars: expected > 0, got 0")
	}
}

func TestCalcSkillContext_NoSkillMd(t *testing.T) {
	dir := t.TempDir()

	descChars, bodyChars, err := CalcSkillContext(dir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if descChars != 0 {
		t.Errorf("descChars: expected 0, got %d", descChars)
	}
	if bodyChars != 0 {
		t.Errorf("bodyChars: expected 0, got %d", bodyChars)
	}
}
