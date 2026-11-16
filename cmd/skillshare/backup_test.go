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

func TestRestoreHelp_ProjectSkills(t *testing.T) {
	output := captureStdout(t, printRestoreHelp)
	if !strings.Contains(output, "named skills or agents") || strings.Contains(output, "Restore both skills and agents") {
		t.Fatalf("inaccurate scope help: %s", output)
	}
}

func TestRestorePreviews_ExplicitDirectory(t *testing.T) {
	root := t.TempDir()
	stamp := "2024-01-15_14-30-45"
	entry := filepath.Join(root, stamp, "custom")
	if err := os.MkdirAll(entry, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(entry, "saved.md"), []byte("saved"), 0644); err != nil {
		t.Fatal(err)
	}
	dest := filepath.Join(t.TempDir(), "missing")
	if err := previewRestoreFromTimestampInDir(root, "custom", dest, stamp, backup.RestoreOptions{}); err != nil {
		t.Fatal(err)
	}
	if err := previewRestoreFromLatestInDir(root, "custom", dest, backup.RestoreOptions{}); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(dest); !os.IsNotExist(err) {
		t.Fatalf("preview created destination: %v", err)
	}
}

func TestRestoreProjectHelpBeforeConfig(t *testing.T) {
	output := captureStdout(t, func() {
		if err := cmdRestore([]string{"-p", "--help"}); err != nil {
			t.Fatalf("help required project configuration: %v", err)
		}
	})
	if !strings.Contains(output, "named skills or agents") {
		t.Fatalf("project restore help missing: %s", output)
	}
}
