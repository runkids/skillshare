package main

import (
	"strings"
	"testing"

	"skillshare/internal/config"
)

func TestFormatTokenK(t *testing.T) {
	tests := []struct {
		input int
		want  string
	}{
		{0, "0"},
		{1, "1"},
		{830, "830"},
		{999, "999"},
		{1000, "1.0K"},
		{1500, "1.5K"},
		{12400, "12.4K"},
		{100000, "100.0K"},
		{1234567, "1234.6K"},
	}
	for _, tt := range tests {
		if got := formatTokenK(tt.input); got != tt.want {
			t.Errorf("formatTokenK(%d) = %q, want %q", tt.input, got, tt.want)
		}
	}
}

func TestFormatTokenComma(t *testing.T) {
	tests := []struct {
		input int
		want  string
	}{
		{0, "0"},
		{830, "830"},
		{1000, "1,000"},
		{10000, "10,000"},
		{50123, "50,123"},
		{1234567, "1,234,567"},
	}
	for _, tt := range tests {
		if got := formatTokenComma(tt.input); got != tt.want {
			t.Errorf("formatTokenComma(%d) = %q, want %q", tt.input, got, tt.want)
		}
	}
}

func TestGroupByTokenCost(t *testing.T) {
	entries := []analyzeTargetEntry{
		{Name: "claude", AlwaysLoaded: analyzeCharTokens{EstimatedTokens: 1000}, OnDemandMax: analyzeCharTokens{EstimatedTokens: 5000}},
		{Name: "cursor", AlwaysLoaded: analyzeCharTokens{EstimatedTokens: 1000}, OnDemandMax: analyzeCharTokens{EstimatedTokens: 5000}},
		{Name: "codex", AlwaysLoaded: analyzeCharTokens{EstimatedTokens: 800}, OnDemandMax: analyzeCharTokens{EstimatedTokens: 3000}},
	}
	groups := groupByTokenCost(entries)
	if len(groups) != 2 {
		t.Fatalf("expected 2 groups, got %d", len(groups))
	}
	if got := strings.Join(groups[0].Targets, ","); got != "claude,cursor" {
		t.Errorf("first group targets = %q, want \"claude,cursor\"", got)
	}
	if groups[0].AlwaysLoaded != 1000 {
		t.Errorf("first group always_loaded = %d, want 1000", groups[0].AlwaysLoaded)
	}
	if got := strings.Join(groups[1].Targets, ","); got != "codex" {
		t.Errorf("second group targets = %q, want \"codex\"", got)
	}
}

func TestGroupByTokenCost_AllSame(t *testing.T) {
	entries := []analyzeTargetEntry{
		{Name: "a", AlwaysLoaded: analyzeCharTokens{EstimatedTokens: 500}, OnDemandMax: analyzeCharTokens{EstimatedTokens: 2000}},
		{Name: "b", AlwaysLoaded: analyzeCharTokens{EstimatedTokens: 500}, OnDemandMax: analyzeCharTokens{EstimatedTokens: 2000}},
	}
	groups := groupByTokenCost(entries)
	if len(groups) != 1 {
		t.Fatalf("expected 1 group, got %d", len(groups))
	}
	if len(groups[0].Targets) != 2 {
		t.Errorf("expected 2 targets in group, got %d", len(groups[0].Targets))
	}
}

func TestCheckBudget_BelowThreshold(t *testing.T) {
	entries := []analyzeTargetEntry{
		{
			Name:         "claude",
			AlwaysLoaded: analyzeCharTokens{EstimatedTokens: 5000},
			OnDemandMax:  analyzeCharTokens{EstimatedTokens: 50000},
		},
	}
	budget := config.ContextBudgetConfig{}
	violations := checkBudget(entries, budget)
	if len(violations) != 0 {
		t.Errorf("expected 0 violations, got %d", len(violations))
	}
}

func TestCheckBudget_ExceedsAlwaysLoaded(t *testing.T) {
	skills := []analyzeSkillEntry{
		{Name: "big", DescriptionTokens: 5000},
		{Name: "medium", DescriptionTokens: 3000},
		{Name: "small", DescriptionTokens: 1500},
		{Name: "tiny", DescriptionTokens: 500},
	}
	entries := []analyzeTargetEntry{
		{
			Name:         "claude",
			AlwaysLoaded: analyzeCharTokens{EstimatedTokens: 10000},
			OnDemandMax:  analyzeCharTokens{EstimatedTokens: 50000},
			Skills:       skills,
		},
	}
	budget := config.ContextBudgetConfig{WarnAlwaysLoadedTokens: intPtr(5000)}
	violations := checkBudget(entries, budget)
	if len(violations) != 1 {
		t.Fatalf("expected 1 violation, got %d", len(violations))
	}
	v := violations[0]
	if v.Type != "always_loaded" {
		t.Errorf("expected type always_loaded, got %s", v.Type)
	}
	if v.Actual != 10000 {
		t.Errorf("expected actual 10000, got %d", v.Actual)
	}
	if v.Budget != 5000 {
		t.Errorf("expected budget 5000, got %d", v.Budget)
	}
	if len(v.Offenders) != 3 {
		t.Errorf("expected 3 offenders, got %d", len(v.Offenders))
	}
	if v.Offenders[0].Name != "big" {
		t.Errorf("expected first offender \"big\", got %q", v.Offenders[0].Name)
	}
}

func TestCheckBudget_ZeroDisables(t *testing.T) {
	entries := []analyzeTargetEntry{
		{
			Name:         "claude",
			AlwaysLoaded: analyzeCharTokens{EstimatedTokens: 999999},
			OnDemandMax:  analyzeCharTokens{EstimatedTokens: 999999},
		},
	}
	budget := config.ContextBudgetConfig{
		WarnAlwaysLoadedTokens: intPtr(0),
		WarnOnDemandTokens:     intPtr(0),
	}
	violations := checkBudget(entries, budget)
	if len(violations) != 0 {
		t.Errorf("expected 0 violations with zero thresholds, got %d", len(violations))
	}
}

func TestCheckBudget_BothExceed(t *testing.T) {
	entries := []analyzeTargetEntry{
		{
			Name:         "claude",
			AlwaysLoaded: analyzeCharTokens{EstimatedTokens: 20000},
			OnDemandMax:  analyzeCharTokens{EstimatedTokens: 200000},
			Skills: []analyzeSkillEntry{
				{Name: "a", DescriptionTokens: 10000, BodyTokens: 100000},
				{Name: "b", DescriptionTokens: 5000, BodyTokens: 50000},
			},
		},
	}
	budget := config.ContextBudgetConfig{
		WarnAlwaysLoadedTokens: intPtr(10000),
		WarnOnDemandTokens:     intPtr(100000),
	}
	violations := checkBudget(entries, budget)
	if len(violations) != 2 {
		t.Fatalf("expected 2 violations, got %d", len(violations))
	}
	if violations[0].Type != "always_loaded" {
		t.Errorf("first violation type = %q, want always_loaded", violations[0].Type)
	}
	if violations[1].Type != "on_demand" {
		t.Errorf("second violation type = %q, want on_demand", violations[1].Type)
	}
}
