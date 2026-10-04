package trash

import (
	"errors"
	"os"
	"path/filepath"
	"testing"

	"skillshare/internal/sourcefs"
)

// Restoring at or below a declared entry is refused whether its link is live
// or offline: offline, the restore would create the entry as a real directory
// and mask the external tree when it returns.
func TestRestore_RefusesDeclaredEntry(t *testing.T) {
	for _, name := range []string{"team", "team/skill"} {
		for _, live := range []bool{false, true} {
			t.Run(name+map[bool]string{false: "/missing", true: "/live"}[live], func(t *testing.T) {
				tmpDir := t.TempDir()
				trashBase := filepath.Join(tmpDir, "trash")
				destDir := filepath.Join(tmpDir, "skills")
				external := filepath.Join(tmpDir, "external")
				os.MkdirAll(filepath.Join(external, "a"), 0755)
				os.WriteFile(filepath.Join(external, "a", "SKILL.md"), []byte("# a"), 0644)
				os.MkdirAll(destDir, 0755)
				os.WriteFile(filepath.Join(destDir, ".skillfollow"), []byte("team\n"), 0644)
				if live {
					if err := os.Symlink(external, filepath.Join(destDir, "team")); err != nil {
						t.Fatal(err)
					}
				}

				srcDir := filepath.Join(tmpDir, "src", filepath.FromSlash(name))
				os.MkdirAll(srcDir, 0755)
				os.WriteFile(filepath.Join(srcDir, "SKILL.md"), []byte("# trashed"), 0644)
				if _, err := MoveToTrash(srcDir, name, trashBase); err != nil {
					t.Fatal(err)
				}
				entry := FindByName(trashBase, name)
				if entry == nil {
					t.Fatalf("expected to find %s in trash", name)
				}

				if err := Restore(entry, destDir); !errors.Is(err, sourcefs.ErrLink) {
					t.Fatalf("restore into a declared entry: got %v, want ErrLink", err)
				}
				if FindByName(trashBase, name) == nil {
					t.Fatal("trash entry should stay after a refused restore")
				}
				info, err := os.Lstat(filepath.Join(destDir, "team"))
				if !live {
					if !os.IsNotExist(err) {
						t.Fatalf("restore created the declared entry: %v", err)
					}
					return
				}
				if err != nil || info.Mode()&os.ModeSymlink == 0 {
					t.Fatalf("the declared link was replaced: %v %v", info, err)
				}
				if entries, _ := os.ReadDir(external); len(entries) != 1 {
					t.Fatalf("external tree changed: %v", entries)
				}
			})
		}
	}
}
