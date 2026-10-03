package ui

import (
	"fmt"
	"time"

	"skillshare/internal/theme"

	"github.com/charmbracelet/lipgloss"
	"github.com/mattn/go-runewidth"
)

// Marks for result rows. Plain information takes no mark.
const (
	MarkOK   = "✓"
	MarkWarn = "!"
	MarkFail = "✗"
	MarkNone = " "
)

// Row prints one result line, "✓ label  value", padding the label to width
// so the values of neighboring rows line up two spaces after the longest.
func Row(mark, label, value string, width int) {
	pad := width - lipgloss.Width(label)
	if pad < 0 {
		pad = 0
	}
	fmt.Printf("%s %s%*s  %s\n", StyledMark(mark), label, pad, "", value)
}

// RowWidth is the label width that lines up rows with these labels.
func RowWidth(labels ...string) int {
	width := answerLabelWidth
	for _, l := range labels {
		width = max(width, lipgloss.Width(l))
	}
	return width
}

// StyledMark colors a row mark: green ✓, yellow !, red ✗.
func StyledMark(mark string) string {
	switch mark {
	case MarkOK:
		return theme.Success().Render(mark)
	case MarkWarn:
		return theme.Warning().Render(mark)
	case MarkFail:
		return theme.Danger().Render(mark)
	}
	return mark
}

// Section starts a block of rows with a bold name after a blank line.
func Section(name string) {
	fmt.Println()
	fmt.Println(theme.Primary().Bold(true).Render(name))
}

// Done prints a command's closing line: a mark (none for MarkNone), the
// result in bold and how long it took, when that is known.
func Done(mark, text string, took time.Duration) {
	line := theme.Primary().Bold(true).Render(text)
	if mark != MarkNone {
		line = StyledMark(mark) + " " + line
	}
	fmt.Println(line + Took(took))
}

// DryRun closes a dry run.
func DryRun() {
	fmt.Println(theme.Dim().Render("Dry run — nothing was written"))
}

// Note prints a dim line under the row it explains.
func Note(text string) {
	fmt.Println("  " + theme.Dim().Render(text))
}

// Next prints commands to try next as pairs of command and note.
func Next(pairs ...string) {
	if len(pairs) == 0 {
		return
	}
	fmt.Println()
	fmt.Println(theme.Primary().Bold(true).Render("Next"))
	width := 0
	for i := 0; i < len(pairs); i += 2 {
		width = max(width, runewidth.StringWidth(pairs[i]))
	}
	for i := 0; i+1 < len(pairs); i += 2 {
		fmt.Printf("  %s  %s\n", theme.Accent().Render(fmt.Sprintf("%-*s", width, pairs[i])), theme.Dim().Render(pairs[i+1]))
	}
}

// Took is a dim " · 0.4s" after a result, left out under 0.05s, where it
// would read 0.0s.
func Took(d time.Duration) string {
	if d < 50*time.Millisecond {
		return ""
	}
	return " " + DimText(fmt.Sprintf("· %.1fs", d.Seconds()))
}
