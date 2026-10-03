package main

import (
	"strings"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	xansi "github.com/charmbracelet/x/ansi"

	"skillshare/internal/backup"
)

func testRestoreModel() restoreTUIModel {
	m := newRestoreTUIModel([]backup.TargetBackupSummary{
		{TargetName: "claude", BackupCount: 2, Latest: time.Now(), Oldest: time.Now()},
	}, "", nil, "")
	m.termWidth = 120
	return m
}

func TestRestoreTitleLine_NamesTheChosenTarget(t *testing.T) {
	m := testRestoreModel()
	if got := xansi.Strip(m.renderTitleLine()); !strings.Contains(got, "1 target") {
		t.Fatalf("target phase title %q should count targets", got)
	}
	m.selectedTarget = "claude"
	m.versionItems = []backup.BackupVersion{{Label: "2026-10-03 16:00"}, {Label: "2026-10-02 09:00"}}
	if got := xansi.Strip(m.renderTitleLine()); !strings.Contains(got, "claude") || !strings.Contains(got, "2 backups") {
		t.Fatalf("version phase title %q should name the target and count its backups", got)
	}
}

func TestRestoreDeleteAsksOnTheKeyLine(t *testing.T) {
	m := testRestoreModel()
	m.phase = phaseConfirm
	m.confirmAction = "delete"
	m.selectedTarget = "claude"
	m.selectedVersion = &backup.BackupVersion{Label: "2026-10-03 16:00", SkillCount: 3, TotalSize: -1}

	got := xansi.Strip(m.renderBottom())
	if !strings.Contains(got, "Delete backup 2026-10-03 16:00?") || !strings.Contains(got, "3 skills") {
		t.Fatalf("delete confirmation should name the backup and its skills, got %q", got)
	}
	next, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("n")})
	if next.(restoreTUIModel).phase != phaseVersionList {
		t.Fatal("n should go back to the backups")
	}
}
