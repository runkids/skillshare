package sync

import (
	"os"
	"path/filepath"
	"testing"
)

func TestDiscoverSourceSkillsAll_IncludesDisabled(t *testing.T) {
	dir := t.TempDir()

	enabledDir := filepath.Join(dir, "enabled-skill")
	os.MkdirAll(enabledDir, 0755)
	os.WriteFile(filepath.Join(enabledDir, "SKILL.md"), []byte("---\nname: enabled\n---\n# Enabled"), 0644)

	disabledDir := filepath.Join(dir, "disabled-skill")
	os.MkdirAll(disabledDir, 0755)
	os.WriteFile(filepath.Join(disabledDir, "SKILL.md"), []byte("---\nname: disabled\n---\n# Disabled"), 0644)

	os.WriteFile(filepath.Join(dir, ".skillignore"), []byte("disabled-skill\n"), 0644)

	skills, err := DiscoverSourceSkillsAll(dir)
	if err != nil {
		t.Fatalf("DiscoverSourceSkillsAll: %v", err)
	}

	if len(skills) != 2 {
		t.Fatalf("expected 2 skills, got %d", len(skills))
	}

	var foundEnabled, foundDisabled bool
	for _, s := range skills {
		switch s.FlatName {
		case "enabled-skill":
			if s.Disabled {
				t.Error("enabled-skill should not be disabled")
			}
			foundEnabled = true
		case "disabled-skill":
			if !s.Disabled {
				t.Error("disabled-skill should be disabled")
			}
			foundDisabled = true
		}
	}
	if !foundEnabled {
		t.Error("enabled-skill not found in results")
	}
	if !foundDisabled {
		t.Error("disabled-skill not found in results")
	}
}

func TestDiscoverSourceSkillsAll_NoIgnoreFile(t *testing.T) {
	dir := t.TempDir()

	skillDir := filepath.Join(dir, "my-skill")
	os.MkdirAll(skillDir, 0755)
	os.WriteFile(filepath.Join(skillDir, "SKILL.md"), []byte("# My Skill"), 0644)

	skills, err := DiscoverSourceSkillsAll(dir)
	if err != nil {
		t.Fatalf("DiscoverSourceSkillsAll: %v", err)
	}

	if len(skills) != 1 {
		t.Fatalf("expected 1 skill, got %d", len(skills))
	}
	if skills[0].Disabled {
		t.Error("skill should not be disabled when no .skillignore exists")
	}
}

func TestDiscoverSourceSkillsAll_RepoLevelIgnore(t *testing.T) {
	dir := t.TempDir()

	// Create a tracked repo with .git dir and two skills
	repoDir := filepath.Join(dir, "_team-repo")
	os.MkdirAll(filepath.Join(repoDir, ".git"), 0755)

	enabledDir := filepath.Join(repoDir, "enabled-skill")
	os.MkdirAll(enabledDir, 0755)
	os.WriteFile(filepath.Join(enabledDir, "SKILL.md"), []byte("# Enabled"), 0644)

	disabledDir := filepath.Join(repoDir, "disabled-skill")
	os.MkdirAll(disabledDir, 0755)
	os.WriteFile(filepath.Join(disabledDir, "SKILL.md"), []byte("# Disabled"), 0644)

	// Repo-level .skillignore
	os.WriteFile(filepath.Join(repoDir, ".skillignore"), []byte("disabled-skill\n"), 0644)

	skills, err := DiscoverSourceSkillsAll(dir)
	if err != nil {
		t.Fatalf("DiscoverSourceSkillsAll: %v", err)
	}

	if len(skills) != 2 {
		t.Fatalf("expected 2 skills, got %d", len(skills))
	}

	for _, s := range skills {
		if s.FlatName == "_team-repo__disabled-skill" {
			if !s.Disabled {
				t.Error("repo-level disabled skill should have Disabled=true")
			}
			if !s.IsInRepo {
				t.Error("repo-level disabled skill should have IsInRepo=true")
			}
		}
		if s.FlatName == "_team-repo__enabled-skill" {
			if s.Disabled {
				t.Error("enabled skill should not be disabled")
			}
		}
	}
}
