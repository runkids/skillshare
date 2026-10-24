// Package textdiff renders a line diff of two texts.
package textdiff

import (
	"os"
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

// maxDiffFileSize is the maximum file size for generating unified diffs.
// Files larger than this are skipped to avoid performance issues.
const maxDiffFileSize = 1_000_000 // 1MB

// Files renders Lines for the files at oldPath and newPath. A missing file
// reads as empty. Returns "(binary file differs)" if either file contains null
// bytes in the first 8000 chars, and "(file too large for diff)" if either
// file exceeds maxDiffFileSize.
func Files(oldPath, newPath string) string {
	if fileExceedsSize(oldPath, maxDiffFileSize) || fileExceedsSize(newPath, maxDiffFileSize) {
		return "(file too large for diff)"
	}

	oldContent := readFileString(oldPath)
	newContent := readFileString(newPath)

	if isBinary(oldContent) || isBinary(newContent) {
		return "(binary file differs)"
	}

	return Lines(oldContent, newContent)
}

// readFileString reads a file and returns its content as a string.
// Returns "" on error.
func readFileString(path string) string {
	data, err := os.ReadFile(path)
	if err != nil {
		return ""
	}
	return string(data)
}

// fileExceedsSize returns true if the file at path is larger than limit bytes.
// Returns false if the file doesn't exist or can't be stat'd.
func fileExceedsSize(path string, limit int64) bool {
	info, err := os.Stat(path)
	if err != nil {
		return false
	}
	return info.Size() > limit
}

// isBinary checks whether a string contains null bytes in the first 8000 characters.
func isBinary(content string) bool {
	limit := len(content)
	if limit > 8000 {
		limit = 8000
	}
	for i := 0; i < limit; i++ {
		if content[i] == 0 {
			return true
		}
	}
	return false
}
