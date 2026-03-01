package main

import (
	"fmt"
	"strings"
	"time"

	"github.com/charmbracelet/lipgloss"
)

// applyDetailScroll applies vertical scrolling to detail panel content.
// detailScroll is the current scroll offset; viewHeight is the visible line count.
// Returns the visible portion with a scroll indicator when needed.
func applyDetailScroll(content string, detailScroll, viewHeight int) string {
	lines := strings.Split(content, "\n")

	maxDetailLines := viewHeight
	if maxDetailLines < 5 {
		maxDetailLines = 5
	}

	totalLines := len(lines)
	if totalLines <= maxDetailLines {
		return content
	}

	maxScroll := totalLines - maxDetailLines
	offset := min(detailScroll, maxScroll)

	end := min(offset+maxDetailLines, totalLines)
	visible := lines[offset:end]

	var b strings.Builder
	for _, line := range visible {
		b.WriteString(line)
		b.WriteString("\n")
	}

	if offset > 0 || offset < maxScroll {
		indicator := fmt.Sprintf("── Ctrl+d/u scroll (%d/%d) ──", offset+1, maxScroll+1)
		b.WriteString(tc.Dim.Render(indicator))
		b.WriteString("\n")
	}

	return b.String()
}

// formatDurationShort returns a compact human-readable duration string.
// Covers: just now, minutes, hours, days, months, years.
func formatDurationShort(d time.Duration) string {
	switch {
	case d < time.Minute:
		return "just now"
	case d < time.Hour:
		return fmt.Sprintf("%dm", int(d.Minutes()))
	case d < 24*time.Hour:
		return fmt.Sprintf("%dh", int(d.Hours()))
	case d < 30*24*time.Hour:
		return fmt.Sprintf("%dd", int(d.Hours()/24))
	default:
		months := int(d.Hours() / 24 / 30)
		if months < 12 {
			return fmt.Sprintf("%dmo", months)
		}
		return fmt.Sprintf("%dy", months/12)
	}
}

// Minimum terminal width for horizontal split; below this use vertical layout.
const tuiMinSplitWidth = 80

// renderHorizontalSplit renders a left-right split with a vertical border column.
// leftContent and rightContent are pre-rendered strings.
func renderHorizontalSplit(leftContent, rightContent string, leftWidth, rightWidth, panelHeight int) string {
	leftPanel := lipgloss.NewStyle().
		Width(leftWidth).MaxWidth(leftWidth).
		Height(panelHeight).MaxHeight(panelHeight).
		Render(leftContent)

	borderStyle := tc.Border.
		Height(panelHeight).MaxHeight(panelHeight)
	borderCol := strings.Repeat("│\n", panelHeight)
	borderPanel := borderStyle.Render(strings.TrimRight(borderCol, "\n"))

	rightPanel := lipgloss.NewStyle().
		Width(rightWidth).MaxWidth(rightWidth).
		Height(panelHeight).MaxHeight(panelHeight).
		PaddingLeft(1).
		Render(rightContent)

	return lipgloss.JoinHorizontal(lipgloss.Top, leftPanel, borderPanel, rightPanel)
}

// truncateStr truncates a string to maxLen, appending "..." if needed.
func truncateStr(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	if maxLen <= 3 {
		return s[:maxLen]
	}
	return s[:maxLen-3] + "..."
}
