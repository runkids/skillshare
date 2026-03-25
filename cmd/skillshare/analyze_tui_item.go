package main

import (
	"fmt"
	"io"
	"sort"
	"strings"

	"github.com/charmbracelet/bubbles/list"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// analyzeSkillItem wraps analyzeSkillEntry for the bubbles/list delegate.
type analyzeSkillItem struct {
	entry     analyzeSkillEntry
	maxTokens int // largest desc tokens in current target (for bar scaling)
}

func (i analyzeSkillItem) FilterValue() string { return i.entry.Name }

// computeThresholds returns P25 and P75 percentile values from a sorted token slice.
func computeThresholds(tokens []int) (low, high int) {
	n := len(tokens)
	if n == 0 {
		return 0, 0
	}
	sorted := make([]int, n)
	copy(sorted, tokens)
	sort.Ints(sorted)
	if n == 1 {
		return sorted[0], sorted[0]
	}
	low = sorted[n/4]
	high = sorted[n*3/4]
	return low, high
}

// tokenColorCode returns lipgloss color code based on thresholds.
func tokenColorCode(tokens, low, high int) string {
	if tokens >= high {
		return "1" // red
	}
	if tokens >= low {
		return "3" // yellow
	}
	return "2" // green
}

// renderTokenBar renders a proportional bar chart using block characters.
func renderTokenBar(tokens, maxTokens, maxWidth int) string {
	if maxTokens <= 0 || tokens <= 0 || maxWidth <= 0 {
		return ""
	}
	width := tokens * maxWidth / maxTokens
	if width < 1 {
		width = 1
	}
	return strings.Repeat("█", width)
}

// analyzeSkillDelegate renders each skill row in the list.
type analyzeSkillDelegate struct {
	thresholdLow  int
	thresholdHigh int
}

func (d analyzeSkillDelegate) Height() int                             { return 1 }
func (d analyzeSkillDelegate) Spacing() int                            { return 0 }
func (d analyzeSkillDelegate) Update(_ tea.Msg, _ *list.Model) tea.Cmd { return nil }

func (d analyzeSkillDelegate) Render(w io.Writer, m list.Model, index int, listItem list.Item) {
	item, ok := listItem.(analyzeSkillItem)
	if !ok {
		return
	}

	isSelected := index == m.Index()
	name := truncateName(item.entry.Name, analyzeNameMaxLen)
	tokenStr := formatTokensStr(item.entry.DescriptionChars)

	colorCode := tokenColorCode(item.entry.DescriptionTokens, d.thresholdLow, d.thresholdHigh)
	dot := lipgloss.NewStyle().Foreground(lipgloss.Color(colorCode)).Render("●")

	// Bar chart: use remaining width after name + token columns
	barMaxWidth := 12
	bar := renderTokenBar(item.entry.DescriptionTokens, item.maxTokens, barMaxWidth)
	barStyled := lipgloss.NewStyle().Foreground(lipgloss.Color(colorCode)).Render(bar)

	line := fmt.Sprintf("  %s %-32s %8s  %s", dot, name, tokenStr, barStyled)

	if isSelected {
		line = tc.ListRowSelected.Render(line)
	}

	fmt.Fprint(w, line)
}
