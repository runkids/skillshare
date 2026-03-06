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
