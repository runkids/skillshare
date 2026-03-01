package main

import (
	"fmt"
	"strings"
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
