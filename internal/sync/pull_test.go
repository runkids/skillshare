package sync

import (
	"os"
	"path/filepath"
	"testing"
)

func TestFindLocalSkills_EmptyTarget(t *testing.T) {
	tmp := t.TempDir()
	src := filepath.Join(tmp, "source")
	tgt := filepath.Join(tmp, "target")
	os.MkdirAll(src, 0755)
	os.MkdirAll(tgt, 0755)

	skills, err := FindLocalSkills(tgt, src)
	if err != nil {
		t.Fatal(err)
	}
	if len(skills) != 0 {
		t.Errorf("expected 0 skills in empty target, got %d", len(skills))
	}
}

func TestFindLocalSkills_SymlinkTarget(t *testing.T) {
	tmp := t.TempDir()
	src := filepath.Join(tmp, "source")
	tgt := filepath.Join(tmp, "target")
	os.MkdirAll(src, 0755)

	// Target is symlink to source — symlink mode, no local skills
	if err := os.Symlink(src, tgt); err != nil {
		t.Fatal(err)
	}

	skills, err := FindLocalSkills(tgt, src)
	if err != nil {
		t.Fatal(err)
	}
	if len(skills) != 0 {
		t.Errorf("expected 0 skills for symlink target, got %d", len(skills))
	}
}

func TestFindLocalSkills_MergeMode(t *testing.T) {
	tmp := t.TempDir()
	src := filepath.Join(tmp, "source")
	tgt := filepath.Join(tmp, "target")
	skillSrc := filepath.Join(src, "synced")

	os.MkdirAll(skillSrc, 0755)
	os.MkdirAll(tgt, 0755)

	// One symlinked skill (from sync)
	if err := os.Symlink(skillSrc, filepath.Join(tgt, "synced")); err != nil {
		t.Fatal(err)
	}
	// One local skill (user-created)
	localSkill := filepath.Join(tgt, "my-local")
	os.MkdirAll(localSkill, 0755)
	os.WriteFile(filepath.Join(localSkill, "SKILL.md"), []byte("local skill"), 0644)

	skills, err := FindLocalSkills(tgt, src)
	if err != nil {
		t.Fatal(err)
	}
	if len(skills) != 1 {
		t.Fatalf("expected 1 local skill, got %d", len(skills))
	}
	if skills[0].Name != "my-local" {
		t.Errorf("expected local skill name 'my-local', got %q", skills[0].Name)
	}
}

func TestFindLocalSkills_SkipsCopyManaged(t *testing.T) {
	tmp := t.TempDir()
	src := filepath.Join(tmp, "source")
	tgt := filepath.Join(tmp, "target")

	os.MkdirAll(src, 0755)
	os.MkdirAll(tgt, 0755)

	// Create a copy-mode managed skill
	managedSkill := filepath.Join(tgt, "managed")
	os.MkdirAll(managedSkill, 0755)

	// Write manifest marking it as managed
	m := &Manifest{Managed: map[string]string{"managed": "abc123"}}
	if err := WriteManifest(tgt, m); err != nil {
		t.Fatal(err)
	}

	// Also create a truly local skill
	localSkill := filepath.Join(tgt, "local-only")
	os.MkdirAll(localSkill, 0755)

	skills, err := FindLocalSkills(tgt, src)
	if err != nil {
		t.Fatal(err)
	}
	if len(skills) != 1 {
		t.Fatalf("expected 1 local skill (skipping managed), got %d", len(skills))
	}
	if skills[0].Name != "local-only" {
		t.Errorf("expected 'local-only', got %q", skills[0].Name)
	}
}

func TestPullSkill_NewSkill(t *testing.T) {
	tmp := t.TempDir()
	src := filepath.Join(tmp, "source")
	tgt := filepath.Join(tmp, "target")

	os.MkdirAll(src, 0755)
	localSkill := filepath.Join(tgt, "my-skill")
	os.MkdirAll(localSkill, 0755)
	os.WriteFile(filepath.Join(localSkill, "SKILL.md"), []byte("# My Skill"), 0644)

	skill := LocalSkillInfo{
		Name: "my-skill",
		Path: localSkill,
	}

	if err := PullSkill(skill, src, false); err != nil {
		t.Fatal(err)
	}

	// Verify skill was copied to source
	if _, err := os.Stat(filepath.Join(src, "my-skill", "SKILL.md")); err != nil {
		t.Error("expected SKILL.md to exist in source after pull")
	}
}

func TestPullSkill_AlreadyExists(t *testing.T) {
	tmp := t.TempDir()
	src := filepath.Join(tmp, "source")
	tgt := filepath.Join(tmp, "target")

	// Skill exists in both source and target
	os.MkdirAll(filepath.Join(src, "my-skill"), 0755)
	localSkill := filepath.Join(tgt, "my-skill")
	os.MkdirAll(localSkill, 0755)

	skill := LocalSkillInfo{
		Name: "my-skill",
		Path: localSkill,
	}

	err := PullSkill(skill, src, false)
	if err == nil {
		t.Error("expected error when skill already exists")
	}
}

func TestPullSkill_ForceOverwrite(t *testing.T) {
	tmp := t.TempDir()
	src := filepath.Join(tmp, "source")
	tgt := filepath.Join(tmp, "target")

	// Skill exists in source with old content
	os.MkdirAll(filepath.Join(src, "my-skill"), 0755)
	os.WriteFile(filepath.Join(src, "my-skill", "SKILL.md"), []byte("old"), 0644)

	// Skill in target with new content
	localSkill := filepath.Join(tgt, "my-skill")
	os.MkdirAll(localSkill, 0755)
	os.WriteFile(filepath.Join(localSkill, "SKILL.md"), []byte("new"), 0644)

	skill := LocalSkillInfo{
		Name: "my-skill",
		Path: localSkill,
	}

	if err := PullSkill(skill, src, true); err != nil {
		t.Fatal(err)
	}

	// Verify source was overwritten with target content
	data, err := os.ReadFile(filepath.Join(src, "my-skill", "SKILL.md"))
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != "new" {
		t.Errorf("expected 'new' content after force pull, got %q", string(data))
	}
}
