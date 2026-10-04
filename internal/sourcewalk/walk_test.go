package sourcewalk

import (
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

func fixture(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	for _, name := range []string{"a/child", "skip/child", "z"} {
		path := filepath.Join(root, name)
		if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(name), 0644); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.Symlink(filepath.Join(root, "a"), filepath.Join(root, "link")); err != nil {
		t.Fatal(err)
	}
	return root
}

func TestWalkParity(t *testing.T) {
	root := fixture(t)
	for _, skip := range []string{"", "skip", "z", "."} {
		t.Run("skip="+skip, func(t *testing.T) {
			collect := func(paths *[]string) filepath.WalkFunc {
				return func(path string, info os.FileInfo, err error) error {
					if err != nil {
						return err
					}
					rel, _ := filepath.Rel(root, path)
					*paths = append(*paths, rel+":"+info.Mode().String())
					if rel == skip {
						return filepath.SkipDir
					}
					return nil
				}
			}
			var want, got []string
			wantErr := filepath.Walk(root, collect(&want))
			gotErr := Walk(root, Options{}, collect(&got))
			if !reflect.DeepEqual(got, want) || !errors.Is(gotErr, wantErr) {
				t.Fatalf("got %v, %v; want %v, %v", got, gotErr, want, wantErr)
			}
		})
	}
}

func TestWalkDirParity(t *testing.T) {
	root := fixture(t)
	for _, skip := range []string{"", "skip", "z", "."} {
		t.Run("skip="+skip, func(t *testing.T) {
			collect := func(paths *[]string) fs.WalkDirFunc {
				return func(path string, entry fs.DirEntry, err error) error {
					if err != nil {
						return err
					}
					rel, _ := filepath.Rel(root, path)
					*paths = append(*paths, rel+":"+entry.Type().String())
					if rel == skip {
						return filepath.SkipDir
					}
					return nil
				}
			}
			var want, got []string
			wantErr := filepath.WalkDir(root, collect(&want))
			gotErr := WalkDir(root, Options{}, collect(&got))
			if !reflect.DeepEqual(got, want) || !errors.Is(gotErr, wantErr) {
				t.Fatalf("got %v, %v; want %v, %v", got, gotErr, want, wantErr)
			}
		})
	}
}

func TestSymlinkRoot(t *testing.T) {
	root := fixture(t)
	link := filepath.Join(t.TempDir(), "source")
	if err := os.Symlink(root, link); err != nil {
		t.Fatal(err)
	}
	var walkPaths, dirPaths []string
	if err := Walk(link, Options{}, func(path string, _ os.FileInfo, err error) error { walkPaths = append(walkPaths, path); return err }); err != nil {
		t.Fatal(err)
	}
	if err := WalkDir(link, Options{}, func(path string, _ fs.DirEntry, err error) error { dirPaths = append(dirPaths, path); return err }); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(walkPaths, dirPaths) || len(walkPaths) != 7 || walkPaths[0] != root {
		t.Fatalf("unexpected paths: %v / %v", walkPaths, dirPaths)
	}
	got, err := ReadDir(link, Options{})
	if err != nil {
		t.Fatal(err)
	}
	want, err := os.ReadDir(root)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != len(want) {
		t.Fatalf("entry count: %d != %d", len(got), len(want))
	}
	for i, entry := range got {
		if entry.Name() != want[i].Name() || entry.Type() != want[i].Type() {
			t.Fatalf("entry %d: %v != %v", i, entry, want[i])
		}
	}
	if len(got) != len(want) {
		t.Fatalf("entry count: %d != %d", len(got), len(want))
	}
}

func TestWalkErrors(t *testing.T) {
	sentinel := errors.New("callback failure")
	root := fixture(t)
	if err := Walk(root, Options{}, func(string, os.FileInfo, error) error { return sentinel }); err != sentinel {
		t.Fatal(err)
	}
	if err := WalkDir(root, Options{}, func(string, fs.DirEntry, error) error { return sentinel }); err != sentinel {
		t.Fatal(err)
	}
	missing := filepath.Join(root, "missing")
	for _, swallow := range []bool{false, true} {
		check := func(path string, err error) error {
			if path != missing || !os.IsNotExist(err) {
				t.Fatalf("unexpected callback: %s, %v", path, err)
			}
			if swallow {
				return nil
			}
			return err
		}
		err := Walk(missing, Options{}, func(path string, info os.FileInfo, err error) error {
			if info != nil {
				t.Fatal(info)
			}
			return check(path, err)
		})
		if (err == nil) != swallow {
			t.Fatalf("Walk: %v", err)
		}
		err = WalkDir(missing, Options{}, func(path string, entry fs.DirEntry, err error) error {
			if entry != nil {
				t.Fatal(entry)
			}
			return check(path, err)
		})
		if (err == nil) != swallow {
			t.Fatalf("WalkDir: %v", err)
		}
	}
	if _, err := ReadDir(missing, Options{}); !os.IsNotExist(err) {
		t.Fatal(err)
	}
}

func TestReadDirLogicalErrorPaths(t *testing.T) {
	root := fixture(t)
	link := filepath.Join(t.TempDir(), "source")
	if err := os.Symlink(root, link); err != nil {
		t.Fatal(err)
	}
	got, err := ReadDir(link, Options{})
	if err != nil {
		t.Fatal(err)
	}
	want, err := os.ReadDir(link)
	if err != nil {
		t.Fatal(err)
	}
	// A removed file makes Info report an error after the directory was read.
	if err := os.Remove(filepath.Join(root, "z")); err != nil {
		t.Fatal(err)
	}
	_, gotErr := got[len(got)-1].Info()
	_, wantErr := want[len(want)-1].Info()
	if !reflect.DeepEqual(gotErr, wantErr) {
		t.Fatalf("Info error: %v != %v", gotErr, wantErr)
	}
	// A link to a file is resolved successfully but cannot be read as a directory.
	fileLink := filepath.Join(t.TempDir(), "file")
	if err := os.Symlink(filepath.Join(root, "a", "child"), fileLink); err != nil {
		t.Fatal(err)
	}
	_, gotErr = ReadDir(fileLink, Options{})
	_, wantErr = os.ReadDir(fileLink)
	if !reflect.DeepEqual(gotErr, wantErr) {
		t.Fatalf("ReadDir error: %v != %v", gotErr, wantErr)
	}
}
