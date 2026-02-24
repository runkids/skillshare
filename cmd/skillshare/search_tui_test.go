package main

import (
	"strings"
	"testing"

	"skillshare/internal/search"
)

func TestSearchSelectItem_Title_Hub(t *testing.T) {
	tests := []struct {
		name      string
		result    search.SearchResult
		selected  bool
		wantCheck string
		wantBadge string
	}{
		{
			name:      "unselected clean hub skill",
			result:    search.SearchResult{Name: "my-skill", RiskLabel: "clean"},
			wantCheck: "[ ]",
			wantBadge: "[clean]",
		},
		{
			name:      "selected high hub skill",
			result:    search.SearchResult{Name: "risky", RiskLabel: "high"},
			selected:  true,
			wantCheck: "[x]",
			wantBadge: "[high]",
		},
		{
			name:      "no risk label",
			result:    search.SearchResult{Name: "no-audit"},
			wantCheck: "[ ]",
			wantBadge: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			item := searchSelectItem{
				idx:      0,
				result:   tt.result,
				isHub:    true,
				selected: tt.selected,
			}
			title := item.Title()

			if !strings.HasPrefix(title, tt.wantCheck) {
				t.Errorf("Title() = %q, want prefix %q", title, tt.wantCheck)
			}
			if !strings.Contains(title, tt.result.Name) {
				t.Errorf("Title() = %q, want to contain name %q", title, tt.result.Name)
			}
			if tt.wantBadge != "" {
				if !strings.Contains(title, tt.wantBadge) {
					t.Errorf("Title() = %q, want to contain badge %q", title, tt.wantBadge)
				}
			}
			// Should NOT contain stars for hub
			if strings.Contains(title, "★") {
				t.Errorf("Title() = %q, hub items should not have stars", title)
			}
		})
	}
}

func TestSearchSelectItem_Title_GitHub(t *testing.T) {
	item := searchSelectItem{
		idx:    0,
		result: search.SearchResult{Name: "cool-skill", Stars: 1234},
		isHub:  false,
	}
	title := item.Title()

	if !strings.HasPrefix(title, "[ ]") {
		t.Errorf("Title() = %q, want prefix '[ ]'", title)
	}
	if !strings.Contains(title, "★") {
		t.Errorf("Title() = %q, want to contain ★", title)
	}
	if !strings.Contains(title, "1.2k") {
		t.Errorf("Title() = %q, want to contain formatted stars '1.2k'", title)
	}
}

func TestSearchSelectItem_Checkbox(t *testing.T) {
	unchecked := searchSelectItem{result: search.SearchResult{Name: "a"}}
	checked := searchSelectItem{result: search.SearchResult{Name: "a"}, selected: true}

	if !strings.HasPrefix(unchecked.Title(), "[ ]") {
		t.Errorf("unchecked Title() = %q, want prefix '[ ]'", unchecked.Title())
	}
	if !strings.HasPrefix(checked.Title(), "[x]") {
		t.Errorf("checked Title() = %q, want prefix '[x]'", checked.Title())
	}
}

func TestSearchSelectItem_Description(t *testing.T) {
	tests := []struct {
		name       string
		result     search.SearchResult
		wantParts  []string
		absentTags bool
	}{
		{
			name: "hub with tags",
			result: search.SearchResult{
				Source: "owner/repo",
				Tags:   []string{"ai", "coding"},
			},
			wantParts: []string{"owner/repo", "#ai", "#coding"},
		},
		{
			name: "github no tags",
			result: search.SearchResult{
				Source: "user/repo/path",
			},
			wantParts:  []string{"user/repo/path"},
			absentTags: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			item := searchSelectItem{result: tt.result, isHub: true}
			desc := item.Description()

			for _, p := range tt.wantParts {
				if !strings.Contains(desc, p) {
					t.Errorf("Description() = %q, want to contain %q", desc, p)
				}
			}
			if tt.absentTags && strings.Contains(desc, "#") {
				t.Errorf("Description() = %q, should not contain tags", desc)
			}
		})
	}
}

func TestSearchSelectItem_FilterValue(t *testing.T) {
	item := searchSelectItem{
		result: search.SearchResult{
			Name:        "my-skill",
			Description: "A useful skill",
			Tags:        []string{"ai", "dev"},
		},
	}
	fv := item.FilterValue()

	for _, want := range []string{"my-skill", "A useful skill", "ai", "dev"} {
		if !strings.Contains(fv, want) {
			t.Errorf("FilterValue() = %q, want to contain %q", fv, want)
		}
	}
}
