package sourcewalk

import (
	"os"
	"path/filepath"
	"reflect"
	"runtime"
	"strings"
	"testing"
)

func TestParseDeclarations(t *testing.T) {
	root := t.TempDir()
	invalid := []string{".", "..", "a/b", `a\b`, "/absolute", "C:", "C:relative", `\\server\share`, "a/../b", "*", "a?", "[ab]", "!a", "a{b,c}"}
	base := "  # comment\n\n alpha \nalpha\n" + strings.Join(invalid, "\n")
	if err := os.WriteFile(filepath.Join(root, ".skillfollow"), []byte(base), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, ".skillfollow.local"), []byte("alpha\nbeta\n"), 0644); err != nil {
		t.Fatal(err)
	}
	got := readDeclarations(root)
	if !reflect.DeepEqual(got.names, []string{"alpha", "beta"}) || !got.local || !got.active {
		t.Fatalf("%+v", got)
	}
	if len(got.warnings) != len(invalid) {
		t.Fatalf("warnings: %v", got.warnings)
	}
	for _, warning := range got.warnings {
		if !strings.Contains(warning, ".skillfollow:") {
			t.Fatal(warning)
		}
	}
}

func TestParseMissingDeclarations(t *testing.T) {
	got := readDeclarations(t.TempDir())
	if got.active || got.local || len(got.names) != 0 || len(got.warnings) != 0 {
		t.Fatalf("%+v", got)
	}
}

func TestFollowUnreadableDeclaration(t *testing.T) {
	if runtime.GOOS == "windows" || os.Geteuid() == 0 {
		t.Skip("requires Unix permission enforcement")
	}
	for _, file := range []string{".skillfollow", ".skillfollow.local"} {
		t.Run(file, func(t *testing.T) {
			root := t.TempDir()
			path := filepath.Join(root, file)
			if err := os.WriteFile(path, []byte("group\n"), 0000); err != nil {
				t.Fatal(err)
			}
			t.Cleanup(func() { _ = os.Chmod(path, 0600) })
			set := Follow(root, FollowOptions{})
			if err := set.Err(); err == nil || !strings.Contains(err.Error(), file) {
				t.Fatalf("Err() = %v; want declaration read error", err)
			}
			if err := WalkDir(root, Options{Follow: &set}, func(_ string, _ os.DirEntry, _ error) error {
				t.Fatal("incomplete declarations must prevent traversal")
				return nil
			}); err == nil {
				t.Fatal("WalkDir accepted incomplete declarations")
			}
		})
	}
}
