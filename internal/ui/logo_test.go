package ui

import (
	"bytes"
	"strings"
	"testing"
	"time"

	"github.com/charmbracelet/lipgloss"
	"github.com/muesli/termenv"
)

func TestWordmarkColors_FinalFrameIsTwoTone(t *testing.T) {
	colors := wordmarkColors(1)
	skill, share := firstPixel(t, colors, 0), firstPixel(t, colors, wordmarkSplit())
	if *skill != wordmarkSkillColor() {
		t.Errorf("SKILL at progress 1 = %v, want %v", *skill, wordmarkSkillColor())
	}
	if *share != wordmarkShareColor() {
		t.Errorf("SHARE at progress 1 = %v, want %v", *share, wordmarkShareColor())
	}
}

func TestWordmarkColors_FirstFrameIsNotYetColored(t *testing.T) {
	if got := firstPixel(t, wordmarkColors(0), 0); *got == wordmarkSkillColor() {
		t.Errorf("SKILL at progress 0 = %v, want the grey outline", *got)
	}
}

func TestWordmarkFrame_IsThreeLinesHigh(t *testing.T) {
	if got := len(wordmarkFrame(1)); got != 3 {
		t.Errorf("wordmark has %d lines, want 3", got)
	}
}

func TestLogoColorsSupported_NeedsAtLeast256Colors(t *testing.T) {
	orig := lipgloss.ColorProfile()
	t.Cleanup(func() { lipgloss.SetColorProfile(orig) })

	for profile, want := range map[termenv.Profile]bool{
		termenv.TrueColor: true,
		termenv.ANSI256:   true,
		termenv.ANSI:      false,
	} {
		lipgloss.SetColorProfile(profile)
		if got := logoColorsSupported(); got != want {
			t.Errorf("profile %v: logoColorsSupported() = %v, want %v", profile, got, want)
		}
	}
}

func TestPlayLogo_HidesCursorWhileAnimating(t *testing.T) {
	var out bytes.Buffer
	playLogo(&out, func(time.Duration) {})

	got := out.String()
	if !strings.HasPrefix(got, "\x1b[?25l") {
		t.Errorf("animation must hide the cursor before the first frame; starts with %q", got[:min(len(got), 12)])
	}
	if !strings.HasSuffix(got, "\x1b[?25h") {
		t.Errorf("animation must show the cursor again at the end; ends with %q", got[max(0, len(got)-12):])
	}
}

// firstPixel returns the top-most drawn pixel in the first column at or after x.
func firstPixel(t *testing.T, colors [][]*rgb, from int) *rgb {
	t.Helper()
	for x := from; x < len(colors[0]); x++ {
		for y := range colors {
			if colors[y][x] != nil {
				return colors[y][x]
			}
		}
	}
	t.Fatalf("no drawn pixel at or after column %d", from)
	return nil
}
