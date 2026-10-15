// Package textdiff renders a line diff of two texts.
package textdiff

import (
	"strings"

	"github.com/sergi/go-diff/diffmatchpatch"
)

// Lines returns every line of the two texts: added lines prefixed "+ ",
// removed lines "- ", and unchanged lines "  ".
func Lines(old, new string) string {
	dmp := diffmatchpatch.New()
	a, b, lineArray := dmp.DiffLinesToChars(old, new)
	diffs := dmp.DiffMain(a, b, false)
	diffs = dmp.DiffCharsToLines(diffs, lineArray)
	diffs = dmp.DiffCleanupSemantic(diffs)
	return format(diffs)
}

// format writes diffmatchpatch.Diff slices as whole lines.
// Insert lines are prefixed with "+ ", Delete lines with "- ", Equal lines with "  ".
func format(diffs []diffmatchpatch.Diff) string {
	var b strings.Builder

	for _, d := range diffs {
		lines := strings.Split(d.Text, "\n")
		// If the text ends with \n, Split produces a trailing empty string.
		// Don't emit a line for that trailing empty element.
		if len(lines) > 0 && lines[len(lines)-1] == "" {
			lines = lines[:len(lines)-1]
		}

		var prefix string
		switch d.Type {
		case diffmatchpatch.DiffInsert:
			prefix = "+ "
		case diffmatchpatch.DiffDelete:
			prefix = "- "
		case diffmatchpatch.DiffEqual:
			prefix = "  "
		}

		for _, line := range lines {
			b.WriteString(prefix)
			b.WriteString(line)
			b.WriteString("\n")
		}
	}

	return b.String()
}
