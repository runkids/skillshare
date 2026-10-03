package main

import (
	"io"
	"sort"
	"strings"

	"skillshare/internal/theme"

	"github.com/charmbracelet/bubbles/list"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	ssync "skillshare/internal/sync"
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

// Pre-computed colored dots to avoid lipgloss.NewStyle() allocation per Render() call.
var analyzeDots = map[string]string{
	"1": lipgloss.NewStyle().Foreground(lipgloss.Color("1")).Render("●"), // red
	"2": lipgloss.NewStyle().Foreground(lipgloss.Color("2")).Render("●"), // green
	"3": lipgloss.NewStyle().Foreground(lipgloss.Color("3")).Render("●"), // yellow
}

// Pre-computed lint icons (same pattern as analyzeDots).
var (
	lintIconError   = theme.Danger().Render("✗") + " "
	lintIconWarning = theme.Warning().Render("!") + " "
)

func lintIcon(issues []ssync.LintIssue) string {
	if len(issues) == 0 {
		return ""
	}
	for _, issue := range issues {
		if issue.Severity == ssync.LintError {
			return lintIconError
		}
	}
	return lintIconWarning
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

	width := m.Width()
	tokenStr := formatTokensStr(item.entry.DescriptionTokens)
	dot := analyzeDots[tokenColorCode(item.entry.DescriptionTokens, d.thresholdLow, d.thresholdHigh)]
	left := dot + " " + lintIcon(item.entry.LintIssues) + item.entry.Name
	renderPrefixRow(w, alignRow(left, theme.Dim().Render(tokenStr), width-rowIndent), width, index == m.Index())
}
