package main

import (
	"fmt"
	"strings"

	"skillshare/internal/oplog"
)


// logItem wraps oplog.Entry to implement bubbles/list.Item interface.
type logItem struct {
	entry  oplog.Entry
	source string // "operations" or "audit" — identifies which log file
}

// Title returns a one-line header: [2024-01-15 14:30] SYNC — ok
// Plain text only — embedded ANSI breaks bubbletea's DefaultDelegate truncation.
func (i logItem) Title() string {
	ts := formatLogTimestamp(i.entry.Timestamp)
	return fmt.Sprintf("[%s] %s — %s",
		ts,
		strings.ToUpper(i.entry.Command),
		i.entry.Status,
	)
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

// toLogItems converts oplog entries to logItem slice.
func toLogItems(entries []oplog.Entry, source string) []logItem {
	items := make([]logItem, len(entries))
	for i, e := range entries {
		items[i] = logItem{entry: e, source: source}
	}
	return items
}
