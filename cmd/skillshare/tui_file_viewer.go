package main

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"

	"skillshare/internal/theme"
)

// fileViewer is what the file viewers of list and extras show: the files
// of a skill or extra on the left and the open file on the right, in the
// shared frame. An agent is one file, so it has no tree.
type fileViewer struct {
	command, name  string
	nodes          []treeNode
	cursor, scroll int // tree cursor and first visible tree row
	content        string
	contentScroll  int
	noTree         bool
	note           string    // the line above the keys, e.g. what a finding says
	extraHints     []keyHint // keys of the screen that opened the viewer
}

// fileViewerHeight is how many lines the tree and the file get.
func fileViewerHeight(termHeight int) int {
	return max(termHeight-frameChrome, 5)
}

// fileViewerTextWidth is how wide the open file is drawn: the pane's width
// after its padding, so a narrow terminal wraps the text instead of
// clipping it.
func fileViewerTextWidth(termWidth int, noTree bool) int {
	if noTree {
		return max(termWidth-3, 10)
	}
	return max(termWidth-sidebarWidth(termWidth)-3, 10)
}

func renderFileViewer(width, height int, v fileViewer) string {
	facts := []string{printableName(v.name)}
	if !v.noTree && v.cursor < len(v.nodes) {
		facts = append(facts, printableName(v.nodes[v.cursor].relPath))
	}
	title := renderFrameTitle(width, v.command, facts, nil)
	bodyHeight := fileViewerHeight(height)
	text, pos := scrollLines(v.content, v.contentScroll, bodyHeight)

	hints := append(append([]keyHint{}, v.extraHints...), keyHint{"ctrl+d/u", "scroll"}, keyHint{"g/G", "top/bottom"}, keyHint{"esc", "back"})
	var body string
	if v.noTree {
		body = lipgloss.NewStyle().PaddingLeft(1).Height(bodyHeight).MaxHeight(bodyHeight).Render(text)
	} else {
		sw := sidebarWidth(width)
		tree := renderFileTree(v.nodes, v.cursor, v.scroll, sw-1, bodyHeight)
		body = renderFrameSplit(tree, text, sw, width-sw, bodyHeight)
		hints = append([]keyHint{{"↑↓", "files"}, {"→", "expand"}, {"←", "collapse"}}, hints...)
	}
	note := truncateANSI("  "+v.note, width)
	return title + "\n\n" + body + "\n" + note + "\n" + renderKeyLine(width, hints, pos)
}

// renderFileTree draws the visible rows of the tree; the open file or
// folder is accent-colored.
func renderFileTree(nodes []treeNode, cursor, scroll, width, height int) string {
	if len(nodes) == 0 {
		return " " + theme.Dim().Render("No files")
	}
	start := max(min(scroll, len(nodes)-height), 0)
	end := min(start+height, len(nodes))
	lines := make([]string, 0, end-start)
	for i := start; i < end; i++ {
		n := nodes[i]
		mark, name := "  ", n.name
		if n.isDir {
			mark, name = "▸ ", name+"/"
			if n.expanded {
				mark = "▾ "
			}
		}
		label := truncateANSI(" "+strings.Repeat("  ", n.depth)+mark+printableName(name), width)
		switch {
		case i == cursor:
			label = theme.Accent().Bold(true).Render(label)
		case n.isDir:
			label = theme.Dim().Render(label)
		}
		lines = append(lines, label)
	}
	return strings.Join(lines, "\n")
}

// scrollLines returns height lines of text from offset, and the position
// ("3/12") when the text is longer than height.
func scrollLines(text string, offset, height int) (string, string) {
	lines := strings.Split(text, "\n")
	if len(lines) <= height {
		return text, ""
	}
	maxScroll := len(lines) - height
	offset = min(offset, maxScroll)
	return strings.Join(lines[offset:offset+height], "\n"), framePosition(offset+1, maxScroll+1)
}

// printableText swaps control characters in a file's text for visible
// symbols of the same width (ESC shows as ␛), so a skill under review
// cannot move the cursor or send escape sequences to the terminal.
// Newlines and tabs are kept. The zero-width and bidirectional characters
// the audit flags are named, e.g. <U+202E>, so they cannot hide or reorder
// the text around them.
func printableText(s string) string {
	var b strings.Builder
	for _, r := range strings.ReplaceAll(s, "\r\n", "\n") {
		switch {
		case r == '\n' || r == '\t':
			b.WriteRune(r)
		case r < 0x20:
			b.WriteRune(0x2400 + r)
		case r == 0x7f:
			b.WriteRune('␡')
		case r >= 0x80 && r < 0xa0:
			b.WriteRune('\ufffd')
		case r >= 0x200b && r <= 0x200d, r == 0x2060, r == 0xfeff, // zero-width
			r >= 0x202a && r <= 0x202e, r >= 0x2066 && r <= 0x2069: // bidirectional
			fmt.Fprintf(&b, "<U+%04X>", r)
		default:
			b.WriteRune(r)
		}
	}
	return b.String()
}

// printableName is printableText for a name that must stay on one line,
// such as a file name from a skill under review.
func printableName(s string) string {
	return strings.NewReplacer("\n", "␤", "\t", "␉").Replace(printableText(s))
}
