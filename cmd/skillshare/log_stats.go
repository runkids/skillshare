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

func renderStatsCLI(stats logStats) string {
	if stats.Total == 0 {
		return ui.Dim + "No log entries" + ui.Reset + "\n"
	}

	var b strings.Builder
	b.WriteString(ui.Bold + "Log summary" + ui.Reset + ui.DimText(" · "+plural(stats.Total, "operation")) + "\n")

	// Sort commands by count descending
	type cmdEntry struct {
		name  string
		stats commandStats
	}
	var cmds []cmdEntry
	names := make([]string, 0, len(stats.ByCommand))
	for name, cs := range stats.ByCommand {
		cmds = append(cmds, cmdEntry{name, cs})
		names = append(names, name)
	}
	sort.Slice(cmds, func(i, j int) bool {
		if cmds[i].stats.Total != cmds[j].stats.Total {
			return cmds[i].stats.Total > cmds[j].stats.Total
		}
		return cmds[i].name < cmds[j].name
	})

	width := ui.RowWidth(names...)
	okTotal := 0
	for _, cmd := range cmds {
		okTotal += cmd.stats.OK
		mark := ui.MarkOK
		if cmd.stats.OK < cmd.stats.Total {
			mark = ui.MarkWarn
		}
		pct := float64(cmd.stats.Total) / float64(stats.Total) * 100
		fmt.Fprintf(&b, "%s %-*s  %d%s\n", ui.StyledMark(mark), width, cmd.name, cmd.stats.Total,
			ui.DimText(fmt.Sprintf(" · %.0f%% · %d/%d ok", pct, cmd.stats.OK, cmd.stats.Total)))
	}

	mark := ui.MarkOK
	if stats.SuccessRate < 0.7 {
		mark = ui.MarkFail
	} else if stats.SuccessRate < 0.9 {
		mark = ui.MarkWarn
	}
	summary := fmt.Sprintf("%d of %d ok (%.0f%%)", okTotal, stats.Total, stats.SuccessRate*100)
	last := ""
	if stats.LastOperation != nil {
		last = " · last: " + stats.LastOperation.Command
		if ts, err := time.Parse(time.RFC3339, stats.LastOperation.Timestamp); err == nil {
			last += ", " + timeAgo(ts)
		}
	}
	fmt.Fprintf(&b, "\n%s %s%s\n", ui.StyledMark(mark), ui.Bold+summary+ui.Reset, ui.DimText(last))
	return b.String()
}
