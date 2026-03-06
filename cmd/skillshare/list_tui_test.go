package main

import (
	"strings"
	"testing"

	xansi "github.com/charmbracelet/x/ansi"
)

func TestListSplitActive(t *testing.T) {
	if listSplitActive(tuiMinSplitWidth - 1) {
		t.Fatalf("expected split layout to be disabled below minimum width")
	}
	if !listSplitActive(tuiMinSplitWidth) {
		t.Fatalf("expected split layout to be enabled at minimum width")
	}
}

func TestListPanelWidthBounds(t *testing.T) {
	if got := listPanelWidth(80); got != 30 {
		t.Fatalf("listPanelWidth(80) = %d, want 30", got)
	}
	if got := listPanelWidth(200); got != 46 {
		t.Fatalf("listPanelWidth(200) = %d, want capped 46", got)
	}
}

func TestListDetailStatusBits(t *testing.T) {
	got := detailStatusBits(skillEntry{
		Name:        "demo",
		RelPath:     "demo",
		RepoName:    "org/repo",
		InstalledAt: "2026-03-01",
	})

	for _, want := range []string{"tracked"} {
		if !strings.Contains(got, want) {
			t.Fatalf("detailStatusBits() missing %q in %q", want, got)
		}
	}
}

func TestListSummaryFooterCounts(t *testing.T) {
	m := listTUIModel{
		allItems: []skillItem{
			{entry: skillEntry{Name: "local", RelPath: "local"}},
			{entry: skillEntry{Name: "tracked", RelPath: "tracked", RepoName: "team/repo"}},
			{entry: skillEntry{Name: "remote", RelPath: "remote", Source: "github.com/example/repo"}},
		},
		matchCount: 2,
	}

	got := m.renderSummaryFooter()
	for _, want := range []string{"2/3 visible", "1 local", "1 tracked", "1 remote"} {
		if !strings.Contains(got, want) {
			t.Fatalf("renderSummaryFooter() missing %q in %q", want, got)
		}
	}
}

func TestRenderDetailHeader_ShowsNameAndGroup(t *testing.T) {
	got := renderDetailHeader(skillEntry{
		Name:        "remote",
		RelPath:     "web-dev/accessibility",
		Source:      "github.com/example/accessibility",
		InstalledAt: "2026-03-03",
	}, &detailData{
		SyncedTargets: []string{"claude", "cursor"},
	}, 80)

	plain := xansi.Strip(got)

	// First non-empty line should show the full path (group / name)
	lines := strings.Split(plain, "\n")
	first := ""
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if trimmed != "" {
			first = trimmed
			break
		}
	}
	// colorSkillPath renders "web-dev / accessibility" with separator
	if !strings.Contains(first, "web-dev") || !strings.Contains(first, "accessibility") {
		t.Fatalf("detail header first line = %q, want group/name path", first)
	}
}

func TestListViewSplit_HeaderKeepsSkillNameWhenDetailScrolled(t *testing.T) {
	items := []skillItem{
		{
			entry: skillEntry{
				Name:        "remote",
				RelPath:     "web-dev/accessibility",
				Source:      "github.com/example/accessibility",
				InstalledAt: "2026-03-03",
			},
		},
	}

	m := newListTUIModel(nil, items, len(items), "global", t.TempDir(), nil)
	m.termWidth = 120
	m.termHeight = 30
	m.detailScroll = 999
	m.syncListSize()

	got := xansi.Strip(m.viewSplit())

	// Path (group / name) should appear in the detail pane
	if !strings.Contains(got, "web-dev") || !strings.Contains(got, "accessibility") {
		t.Fatalf("viewSplit() missing skill path in detail pane: %q", got)
	}

	// Date should appear in the metadata line
	if !strings.Contains(got, "2026-03-03") {
		t.Fatalf("viewSplit() missing install date in detail pane: %q", got)
	}

	// Skill name should appear before the date
	nameIdx := strings.Index(got, "accessibility")
	dateIdx := strings.Index(got, "2026-03-03")
	if nameIdx > dateIdx {
		t.Fatalf("expected skill name before date; output: %q", got)
	}
}

func TestApplyFilter_WithTags(t *testing.T) {
	items := []skillItem{
		{entry: skillEntry{Name: "local-skill", RelPath: "local-skill"}},
		{entry: skillEntry{Name: "react-tips", RelPath: "frontend/react-tips"}},
		{entry: skillEntry{Name: "audit", RelPath: "_team-repo/security/audit", RepoName: "team/repo"}},
		{entry: skillEntry{Name: "lint", RelPath: "_team-repo/lint", RepoName: "team/repo"}},
		{entry: skillEntry{Name: "remote-a", RelPath: "remote-a", Source: "github.com/foo/bar"}},
	}

	m := newListTUIModel(nil, items, len(items), "global", t.TempDir(), nil)

	// Filter by type:tracked — should match 2 items
	m.filterText = "t:tracked"
	m.applyFilter()
	if m.matchCount != 2 {
		t.Fatalf("type:tracked matchCount = %d, want 2", m.matchCount)
	}

	// Filter by type:local — should match 2 items (local-skill + react-tips)
	m.filterText = "t:local"
	m.applyFilter()
	if m.matchCount != 2 {
		t.Fatalf("type:local matchCount = %d, want 2", m.matchCount)
	}

	// Filter by group:security — should match 1 item
	m.filterText = "g:security"
	m.applyFilter()
	if m.matchCount != 1 {
		t.Fatalf("group:security matchCount = %d, want 1", m.matchCount)
	}

	// Filter by repo:team — should match 2 tracked items
	m.filterText = "r:team"
	m.applyFilter()
	if m.matchCount != 2 {
		t.Fatalf("repo:team matchCount = %d, want 2", m.matchCount)
	}

	// Combined: type:tracked + group:security — should match 1
	m.filterText = "t:tracked g:security"
	m.applyFilter()
	if m.matchCount != 1 {
		t.Fatalf("combined tag matchCount = %d, want 1", m.matchCount)
	}

	// Free text only — should match react-tips
	m.filterText = "react"
	m.applyFilter()
	if m.matchCount != 1 {
		t.Fatalf("free text matchCount = %d, want 1", m.matchCount)
	}

	// Clear filter — should restore all
	m.filterText = ""
	m.applyFilter()
	if m.matchCount != len(items) {
		t.Fatalf("cleared matchCount = %d, want %d", m.matchCount, len(items))
	}
}
