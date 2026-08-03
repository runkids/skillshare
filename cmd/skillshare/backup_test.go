package main

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"skillshare/internal/backup"
)

func TestPlanBackupCleanup_OverSizeCapKeepsNewestBackup(t *testing.T) {
	backupPath := filepath.Join(t.TempDir(), "2026-07-30_00-00-00")
	if err := os.MkdirAll(filepath.Join(backupPath, "claude"), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(backupPath, "claude", "big.bin"), make([]byte, 2<<20), 0644); err != nil {
		t.Fatal(err)
	}

	backups := []backup.BackupInfo{{
		Timestamp: "2026-07-30_00-00-00",
		Path:      backupPath,
		Date:      time.Now(),
	}}
	removed, freed := planBackupCleanup(backups, backup.CleanupConfig{MaxSizeMB: 1}, time.Now())

	if removed != 0 {
		t.Errorf("removed = %d, want 0", removed)
	}
	if freed != 0 {
		t.Errorf("freed = %d, want 0", freed)
	}
}

func TestPlanBackupCleanup_SizeCapCountsOnlyRetainedBackups(t *testing.T) {
	root := t.TempDir()
	sizes := []int{400 << 10, 700 << 10, 400 << 10}
	backups := make([]backup.BackupInfo, 0, len(sizes))

	for i, size := range sizes {
		path := filepath.Join(root, time.Now().Add(-time.Duration(i)*time.Hour).Format("2006-01-02_15-04-05"))
		if err := os.MkdirAll(filepath.Join(path, "claude"), 0755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(path, "claude", "payload.bin"), make([]byte, size), 0644); err != nil {
			t.Fatal(err)
		}
		backups = append(backups, backup.BackupInfo{
			Timestamp: filepath.Base(path),
			Path:      path,
			Date:      time.Now().Add(-time.Duration(i) * time.Hour),
		})
	}

	removed, freed := planBackupCleanup(backups, backup.CleanupConfig{MaxSizeMB: 1}, time.Now())
	if removed != 1 {
		t.Errorf("removed = %d, want 1", removed)
	}
	if freed != int64(sizes[1]) {
		t.Errorf("freed = %d, want %d", freed, sizes[1])
	}
}
