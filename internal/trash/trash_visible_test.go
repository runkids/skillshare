package trash

import (
	"os"
	"path/filepath"
	"testing"
)

func markVisibleProject(t *testing.T, root string) string {
	t.Helper()
	dir := filepath.Join(root, "skillshare")
	if err := os.MkdirAll(dir, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "config.yaml"), []byte("targets: []\n"), 0644); err != nil {
		t.Fatal(err)
	}
	return dir
}

func TestProjectTrashDir_FollowsProjectDirectory(t *testing.T) {
	t.Run("hidden default", func(t *testing.T) {
		root := t.TempDir()
		if got, want := ProjectTrashDir(root), filepath.Join(root, ".skillshare", "trash"); got != want {
			t.Errorf("ProjectTrashDir() = %q, want %q", got, want)
		}
	})

	t.Run("visible directory", func(t *testing.T) {
		root := t.TempDir()
		dir := markVisibleProject(t, root)
		if got, want := ProjectTrashDir(root), filepath.Join(dir, "trash"); got != want {
			t.Errorf("ProjectTrashDir() = %q, want %q", got, want)
		}
		if got, want := ProjectAgentTrashDir(root), filepath.Join(dir, "trash", "agents"); got != want {
			t.Errorf("ProjectAgentTrashDir() = %q, want %q", got, want)
		}
	})
}
