package projects

import (
	"maps"
	"os"
	"path/filepath"
	"runtime"
	"slices"
	"testing"

	"skillshare/internal/config"
)

func TestScanCheckoutsSkipsLinks(t *testing.T) {
	root := t.TempDir()
	outside := t.TempDir()
	for _, dir := range []string{filepath.Join(root, "app", ".git"), filepath.Join(outside, "other", ".git"), filepath.Join(root, "linked-git")} {
		if err := os.MkdirAll(dir, 0755); err != nil {
			t.Fatal(err)
		}
	}
	// A linked checkout folder and a linked .git could both point at another checkout.
	if err := os.Symlink(filepath.Join(outside, "other"), filepath.Join(root, "linked")); err != nil {
		t.Skip("symlinks unavailable:", err)
	}
	if err := os.Symlink(filepath.Join(outside, "other", ".git"), filepath.Join(root, "linked-git", ".git")); err != nil {
		t.Fatal(err)
	}
	found, _, err := ScanCheckouts(root, []string{".", "app"})
	if err != nil {
		t.Fatal(err)
	}
	if !slices.Equal(found, []string{"app"}) {
		t.Fatalf("found %v, want [app]", found)
	}
	if _, _, err := ScanCheckouts(root, []string{"linked"}); err == nil {
		t.Fatal("a linked scan folder was accepted")
	}
}

func TestScanCheckoutsThroughLinkedRoot(t *testing.T) {
	real := filepath.Join(t.TempDir(), "suite")
	if err := os.MkdirAll(filepath.Join(real, "app", ".git"), 0755); err != nil {
		t.Fatal(err)
	}
	link := filepath.Join(t.TempDir(), "suite")
	if err := os.Symlink(real, link); err != nil {
		t.Skip("symlinks unavailable:", err)
	}
	found, _, err := ScanCheckouts(link, []string{"."})
	if err != nil {
		t.Fatal(err)
	}
	if !slices.Equal(found, []string{"app"}) {
		t.Fatalf("found %v, want [app]", found)
	}
}

func TestPlanScanNamesClashingCheckoutsFromTheirPath(t *testing.T) {
	home := t.TempDir()
	root := filepath.Join(home, "suite")
	cfg := &config.Config{Projects: map[string]config.ManagedProject{
		filepath.Join(home, "other", "worktrees-app-b"): {Targets: []string{"codex"}},
	}}
	entry := config.ManagedProject{Targets: []string{"codex"}}
	// Sorted as a scan returns them: the worktrees come before their checkouts.
	found := []string{".worktrees/app-a", ".worktrees/app-b", "app-a", "app-b"}
	rows, adds, err := planScan(cfg, root, entry, found, "", home, home)
	if err != nil {
		t.Fatal(err)
	}
	names := map[string]string{}
	for _, row := range rows {
		rel, _ := filepath.Rel(root, row.Root)
		names[filepath.ToSlash(rel)] = row.Result + ":" + row.Name
	}
	want := map[string]string{".": "added:suite", "app-a": "added:app-a", "app-b": "added:app-b", ".worktrees/app-a": "added:worktrees-app-a", ".worktrees/app-b": "skipped:worktrees-app-b"}
	if !maps.Equal(names, want) {
		t.Fatalf("rows %v, want %v", names, want)
	}
	if adds[filepath.Join(root, "app-a")].Name != "" || adds[filepath.Join(root, ".worktrees", "app-a")].Name != "worktrees-app-a" {
		t.Fatalf("saved names: %+v", adds)
	}
}

func TestPlanScanSkipsCheckoutWithItsOwnSkillshare(t *testing.T) {
	root := t.TempDir()
	// A clone of a shared skills repository: config.yaml is gitignored.
	if err := os.MkdirAll(filepath.Join(root, "tools", ".skillshare", "skills"), 0755); err != nil {
		t.Fatal(err)
	}
	rows, adds, err := planScan(&config.Config{}, root, config.ManagedProject{Targets: []string{"codex"}}, []string{"tools"}, "", root, root)
	if err != nil {
		t.Fatal(err)
	}
	if len(adds) != 1 || rows[1].Result != ScanSkipped {
		t.Fatalf("rows %+v, adds %v", rows, adds)
	}
}

func TestScanCheckoutsSkipsUnreadableFolders(t *testing.T) {
	if os.Geteuid() == 0 || runtime.GOOS == "windows" {
		t.Skip("test requires POSIX directory permissions and a non-root user")
	}
	root := t.TempDir()
	for _, dir := range []string{filepath.Join(root, "app", ".git"), filepath.Join(root, "private", "secret", ".git")} {
		if err := os.MkdirAll(dir, 0755); err != nil {
			t.Fatal(err)
		}
	}
	private := filepath.Join(root, "private")
	if err := os.Chmod(private, 0); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chmod(private, 0755) })
	// The scan folder itself sits below the unreadable one.
	found, warnings, err := ScanCheckouts(root, []string{".", "private/secret"})
	if err != nil {
		t.Fatal(err)
	}
	if !slices.Equal(found, []string{"app"}) || len(warnings) != 2 || warnings[0].Folder != "private" || warnings[1].Folder != "private/secret" {
		t.Fatalf("found %v, warnings %v", found, warnings)
	}
}
