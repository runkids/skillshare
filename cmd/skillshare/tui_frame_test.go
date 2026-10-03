package main

import (
	"strings"
	"testing"

	"github.com/charmbracelet/lipgloss"
	xansi "github.com/charmbracelet/x/ansi"
)

func TestFrameLines_FitTheWidthWhenTheTextIsLong(t *testing.T) {
	long := strings.Repeat("very-long-search-query ", 6)
	for name, line := range map[string]string{
		"title":   renderFrameTitle(40, "search", []string{long}, nil),
		"confirm": renderConfirmLine(40, "Install "+long+"?", false, "skillshare install x"),
		"filter":  renderFilterLine(40, "/ "+long, 3),
	} {
		if w := lipgloss.Width(line); w > 40 {
			t.Errorf("%s line is %d columns wide at width 40: %q", name, w, xansi.Strip(line))
		}
	}
}

func TestFrameTitle_KeepsTheRightSideWhenTruncating(t *testing.T) {
	line := xansi.Strip(renderFrameTitle(40, "search", []string{strings.Repeat("query ", 10)}, []frameTab{{"Skills 8", true}}))
	if !strings.HasSuffix(line, "Skills 8 ") {
		t.Fatalf("the active tab should stay at the end: %q", line)
	}
}
