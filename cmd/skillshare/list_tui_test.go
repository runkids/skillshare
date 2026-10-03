package main

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
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

func TestListTitleLine_ShowsScopeCountsAndTabs(t *testing.T) {
	items := []skillItem{
		{entry: skillEntry{Name: "react", RelPath: "react"}},
		{entry: skillEntry{Name: "vue", RelPath: "vue"}},
		{entry: skillEntry{Name: "helper", RelPath: "helper.md", Kind: "agent"}},
	}
	m := newListTUIModel(nil, items, len(items), "global", t.TempDir(), "", nil, kindAll)
	m.termWidth = 100
	m.filterText = "react"
	m.applyFilter()

	got := xansi.Strip(m.renderTitleLine())
	for _, want := range []string{"skillshare list", "global", "1 of 2 skills", "Skills 2", "Agents 1"} {
		if !strings.Contains(got, want) {
			t.Fatalf("title line missing %q in %q", want, got)
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

	m := newListTUIModel(nil, items, len(items), "global", t.TempDir(), "", nil, kindAll)
	m.termWidth = 120
	m.termHeight = 30
	m.detailScroll = 999
	m.syncListSize()

	got := xansi.Strip(m.viewSplit())

	// Path (group / name) should appear in the detail pane
	if !strings.Contains(got, "web-dev") || !strings.Contains(got, "accessibility") {
		t.Fatalf("viewSplit() missing skill path in detail pane: %q", got)
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

	m := newListTUIModel(nil, items, len(items), "global", t.TempDir(), "", nil, kindAll)

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

func TestTabCounts(t *testing.T) {
	items := []skillItem{
		{entry: skillEntry{Name: "s1", RelPath: "s1", Kind: "skill"}},
		{entry: skillEntry{Name: "s2", RelPath: "s2", Kind: "skill"}},
		{entry: skillEntry{Name: "a1", RelPath: "a1.md", Kind: "agent"}},
	}
	m := newListTUIModel(nil, items, len(items), "global", t.TempDir(), "", nil, kindAll)
	want := [2]int{2, 1}
	if m.tabCounts != want {
		t.Fatalf("tabCounts = %v, want %v", m.tabCounts, want)
	}
}

func TestTabSwitchFiltersItems(t *testing.T) {
	items := []skillItem{
		{entry: skillEntry{Name: "skill-a", RelPath: "skill-a", Kind: "skill"}},
		{entry: skillEntry{Name: "skill-b", RelPath: "skill-b", Kind: "skill"}},
		{entry: skillEntry{Name: "agent-a", RelPath: "agent-a.md", Kind: "agent"}},
	}
	m := newListTUIModel(nil, items, len(items), "global", t.TempDir(), "", nil, kindAll)

	// Default Skills tab
	if m.matchCount != 2 {
		t.Fatalf("Skills tab matchCount = %d, want 2", m.matchCount)
	}

	// tab switches to Agents
	next, _ := m.Update(tea.KeyMsg{Type: tea.KeyTab})
	m = next.(listTUIModel)
	if m.matchCount != 1 {
		t.Fatalf("Agents tab matchCount = %d, want 1", m.matchCount)
	}
}

func TestStatusFilterCycle(t *testing.T) {
	items := []skillItem{
		{entry: skillEntry{Name: "on-a", RelPath: "on-a"}},
		{entry: skillEntry{Name: "on-b", RelPath: "on-b"}},
		{entry: skillEntry{Name: "off-a", RelPath: "off-a", Disabled: true}},
	}
	m := newListTUIModel(nil, items, len(items), "global", t.TempDir(), "", nil, kindAll)

	// statusFilterAll — all 3 visible
	if m.matchCount != 3 {
		t.Fatalf("All status matchCount = %d, want 3", m.matchCount)
	}

	// statusFilterEnabled — only the 2 enabled
	m.statusFilter = statusFilterEnabled
	m.applyFilter()
	if m.matchCount != 2 {
		t.Fatalf("Enabled status matchCount = %d, want 2", m.matchCount)
	}

	// statusFilterDisabled — only the 1 disabled
	m.statusFilter = statusFilterDisabled
	m.applyFilter()
	if m.matchCount != 1 {
		t.Fatalf("Disabled status matchCount = %d, want 1", m.matchCount)
	}
}

func TestStatusFilterComposesWithTabAndText(t *testing.T) {
	items := []skillItem{
		{entry: skillEntry{Name: "react", RelPath: "react", Kind: "skill"}},
		{entry: skillEntry{Name: "react-old", RelPath: "react-old", Kind: "skill", Disabled: true}},
		{entry: skillEntry{Name: "react-agent", RelPath: "react-agent.md", Kind: "agent", Disabled: true}},
	}
	m := newListTUIModel(nil, items, len(items), "global", t.TempDir(), "", nil, kindAll)

	// Skills tab + Disabled status + text "react" → only "react-old"
	m.activeTab = listTabSkills
	m.statusFilter = statusFilterDisabled
	m.filterText = "react"
	m.applyFilter()
	if m.matchCount != 1 {
		t.Fatalf("Skills+Disabled+react matchCount = %d, want 1", m.matchCount)
	}
}

func TestListTitleLine_ShowsStatusFromFlag(t *testing.T) {
	m := newListTUIModel(nil, nil, 0, "global", t.TempDir(), "", nil, kindAll)
	m.statusFilter = statusFilterDisabled
	if got := xansi.Strip(m.renderTitleLine()); !strings.Contains(got, "disabled only") {
		t.Fatalf("title line = %q, want 'disabled only'", got)
	}
}

func TestListOpensAgentsTabWhenThereAreOnlyAgents(t *testing.T) {
	items := []skillItem{{entry: skillEntry{Name: "helper", RelPath: "helper.md", Kind: "agent"}}}
	m := newListTUIModel(nil, items, len(items), "global", t.TempDir(), "", nil, kindAll)
	if m.activeTab != listTabAgents {
		t.Fatalf("activeTab = %d, want the Agents tab", m.activeTab)
	}
}

func TestListEsc_ClearsTheFilterBeforeQuitting(t *testing.T) {
	items := []skillItem{{entry: skillEntry{Name: "react", RelPath: "react"}}, {entry: skillEntry{Name: "vue", RelPath: "vue"}}}
	m := newListTUIModel(nil, items, len(items), "global", t.TempDir(), "", nil, kindAll)
	m.filterText = "react"
	m.applyFilter()

	next, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEsc})
	m = next.(listTUIModel)
	if cmd != nil || m.filterText != "" || m.matchCount != 2 {
		t.Fatalf("first esc should clear the filter and stay; filter=%q matches=%d", m.filterText, m.matchCount)
	}

	next, _ = m.Update(tea.KeyMsg{Type: tea.KeyEsc})
	if !next.(listTUIModel).quitting {
		t.Fatal("second esc should quit")
	}
}

func TestListUninstallAsksOnTheKeyLine(t *testing.T) {
	items := []skillItem{{entry: skillEntry{Name: "react", RelPath: "react"}}}
	m := newListTUIModel(nil, items, len(items), "global", t.TempDir(), "", nil, kindAll)
	m.termWidth, m.termHeight = 120, 30
	m.syncListSize()

	m = pressKey(t, m, "d")

	got := xansi.Strip(m.View())
	if !strings.Contains(got, "Uninstall react?") || !strings.Contains(got, "skillshare uninstall -g react") {
		t.Fatalf("view should ask on the key line with the command; got %q", got)
	}
	if !strings.Contains(got, "skillshare list") {
		t.Fatalf("the list should stay on screen while asking; got %q", got)
	}
}

func TestInitialKindSetsTab(t *testing.T) {
	m := newListTUIModel(nil, nil, 0, "global", t.TempDir(), "", nil, kindAgents)
	if m.activeTab != listTabAgents {
		t.Fatalf("initialKind=kindAgents → activeTab = %d, want %d", m.activeTab, listTabAgents)
	}

	m2 := newListTUIModel(nil, nil, 0, "global", t.TempDir(), "", nil, kindSkills)
	if m2.activeTab != listTabSkills {
		t.Fatalf("initialKind=kindSkills → activeTab = %d, want %d", m2.activeTab, listTabSkills)
	}
}

func TestTabWithFilterComposition(t *testing.T) {
	items := []skillItem{
		{entry: skillEntry{Name: "react", RelPath: "react", Kind: "skill"}},
		{entry: skillEntry{Name: "vue", RelPath: "vue", Kind: "skill"}},
		{entry: skillEntry{Name: "react-agent", RelPath: "react-agent.md", Kind: "agent"}},
	}
	m := newListTUIModel(nil, items, len(items), "global", t.TempDir(), "", nil, kindAll)

	// Skills tab + text filter "react" → only skill "react"
	m.activeTab = listTabSkills
	m.filterText = "react"
	m.applyFilter()
	if m.matchCount != 1 {
		t.Fatalf("Skills+react matchCount = %d, want 1", m.matchCount)
	}

	// Agents tab + text filter "react" → only agent "react-agent"
	m.activeTab = listTabAgents
	m.applyFilter()
	if m.matchCount != 1 {
		t.Fatalf("Agents+react matchCount = %d, want 1", m.matchCount)
	}
}

func TestRenderKeyLine_KeepsTheKeysHintWhenNarrow(t *testing.T) {
	hints := []keyHint{{"↑↓", "move"}, {"/", "filter"}, {"enter", "open"}, {"d", "uninstall"}, {"?", "keys"}}
	got := xansi.Strip(renderKeyLine(40, hints, "1/8"))
	if xansi.StringWidth(got) > 40 || !strings.Contains(got, "? keys") || !strings.Contains(got, "1/8") {
		t.Fatalf("narrow key line = %q, want it to fit 40 columns and keep '? keys' and the position", got)
	}
}
