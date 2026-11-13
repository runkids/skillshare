package sync

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

func TestDiscoverExtraFiles_PreservesLockNamedContent(t *testing.T) {
	for _, kind := range []string{"nonempty", "nested", "directory", "symlink"} {
		t.Run(kind, func(t *testing.T) {
			root := t.TempDir()
			rel := ".skillshare-extras.lock"
			switch kind {
			case "nested":
				rel = filepath.Join("team", rel)
			case "directory":
				rel = filepath.Join(rel, "rules.md")
			}
			path := filepath.Join(root, rel)
			if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
				t.Fatal(err)
			}
			if kind == "symlink" {
				target := filepath.Join(root, "rules.md")
				if err := os.WriteFile(target, []byte("# Rules\n"), 0644); err != nil {
					t.Fatal(err)
				}
				if err := os.Symlink("rules.md", path); err != nil {
					t.Skipf("symlink unavailable: %v", err)
				}
			} else {
				var content []byte
				if kind != "nested" {
					content = []byte("# Rules\n")
				}
				if err := os.WriteFile(path, content, 0644); err != nil {
					t.Fatal(err)
				}
			}
			files, err := DiscoverExtraFiles(root)
			want := []string{rel}
			if kind == "symlink" {
				want = append(want, "rules.md")
			}
			if err != nil || !reflect.DeepEqual(files, want) {
				t.Fatalf("ordinary content hidden: files=%v want=%v err=%v", files, want, err)
			}
		})
	}
}
