package oplog

import (
	"fmt"
	"strconv"
	"strings"
	"time"
)

// Filter holds optional criteria for narrowing log entries.
type Filter struct {
	Cmd    string    // command name (case-insensitive match)
	Status string    // status value (case-insensitive match)
	Since  time.Time // entries before this time are excluded
}

// IsEmpty returns true when no filter criteria are set.
func (f Filter) IsEmpty() bool {
	return f.Cmd == "" && f.Status == "" && f.Since.IsZero()
}

// FilterEntries returns the subset of entries matching f.
// An empty filter returns entries unchanged.
func FilterEntries(entries []Entry, f Filter) []Entry {
	if f.IsEmpty() {
		return entries
	}

	cmd := strings.ToLower(f.Cmd)
	status := strings.ToLower(f.Status)

	out := make([]Entry, 0, len(entries))
	for _, e := range entries {
		if cmd != "" && strings.ToLower(e.Command) != cmd {
			continue
		}
		if status != "" && strings.ToLower(e.Status) != status {
			continue
		}
		if !f.Since.IsZero() {
			ts, err := time.Parse(time.RFC3339, e.Timestamp)
			if err != nil {
				continue // skip unparseable timestamps
			}
			if ts.Before(f.Since) {
				continue
			}
		}
		out = append(out, e)
	}
	return out
}

// ParseSince parses a human-friendly time specification into a time.Time.
// Supported formats:
//   - Relative: "30m", "2h", "2d", "1w" (minutes, hours, days, weeks)
//   - Absolute date: "2006-01-02"
//   - RFC3339: "2006-01-02T15:04:05Z07:00"
func ParseSince(s string) (time.Time, error) {
	s = strings.TrimSpace(s)
	if s == "" {
		return time.Time{}, nil
	}

	// Try relative duration: number + suffix
	if len(s) >= 2 {
		suffix := s[len(s)-1]
		numStr := s[:len(s)-1]
		if n, err := strconv.Atoi(numStr); err == nil && n > 0 {
			now := time.Now()
			switch suffix {
			case 'm':
				return now.Add(-time.Duration(n) * time.Minute), nil
			case 'h':
				return now.Add(-time.Duration(n) * time.Hour), nil
			case 'd':
				return now.AddDate(0, 0, -n), nil
			case 'w':
				return now.AddDate(0, 0, -n*7), nil
			}
		}
	}

	// Try absolute date
	if t, err := time.Parse("2006-01-02", s); err == nil {
		return t, nil
	}

	// Try RFC3339
	if t, err := time.Parse(time.RFC3339, s); err == nil {
		return t, nil
	}

	return time.Time{}, fmt.Errorf("invalid time format %q (use: 30m, 2h, 2d, 1w, 2006-01-02, or RFC3339)", s)
}
