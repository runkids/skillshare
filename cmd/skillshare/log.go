package main

import (
	"fmt"
	"os"
	"strconv"
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
	auditOnly := false
	clear := false
	limit := 20

	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "--audit", "-a":
			auditOnly = true
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

	if auditOnly {
		return printLogSection(configPath, oplog.AuditFile, "Audit", limit)
	}

	if err := printLogSection(configPath, oplog.OpsFile, "Operations", limit); err != nil {
		return err
	}
	fmt.Println()
	return printLogSection(configPath, oplog.AuditFile, "Audit", limit)
}

func printLogSection(configPath, filename, label string, limit int) error {
	entries, err := oplog.Read(configPath, filename, limit)
	if err != nil {
		return fmt.Errorf("failed to read log: %w", err)
	}

	ui.HeaderBox("skillshare log", fmt.Sprintf("%s (last %d)", label, len(entries)))
	if len(entries) == 0 {
		ui.Info("No %s log entries", strings.ToLower(label))
		return nil
	}

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
			fmt.Printf("  %s%s%s  %-9s  %-96s  %s  %s\n",
				ui.Gray, ts, ui.Reset, cmd, detail, status, dur)
		} else {
			fmt.Printf("  %s  %-9s  %-96s  %-7s  %s\n",
				ts, e.Command, detail, e.Status, dur)
		}

		printLogAuditSkillLines(e)
	}
}

func printLogAuditSkillLines(e oplog.Entry) {
	if e.Command != "audit" || e.Args == nil {
		return
	}

	if failedSkills, ok := logArgStringSlice(e.Args, "failed_skills"); ok && len(failedSkills) > 0 {
		printLogNamedSkills("failed skills", failedSkills)
	}
	if warningSkills, ok := logArgStringSlice(e.Args, "warning_skills"); ok && len(warningSkills) > 0 {
		printLogNamedSkills("warning skills", warningSkills)
	}
}

func printLogNamedSkills(label string, skills []string) {
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
		fmt.Printf("                     -> %s: %s\n", currentLabel, strings.Join(skills[i:end], ", "))
	}
}

func formatLogTimestamp(ts string) string {
	t, err := time.Parse(time.RFC3339, ts)
	if err != nil {
		if len(ts) >= 16 {
			return ts[:16]
		}
		return ts
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
	detail := ""
	if e.Args != nil {
		switch e.Command {
		case "sync":
			detail = formatSyncLogDetail(e.Args)
		case "audit":
			detail = formatAuditLogDetail(e.Args)
		default:
			detail = formatGenericLogDetail(e.Args)
		}
	}

	if e.Message != "" && detail != "" {
		return truncateLogString(detail+" ("+e.Message+")", 96)
	}
	if e.Message != "" {
		return truncateLogString(e.Message, 96)
	}
	if detail != "" {
		return truncateLogString(detail, 96)
	}
	return ""
}

func formatSyncLogDetail(args map[string]any) string {
	parts := make([]string, 0, 5)

	if total, ok := logArgInt(args, "targets_total", "targets"); ok {
		parts = append(parts, fmt.Sprintf("targets=%d", total))
	}
	if failed, ok := logArgInt(args, "targets_failed"); ok && failed > 0 {
		parts = append(parts, fmt.Sprintf("failed=%d", failed))
	}
	if dryRun, ok := logArgBool(args, "dry_run"); ok && dryRun {
		parts = append(parts, "dry-run")
	}
	if force, ok := logArgBool(args, "force"); ok && force {
		parts = append(parts, "force")
	}
	if scope, ok := logArgString(args, "scope"); ok && scope != "" {
		parts = append(parts, "scope="+scope)
	}

	if len(parts) == 0 {
		return formatGenericLogDetail(args)
	}
	return strings.Join(parts, ", ")
}

func formatAuditLogDetail(args map[string]any) string {
	parts := make([]string, 0, 8)

	scope, hasScope := logArgString(args, "scope")
	name, hasName := logArgString(args, "name")
	if hasScope && scope == "single" && hasName && name != "" {
		parts = append(parts, "skill="+name)
	} else if hasScope && scope == "all" {
		parts = append(parts, "all-skills")
	} else if hasName && name != "" {
		parts = append(parts, name)
	}

	if mode, ok := logArgString(args, "mode"); ok && mode != "" {
		parts = append(parts, "mode="+mode)
	}
	if scanned, ok := logArgInt(args, "scanned"); ok {
		parts = append(parts, fmt.Sprintf("scanned=%d", scanned))
	}
	if passed, ok := logArgInt(args, "passed"); ok {
		parts = append(parts, fmt.Sprintf("passed=%d", passed))
	}
	if warning, ok := logArgInt(args, "warning"); ok && warning > 0 {
		parts = append(parts, fmt.Sprintf("warning=%d", warning))
	}
	if failed, ok := logArgInt(args, "failed"); ok && failed > 0 {
		parts = append(parts, fmt.Sprintf("failed=%d", failed))
	}

	critical, hasCritical := logArgInt(args, "critical")
	high, hasHigh := logArgInt(args, "high")
	medium, hasMedium := logArgInt(args, "medium")
	if (hasCritical && critical > 0) || (hasHigh && high > 0) || (hasMedium && medium > 0) {
		parts = append(parts, fmt.Sprintf("sev(c/h/m)=%d/%d/%d", critical, high, medium))
	}

	if scanErrors, ok := logArgInt(args, "scan_errors"); ok && scanErrors > 0 {
		parts = append(parts, fmt.Sprintf("scan-errors=%d", scanErrors))
	}

	if len(parts) == 0 {
		return formatGenericLogDetail(args)
	}
	return strings.Join(parts, ", ")
}

func formatGenericLogDetail(args map[string]any) string {
	parts := make([]string, 0, 4)

	if source, ok := logArgString(args, "source"); ok {
		parts = append(parts, source)
	}
	if name, ok := logArgString(args, "name"); ok {
		parts = append(parts, name)
	}
	if target, ok := logArgString(args, "target"); ok {
		parts = append(parts, target)
	}
	if targets, ok := logArgInt(args, "targets"); ok {
		parts = append(parts, fmt.Sprintf("targets=%d", targets))
	}
	if summary, ok := logArgString(args, "summary"); ok {
		parts = append(parts, summary)
	}

	return strings.Join(parts, ", ")
}

func logArgString(args map[string]any, key string) (string, bool) {
	v, ok := args[key]
	if !ok || v == nil {
		return "", false
	}

	switch s := v.(type) {
	case string:
		return s, true
	default:
		return fmt.Sprintf("%v", v), true
	}
}

func logArgInt(args map[string]any, keys ...string) (int, bool) {
	for _, key := range keys {
		v, ok := args[key]
		if !ok || v == nil {
			continue
		}

		switch n := v.(type) {
		case int:
			return n, true
		case int64:
			return int(n), true
		case float64:
			return int(n), true
		case string:
			parsed, err := strconv.Atoi(strings.TrimSpace(n))
			if err == nil {
				return parsed, true
			}
		}
	}
	return 0, false
}

func logArgBool(args map[string]any, key string) (bool, bool) {
	v, ok := args[key]
	if !ok || v == nil {
		return false, false
	}

	switch b := v.(type) {
	case bool:
		return b, true
	case string:
		parsed, err := strconv.ParseBool(strings.TrimSpace(b))
		if err == nil {
			return parsed, true
		}
	}
	return false, false
}

func logArgStringSlice(args map[string]any, key string) ([]string, bool) {
	v, ok := args[key]
	if !ok || v == nil {
		return nil, false
	}

	switch raw := v.(type) {
	case []string:
		if len(raw) == 0 {
			return nil, false
		}
		return raw, true
	case []any:
		items := make([]string, 0, len(raw))
		for _, it := range raw {
			s := strings.TrimSpace(fmt.Sprintf("%v", it))
			if s != "" {
				items = append(items, s)
			}
		}
		if len(items) == 0 {
			return nil, false
		}
		return items, true
	case string:
		s := strings.TrimSpace(raw)
		if s == "" {
			return nil, false
		}
		return []string{s}, true
	default:
		return nil, false
	}
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

View operations and audit logs for debugging and compliance.

Options:
  --audit, -a       Show only audit log
  --tail, -t <N>    Show last N entries (default: 20)
  --clear, -c       Clear the selected log file
  --project, -p     Use project-level log
  --global, -g      Use global log
  --help, -h        Show this help

Examples:
  skillshare log                  Show operations and audit logs
  skillshare log --audit          Show only audit log
  skillshare log --tail 50        Show last 50 entries per section
  skillshare log --clear          Clear operations log
  skillshare log --clear --audit  Clear audit log
  skillshare log -p               Show project operations and audit logs`)
}
