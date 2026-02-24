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
	logTimeWidth         = 16
	logCmdWidth          = 9
	logStatusWidth       = 7
	logDurationWidth     = 7
	logMinWrapWidth      = 24
)

var logANSIRegex = regexp.MustCompile(`\x1b\[[0-9;]*m`)

func printLogEntries(entries []oplog.Entry) {
	if ui.IsTTY() {
		printLogEntriesTTYTwoLine(os.Stdout, entries, logTerminalWidth())
		return
	}

	printLogEntriesNonTTY(os.Stdout, entries)
}

func printLogEntriesTTYTwoLine(w io.Writer, entries []oplog.Entry, termWidth int) {
	printLogTableHeaderTTY(w)

	for i, e := range entries {
		if i > 0 {
			fmt.Fprintln(w)
		}

		ts := formatLogTimestamp(e.Timestamp)
		cmd := padLogCell(strings.ToUpper(e.Command), logCmdWidth)
		status := colorizeLogStatusCell(padLogCell(e.Status, logStatusWidth), e.Status)
		dur := formatLogDuration(e.Duration)
		durCell := padLogCell(dur, logDurationWidth)

		fmt.Fprintf(w, "  %s%s%s | %s | %s | %s\n",
			ui.Gray,
			padLogCell(ts, logTimeWidth),
			ui.Reset,
			cmd,
			status,
			durCell,
		)

		printLogDetailMultiLine(w, e, termWidth)
	}
}

func printLogDetailMultiLine(w io.Writer, e oplog.Entry, termWidth int) {
	pairs := formatLogDetailPairs(e)

	if e.Message != "" {
		pairs = append(pairs, logDetailPair{key: "message", value: e.Message})
	}

	if len(pairs) == 0 {
		return
	}

	const indent = "  "
	const listIndent = "    - "
	wrapWidth := termWidth - logDisplayWidth(indent) - 20 // leave room for key
	if wrapWidth < logMinWrapWidth {
		wrapWidth = logMinWrapWidth
	}

	for _, p := range pairs {
		if p.isList && len(p.listValues) > 0 {
			fmt.Fprintf(w, "%s%s%s:%s\n", indent, ui.Gray, p.key, ui.Reset)
			for _, v := range p.listValues {
				fmt.Fprintf(w, "%s%s%s%s%s\n", indent, ui.Gray, listIndent, ui.Reset, v)
			}
		} else if p.value != "" {
			fmt.Fprintf(w, "%s%s%s:%s %s\n", indent, ui.Gray, p.key, ui.Reset, p.value)
		}
	}
}

func printLogEntriesNonTTY(w io.Writer, entries []oplog.Entry) {
	for _, e := range entries {
		ts := formatLogTimestamp(e.Timestamp)
		detail := formatLogDetail(e, true)
		dur := formatLogDuration(e.Duration)

		fmt.Fprintf(w, "  %s  %-9s  %-96s  %-7s  %s\n",
			ts, e.Command, detail, e.Status, dur)

		printLogAuditSkillLinesNonTTY(w, e)
	}
}

func printLogAuditSkillLinesNonTTY(w io.Writer, e oplog.Entry) {
	if e.Command != "audit" || e.Args == nil {
		return
	}

	if failedSkills, ok := logArgStringSlice(e.Args, "failed_skills"); ok && len(failedSkills) > 0 {
		printLogNamedSkillsNonTTY(w, "failed skills", failedSkills)
	}
	if warningSkills, ok := logArgStringSlice(e.Args, "warning_skills"); ok && len(warningSkills) > 0 {
		printLogNamedSkillsNonTTY(w, "warning skills", warningSkills)
	}
	if lowSkills, ok := logArgStringSlice(e.Args, "low_skills"); ok && len(lowSkills) > 0 {
		printLogNamedSkillsNonTTY(w, "low skills", lowSkills)
	}
	if infoSkills, ok := logArgStringSlice(e.Args, "info_skills"); ok && len(infoSkills) > 0 {
		printLogNamedSkillsNonTTY(w, "info skills", infoSkills)
	}
}

func printLogNamedSkillsNonTTY(w io.Writer, label string, skills []string) {
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
		fmt.Fprintf(w, "                     -> %s: %s\n", currentLabel, strings.Join(skills[i:end], ", "))
	}
}

func printLogTableHeaderTTY(w io.Writer) {
	header := fmt.Sprintf("  %-16s | %-9s | %-7s | %-7s", "TIME", "CMD", "STATUS", "DUR")
	separator := fmt.Sprintf(
		"  %s-+-%s-+-%s-+-%s",
		strings.Repeat("-", logTimeWidth),
		strings.Repeat("-", logCmdWidth),
		strings.Repeat("-", logStatusWidth),
		strings.Repeat("-", logDurationWidth),
	)

	fmt.Fprintf(w, "%s%s%s\n", ui.Cyan, header, ui.Reset)
	fmt.Fprintf(w, "%s%s%s\n", ui.Gray, separator, ui.Reset)
}

func colorizeLogStatusCell(cell, status string) string {
	switch status {
	case "ok":
		return ui.Green + cell + ui.Reset
	case "error":
		return ui.Red + cell + ui.Reset
	case "partial":
		return ui.Yellow + cell + ui.Reset
	case "blocked":
		return ui.Red + cell + ui.Reset
	default:
		return cell
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
