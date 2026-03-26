package main

import (
	"testing"

	"github.com/charmbracelet/bubbles/list"
)

func TestComputeThresholds(t *testing.T) {
	tests := []struct {
		name     string
		tokens   []int
		wantLow  int
		wantHigh int
	}{
		{"single", []int{100}, 100, 100},
		{"two", []int{10, 90}, 10, 90},
		{"four even", []int{10, 20, 30, 40}, 20, 40},
		{"empty", []int{}, 0, 0},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			low, high := computeThresholds(tt.tokens)
			if low != tt.wantLow {
				t.Errorf("low: got %d, want %d", low, tt.wantLow)
			}
			if high != tt.wantHigh {
				t.Errorf("high: got %d, want %d", high, tt.wantHigh)
			}
		})
	}
}

func TestTokenColor(t *testing.T) {
	tests := []struct {
		tokens    int
		low, high int
		wantColor string
	}{
		{100, 20, 80, "1"},
		{50, 20, 80, "3"},
		{10, 20, 80, "2"},
		{80, 20, 80, "1"},
		{20, 20, 80, "3"},
	}
	for _, tt := range tests {
		color := tokenColorCode(tt.tokens, tt.low, tt.high)
		if color != tt.wantColor {
			t.Errorf("tokenColorCode(%d, %d, %d) = %q, want %q",
				tt.tokens, tt.low, tt.high, color, tt.wantColor)
		}
	}
}

func TestRenderBar(t *testing.T) {
	bar := renderTokenBar(50, 100, 10)
	if len([]rune(bar)) != 5 {
		t.Errorf("bar length: got %d, want 5", len([]rune(bar)))
	}
	bar = renderTokenBar(0, 100, 10)
	if bar != "" {
		t.Errorf("0 tokens bar: got %q, want empty", bar)
	}
	bar = renderTokenBar(50, 0, 10)
	if bar != "" {
		t.Errorf("maxTokens 0: got %q, want empty", bar)
	}
}

func TestCycleSort(t *testing.T) {
	delegate := analyzeSkillDelegate{}
	l := list.New(nil, delegate, 80, 20)
	m := &analyzeTUIModel{
		sortBy:  "tokens",
		sortAsc: false,
		list:    l,
	}

	m.cycleSort()
	if m.sortBy != "tokens" || !m.sortAsc {
		t.Errorf("step 1: got %s/%v", m.sortBy, m.sortAsc)
	}

	m.cycleSort()
	if m.sortBy != "name" || !m.sortAsc {
		t.Errorf("step 2: got %s/%v", m.sortBy, m.sortAsc)
	}

	m.cycleSort()
	if m.sortBy != "name" || m.sortAsc {
		t.Errorf("step 3: got %s/%v", m.sortBy, m.sortAsc)
	}

	m.cycleSort()
	if m.sortBy != "tokens" || m.sortAsc {
		t.Errorf("step 4: got %s/%v", m.sortBy, m.sortAsc)
	}
}

func TestSwitchTarget(t *testing.T) {
	delegate := analyzeSkillDelegate{}
	l := list.New(nil, delegate, 80, 20)
	m := &analyzeTUIModel{
		list:   l,
		sortBy: "tokens",
		groups: []analyzeTargetGroup{
			{
				entry: analyzeTargetEntry{
					Name:       "claude",
					SkillCount: 2,
					Skills: []analyzeSkillEntry{
						{Name: "big", DescriptionChars: 400, DescriptionTokens: 100, description: "big skill"},
						{Name: "small", DescriptionChars: 40, DescriptionTokens: 10, description: "small"},
					},
				},
				names: []string{"claude"},
			},
			{
				entry: analyzeTargetEntry{
					Name:       "cursor",
					SkillCount: 1,
					Skills: []analyzeSkillEntry{
						{Name: "only", DescriptionChars: 200, DescriptionTokens: 50, description: "only skill"},
					},
				},
				names: []string{"cursor"},
			},
		},
		groupIdx: 0,
	}

	m.switchTarget()

	if len(m.allItems) != 2 {
		t.Fatalf("allItems: got %d, want 2", len(m.allItems))
	}
	if m.matchCount != 2 {
		t.Errorf("matchCount: got %d, want 2", m.matchCount)
	}
	if m.thresholdLow == 0 && m.thresholdHigh == 0 {
		t.Error("thresholds not computed")
	}

	m.groupIdx = 1
	m.switchTarget()

	if len(m.allItems) != 1 {
		t.Fatalf("allItems after switch: got %d, want 1", len(m.allItems))
	}
	if m.matchCount != 1 {
		t.Errorf("matchCount after switch: got %d, want 1", m.matchCount)
	}
}
