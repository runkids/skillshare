package main

import "testing"

func TestLogListWidth(t *testing.T) {
	tests := []struct {
		termWidth int
		want      int
	}{
		{50, 30},  // 50*2/5=20 → clamped to min 30
		{70, 30},  // 70*2/5=28 → clamped to min 30
		{80, 32},  // 80*2/5=32
		{100, 40}, // 100*2/5=40
		{120, 48}, // 120*2/5=48
		{160, 60}, // 160*2/5=64 → clamped to max 60
		{200, 60}, // 200*2/5=80 → clamped to max 60
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
		{50, 30},  // 50-30-3=17 → clamped to min 30
		{70, 37},  // 70-30-3=37
		{100, 57}, // 100-40-3=57
		{120, 69}, // 120-48-3=69
		{160, 97}, // 160-60-3=97
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
