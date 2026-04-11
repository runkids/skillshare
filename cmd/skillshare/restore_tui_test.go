package main

import (
	"path/filepath"
	"strings"
	"testing"
	"time"

	"skillshare/internal/backup"
	"skillshare/internal/config"
)

func TestRestoreTUIRenderTargetDetail_ResolvesAlternateConfiguredTarget(t *testing.T) {
	home := setupGlobalResourceTestEnv(t)
	backupDir := filepath.Join(t.TempDir(), "backups")
	mustAddSkill(t, filepath.Join(backupDir, "2025-03-20_18-45-00", "universal"), "alpha")

	model := newRestoreTUIModel(nil, backupDir, map[string]config.TargetConfig{
		"agents": {Path: filepath.Join(home, ".agents", "skills")},
	}, "")

	detail := model.renderTargetDetail(backup.TargetBackupSummary{
		TargetName:  "universal",
		BackupCount: 1,
		Latest:      time.Date(2025, 3, 20, 18, 45, 0, 0, time.Local),
		Oldest:      time.Date(2025, 3, 20, 18, 45, 0, 0, time.Local),
	})

	if !strings.Contains(detail, filepath.Join(home, ".agents", "skills")) {
		t.Fatalf("renderTargetDetail() missing resolved target path:\n%s", detail)
	}
	if !strings.Contains(detail, "Status:") {
		t.Fatalf("renderTargetDetail() missing target status:\n%s", detail)
	}
}

func TestRestoreTUIRenderVersionDetail_ResolvesAlternateConfiguredTargetForDiff(t *testing.T) {
	home := setupGlobalResourceTestEnv(t)
	targetPath := filepath.Join(home, ".agents", "skills")
	mustAddSkill(t, targetPath, "local")

	model := newRestoreTUIModel(nil, t.TempDir(), map[string]config.TargetConfig{
		"agents": {Path: targetPath},
	}, "")
	model.selectedTarget = "universal"

	detail := model.renderVersionDetail(backup.BackupVersion{
		Label:      "2025-03-20 18:45:00",
		Timestamp:  time.Date(2025, 3, 20, 18, 45, 0, 0, time.Local),
		SkillCount: 1,
		SkillNames: []string{"alpha"},
	})

	if !strings.Contains(detail, "Diff vs current target") {
		t.Fatalf("renderVersionDetail() missing diff section:\n%s", detail)
	}
	if !strings.Contains(detail, "Restore:") {
		t.Fatalf("renderVersionDetail() missing restore diff:\n%s", detail)
	}
	if !strings.Contains(detail, "Remove:") {
		t.Fatalf("renderVersionDetail() missing remove diff:\n%s", detail)
	}
}
