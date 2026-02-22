package install

import (
	"os"
	"path/filepath"
	"testing"
)

func createSkillWithMeta(t *testing.T, baseDir, name string, meta *SkillMeta) {
	t.Helper()
	dir := filepath.Join(baseDir, name)
	os.MkdirAll(dir, 0755)
	os.WriteFile(filepath.Join(dir, "SKILL.md"), []byte("---\nname: "+name+"\n---\n"), 0644)
	if meta != nil {
		if err := WriteMeta(dir, meta); err != nil {
			t.Fatalf("write meta for %s: %v", name, err)
		}
	}
}

func TestGetUpdatableSkills_FindsMeta(t *testing.T) {
	src := t.TempDir()
	createSkillWithMeta(t, src, "remote-skill", &SkillMeta{
		Source: "github.com/user/repo",
		Type:   "github",
	})

	skills, err := GetUpdatableSkills(src)
	if err != nil {
		t.Fatal(err)
	}
	if len(skills) != 1 {
		t.Fatalf("expected 1 updatable skill, got %d", len(skills))
	}
	if skills[0] != "remote-skill" {
		t.Errorf("expected 'remote-skill', got %q", skills[0])
	}
}

func TestGetUpdatableSkills_SkipsTrackedRepos(t *testing.T) {
	src := t.TempDir()
	// Tracked repo (starts with _) should be skipped
	createSkillWithMeta(t, src, "_team-repo", &SkillMeta{
		Source: "github.com/team/repo",
		Type:   "github",
	})
	// Also create a nested skill inside tracked repo
	nestedDir := filepath.Join(src, "_team-repo", "sub-skill")
	os.MkdirAll(nestedDir, 0755)
	os.WriteFile(filepath.Join(nestedDir, "SKILL.md"), []byte("nested"), 0644)

	skills, err := GetUpdatableSkills(src)
	if err != nil {
		t.Fatal(err)
	}
	if len(skills) != 0 {
		t.Errorf("expected 0 updatable skills (tracked repos skipped), got %d: %v", len(skills), skills)
	}
}

func TestGetUpdatableSkills_SkipsNoSource(t *testing.T) {
	src := t.TempDir()
	// Skill with no source in metadata should be skipped
	createSkillWithMeta(t, src, "local-only", &SkillMeta{
		Type: "local",
		// Source is empty
	})
	// Skill with no metadata at all
	noMetaDir := filepath.Join(src, "no-meta")
	os.MkdirAll(noMetaDir, 0755)
	os.WriteFile(filepath.Join(noMetaDir, "SKILL.md"), []byte("no meta"), 0644)

	skills, err := GetUpdatableSkills(src)
	if err != nil {
		t.Fatal(err)
	}
	if len(skills) != 0 {
		t.Errorf("expected 0 updatable skills, got %d: %v", len(skills), skills)
	}
}

func TestGetUpdatableSkills_Nested(t *testing.T) {
	src := t.TempDir()
	// Nested skill (non-tracked) with remote source
	nestedDir := filepath.Join(src, "group", "my-skill")
	os.MkdirAll(nestedDir, 0755)
	os.WriteFile(filepath.Join(nestedDir, "SKILL.md"), []byte("nested"), 0644)
	WriteMeta(nestedDir, &SkillMeta{Source: "github.com/u/r", Type: "github"})

	skills, err := GetUpdatableSkills(src)
	if err != nil {
		t.Fatal(err)
	}
	if len(skills) != 1 {
		t.Fatalf("expected 1 nested updatable skill, got %d", len(skills))
	}
	// relPath should be "group/my-skill"
	if skills[0] != filepath.Join("group", "my-skill") {
		t.Errorf("expected 'group/my-skill', got %q", skills[0])
	}
}

func TestGetTrackedRepos_FindsGitRepos(t *testing.T) {
	src := t.TempDir()
	// Create a tracked repo with .git directory
	repoDir := filepath.Join(src, "_team-skills")
	os.MkdirAll(filepath.Join(repoDir, ".git"), 0755)

	repos, err := GetTrackedRepos(src)
	if err != nil {
		t.Fatal(err)
	}
	if len(repos) != 1 {
		t.Fatalf("expected 1 tracked repo, got %d", len(repos))
	}
	if repos[0] != "_team-skills" {
		t.Errorf("expected '_team-skills', got %q", repos[0])
	}
}

func TestGetTrackedRepos_SkipsNonGit(t *testing.T) {
	src := t.TempDir()
	// _-prefixed directory without .git
	os.MkdirAll(filepath.Join(src, "_not-a-repo"), 0755)

	repos, err := GetTrackedRepos(src)
	if err != nil {
		t.Fatal(err)
	}
	if len(repos) != 0 {
		t.Errorf("expected 0 repos (no .git), got %d", len(repos))
	}
}

func TestGetTrackedRepos_FindsNested(t *testing.T) {
	src := t.TempDir()
	// Nested tracked repo in organizational directory
	nestedRepo := filepath.Join(src, "frontend", "_ui-skills")
	os.MkdirAll(filepath.Join(nestedRepo, ".git"), 0755)

	repos, err := GetTrackedRepos(src)
	if err != nil {
		t.Fatal(err)
	}
	if len(repos) != 1 {
		t.Fatalf("expected 1 nested tracked repo, got %d", len(repos))
	}
	if repos[0] != filepath.Join("frontend", "_ui-skills") {
		t.Errorf("expected 'frontend/_ui-skills', got %q", repos[0])
	}
}
