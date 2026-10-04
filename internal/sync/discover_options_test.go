package sync

import (
	"os"
	"path/filepath"
	"testing"

	"skillshare/internal/sourcewalk"
)

func TestDiscoveryOptionsTwins(t *testing.T) {
	root, external := t.TempDir(), t.TempDir()
	for _, dir := range []string{".git", "a"} {
		if err := os.Mkdir(filepath.Join(external, dir), 0755); err != nil {
			t.Fatal(err)
		}
	}
	body := "---\nname: a\ndescription: Followed fixture\nmetadata:\n  targets: [claude]\n---\n# Content\n"
	if err := os.WriteFile(filepath.Join(external, "a", "SKILL.md"), []byte(body), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(external, filepath.Join(root, "_repo")); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, ".skillfollow"), []byte("_repo\n"), 0644); err != nil {
		t.Fatal(err)
	}
	follow := sourcewalk.Follow(root, sourcewalk.FollowOptions{})
	opts := DiscoveryOptions{Follow: &follow}
	skills, repos, err := DiscoverSourceSkillsLiteWithOptions(root, opts)
	if err != nil || len(skills) != 1 || len(repos) != 1 || repos[0] != "_repo" || skills[0].Targets != nil {
		t.Fatalf("Lite: %+v %v %v", skills, repos, err)
	}
	if err := os.WriteFile(filepath.Join(root, ".skillignore"), []byte("_repo/a\n"), 0644); err != nil {
		t.Fatal(err)
	}
	skills, err = DiscoverSourceSkillsAllWithOptions(root, opts)
	if err != nil || len(skills) != 1 || !skills[0].Disabled || len(skills[0].Targets) != 1 {
		t.Fatalf("All: %+v %v", skills, err)
	}
	skills, err = DiscoverSourceSkillsForAnalyzeWithOptions(root, opts)
	if err != nil || len(skills) != 1 || !skills[0].Disabled || skills[0].DescChars == 0 || skills[0].BodyChars == 0 {
		t.Fatalf("Analyze: %+v %v", skills, err)
	}
	skills, repos, err = DiscoverSourceSkillsLiteWithOptions(root, opts)
	if err != nil || len(skills) != 0 || len(repos) != 1 {
		t.Fatalf("ignored Lite: %+v %v %v", skills, repos, err)
	}
}
