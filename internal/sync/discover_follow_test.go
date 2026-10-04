package sync

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"skillshare/internal/install"
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

// A tracked repository nested below the first path component applies its own
// .skillignore, whether it is reached through a followed group or a plain one.
func TestDiscoverSourceSkillsNestedRepoSkillignore(t *testing.T) {
	for _, followed := range []bool{false, true} {
		t.Run(map[bool]string{false: "plain", true: "followed"}[followed], func(t *testing.T) {
			root, group := t.TempDir(), filepath.Join(t.TempDir(), "group")
			if !followed {
				group = filepath.Join(root, "group")
			}
			repo := filepath.Join(group, "sub", "_repo")
			for _, rel := range []string{".git", "keep", "drop"} {
				if err := os.MkdirAll(filepath.Join(repo, rel), 0755); err != nil {
					t.Fatal(err)
				}
				if rel != ".git" {
					if err := os.WriteFile(filepath.Join(repo, rel, "SKILL.md"), []byte("# Skill"), 0644); err != nil {
						t.Fatal(err)
					}
				}
			}
			if err := os.WriteFile(filepath.Join(repo, ".skillignore"), []byte("drop\n"), 0644); err != nil {
				t.Fatal(err)
			}
			opts := DiscoveryOptions{}
			if followed {
				if err := os.Symlink(group, filepath.Join(root, "group")); err != nil {
					t.Fatal(err)
				}
				if err := os.WriteFile(filepath.Join(root, ".skillfollow"), []byte("group\n"), 0644); err != nil {
					t.Fatal(err)
				}
				set := sourcewalk.Follow(root, sourcewalk.FollowOptions{})
				opts.Follow = &set
			}
			skills, _, err := DiscoverSourceSkillsWithOptions(root, opts)
			if err != nil || len(skills) != 1 || skills[0].RelPath != "group/sub/_repo/keep" {
				t.Fatalf("%+v %v", skills, err)
			}
		})
	}
}

// A skill in a tracked repository below the first path component belongs to
// that repository, so its .metadata.json target override applies, whether the
// repository sits in a followed group or a plain one (install --into).
func TestDiscoverSourceSkillsNestedRepoOwnership(t *testing.T) {
	for _, followed := range []bool{false, true} {
		t.Run(map[bool]string{false: "plain", true: "followed"}[followed], func(t *testing.T) {
			root, group := t.TempDir(), filepath.Join(t.TempDir(), "group")
			if !followed {
				group = filepath.Join(root, "group")
			}
			repo := filepath.Join(group, "sub", "_repo")
			if err := os.MkdirAll(filepath.Join(repo, ".git"), 0755); err != nil {
				t.Fatal(err)
			}
			writeSkillMD(t, filepath.Join(repo, "a"), "---\nname: a\nmetadata:\n  targets:\n    - cursor\n---\n# a")
			store := install.NewMetadataStore()
			store.SetTargetOverride("group/sub/_repo/a", []string{"claude"})
			if err := store.Save(root); err != nil {
				t.Fatal(err)
			}
			opts := DiscoveryOptions{}
			if followed {
				if err := os.Symlink(group, filepath.Join(root, "group")); err != nil {
					t.Fatal(err)
				}
				if err := os.WriteFile(filepath.Join(root, ".skillfollow"), []byte("group\n"), 0644); err != nil {
					t.Fatal(err)
				}
				set := sourcewalk.Follow(root, sourcewalk.FollowOptions{})
				opts.Follow = &set
			}
			skills, _, err := DiscoverSourceSkillsWithOptions(root, opts)
			if err != nil || len(skills) != 1 {
				t.Fatalf("%+v %v", skills, err)
			}
			got := skills[0]
			if !got.IsInRepo || got.RepoRelPath != "group/sub/_repo" || !reflect.DeepEqual(got.Targets, []string{"claude"}) {
				t.Fatalf("IsInRepo=%v RepoRelPath=%q Targets=%v", got.IsInRepo, got.RepoRelPath, got.Targets)
			}
		})
	}
}
