package sourcewalk

import (
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

func followFixture(t *testing.T) (string, string) {
	t.Helper()
	root, ext := t.TempDir(), t.TempDir()
	for _, name := range []string{"a/file", "skip/file", "z"} {
		path := filepath.Join(ext, name)
		if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, nil, 0644); err != nil {
			t.Fatal(err)
		}
	}
	linkTo(t, ext, "nested", t.TempDir())
	linkTo(t, root, "group", ext)
	linkTo(t, root, "undeclared", t.TempDir())
	declare(t, root, "group")
	return root, ext
}

func TestFollowWalkLogicalOneHop(t *testing.T) {
	physicalRoot, _ := followFixture(t)
	root := filepath.Join(t.TempDir(), "source")
	linkTo(t, filepath.Dir(root), "source", physicalRoot)
	for _, dirWalk := range []bool{false, true} {
		set := Follow(root, FollowOptions{})
		var got []string
		collect := func(path string, dir bool, err error) error {
			if err != nil {
				return err
			}
			rel, err := filepath.Rel(root, path)
			if err != nil {
				return err
			}
			got = append(got, rel)
			if rel == "group" && !dir {
				t.Fatal("followed root not a directory")
			}
			if rel == "group/nested" && dir {
				t.Fatal("second hop followed")
			}
			return nil
		}
		var err error
		if dirWalk {
			err = WalkDir(root, Options{Follow: &set}, func(p string, d fs.DirEntry, e error) error { return collect(p, d != nil && d.IsDir(), e) })
		} else {
			err = Walk(root, Options{Follow: &set}, func(p string, d os.FileInfo, e error) error { return collect(p, d != nil && d.IsDir(), e) })
		}
		want := []string{".", ".skillfollow", "group", "group/a", "group/a/file", "group/nested", "group/skip", "group/skip/file", "group/z", "undeclared"}
		if err != nil || !reflect.DeepEqual(got, want) {
			t.Fatalf("dir=%v: %v %v", dirWalk, got, err)
		}
	}
	set := Follow(root, FollowOptions{})
	entries, err := ReadDir(root, Options{Follow: &set})
	if err != nil {
		t.Fatal(err)
	}
	for _, d := range entries {
		if d.Name() == "group" {
			info, err := d.Info()
			if err != nil || !d.IsDir() || !info.IsDir() || info.Name() != "group" {
				t.Fatalf("%v %v", d, err)
			}
		}
	}
}

func TestFollowWalkSkipParity(t *testing.T) {
	root, ext := followFixture(t)
	for _, skip := range []string{".", "a", "a/file", "skip", "z"} {
		for _, skipErr := range []error{filepath.SkipDir, filepath.SkipAll, errors.New("stop")} {
			for _, dirWalk := range []bool{false, true} {
				set := Follow(root, FollowOptions{})
				var want, got []string
				callback := func(base string, paths *[]string) func(string, error) error {
					return func(p string, err error) error {
						if err != nil {
							return err
						}
						rel, _ := filepath.Rel(base, p)
						*paths = append(*paths, rel)
						if rel == skip {
							return skipErr
						}
						return nil
					}
				}
				expected := callback(ext, &want)
				actual := callback(filepath.Join(root, "group"), &got)
				var wantErr, gotErr error
				if dirWalk {
					wantErr = filepath.WalkDir(ext, func(p string, _ fs.DirEntry, e error) error { return expected(p, e) })
					gotErr = WalkDir(root, Options{Follow: &set}, func(p string, _ fs.DirEntry, e error) error {
						if p == root || p == filepath.Join(root, ".skillfollow") || p == filepath.Join(root, "undeclared") {
							return e
						}
						return actual(p, e)
					})
				} else {
					wantErr = filepath.Walk(ext, func(p string, _ os.FileInfo, e error) error { return expected(p, e) })
					gotErr = Walk(root, Options{Follow: &set}, func(p string, _ os.FileInfo, e error) error {
						if p == root || p == filepath.Join(root, ".skillfollow") || p == filepath.Join(root, "undeclared") {
							return e
						}
						return actual(p, e)
					})
				}
				if !reflect.DeepEqual(got, want) || !errors.Is(gotErr, wantErr) {
					t.Fatalf("skip=%s err=%v dir=%v: %v %v; want %v %v", skip, skipErr, dirWalk, got, gotErr, want, wantErr)
				}
			}
		}
	}
}

func TestFollowWalkReadFailureMarksMissing(t *testing.T) {
	for _, dirWalk := range []bool{false, true} {
		root, ext := followFixture(t)
		set := Follow(root, FollowOptions{})
		sawError := false
		callback := func(p string, err error) error {
			if err != nil {
				sawError = true
				if pe, ok := err.(*os.PathError); !ok || pe.Path != p {
					t.Fatalf("nonlogical error: %v", err)
				}
				return nil
			}
			// Deterministically make a later subtree unreadable, even when tests run as root.
			if p == filepath.Join(root, "group", "a") {
				if err := os.RemoveAll(filepath.Join(ext, "skip")); err != nil {
					t.Fatal(err)
				}
			}
			return nil
		}
		var err error
		if dirWalk {
			err = WalkDir(root, Options{Follow: &set}, func(p string, _ fs.DirEntry, e error) error { return callback(p, e) })
		} else {
			err = Walk(root, Options{Follow: &set}, func(p string, _ os.FileInfo, e error) error { return callback(p, e) })
		}
		if err != nil || !sawError || len(set.Unavailable()) != 1 || set.Unavailable()[0].State != Missing || len(set.Followed()) != 0 {
			t.Fatalf("%v %v %+v", err, sawError, set.Entries())
		}
	}
}

func TestFollowSetDoesNotApplyDeclarationsToChildRoots(t *testing.T) {
	root, ext := followFixture(t)
	// The same name as a source declaration must not gain another hop when a
	// nested consumer reuses the operation's FollowSet at a child root.
	linkTo(t, ext, "group", t.TempDir())
	set := Follow(root, FollowOptions{})
	childRoot := filepath.Join(root, "group")
	for _, dirWalk := range []bool{false, true} {
		seen := false
		check := func(path string, isDir bool, err error) error {
			if err != nil {
				return err
			}
			if path == filepath.Join(childRoot, "group") {
				seen = true
				if isDir {
					t.Fatal("declaration reapplied below source root")
				}
			}
			return nil
		}
		var err error
		if dirWalk {
			err = WalkDir(childRoot, Options{Follow: &set}, func(path string, entry fs.DirEntry, err error) error {
				return check(path, entry != nil && entry.IsDir(), err)
			})
		} else {
			err = Walk(childRoot, Options{Follow: &set}, func(path string, info os.FileInfo, err error) error {
				return check(path, info != nil && info.IsDir(), err)
			})
		}
		if err != nil || !seen {
			t.Fatalf("seen=%v err=%v", seen, err)
		}
	}
	entries, err := ReadDir(childRoot, Options{Follow: &set})
	if err != nil {
		t.Fatal(err)
	}
	for _, entry := range entries {
		if entry.Name() == "group" && entry.IsDir() {
			t.Fatal("ReadDir reapplied declaration")
		}
	}
}
