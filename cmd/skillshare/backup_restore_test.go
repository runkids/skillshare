package main

import (
	"path/filepath"
	"testing"

	"skillshare/internal/backup"
	"skillshare/internal/config"
)

func TestCmdRestore_AliasRestoresCanonicalLatestBackup(t *testing.T) {
	home := setupGlobalResourceTestEnv(t)
	sourceDir := filepath.Join(t.TempDir(), "source")
	mustAddSkill(t, sourceDir, "alpha")

	cfg := &config.Config{
		Source: sourceDir,
		Targets: map[string]config.TargetConfig{
			"universal": {Path: filepath.Join(home, ".agents", "skills")},
		},
	}
	if err := cfg.Save(); err != nil {
		t.Fatalf("save config: %v", err)
	}

	mustWriteFile(t, filepath.Join(backup.BackupDir(), "2025-03-20_18-45-00", "universal", "alpha", "SKILL.md"), "# Alpha\n")

	if err := cmdRestore([]string{"agents", "--force"}); err != nil {
		t.Fatalf("cmdRestore(alias latest) error = %v", err)
	}

	assertFileContent(t, filepath.Join(home, ".agents", "skills", "alpha", "SKILL.md"), "# Alpha\n")
}
