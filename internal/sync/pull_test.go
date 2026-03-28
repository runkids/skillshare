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

	skills, err := FindLocalSkills(tgt, src, "merge")
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

	skills, err := FindLocalSkills(tgt, src, "symlink")
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

	skills, err := FindLocalSkills(tgt, src, "merge")
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

	skills, err := FindLocalSkills(tgt, src, "copy")
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

func TestFindLocalSkills_CopyToMergeSwitch(t *testing.T) {
	tmp := t.TempDir()
	src := filepath.Join(tmp, "source")
	tgt := filepath.Join(tmp, "target")

	os.MkdirAll(src, 0755)
	os.MkdirAll(tgt, 0755)

	// Simulate: skills were synced in copy mode, manifest still has them
	copiedSkill := filepath.Join(tgt, "copied")
	os.MkdirAll(copiedSkill, 0755)
	os.WriteFile(filepath.Join(copiedSkill, "SKILL.md"), []byte("copied"), 0644)

	m := &Manifest{Managed: map[string]string{"copied": "abc123"}}
	if err := WriteManifest(tgt, m); err != nil {
		t.Fatal(err)
	}

	// Mode changed to merge — stale manifest entries should be ignored
	skills, err := FindLocalSkills(tgt, src, "merge")
	if err != nil {
		t.Fatal(err)
	}
	if len(skills) != 1 {
		t.Fatalf("expected 1 skill after copy→merge switch, got %d", len(skills))
	}
	if skills[0].Name != "copied" {
		t.Errorf("expected 'copied', got %q", skills[0].Name)
	}
}

func TestFindLocalSkills_EmptyModePassedDirectly(t *testing.T) {
	tmp := t.TempDir()
	src := filepath.Join(tmp, "source")
	tgt := filepath.Join(tmp, "target")

	os.MkdirAll(src, 0755)
	os.MkdirAll(tgt, 0755)

	// Physical dir with copy-mode manifest
	os.MkdirAll(filepath.Join(tgt, "skill-a"), 0755)
	m := &Manifest{Managed: map[string]string{"skill-a": "abc123"}}
	if err := WriteManifest(tgt, m); err != nil {
		t.Fatal(err)
	}

	// Empty mode means the caller didn't resolve the global default.
	// FindLocalSkills treats "" as non-copy, so manifest is ignored.
	// Callers are responsible for resolving "" → global mode before calling.
	skills, err := FindLocalSkills(tgt, src, "")
	if err != nil {
		t.Fatal(err)
	}
	if len(skills) != 1 {
		t.Fatalf("expected 1 skill with raw empty mode, got %d", len(skills))
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

func TestPullSkill_NestedDirectories(t *testing.T) {
	tmp := t.TempDir()
	src := filepath.Join(tmp, "source")
	tgt := filepath.Join(tmp, "target")

	os.MkdirAll(src, 0755)

	// Create a skill with nested subdirectories in the target
	localSkill := filepath.Join(tgt, "my-skill")
	os.MkdirAll(filepath.Join(localSkill, "prompts"), 0755)
	os.MkdirAll(filepath.Join(localSkill, "templates", "react"), 0755)
	os.WriteFile(filepath.Join(localSkill, "SKILL.md"), []byte("# My Skill"), 0644)
	os.WriteFile(filepath.Join(localSkill, "prompts", "default.md"), []byte("prompt content"), 0644)
	os.WriteFile(filepath.Join(localSkill, "templates", "base.md"), []byte("base template"), 0644)
	os.WriteFile(filepath.Join(localSkill, "templates", "react", "component.md"), []byte("react template"), 0644)

	skill := LocalSkillInfo{
		Name: "my-skill",
		Path: localSkill,
	}

	if err := PullSkill(skill, src, false); err != nil {
		t.Fatal(err)
	}

	// Verify ALL files were copied, including nested ones
	checks := []struct {
		path string
		want string
	}{
		{"my-skill/SKILL.md", "# My Skill"},
		{"my-skill/prompts/default.md", "prompt content"},
		{"my-skill/templates/base.md", "base template"},
		{"my-skill/templates/react/component.md", "react template"},
	}
	for _, c := range checks {
		data, err := os.ReadFile(filepath.Join(src, c.path))
		if err != nil {
			t.Errorf("expected %s to exist in source after pull: %v", c.path, err)
			continue
		}
		if string(data) != c.want {
			t.Errorf("%s: got %q, want %q", c.path, string(data), c.want)
		}
	}
}

func TestPullSkill_GitRepoSkipsGit(t *testing.T) {
	tmp := t.TempDir()
	src := filepath.Join(tmp, "source")
	tgt := filepath.Join(tmp, "target")

	os.MkdirAll(src, 0755)

	// Simulate a git-cloned repo in the target (like superpowers)
	localSkill := filepath.Join(tgt, "superpowers")
	os.MkdirAll(filepath.Join(localSkill, ".git", "objects"), 0755)
	os.MkdirAll(filepath.Join(localSkill, "agents", "brainstorming"), 0755)
	os.MkdirAll(filepath.Join(localSkill, "commands", "commit"), 0755)
	os.MkdirAll(filepath.Join(localSkill, "docs"), 0755)

	// .git files
	os.WriteFile(filepath.Join(localSkill, ".git", "HEAD"), []byte("ref: refs/heads/main"), 0644)
	os.WriteFile(filepath.Join(localSkill, ".git", "config"), []byte("[core]"), 0644)

	// Skill files
	os.WriteFile(filepath.Join(localSkill, "agents", "brainstorming", "SKILL.md"), []byte("# Brainstorming"), 0644)
	os.WriteFile(filepath.Join(localSkill, "commands", "commit", "SKILL.md"), []byte("# Commit"), 0644)
	os.WriteFile(filepath.Join(localSkill, "docs", "README.md"), []byte("# Docs"), 0644)
	os.WriteFile(filepath.Join(localSkill, "gemini-extension.js"), []byte("// ext"), 0644)

	skill := LocalSkillInfo{
		Name: "superpowers",
		Path: localSkill,
	}

	if err := PullSkill(skill, src, false); err != nil {
		t.Fatal(err)
	}

	// Verify skill files were copied
	checks := []struct {
		path string
		want string
	}{
		{"superpowers/agents/brainstorming/SKILL.md", "# Brainstorming"},
		{"superpowers/commands/commit/SKILL.md", "# Commit"},
		{"superpowers/docs/README.md", "# Docs"},
		{"superpowers/gemini-extension.js", "// ext"},
	}
	for _, c := range checks {
		data, err := os.ReadFile(filepath.Join(src, c.path))
		if err != nil {
			t.Errorf("expected %s to exist in source after pull: %v", c.path, err)
			continue
		}
		if string(data) != c.want {
			t.Errorf("%s: got %q, want %q", c.path, string(data), c.want)
		}
	}

	// Verify .git was NOT copied
	if _, err := os.Stat(filepath.Join(src, "superpowers", ".git")); !os.IsNotExist(err) {
		t.Error(".git directory should NOT be copied to source")
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
