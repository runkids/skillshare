package sync

import (
	"os"
	"path/filepath"
	"testing"

	"skillshare/internal/sourcewalk"
)

func TestDiscoverSourceSkillsWithFollow(t *testing.T) {
	root, ext := t.TempDir(), t.TempDir()
	for _, rel := range []string{"repo/.git", "repo/a", "group/b", "hidden/c"} {
		dir := filepath.Join(ext, rel)
		if err := os.MkdirAll(dir, 0755); err != nil {
			t.Fatal(err)
		}
		if filepath.Base(dir) != ".git" {
			if err := os.WriteFile(filepath.Join(dir, "SKILL.md"), []byte("# Skill"), 0644); err != nil {
				t.Fatal(err)
			}
		}
	}
	for name, dir := range map[string]string{"_repo": "repo", "group": "group", "hidden": "hidden"} {
		if err := os.Symlink(filepath.Join(ext, dir), filepath.Join(root, name)); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.WriteFile(filepath.Join(root, ".skillfollow"), []byte("_repo\ngroup\n"), 0644); err != nil {
		t.Fatal(err)
	}
	legacy, err := DiscoverSourceSkills(root)
	if err != nil || len(legacy) != 0 {
		t.Fatalf("legacy: %v %v", legacy, err)
	}
	set := sourcewalk.Follow(root, sourcewalk.FollowOptions{})
	skills, _, err := DiscoverSourceSkillsWithOptions(root, DiscoveryOptions{Follow: &set})
	if err != nil || len(skills) != 2 {
		t.Fatalf("%+v %v", skills, err)
	}
	for i, rel := range []string{"_repo/a", "group/b"} {
		if skills[i].RelPath != rel || skills[i].SourcePath != filepath.Join(root, rel) || skills[i].IsInRepo != (i == 0) {
			t.Fatalf("%+v", skills[i])
		}
	}
	if skills[0].FlatName != "_repo__a" || skills[1].FlatName != "group__b" {
		t.Fatalf("%+v", skills)
	}
}
