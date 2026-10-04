package sourcewalk

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"skillshare/internal/sourcefs"
)

func declare(t *testing.T, root string, names ...string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(root, ".skillfollow"), []byte(strings.Join(names, "\n")), 0644); err != nil {
		t.Fatal(err)
	}
}
func linkTo(t *testing.T, root, name, target string) {
	t.Helper()
	if err := os.Symlink(target, filepath.Join(root, name)); err != nil {
		t.Fatal(err)
	}
}
func TestFollowClassification(t *testing.T) {
	for _, state := range []State{Missing, NotLink, InvalidTarget, Cycle, TargetOverlap, InsideGitRoot, EntryOverlap, SingleSkill, Followed} {
		t.Run(string(state), func(t *testing.T) {
			root, ext := t.TempDir(), t.TempDir()
			declare(t, root, "a")
			opts := FollowOptions{}
			switch state {
			case Missing:
				ext = filepath.Join(ext, "absent")
			case NotLink:
				if err := os.Mkdir(filepath.Join(root, "a"), 0755); err != nil {
					t.Fatal(err)
				}
			case InvalidTarget:
				ext = filepath.Join(ext, "file")
				if err := os.WriteFile(ext, nil, 0644); err != nil {
					t.Fatal(err)
				}
			case Cycle:
				ext = root
			case TargetOverlap:
				opts.TargetPaths = []string{filepath.Dir(ext)}
			case InsideGitRoot:
				opts.GitRoot = filepath.Dir(ext)
			case EntryOverlap:
				declare(t, root, "a", "b")
				linkTo(t, root, "b", ext)
			case SingleSkill:
				if err := os.WriteFile(filepath.Join(ext, "SKILL.md"), nil, 0644); err != nil {
					t.Fatal(err)
				}
			}
			if state != NotLink {
				linkTo(t, root, "a", ext)
			}
			set := Follow(root, opts)
			entries := set.Entries()
			if entries[0].State != state {
				t.Fatalf("%+v", entries)
			}
			if state == EntryOverlap && (entries[1].State != state || !strings.Contains(entries[0].Reason, "b")) {
				t.Fatalf("%+v", entries)
			}
		})
	}
}
func TestFollowSafetyPrecedence(t *testing.T) {
	root, ext := t.TempDir(), t.TempDir()
	target := filepath.Join(ext, "target")
	if err := os.MkdirAll(filepath.Join(target, "a", ".git"), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(target, "a", "SKILL.md"), nil, 0644); err != nil {
		t.Fatal(err)
	}
	linkTo(t, root, "_a", filepath.Join(target, "a"))
	declare(t, root, "_a")
	for _, tc := range []struct {
		opts FollowOptions
		want State
	}{
		{FollowOptions{TargetPaths: []string{target}, GitRoot: ext}, TargetOverlap},
		{FollowOptions{TargetPaths: []string{filepath.Join(target, "a", "missing")}}, TargetOverlap},
		{FollowOptions{GitRoot: ext}, InsideGitRoot},
		{FollowOptions{}, SingleSkill},
	} {
		set := Follow(root, tc.opts)
		if set.Entries()[0].State != tc.want {
			t.Fatalf("%+v", set.Entries())
		}
	}
}
func TestCanonicalizeMissingAncestor(t *testing.T) {
	root, ext := t.TempDir(), t.TempDir()
	linkTo(t, root, "alias", ext)
	got, err := Canonicalize(filepath.Join(root, "alias", "missing", "child"))
	if err != nil || got != filepath.Join(ext, "missing", "child") {
		t.Fatalf("%s %v", got, err)
	}
}
func TestFollowQueries(t *testing.T) {
	root, ext := t.TempDir(), t.TempDir()
	linkTo(t, root, "group", ext)
	linkTo(t, root, "hidden", t.TempDir())
	declare(t, root, "group", "missing")
	set := Follow(root, FollowOptions{})
	if len(set.Followed()) != 1 || len(set.Unavailable()) != 1 || !set.Owns(filepath.Join(ext, "child")) || set.Owns(ext+"-other") {
		t.Fatalf("%+v", set)
	}
	if entry, ok := set.InFollowed("group/child"); !ok || entry.Name != "group" {
		t.Fatalf("%+v %v", entry, ok)
	}
	if _, ok := set.InFollowed("groupish/child"); ok {
		t.Fatal("prefix boundary")
	}
	if _, ok := set.InFollowed("../group/child"); ok {
		t.Fatal("escape")
	}
	entries := set.Entries()
	entries[0].State = Missing
	if set.Followed()[0].State != Followed {
		t.Fatal("Entries aliases state")
	}
}

func TestFollowOverlapChain(t *testing.T) {
	root, ext := t.TempDir(), t.TempDir()
	for _, name := range []string{"left", "right"} {
		if err := os.Mkdir(filepath.Join(ext, name), 0755); err != nil {
			t.Fatal(err)
		}
	}
	declare(t, root, "a", "b", "c")
	linkTo(t, root, "a", filepath.Join(ext, "left"))
	linkTo(t, root, "b", ext)
	linkTo(t, root, "c", filepath.Join(ext, "right"))
	set := Follow(root, FollowOptions{})
	for _, e := range set.Entries() {
		if e.State != EntryOverlap {
			t.Fatalf("%+v", set.Entries())
		}
	}
}

func TestFollowCyclePrecedesOtherSafetyRules(t *testing.T) {
	root := t.TempDir()
	child := filepath.Join(root, "dir")
	if err := os.Mkdir(child, 0755); err != nil {
		t.Fatal(err)
	}
	declare(t, root, "a", "b", "c")
	linkTo(t, root, "a", root)
	linkTo(t, root, "b", child)
	linkTo(t, root, "c", filepath.Dir(root))
	set := Follow(root, FollowOptions{TargetPaths: []string{root}, GitRoot: filepath.Dir(root)})
	for _, e := range set.Entries() {
		if e.State != Cycle {
			t.Fatalf("%+v", set.Entries())
		}
	}
}

func TestFollowDanglingAndRegularFile(t *testing.T) {
	root := t.TempDir()
	declare(t, root, "dangling", "file")
	linkTo(t, root, "dangling", filepath.Join(t.TempDir(), "missing"))
	if err := os.WriteFile(filepath.Join(root, "file"), nil, 0644); err != nil {
		t.Fatal(err)
	}
	set := Follow(root, FollowOptions{})
	if set.Entries()[0].State != Missing || set.Entries()[1].State != InvalidTarget {
		t.Fatalf("%+v", set.Entries())
	}
}

func TestWriteBoundary(t *testing.T) {
	root := t.TempDir()
	var none *FollowSet
	if err := none.WriteBoundary(root, "team"); err != nil {
		t.Errorf("nil set: %v", err)
	}
	if err := os.WriteFile(filepath.Join(root, ".skillfollow"), []byte("team\n"), 0644); err != nil {
		t.Fatal(err)
	}
	declared := Follow(root, FollowOptions{})
	if err := declared.WriteBoundary(root, "team/sub"); !errors.Is(err, sourcefs.ErrLink) || !strings.Contains(err.Error(), filepath.Join(root, "team")) {
		t.Errorf("declared entry: got %v, want link error naming team", err)
	}
	if err := declared.WriteBoundary(root, "keep"); err != nil {
		t.Errorf("undeclared name: %v", err)
	}
	if err := os.Remove(filepath.Join(root, ".skillfollow")); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(filepath.Join(root, ".skillfollow"), 0755); err != nil {
		t.Fatal(err)
	}
	unreadable := Follow(root, FollowOptions{})
	if err := unreadable.WriteBoundary(root, "keep"); err == nil || !strings.Contains(err.Error(), "read skillfollow declaration") {
		t.Errorf("unreadable declaration: got %v, want the read error", err)
	}
}
