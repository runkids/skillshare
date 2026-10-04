package main

import (
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"skillshare/internal/sourcefs"
	"skillshare/internal/sourcewalk"
	"skillshare/internal/testutil"
)

// On Windows `follow team` finds the link declared as `Team`, so the ignore
// line follow adds is the declared one. The comparison is swapped because a
// case-sensitive filesystem cannot hold both spellings as one entry.
func TestFollowIgnores_UsesDeclaredSpellingWhenNamesFold(t *testing.T) {
	old := sameFollowName
	sameFollowName = strings.EqualFold // Windows semantics
	t.Cleanup(func() { sameFollowName = old })

	source := t.TempDir()
	testutil.RunGit(t, source, "init", "-q")
	if err := os.Symlink(t.TempDir(), filepath.Join(source, "Team")); err != nil {
		t.Skip(err)
	}
	if err := os.WriteFile(filepath.Join(source, ".skillfollow"), []byte("Team\n"), 0644); err != nil {
		t.Fatal(err)
	}
	root, err := sourcefs.Open(source)
	if err != nil {
		t.Fatal(err)
	}
	defer root.Close()

	set := sourcewalk.Follow(source, sourcewalk.FollowOptions{})
	r := &followResult{}
	if err := followIgnores(root, source, &set, followOptions{name: "team"}, r); err != nil {
		t.Fatal(err)
	}
	if want := []string{"/Team"}; !reflect.DeepEqual(r.IgnoreLines, want) {
		t.Fatalf("ignore lines = %q, want %q", r.IgnoreLines, want)
	}
}
