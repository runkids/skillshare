package main

import (
	"fmt"
	"io"
	"os"
	"regexp"
	"strings"

	"github.com/mattn/go-runewidth"
	"github.com/pterm/pterm"

	"skillshare/internal/oplog"
	"skillshare/internal/ui"
)

const (
	logDetailTruncateLen = 96
	logMinWrapWidth      = 24
)

var logANSIRegex = regexp.MustCompile(`\x1b\[[0-9;]*m`)

// printLogEntries prints one row per entry: a status mark, the command,
// and when it ran with how long it took. Details follow on dim lines,
// wrapped on a terminal and on one truncated line otherwise.
func printLogEntries(entries []oplog.Entry) {
	printLogEntriesTo(os.Stdout, entries, ui.IsTTY(), logTerminalWidth())
}

func printLogEntriesTo(w io.Writer, entries []oplog.Entry, tty bool, termWidth int) {
	names := make([]string, len(entries))
	for i, e := range entries {
		names[i] = e.Command
	}
	width := ui.RowWidth(names...)
	indent := strings.Repeat(" ", width+4)
	for i, e := range entries {
		if tty && i > 0 {
			fmt.Fprintln(w)
		}
		value := formatLogTimestamp(e.Timestamp)
		if d := formatLogDuration(e.Duration); d != "" {
			value += ui.DimText(" · " + d)
		}
		if e.Status != "ok" {
			value += ui.DimText(" · " + e.Status)
		}
		fmt.Fprintf(w, "%s %s  %s\n", ui.StyledMark(logStatusMark(e.Status)), padLogCell(e.Command, width), value)
		if tty {
			printLogDetailMultiLine(w, e, termWidth, indent)
			continue
		}
		if detail := formatLogDetail(e, true); detail != "" {
			fmt.Fprintln(w, indent+detail)
		}
		printLogAuditSkillLinesNonTTY(w, e, indent)
	}
}

// logStatusMark maps an entry status to ✓, ! or ✗.
func logStatusMark(status string) string {
	switch status {
	case "ok":
		return ui.MarkOK
	case "error", "blocked":
		return ui.MarkFail
	default:
		return ui.MarkWarn
	}
}

func printLogDetailMultiLine(w io.Writer, e oplog.Entry, termWidth int, indent string) {
	pairs := formatLogDetailPairs(e)

	if e.Message != "" {
		pairs = append(pairs, logDetailPair{key: "message", value: e.Message})
	}

	if len(pairs) == 0 {
		return
	}

	const listIndent = "  - "
	wrapWidth := termWidth - logDisplayWidth(indent) - 20 // leave room for key
	if wrapWidth < logMinWrapWidth {
		wrapWidth = logMinWrapWidth
	}

	const maxBulletItems = 5 // plain text is not scrollable — keep compact
	for _, p := range pairs {
		if p.isList && len(p.listValues) > 0 {
			fmt.Fprintf(w, "%s%s%s:%s\n", indent, ui.Dim, p.key, ui.Reset)
			show := p.listValues
			remaining := 0
			if len(show) > maxBulletItems {
				remaining = len(show) - maxBulletItems
				show = show[:maxBulletItems]
			}
			for _, v := range show {
				fmt.Fprintf(w, "%s%s%s%s%s\n", indent, ui.Dim, listIndent, ui.Reset, v)
			}
			if remaining > 0 {
				fmt.Fprintf(w, "%s%s%s... and %d more%s\n", indent, ui.Dim, listIndent, remaining, ui.Reset)
			}
		} else if p.value != "" {
			fmt.Fprintf(w, "%s%s%s:%s %s\n", indent, ui.Dim, p.key, ui.Reset, p.value)
		}
	}
}

func printLogAuditSkillLinesNonTTY(w io.Writer, e oplog.Entry, indent string) {
	if e.Command != "audit" || e.Args == nil {
		return
	}

	if failedSkills, ok := logArgStringSlice(e.Args, "failed_skills"); ok && len(failedSkills) > 0 {
		printLogNamedSkillsNonTTY(w, indent, "failed skills", failedSkills)
	}
	if warningSkills, ok := logArgStringSlice(e.Args, "warning_skills"); ok && len(warningSkills) > 0 {
		printLogNamedSkillsNonTTY(w, indent, "warning skills", warningSkills)
	}
	if lowSkills, ok := logArgStringSlice(e.Args, "low_skills"); ok && len(lowSkills) > 0 {
		printLogNamedSkillsNonTTY(w, indent, "low skills", lowSkills)
	}
	if infoSkills, ok := logArgStringSlice(e.Args, "info_skills"); ok && len(infoSkills) > 0 {
		printLogNamedSkillsNonTTY(w, indent, "info skills", infoSkills)
	}
}

func printLogNamedSkillsNonTTY(w io.Writer, indent, label string, skills []string) {
	const namesPerLine = 4
	for i := 0; i < len(skills); i += namesPerLine {
		end := i + namesPerLine
		if end > len(skills) {
			end = len(skills)
		}

		currentLabel := label
		if i > 0 {
			currentLabel = label + " (cont)"
		}
		fmt.Fprintf(w, "%s%s: %s\n", indent, currentLabel, strings.Join(skills[i:end], ", "))
	}
}

func logTerminalWidth() int {
	width := pterm.GetTerminalWidth()
	if width > 0 {
		return width
	}
	return 120
}

func padLogCell(value string, width int) string {
	current := logDisplayWidth(value)
	if current >= width {
		return value
	}
	return value + strings.Repeat(" ", width-current)
}

func logDisplayWidth(s string) int {
	return runewidth.StringWidth(logANSIRegex.ReplaceAllString(s, ""))
}
