package sourcewalk

import (
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"testing"
)

// simulatedJunction makes a regular file at root/name read as a junction to
// target, the way Go 1.23+ reports one on Windows (not a symlink, not a dir).
func simulatedJunction(t *testing.T, root, name, target string) linkOps {
	t.Helper()
	junction := filepath.Join(root, name)
	if err := os.WriteFile(junction, nil, 0644); err != nil {
		t.Fatal(err)
	}
	links := platformLinks()
	isLink, resolve := links.isLink, links.resolve
	links.isLink = func(path string, mode fs.FileMode) bool { return path == junction || isLink(path, mode) }
	links.resolve = func(path string) (string, error) {
		if path == junction {
			return target, nil
		}
		return resolve(path)
	}
	return links
}

// skillFiles walks root with the set and returns the SKILL.md paths it reports.
func skillFiles(t *testing.T, root string, set *FollowSet) []string {
	t.Helper()
	var found []string
	err := Walk(root, Options{Follow: set}, func(path string, info os.FileInfo, err error) error {
		if err == nil && info.Name() == "SKILL.md" {
			found = append(found, path)
		}
		return err
	})
	if err != nil {
		t.Fatal(err)
	}
	return found
}

func TestJunctionInSourceIsDiscoveredOnlyWhenDeclared(t *testing.T) {
	root, ext := t.TempDir(), t.TempDir()
	if err := os.MkdirAll(filepath.Join(ext, "a"), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(ext, "a", "SKILL.md"), []byte("# a"), 0644); err != nil {
		t.Fatal(err)
	}
	links := simulatedJunction(t, root, "_j", ext)

	inactive := follow(root, FollowOptions{}, links)
	if found := skillFiles(t, root, &inactive); len(found) != 0 {
		t.Fatalf("undeclared junction was traversed: %v", found)
	}

	declare(t, root, "other")
	undeclared := follow(root, FollowOptions{}, links)
	if entries := undeclared.Entries(); !slices.ContainsFunc(entries, func(e Entry) bool { return e.Name == "_j" && e.State == UndeclaredLink }) {
		t.Fatalf("undeclared junction: %+v", entries)
	}
	if found := skillFiles(t, root, &undeclared); len(found) != 0 {
		t.Fatalf("undeclared junction was traversed: %v", found)
	}

	declare(t, root, "_j")
	declared := follow(root, FollowOptions{}, links)
	want := filepath.Join(root, "_j", "a", "SKILL.md")
	if found := skillFiles(t, root, &declared); len(found) != 1 || found[0] != want {
		t.Fatalf("declared junction: found %v, want %s", found, want)
	}
}
