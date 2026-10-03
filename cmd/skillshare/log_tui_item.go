package main

import (
	"fmt"
	"io"
	"strings"

	"skillshare/internal/oplog"
	"skillshare/internal/theme"
	"skillshare/internal/ui"

	"github.com/charmbracelet/bubbles/list"
	tea "github.com/charmbracelet/bubbletea"
)

// logItem wraps oplog.Entry to implement bubbles/list.Item interface.
type logItem struct {
	entry  oplog.Entry
	source string // "operations" or "audit" — identifies which log file
	marked bool   // true = selected for deletion
}

// Title returns a one-line header: [2024-01-15 14:30] SYNC — ok
// Plain text only — embedded ANSI breaks bubbletea's DefaultDelegate truncation.
// When marked for deletion, a ✗ prefix is prepended.
func (i logItem) Title() string {
	ts := formatLogTimestamp(i.entry.Timestamp)
	title := fmt.Sprintf("[%s] %s — %s",
		ts,
		strings.ToUpper(i.entry.Command),
		i.entry.Status,
	)
	if i.marked {
		return "✗ " + title
	}
	return title
}

// Description returns duration + one-line summary for the list delegate.
func (i logItem) Description() string {
	var parts []string

	if dur := formatLogDuration(i.entry.Duration); dur != "" {
		parts = append(parts, dur)
	}
	if detail := formatLogDetail(i.entry, true); detail != "" {
		parts = append(parts, detail)
	}

	if len(parts) == 0 {
		return ""
	}
	return strings.Join(parts, " | ")
}

// FilterValue returns searchable text for bubbletea's built-in fuzzy filter.
func (i logItem) FilterValue() string {
	parts := []string{
		formatLogTimestamp(i.entry.Timestamp),
		i.entry.Command,
		i.entry.Status,
		i.source,
	}
	if i.entry.Message != "" {
		parts = append(parts, i.entry.Message)
	}
	if detail := formatLogDetail(i.entry, false); detail != "" {
		parts = append(parts, detail)
	}
	return strings.Join(parts, " ")
}

// logDelegate renders "○ 01-02 15:04  command  ✓".
type logDelegate struct{}

func (logDelegate) Height() int                             { return 1 }
func (logDelegate) Spacing() int                            { return 0 }
func (logDelegate) Update(_ tea.Msg, _ *list.Model) tea.Cmd { return nil }

func (logDelegate) Render(w io.Writer, m list.Model, index int, li list.Item) {
	item, ok := li.(logItem)
	if !ok {
		return
	}
	mark := theme.Dim().Render("○")
	if item.marked {
		mark = theme.Accent().Render("◉")
	}
	when := formatLogTimestamp(item.entry.Timestamp)
	if len(when) == len("2006-01-02 15:04") {
		when = when[5:] // the year is noise in a list sorted by time
	}
	width := m.Width()
	left := mark + " " + theme.Dim().Render(when) + "  " + item.entry.Command
	renderPrefixRow(w, alignRow(left, ui.StyledMark(logStatusMark(item.entry.Status)), width-rowIndent), width, index == m.Index())
}

// toLogItems converts oplog entries to logItem slice.
func toLogItems(entries []oplog.Entry, source string) []logItem {
	items := make([]logItem, len(entries))
	for i, e := range entries {
		items[i] = logItem{entry: e, source: source}
	}
	return items
}
