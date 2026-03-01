package main

import (
	"fmt"
	"sort"
	"strings"
	"time"

	"skillshare/internal/oplog"
	"skillshare/internal/ui"
)

// logStats holds aggregated statistics for a set of log entries.
type logStats struct {
	Total         int
	SuccessRate   float64
	ByCommand     map[string]commandStats
	LastOperation *oplog.Entry
}

// commandStats holds per-command statistics.
type commandStats struct {
	Total   int
	OK      int
	Error   int
	Partial int
	Blocked int
}

func computeLogStats(entries []oplog.Entry) logStats {
	s := logStats{
		ByCommand: make(map[string]commandStats),
	}

	if len(entries) == 0 {
		return s
	}

	s.Total = len(entries)
	s.LastOperation = &entries[0] // entries are newest-first from oplog.Read

	okCount := 0
	for _, e := range entries {
		cs := s.ByCommand[e.Command]
		cs.Total++
		switch e.Status {
		case "ok":
			cs.OK++
			okCount++
		case "error":
			cs.Error++
		case "partial":
			cs.Partial++
		case "blocked":
			cs.Blocked++
		}
		s.ByCommand[e.Command] = cs
	}

	if s.Total > 0 {
		s.SuccessRate = float64(okCount) / float64(s.Total)
	}

	return s
}

// Muted 256-color palette for stats output (less eye-strain than bright ANSI).
const (
	statsGreen  = "\033[32m" // green
	statsRed    = "\033[31m" // red
	statsYellow = "\033[33m" // yellow
)

func renderStatsCLI(stats logStats) string {
	if stats.Total == 0 {
		return ui.Gray + "No log entries" + ui.Reset + "\n"
	}

	var b strings.Builder
	b.WriteString(ui.Cyan + ui.Bold + "Operation Log Summary" + ui.Reset + "\n")
	b.WriteString(ui.Gray + strings.Repeat("─", 45) + ui.Reset + "\n")
	b.WriteString(fmt.Sprintf("%s%-10s%s %s%3d%s operations\n",
		ui.Gray, "Total:", ui.Reset, ui.Bold, stats.Total, ui.Reset))

	// Sort commands by count descending
	type cmdEntry struct {
		name  string
		stats commandStats
	}
	var cmds []cmdEntry
	for name, cs := range stats.ByCommand {
		cmds = append(cmds, cmdEntry{name, cs})
	}
	sort.Slice(cmds, func(i, j int) bool {
		return cmds[i].stats.Total > cmds[j].stats.Total
	})

	for _, cmd := range cmds {
		pct := float64(cmd.stats.Total) / float64(stats.Total) * 100
		okRatio := fmt.Sprintf("✓%d/%d", cmd.stats.OK, cmd.stats.Total)
		ratioColor := statsGreen
		if cmd.stats.OK < cmd.stats.Total {
			ratioColor = statsRed
		}
		b.WriteString(fmt.Sprintf("%s%-10s%s %s%3d%s (%4.1f%%)  %s%s%s\n",
			ui.Gray, cmd.name, ui.Reset,
			ui.Cyan, cmd.stats.Total, ui.Reset,
			pct, ratioColor, okRatio, ui.Reset))
	}

	// OK count with color
	okTotal := 0
	for _, cs := range stats.ByCommand {
		okTotal += cs.OK
	}
	rateColor := statsGreen
	if stats.SuccessRate < 0.7 {
		rateColor = statsRed
	} else if stats.SuccessRate < 0.9 {
		rateColor = statsYellow
	}
	b.WriteString(fmt.Sprintf("\n%sOK:%s %s%s%d/%d%s %s(%.1f%%)%s\n",
		ui.Gray, ui.Reset, rateColor, ui.Bold, okTotal, stats.Total, ui.Reset,
		ui.Gray, stats.SuccessRate*100, ui.Reset))

	if stats.LastOperation != nil {
		ts, err := time.Parse(time.RFC3339, stats.LastOperation.Timestamp)
		ago := "unknown"
		if err == nil {
			ago = formatRelativeTime(time.Since(ts))
		}
		b.WriteString(fmt.Sprintf("%sLast operation:%s %s%s%s (%s ago)\n",
			ui.Gray, ui.Reset, ui.Cyan, stats.LastOperation.Command, ui.Reset, ago))
	}

	return b.String()
}

// formatRelativeTime is an alias for the shared formatDurationShort.
func formatRelativeTime(d time.Duration) string {
	return formatDurationShort(d)
}
