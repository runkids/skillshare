package main

import (
	"fmt"
	"os"
	"strings"
	"time"

	"skillshare/internal/config"
	"skillshare/internal/oplog"
	"skillshare/internal/ui"
)

func cmdLog(args []string) error {
	mode, rest, err := parseModeArgs(args)
	if err != nil {
		return err
	}

	cwd, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("cannot determine working directory: %w", err)
	}

	if mode == modeAuto {
		if projectConfigExists(cwd) {
			mode = modeProject
		} else {
			mode = modeGlobal
		}
	}

	applyModeLabel(mode)

	if mode == modeProject {
		return cmdLogProject(rest, cwd)
	}

	return runLog(rest, config.ConfigPath())
}

func runLog(args []string, configPath string) error {
	audit := false
	clear := false
	limit := 20

	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "--audit", "-a":
			audit = true
		case "--clear", "-c":
			clear = true
		case "--tail", "-t":
			if i+1 < len(args) {
				i++
				n := 0
				if _, err := fmt.Sscanf(args[i], "%d", &n); err == nil && n > 0 {
					limit = n
				}
			}
		case "--help", "-h":
			printLogHelp()
			return nil
		}
	}

	filename := oplog.OpsFile
	label := "Operations"
	if audit {
		filename = oplog.AuditFile
		label = "Audit"
	}

	if clear {
		if err := oplog.Clear(configPath, filename); err != nil {
			return fmt.Errorf("failed to clear log: %w", err)
		}
		ui.Success("%s log cleared", label)
		return nil
	}

	entries, err := oplog.Read(configPath, filename, limit)
	if err != nil {
		return fmt.Errorf("failed to read log: %w", err)
	}

	if len(entries) == 0 {
		ui.Info("No %s log entries", strings.ToLower(label))
		return nil
	}

	ui.Header(fmt.Sprintf("%s Log (last %d)", label, len(entries)))
	printLogEntries(entries)
	return nil
}

func printLogEntries(entries []oplog.Entry) {
	for _, e := range entries {
		ts := formatLogTimestamp(e.Timestamp)
		cmd := formatLogCommand(e.Command)
		detail := formatLogDetail(e)
		status := formatLogStatus(e.Status)
		dur := formatLogDuration(e.Duration)

		if ui.IsTTY() {
			fmt.Printf("  %s%s%s  %-9s  %-30s  %s  %s\n",
				ui.Gray, ts, ui.Reset, cmd, detail, status, dur)
		} else {
			fmt.Printf("  %s  %-9s  %-30s  %-7s  %s\n",
				ts, e.Command, detail, e.Status, dur)
		}
	}
}

func formatLogTimestamp(ts string) string {
	t, err := time.Parse(time.RFC3339, ts)
	if err != nil {
		return ts[:16] // fallback
	}
	return t.Format("2006-01-02 15:04")
}

func formatLogCommand(cmd string) string {
	if !ui.IsTTY() {
		return cmd
	}
	return "\033[1m" + strings.ToUpper(cmd) + "\033[0m"
}

func formatLogDetail(e oplog.Entry) string {
	if e.Message != "" {
		return truncateLogString(e.Message, 30)
	}
	if e.Args == nil {
		return ""
	}

	// Build detail from common args
	parts := make([]string, 0, 3)
	if v, ok := e.Args["source"]; ok {
		parts = append(parts, fmt.Sprintf("%v", v))
	}
	if v, ok := e.Args["name"]; ok {
		parts = append(parts, fmt.Sprintf("%v", v))
	}
	if v, ok := e.Args["skills"]; ok {
		parts = append(parts, fmt.Sprintf("%v", v))
	}
	if v, ok := e.Args["target"]; ok {
		parts = append(parts, fmt.Sprintf("%v", v))
	}
	if v, ok := e.Args["summary"]; ok {
		parts = append(parts, fmt.Sprintf("%v", v))
	}

	return truncateLogString(strings.Join(parts, " "), 30)
}

func formatLogStatus(status string) string {
	if !ui.IsTTY() {
		return status
	}
	switch status {
	case "ok":
		return "\033[32mok\033[0m     "
	case "error":
		return "\033[31merror\033[0m  "
	case "partial":
		return "\033[33mpartial\033[0m"
	case "blocked":
		return "\033[31mblocked\033[0m"
	default:
		return status
	}
}

func formatLogDuration(ms int64) string {
	if ms <= 0 {
		return ""
	}
	if ms < 1000 {
		return fmt.Sprintf("%dms", ms)
	}
	return fmt.Sprintf("%.1fs", float64(ms)/1000)
}

func truncateLogString(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen-3] + "..."
}

// statusFromErr returns "ok" for nil errors and "error" otherwise.
// Used by all command instrumentation to derive oplog status.
func statusFromErr(err error) string {
	if err == nil {
		return "ok"
	}
	return "error"
}

func printLogHelp() {
	fmt.Println(`Usage: skillshare log [options]

View the operation log for debugging and compliance.

Options:
  --audit, -a       Show audit log instead of operations log
  --tail, -t <N>    Show last N entries (default: 20)
  --clear, -c       Clear the log file
  --project, -p     Use project-level log
  --global, -g      Use global log
  --help, -h        Show this help

Examples:
  skillshare log                  Show recent operations
  skillshare log --audit          Show audit log
  skillshare log --tail 50        Show last 50 entries
  skillshare log --clear          Clear operations log
  skillshare log --clear --audit  Clear audit log
  skillshare log -p               Show project operations log`)
}
