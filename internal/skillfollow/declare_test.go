package skillfollow

import (
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"skillshare/internal/sourcefs"
)

func openSource(t *testing.T, files map[string]string) *sourcefs.Root {
	t.Helper()
	dir := t.TempDir()
	for name, content := range files {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(content), 0644); err != nil {
			t.Fatal(err)
		}
	}
	root, err := sourcefs.Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { root.Close() })
	return root
}

func readFile(t *testing.T, root *sourcefs.Root, name string) string {
	t.Helper()
	data, err := os.ReadFile(filepath.Join(root.Dir(), name))
	if err != nil {
		t.Fatal(err)
	}
	return string(data)
}

func TestAddEntry_PreservesCommentsAndOrder(t *testing.T) {
	root := openSource(t, map[string]string{File: "# team links\n_a\n\n# groups\ngroup"})
	added, err := AddEntry(root, File, "_new")
	if err != nil || !added {
		t.Fatalf("AddEntry = %v, %v", added, err)
	}
	if got, want := readFile(t, root, File), "# team links\n_a\n\n# groups\ngroup\n_new\n"; got != want {
		t.Fatalf("content = %q, want %q", got, want)
	}
}

func TestAddEntry_CreatesMissingFile(t *testing.T) {
	root := openSource(t, nil)
	if _, err := AddEntry(root, LocalFile, "_a"); err != nil {
		t.Fatal(err)
	}
	if got := readFile(t, root, LocalFile); got != "_a\n" {
		t.Fatalf("content = %q", got)
	}
}

func TestAddEntry_Idempotent(t *testing.T) {
	root := openSource(t, map[string]string{File: "  _a  \n"})
	added, err := AddEntry(root, File, "_a")
	if err != nil || added {
		t.Fatalf("AddEntry = %v, %v; want false, nil", added, err)
	}
	if got := readFile(t, root, File); got != "  _a  \n" {
		t.Fatalf("file changed: %q", got)
	}
}

func TestAddEntry_KeepsCRLF(t *testing.T) {
	root := openSource(t, map[string]string{File: "_a\r\n"})
	if _, err := AddEntry(root, File, "_b"); err != nil {
		t.Fatal(err)
	}
	if got := readFile(t, root, File); got != "_a\r\n_b\r\n" {
		t.Fatalf("content = %q", got)
	}
}

func TestRemoveEntry_KeepsOtherLines(t *testing.T) {
	root := openSource(t, map[string]string{File: "# keep\n_a\n_b\n\n_a\n# end\n"})
	removed, err := RemoveEntry(root, File, "_a")
	if err != nil || !removed {
		t.Fatalf("RemoveEntry = %v, %v", removed, err)
	}
	if got, want := readFile(t, root, File), "# keep\n_b\n\n# end\n"; got != want {
		t.Fatalf("content = %q, want %q", got, want)
	}
}

func TestRemoveEntry_NotDeclared(t *testing.T) {
	root := openSource(t, map[string]string{File: "# _a\n_b\n"})
	removed, err := RemoveEntry(root, File, "_a")
	if err != nil || removed {
		t.Fatalf("RemoveEntry = %v, %v; want false, nil", removed, err)
	}
	if removed, err := RemoveEntry(root, LocalFile, "_a"); err != nil || removed {
		t.Fatalf("missing file: RemoveEntry = %v, %v", removed, err)
	}
}

func TestDeclares(t *testing.T) {
	root := openSource(t, map[string]string{File: "# _x\n_a\n"})
	for name, want := range map[string]bool{"_a": true, "_x": false, "_b": false} {
		if got, err := Declares(root, File, name); err != nil || got != want {
			t.Errorf("Declares(%q) = %v, %v; want %v", name, got, err, want)
		}
	}
}

// A refused write must leave the old content and no temporary file behind.
func TestEntries_MatchCaseInsensitivelyWhenNamesFold(t *testing.T) {
	old := sameName
	sameName = strings.EqualFold // Windows semantics
	t.Cleanup(func() { sameName = old })

	root := openSource(t, map[string]string{File: "# team\nTeam\n"})
	if added, err := AddEntry(root, File, "team"); err != nil || added {
		t.Fatalf("AddEntry = %v, %v; want already declared", added, err)
	}
	if removed, err := RemoveEntry(root, File, "team"); err != nil || !removed {
		t.Fatalf("RemoveEntry = %v, %v", removed, err)
	}
	if got := readFile(t, root, File); got != "# team\n" {
		t.Fatalf("content = %q", got)
	}
}

func TestAddEntry_FailedWriteLeavesFileUntouched(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("symlinks need Developer Mode")
	}
	outside := filepath.Join(t.TempDir(), "elsewhere")
	if err := os.WriteFile(outside, []byte("_a\n"), 0644); err != nil {
		t.Fatal(err)
	}
	root := openSource(t, nil)
	if err := os.Symlink(outside, filepath.Join(root.Dir(), File)); err != nil {
		t.Fatal(err)
	}
	if _, err := AddEntry(root, File, "_b"); !errors.Is(err, sourcefs.ErrLink) {
		t.Fatalf("AddEntry error = %v, want ErrLink", err)
	}
	if data, _ := os.ReadFile(outside); string(data) != "_a\n" {
		t.Fatalf("link target changed: %q", data)
	}
	entries, err := os.ReadDir(root.Dir())
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 1 {
		t.Fatalf("leftover files: %v", entries)
	}
}

func TestIgnoreLine_AddAndRemove(t *testing.T) {
	root := openSource(t, map[string]string{IgnoreFile: "node_modules/\n/_a\n"})
	added, err := AddIgnoreLine(root, "/_a")
	if err != nil || !added {
		t.Fatalf("AddIgnoreLine = %v, %v", added, err)
	}
	if again, err := AddIgnoreLine(root, "/_a"); err != nil || again {
		t.Fatalf("second AddIgnoreLine = %v, %v; want false, nil", again, err)
	}
	want := "node_modules/\n/_a\n\n# BEGIN SKILLSHARE MANAGED - DO NOT EDIT\n/_a\n# END SKILLSHARE MANAGED\n"
	if got := readFile(t, root, IgnoreFile); got != want {
		t.Fatalf("content = %q, want %q", got, want)
	}
	removed, err := RemoveIgnoreLine(root, "/_a")
	if err != nil || !removed {
		t.Fatalf("RemoveIgnoreLine = %v, %v", removed, err)
	}
	// The user's own line outside the managed block stays.
	want = "node_modules/\n/_a\n\n# BEGIN SKILLSHARE MANAGED - DO NOT EDIT\n# END SKILLSHARE MANAGED\n"
	if got := readFile(t, root, IgnoreFile); got != want {
		t.Fatalf("content = %q, want %q", got, want)
	}
}
