package backup

import (
	"os"
	"path/filepath"
	"testing"
)

func TestProjectBackupDir_FollowsProjectDirectory(t *testing.T) {
	t.Run("hidden default", func(t *testing.T) {
		root := t.TempDir()
		if got, want := ProjectBackupDir(root), filepath.Join(root, ".skillshare", "backups"); got != want {
			t.Errorf("ProjectBackupDir() = %q, want %q", got, want)
		}
	})

	t.Run("visible directory", func(t *testing.T) {
		root := t.TempDir()
		dir := filepath.Join(root, "skillshare")
		if err := os.MkdirAll(dir, 0755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(dir, "config.yaml"), []byte("targets: []\n"), 0644); err != nil {
			t.Fatal(err)
		}

		if got, want := ProjectBackupDir(root), filepath.Join(dir, "backups"); got != want {
			t.Errorf("ProjectBackupDir() = %q, want %q", got, want)
		}
	})
}
