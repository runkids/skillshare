package sourcewalk

import (
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"slices"
	"testing"
)

// eachNameSemantics runs fn with Unix (exact) and Windows (case-folded) entry
// name matching, so both are covered on every host.
func eachNameSemantics(t *testing.T, fn func(t *testing.T, folded bool)) {
	for _, folded := range []bool{false, true} {
		t.Run(map[bool]string{false: "exact", true: "folded"}[folded], func(t *testing.T) {
			old := caseInsensitiveNames
			caseInsensitiveNames = folded
			t.Cleanup(func() { caseInsensitiveNames = old })
			fn(t, folded)
		})
	}
}

func TestParseDeclarationsFoldsCaseInsensitiveNames(t *testing.T) {
	eachNameSemantics(t, func(t *testing.T, folded bool) {
		root := t.TempDir()
		declare(t, root, "Team", "team")
		want := []string{"Team", "team"}
		if folded {
			want = []string{"Team"}
		}
		if got := readDeclarations(root).names; !reflect.DeepEqual(got, want) {
			t.Fatalf("got %v, want %v", got, want)
		}
	})
}

// An offline declaration `Team` is the boundary for a write at `team` on Windows.
func TestInFollowedMatchesDeclaredNameCase(t *testing.T) {
	eachNameSemantics(t, func(t *testing.T, folded bool) {
		root := t.TempDir()
		declare(t, root, "Team")
		set := Follow(root, FollowOptions{})
		if _, ok := set.InFollowed("team/skill"); ok != folded {
			t.Fatalf("InFollowed(team/skill) = %v", ok)
		}
		if err := set.WriteBoundary(root, "team"); (err != nil) != folded {
			t.Fatalf("WriteBoundary(team) = %v", err)
		}
	})
}

func TestUndeclaredLinkMatchesDeclaredNameCase(t *testing.T) {
	eachNameSemantics(t, func(t *testing.T, folded bool) {
		root := t.TempDir()
		linkTo(t, root, "team", t.TempDir())
		declare(t, root, "Team")
		set := Follow(root, FollowOptions{})
		undeclared := slices.ContainsFunc(set.Entries(), func(e Entry) bool { return e.State == UndeclaredLink })
		if undeclared == folded {
			t.Fatalf("entries: %+v", set.Entries())
		}
	})
}

// followedCaseSet builds the snapshot Windows produces for a declaration `Team`
// whose on-disk link is spelled `team`; Unix hosts cannot classify it so.
func followedCaseSet(t *testing.T) (string, *FollowSet) {
	t.Helper()
	root, ext := t.TempDir(), t.TempDir()
	if err := os.MkdirAll(filepath.Join(ext, "skill"), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(ext, "skill", "SKILL.md"), nil, 0644); err != nil {
		t.Fatal(err)
	}
	linkTo(t, root, "team", ext)
	canonicalRoot, err := Canonicalize(root)
	if err != nil {
		t.Fatal(err)
	}
	target, err := Canonicalize(ext)
	if err != nil {
		t.Fatal(err)
	}
	return root, &FollowSet{canonicalRoot: canonicalRoot, entries: []Entry{{Name: "Team", State: Followed, ResolvedTarget: target}}}
}

func TestFollowWalkMatchesDeclaredNameCase(t *testing.T) {
	eachNameSemantics(t, func(t *testing.T, folded bool) {
		root, set := followedCaseSet(t)
		var got []string
		err := Walk(root, Options{Follow: set}, func(path string, _ os.FileInfo, err error) error {
			rel, _ := filepath.Rel(root, path)
			got = append(got, filepath.ToSlash(rel))
			return err
		})
		if err != nil {
			t.Fatal(err)
		}
		if slices.Contains(got, "team/skill/SKILL.md") != folded {
			t.Fatalf("walked %v", got)
		}
		entries, err := ReadDir(root, Options{Follow: set})
		if err != nil || len(entries) != 1 || entries[0].IsDir() != folded {
			t.Fatalf("ReadDir: %v %v", entries, err)
		}
	})
}

func TestMarkMissingMatchesDeclaredNameCase(t *testing.T) {
	eachNameSemantics(t, func(t *testing.T, folded bool) {
		_, set := followedCaseSet(t)
		set.markMissing("team", errors.New("unreadable"))
		if missing := set.Entries()[0].State == Missing; missing != folded {
			t.Fatalf("entries: %+v", set.Entries())
		}
	})
}
