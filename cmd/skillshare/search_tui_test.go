package main

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	xansi "github.com/charmbracelet/x/ansi"

	"skillshare/internal/search"
)

func TestSearchSelectItemMeta_ShowsRiskForAnIndex(t *testing.T) {
	item := searchSelectItem{result: search.SearchResult{Name: "risky", RiskLabel: "high", Stars: 50}, isHub: true}
	if got := xansi.Strip(item.meta()); got != "high" {
		t.Fatalf("meta() = %q, want the risk label", got)
	}
}

func TestSearchSelectItemMeta_ShowsStarsForGitHub(t *testing.T) {
	item := searchSelectItem{result: search.SearchResult{Name: "cool-skill", Stars: 1234}}
	if got := xansi.Strip(item.meta()); got != "★ 1.2k" {
		t.Fatalf("meta() = %q, want formatted stars", got)
	}
}

func TestSearchSelectItem_FilterValue(t *testing.T) {
	item := searchSelectItem{
		result: search.SearchResult{Name: "my-skill", Description: "Does things", Tags: []string{"ai"}, RiskLabel: "low"},
		isHub:  true,
	}
	got := item.FilterValue()
	for _, want := range []string{"my-skill", "Does things", "ai", "low"} {
		if !strings.Contains(got, want) {
			t.Errorf("FilterValue() = %q, want to contain %q", got, want)
		}
	}
}

func TestSearchSelectEnterWithoutSelectionInstallsTheCursorRow(t *testing.T) {
	m := newSearchSelectModel([]search.SearchResult{{Name: "first"}, {Name: "second"}}, "demo", false)

	next, _ := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	got := next.(searchSelectModel)

	if got.outcome != searchSelectInstall || got.selCount != 1 || !got.selected[0] {
		t.Fatalf("enter with nothing selected should install the row under the cursor; outcome=%v selected=%v", got.outcome, got.selected)
	}
}

func TestSearchSelectEscClearsTheFilterBeforeSearchingAgain(t *testing.T) {
	m := newSearchSelectModel([]search.SearchResult{{Name: "first"}, {Name: "second"}}, "demo", false)
	m.filterText = "sec"
	m.applySearchFilter()

	next, _ := m.Update(tea.KeyMsg{Type: tea.KeyEsc})
	got := next.(searchSelectModel)
	if got.filterText != "" || got.quitting {
		t.Fatal("first esc should clear the filter and stay open")
	}
	next, _ = got.Update(tea.KeyMsg{Type: tea.KeyEsc})
	if next.(searchSelectModel).outcome != searchSelectSearchAgain {
		t.Fatal("esc with no filter should go back to the keyword")
	}
}
