package audit

import (
	"os"
	"path/filepath"
	"testing"
)

func TestScanSkill_PreservesLockNamedContentPolicy(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	t.Setenv("SKILLSHARE_CONFIG", filepath.Join(t.TempDir(), "config.yaml"))
	ResetGlobalCache()
	t.Cleanup(ResetGlobalCache)
	for _, kind := range []string{"nonempty", "nested", "directory", "symlink"} {
		t.Run(kind, func(t *testing.T) {
			root := t.TempDir()
			parent := root
			if kind == "nested" {
				parent = filepath.Join(root, "team")
			}
			if err := os.MkdirAll(parent, 0755); err != nil {
				t.Fatal(err)
			}
			path := filepath.Join(parent, ".skillshare-extras.lock")
			if kind == "directory" {
				if err := os.Mkdir(path, 0755); err != nil {
					t.Fatal(err)
				}
				if err := os.WriteFile(filepath.Join(path, "rules.md"), []byte("# Rules\n"), 0644); err != nil {
					t.Fatal(err)
				}
			} else if kind == "symlink" {
				if err := os.WriteFile(filepath.Join(root, "rules.md"), []byte("# Rules\n"), 0644); err != nil {
					t.Fatal(err)
				}
				if err := os.Symlink("rules.md", path); err != nil {
					t.Skipf("symlink unavailable: %v", err)
				}
			} else {
				var content []byte
				if kind == "nonempty" {
					content = []byte("# Rules\n")
				}
				if err := os.WriteFile(path, content, 0644); err != nil {
					t.Fatal(err)
				}
			}
			reserved, err := ScanSkill(root)
			if err != nil {
				t.Fatal(err)
			}
			if err := os.Rename(path, filepath.Join(parent, ".ordinary.lock")); err != nil {
				t.Fatal(err)
			}
			ordinary, err := ScanSkill(root)
			if err != nil {
				t.Fatal(err)
			}
			if reserved.TotalBytes != ordinary.TotalBytes || reserved.SkippedFiles != ordinary.SkippedFiles || reserved.AuditableBytes != ordinary.AuditableBytes {
				t.Fatalf("lock-named content bypassed normal policy: reserved=%+v ordinary=%+v", reserved, ordinary)
			}
			if ordinary.TotalBytes == 0 && ordinary.SkippedFiles == 0 {
				t.Fatal("control did not exercise normal scanner policy")
			}
		})
	}
}
