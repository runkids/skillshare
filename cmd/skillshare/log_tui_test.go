package main

import "testing"

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
		wantMin   int // detail width must be >= 30
	}{
		{50, 30},   // 50-30-3=17 → clamped to min 30
		{70, 37},   // 70-30-3=37
		{100, 67},  // 100-30-3=67
		{120, 87},  // 120-30-3=87
		{160, 117}, // 160-40-3=117
	}
	for _, tt := range tests {
		got := logDetailPanelWidth(tt.termWidth)
		if got != tt.wantMin {
			t.Errorf("logDetailPanelWidth(%d) = %d, want %d", tt.termWidth, got, tt.wantMin)
		}
		if got < 30 {
			t.Errorf("logDetailPanelWidth(%d) = %d, should be >= 30", tt.termWidth, got)
		}
	}
}
