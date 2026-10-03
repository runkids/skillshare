package main

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	xansi "github.com/charmbracelet/x/ansi"

	"skillshare/internal/oplog"
)

func TestLogListWidth(t *testing.T) {
	tests := []struct {
		termWidth int
		want      int
	}{
		{50, 30},  // 50/4=12 → clamped to min 30
		{70, 30},  // 70/4=17 → clamped to min 30
		{80, 30},  // 80/4=20 → clamped to min 30
		{100, 30}, // 100/4=25 → clamped to min 30
		{120, 30}, // 120/4=30
		{160, 40}, // 160/4=40
		{200, 45}, // 200/4=50 → clamped to max 45
	}
	for _, tt := range tests {
		got := logListWidth(tt.termWidth)
		if got != tt.want {
			t.Errorf("logListWidth(%d) = %d, want %d", tt.termWidth, got, tt.want)
		}
	}
}

func TestLogDetailPanelWidth(t *testing.T) {
	tests := []struct {
		termWidth int
		want      int
	}{
		{50, 30},   // 50-30=20 → clamped to min 30
		{70, 40},   // 70-30
		{120, 90},  // 120-30
		{160, 120}, // 160-40
	}
	for _, tt := range tests {
		if got := logDetailPanelWidth(tt.termWidth); got != tt.want {
			t.Errorf("logDetailPanelWidth(%d) = %d, want %d", tt.termWidth, got, tt.want)
		}
	}
}

func testLogModel() logTUIModel {
	items := []logItem{
		{entry: oplog.Entry{Timestamp: "2026-10-03T15:25:00Z", Command: "sync", Status: "ok"}, source: "operations"},
		{entry: oplog.Entry{Timestamp: "2026-10-03T15:24:00Z", Command: "audit", Status: "error"}, source: "audit"},
	}
	m := newLogTUIModel(nil, items, "Operations", "global", "")
	m.termWidth = 120
	return m
}

func TestLogTitleLine_ShowsCountsRateAndTabs(t *testing.T) {
	got := xansi.Strip(testLogModel().renderTitleLine())
	for _, want := range []string{"skillshare log", "global", "2 entries", "50% ok", "Entries", "Stats"} {
		if !strings.Contains(got, want) {
			t.Errorf("title line %q is missing %q", got, want)
		}
	}
}

func TestLogTab_OpensStats(t *testing.T) {
	next, _ := testLogModel().Update(tea.KeyMsg{Type: tea.KeyTab})
	if !next.(logTUIModel).showStats {
		t.Fatal("tab should open the Stats tab")
	}
}

func TestLogDeleteWithoutSelectionAsksAboutTheCursorEntry(t *testing.T) {
	next, _ := testLogModel().Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("d")})
	got := next.(logTUIModel)
	if !got.confirmDelete || len(got.deleteItems) != 1 || got.deleteItems[0].entry.Command != "sync" {
		t.Fatalf("d with nothing selected should ask about the entry under the cursor; confirm=%v items=%v", got.confirmDelete, got.deleteItems)
	}
}
