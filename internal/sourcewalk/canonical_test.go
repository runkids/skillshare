package sourcewalk

import (
	"io/fs"
	"os"
	"path/filepath"
	"testing"
)

func TestJunctionSimulation(t *testing.T) {
	root, ext := t.TempDir(), t.TempDir()
	junction := filepath.Join(root, "junction")
	if err := os.WriteFile(junction, nil, 0644); err != nil {
		t.Fatal(err)
	}
	declare(t, root, "junction")
	links := platformLinks()
	platformCheck, platformResolve := links.isLink, links.resolve
	links.isLink = func(path string, mode fs.FileMode) bool { return path == junction || platformCheck(path, mode) }
	links.resolve = func(path string) (string, error) {
		if path == junction {
			return ext, nil
		}
		return platformResolve(path)
	}
	set := follow(root, FollowOptions{}, links)
	if len(set.Followed()) != 1 || set.Followed()[0].ResolvedTarget != ext {
		t.Fatalf("%+v", set.Entries())
	}
	got, err := canonicalize(filepath.Join(junction, "missing", "child"), links, 0)
	if err != nil || got != filepath.Join(ext, "missing", "child") {
		t.Fatalf("%s %v", got, err)
	}
	// Canonical target boundaries also cross the simulated junction.
	set = follow(root, FollowOptions{TargetPaths: []string{filepath.Join(junction, "missing")}}, links)
	if set.Entries()[0].State != TargetOverlap {
		t.Fatalf("%+v", set.Entries())
	}
}

func TestCanonicalizeLinkLoop(t *testing.T) {
	root := t.TempDir()
	linkTo(t, root, "a", filepath.Join(root, "b"))
	linkTo(t, root, "b", filepath.Join(root, "a"))
	if _, err := Canonicalize(filepath.Join(root, "a")); err == nil {
		t.Fatal("expected bounded loop failure")
	}
}
