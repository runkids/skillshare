package install

import (
	"os"
	"path/filepath"
	"testing"

	"skillshare/internal/sourcewalk"
)

func TestGetTrackedReposWithFollow(t *testing.T) {
	root, ext := t.TempDir(), t.TempDir()
	if err := os.Mkdir(filepath.Join(ext, ".git"), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(ext, filepath.Join(root, "_repo")); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, ".skillfollow"), []byte("_repo\n"), 0644); err != nil {
		t.Fatal(err)
	}
	legacy, err := GetTrackedRepos(root)
	if err != nil || len(legacy) != 0 {
		t.Fatalf("%v %v", legacy, err)
	}
	set := sourcewalk.Follow(root, sourcewalk.FollowOptions{})
	repos, err := GetTrackedReposWithOptions(root, sourcewalk.Options{Follow: &set})
	if err != nil || len(repos) != 1 || repos[0] != "_repo" {
		t.Fatalf("%v %v", repos, err)
	}
}
