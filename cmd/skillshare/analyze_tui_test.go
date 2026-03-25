package main

import "testing"

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
