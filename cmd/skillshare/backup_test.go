package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"skillshare/internal/backup"
)

func TestBackupHelp_DisclosesStagingCleanup(t *testing.T) {
	output := captureStdout(t, printBackupHelp)
	if !strings.Contains(output, "abandoned incomplete directories") {
		t.Fatalf("help does not disclose staging cleanup: %q", output)
	}
}

func TestBackupCleanup_StagingOnly(t *testing.T) {
	root := t.TempDir()
	staging := filepath.Join(root, ".snapshot-orphan")
	if err := os.MkdirAll(staging, 0755); err != nil {
		t.Fatal(err)
	}
	payload := filepath.Join(staging, "partial.txt")
	if err := os.WriteFile(payload, []byte("partial"), 0644); err != nil {
		t.Fatal(err)
	}

	if err := backupCleanupDryRun(root, backup.CleanupConfig{}); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(payload); err != nil {
		t.Fatalf("dry run modified staging: %v", err)
	}
	if err := backupCleanup(root, backup.CleanupConfig{}); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(staging); !os.IsNotExist(err) {
		t.Fatalf("CLI cleanup skipped staging-only directory: %v", err)
	}
}
