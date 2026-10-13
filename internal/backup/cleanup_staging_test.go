package backup

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"
)

func TestCleanupInDir_RemovesOrphanedStaging(t *testing.T) {
	for _, completed := range []bool{false, true} {
		t.Run(map[bool]string{false: "staging only", true: "with snapshot"}[completed], func(t *testing.T) {
			root := t.TempDir()
			staging := filepath.Join(root, stagingDirPrefix+"orphan", "local-skill")
			if err := os.MkdirAll(staging, 0755); err != nil {
				t.Fatal(err)
			}
			writeTestFile(t, filepath.Join(staging, "partial.txt"), "partial")
			if completed {
				if err := os.MkdirAll(filepath.Join(root, "2026-01-01_00-00-00", "claude"), 0755); err != nil {
					t.Fatal(err)
				}
			}
			count, freed, err := PreviewCleanupInDir(root, CleanupConfig{})
			if err != nil || count != 1 || freed != int64(len("partial")) {
				t.Fatalf("preview = (%d, %d, %v), want (1, 7, nil)", count, freed, err)
			}
			assertFileContent(t, filepath.Join(staging, "partial.txt"), "partial")
			if _, err := os.Stat(filepath.Join(root, backupLockName)); !os.IsNotExist(err) {
				t.Fatalf("preview created a lock file: %v", err)
			}
			removed, err := CleanupInDir(root, CleanupConfig{})
			if err != nil || removed != 1 {
				t.Fatalf("CleanupInDir = (%d, %v), want (1, nil)", removed, err)
			}
			if _, err := os.Stat(staging); !os.IsNotExist(err) {
				t.Fatalf("orphan remains: %v", err)
			}
			backups, err := ListInDir(root)
			if err != nil || len(backups) != map[bool]int{false: 0, true: 1}[completed] {
				t.Fatalf("completed snapshots changed: %v, %v", backups, err)
			}
		})
	}
}

func TestBackupInterruptedProcess(t *testing.T) {
	if root := os.Getenv("SKILLSHARE_TEST_INTERRUPTED_BACKUP"); root != "" {
		_, err := createInDir(root, "claude", os.Getenv("SKILLSHARE_TEST_BACKUP_TARGET"), func(_, dst string) error {
			writeTestFile(t, filepath.Join(dst, "partial.txt"), "partial")
			writeTestFile(t, filepath.Join(root, "ready"), dst)
			for {
				time.Sleep(time.Hour)
			}
		})
		t.Fatal(err)
	}

	root, target := t.TempDir(), t.TempDir()
	writeTestFile(t, filepath.Join(target, "SKILL.md"), "local")
	cmd := exec.Command(os.Args[0], "-test.run=^TestBackupInterruptedProcess$")
	cmd.Env = append(os.Environ(), "SKILLSHARE_TEST_INTERRUPTED_BACKUP="+root, "SKILLSHARE_TEST_BACKUP_TARGET="+target)
	if err := cmd.Start(); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if cmd.ProcessState == nil {
			_ = cmd.Process.Kill()
			_ = cmd.Wait()
		}
	})
	var staging string
	deadline := time.Now().Add(10 * time.Second)
	for time.Now().Before(deadline) {
		if data, err := os.ReadFile(filepath.Join(root, "ready")); err == nil && len(data) > 0 {
			staging = string(data)
			break
		}
		time.Sleep(10 * time.Millisecond)
	}
	if staging == "" {
		t.Fatal("child did not reach staging copy")
	}
	if removed, err := CleanupInDir(root, CleanupConfig{}); err != nil || removed != 0 {
		t.Fatalf("cleanup during copy = (%d, %v), want (0, nil)", removed, err)
	}
	assertFileContent(t, filepath.Join(staging, "partial.txt"), "partial")
	if _, _, err := PreviewCleanupInDir(root, CleanupConfig{}); err == nil {
		t.Fatal("preview must report a busy backup directory")
	}
	if err := cmd.Process.Kill(); err != nil {
		t.Fatal(err)
	}
	_ = cmd.Wait()
	if removed, err := CleanupInDir(root, CleanupConfig{}); err != nil || removed != 1 {
		t.Fatalf("cleanup after kill = (%d, %v), want (1, nil)", removed, err)
	}
	if _, err := os.Stat(staging); !os.IsNotExist(err) {
		t.Fatalf("interrupted staging remains: %v", err)
	}
}
