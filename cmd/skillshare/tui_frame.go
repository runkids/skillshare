package main

import (
	"fmt"
	"strings"

	"skillshare/internal/theme"

	"github.com/charmbracelet/lipgloss"
)

// The frame every full-screen TUI shares: a title line with the command,
// its scope and counts on the left and the tabs on the right; the body,
// with the list on the left and details on the right and no lines between
// them; and one key line at the bottom with the position on the right.
// Confirmations and the filter input take over the key line in place, and
// ? swaps the details for the full key list.

// keyHint is one key and what it does, e.g. {"d", "uninstall"}.
type keyHint struct{ key, desc string }

// keyGroup is a titled block of keys in the ? panel.
type keyGroup struct {
	title string
	keys  []keyHint
}

// frameTab is one tab on the title line, e.g. {"Skills 8", true}.
type frameTab struct {
	label  string
	active bool
}

// frameChrome is how many lines the frame adds around the body: the title
// line, the blank line under it, the note line and the key line.
const frameChrome = 4

// renderFrameTitle renders "skillshare <command> · fact · fact" with the
// tabs right-aligned.
func renderFrameTitle(width int, command string, facts []string, tabs []frameTab) string {
	left := " " + theme.Primary().Bold(true).Render("skillshare "+command)
	for _, f := range facts {
		left += theme.Dim().Render(" · " + f)
	}
	labels := make([]string, len(tabs))
	for i, t := range tabs {
		if t.active {
			labels[i] = theme.Accent().Bold(true).Render(t.label)
		} else {
			labels[i] = theme.Dim().Render(t.label)
		}
	}
	return joinEnds(left, strings.Join(labels, "   ")+" ", width)
}

// renderKeyLine renders the common keys with right (e.g. "3/8") at the end.
// When the line is too wide it drops keys from the end, but keeps the last
// one, which is "? keys" where there is one.
func renderKeyLine(width int, hints []keyHint, right string) string {
	right = theme.Dim().Render(right) + " "
	left := "  " + joinKeyHints(hints)
	for len(hints) > 1 && lipgloss.Width(left)+lipgloss.Width(right)+2 > width {
		hints = append(hints[:len(hints)-2:len(hints)-2], hints[len(hints)-1])
		left = "  " + joinKeyHints(hints)
	}
	return joinEnds(left, right, width)
}

// renderConfirmLine asks a yes/no question in place of the key line;
// right is shown dim at the end, e.g. the command that will run.
func renderConfirmLine(width int, question string, danger bool, right string) string {
	style := theme.Primary().Bold(true)
	if danger {
		style = theme.Danger().Bold(true)
	}
	left := "  " + theme.Warning().Render("?") + " " + style.Render(question) + "  " +
		joinKeyHints([]keyHint{{"y", "yes"}, {"n", "no"}})
	return joinEnds(left, theme.Dim().Render(right)+" ", width)
}

func joinKeyHints(hints []keyHint) string {
	parts := make([]string, len(hints))
	for i, h := range hints {
		parts[i] = theme.Primary().Render(h.key) + " " + theme.Dim().Render(h.desc)
	}
	return strings.Join(parts, theme.Dim().Render(" · "))
}

// renderKeysPanel renders the ? panel: every key, grouped.
func renderKeysPanel(groups []keyGroup) string {
	keyWidth := 0
	for _, g := range groups {
		for _, k := range g.keys {
			keyWidth = max(keyWidth, lipgloss.Width(k.key))
		}
	}
	var blocks []string
	for _, g := range groups {
		lines := []string{theme.Primary().Bold(true).Render(g.title)}
		for _, k := range g.keys {
			lines = append(lines, fmt.Sprintf("%-*s  %s", keyWidth, k.key, theme.Dim().Render(k.desc)))
		}
		blocks = append(blocks, strings.Join(lines, "\n"))
	}
	return strings.Join(blocks, "\n\n")
}

// renderFrameSplit places left and right side by side with a gap instead of
// a divider line.
func renderFrameSplit(left, right string, leftWidth, rightWidth, height int) string {
	l := lipgloss.NewStyle().Width(leftWidth).MaxWidth(leftWidth).Height(height).MaxHeight(height).Render(left)
	r := lipgloss.NewStyle().Width(rightWidth).MaxWidth(rightWidth).Height(height).MaxHeight(height).PaddingLeft(2).Render(right)
	return lipgloss.JoinHorizontal(lipgloss.Top, l, r)
}

// framePosition renders "3/8", or "" for an empty list.
func framePosition(current, total int) string {
	if total == 0 {
		return ""
	}
	return formatNumber(current) + "/" + formatNumber(total)
}

// joinEnds pads between left and right so the line spans width.
func joinEnds(left, right string, width int) string {
	gap := width - lipgloss.Width(left) - lipgloss.Width(right)
	if gap < 2 {
		gap = 2
	}
	return left + strings.Repeat(" ", gap) + right
}
