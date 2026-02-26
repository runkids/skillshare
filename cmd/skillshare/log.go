package main

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"

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
	auditOnly := false
	clear := false
	jsonOutput := false
	stats := false
	noTUI := false
	limit := 20
	var filter oplog.Filter

	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "--audit", "-a":
			auditOnly = true
		case "--clear", "-c":
			clear = true
		case "--json":
			jsonOutput = true
		case "--no-tui":
			noTUI = true
		case "--stats":
			stats = true
		case "--cmd":
			if i+1 < len(args) {
				i++
				filter.Cmd = args[i]
			}
		case "--status":
			if i+1 < len(args) {
				i++
				filter.Status = args[i]
			}
		case "--since":
			if i+1 < len(args) {
				i++
				t, err := oplog.ParseSince(args[i])
				if err != nil {
					return err
				}
				filter.Since = t
			}
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

	modeLabel := "global"
	if isProjectLogConfig(configPath) {
		modeLabel = "project"
	}

	if clear {
		filename := oplog.OpsFile
		label := "Operations"
		if auditOnly {
			filename = oplog.AuditFile
			label = "Audit"
		}
		if err := oplog.Clear(configPath, filename); err != nil {
			return fmt.Errorf("failed to clear log: %w", err)
		}
		ui.Success("%s log cleared", label)
		return nil
	}

	// Stats summary
	if stats {
		return runLogStats(configPath, auditOnly, filter)
	}

	// TUI dispatch: interactive viewer when TTY and not explicitly disabled
	if !noTUI && !jsonOutput && ui.IsTTY() {
		return runLogTUIDispatch(configPath, auditOnly, limit, filter, modeLabel)
	}

	if auditOnly {
		return printLogSection(configPath, oplog.AuditFile, "Audit", limit, filter, jsonOutput)
	}

	// When filtering by command, only show the relevant section:
	// "audit" only lives in audit.log; everything else in operations.log.
	if filter.Cmd != "" {
		if filter.Cmd == "audit" {
			return printLogSection(configPath, oplog.AuditFile, "Audit", limit, filter, jsonOutput)
		}
		return printLogSection(configPath, oplog.OpsFile, "Operations", limit, filter, jsonOutput)
	}

	if err := printLogSection(configPath, oplog.OpsFile, "Operations", limit, filter, jsonOutput); err != nil {
		return err
	}
	if !jsonOutput {
		fmt.Println()
	}
	return printLogSection(configPath, oplog.AuditFile, "Audit", limit, filter, jsonOutput)
}

func printLogSection(configPath, filename, label string, limit int, f oplog.Filter, jsonOutput bool) error {
	entries, err := readAndFilter(configPath, filename, limit, f)
	if err != nil {
		return err
	}

	if jsonOutput {
		return printLogEntriesJSON(os.Stdout, entries)
	}

	logPath := filepath.Join(oplog.LogDir(configPath), filename)
	if abs, absErr := filepath.Abs(logPath); absErr == nil {
		logPath = abs
	}
	mode := "global"
	if isProjectLogConfig(configPath) {
		mode = "project"
	}

	subtitle := fmt.Sprintf("%s (last %d)\nmode: %s\nfile: %s", label, len(entries), mode, logPath)
	ui.HeaderBox("skillshare log", subtitle)
	if len(entries) == 0 {
		ui.Info("No %s log entries", strings.ToLower(label))
		return nil
	}

	printLogEntries(entries)
	return nil
}

func printLogEntriesJSON(w io.Writer, entries []oplog.Entry) error {
	enc := json.NewEncoder(w)
	for _, e := range entries {
		if err := enc.Encode(e); err != nil {
			return err
		}
	}
	return nil
}

// logMaxEntries returns the configured max log entries, defaulting to 1000.
// nil (not set) → 1000, 0 → unlimited (passed as 0 to WriteWithLimit), >0 → use value, <0 → default.
func logMaxEntries() int {
	cfg, err := config.Load()
	if err != nil {
		return config.DefaultLogMaxEntries
	}
	if cfg.Log.MaxEntries == nil || *cfg.Log.MaxEntries < 0 {
		return config.DefaultLogMaxEntries
	}
	return *cfg.Log.MaxEntries
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

View operations and audit logs for debugging and compliance.

Options:
  --audit, -a         Show only audit log
  --tail, -t <N>      Show last N entries (default: 20)
  --cmd <name>        Filter by command name (e.g. sync, install, audit)
  --status <status>   Filter by status (ok, error, partial, blocked)
  --since <dur|date>  Filter by time (e.g. 30m, 2h, 2d, 1w, 2006-01-02)
  --json              Output raw JSONL (one JSON object per line)
  --stats             Show aggregated statistics summary
  --no-tui            Disable interactive TUI, print plain text
  --clear, -c         Clear the selected log file
  --project, -p       Use project-level log
  --global, -g        Use global log
  --help, -h          Show this help

Examples:
  skillshare log                    Show operations and audit logs
  skillshare log --audit            Show only audit log
  skillshare log --tail 50          Show last 50 entries per section
  skillshare log --cmd sync         Show only sync entries
  skillshare log --status error     Show only errors
  skillshare log --since 2d         Show entries from last 2 days
  skillshare log --json             Output as JSONL
  skillshare log --json --cmd sync  JSONL filtered by command
  skillshare log --clear            Clear operations log
  skillshare log --clear --audit    Clear audit log
  skillshare log --stats             Show aggregated statistics
  skillshare log -p                 Show project operations and audit logs`)
}

func runLogStats(configPath string, auditOnly bool, f oplog.Filter) error {
	filename := oplog.OpsFile
	if auditOnly || f.Cmd == "audit" {
		filename = oplog.AuditFile
	}

	entries, err := oplog.Read(configPath, filename, 0)
	if err != nil {
		return fmt.Errorf("failed to read log: %w", err)
	}

	if !f.IsEmpty() {
		entries = oplog.FilterEntries(entries, f)
	}

	stats := computeLogStats(entries)
	fmt.Print(renderStatsCLI(stats))
	return nil
}

func isProjectLogConfig(configPath string) bool {
	return filepath.Base(filepath.Dir(configPath)) == ".skillshare"
}

// runLogTUIDispatch launches the interactive TUI with async log loading.
func runLogTUIDispatch(configPath string, auditOnly bool, limit int, filter oplog.Filter, modeLabel string) error {
	if auditOnly {
		return runLogTUIAsync(func() ([]logItem, error) {
			entries, err := readAndFilter(configPath, oplog.AuditFile, limit, filter)
			if err != nil {
				return nil, err
			}
			return toLogItems(entries, "audit"), nil
		}, "Audit", modeLabel, configPath)
	}

	// When filtering by command, only read the relevant log
	if filter.Cmd != "" {
		if filter.Cmd == "audit" {
			return runLogTUIAsync(func() ([]logItem, error) {
				entries, err := readAndFilter(configPath, oplog.AuditFile, limit, filter)
				if err != nil {
					return nil, err
				}
				return toLogItems(entries, "audit"), nil
			}, "Audit", modeLabel, configPath)
		}
		return runLogTUIAsync(func() ([]logItem, error) {
			entries, err := readAndFilter(configPath, oplog.OpsFile, limit, filter)
			if err != nil {
				return nil, err
			}
			return toLogItems(entries, "operations"), nil
		}, "Operations", modeLabel, configPath)
	}

	// Default: read both logs, merge, sort
	return runLogTUIAsync(func() ([]logItem, error) {
		opsEntries, err := readAndFilter(configPath, oplog.OpsFile, 0, filter)
		if err != nil {
			return nil, err
		}
		auditEntries, err := readAndFilter(configPath, oplog.AuditFile, 0, filter)
		if err != nil {
			return nil, err
		}

		items := append(toLogItems(opsEntries, "operations"), toLogItems(auditEntries, "audit")...)

		sort.Slice(items, func(i, j int) bool {
			return items[i].entry.Timestamp > items[j].entry.Timestamp
		})

		if limit > 0 && len(items) > limit {
			items = items[:limit]
		}
		return items, nil
	}, "Operations & Audit", modeLabel, configPath)
}

// readAndFilter reads entries from a log file, applies filter, and limits results.
func readAndFilter(configPath, filename string, limit int, f oplog.Filter) ([]oplog.Entry, error) {
	readLimit := limit
	if !f.IsEmpty() {
		readLimit = 0 // read all, filter narrows down
	}

	entries, err := oplog.Read(configPath, filename, readLimit)
	if err != nil {
		return nil, fmt.Errorf("failed to read log: %w", err)
	}

	if !f.IsEmpty() {
		entries = oplog.FilterEntries(entries, f)
		if limit > 0 && len(entries) > limit {
			entries = entries[:limit]
		}
	}

	return entries, nil
}
