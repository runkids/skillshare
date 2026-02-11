package main

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/mattn/go-runewidth"
	"github.com/pterm/pterm"

	"skillshare/internal/config"
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
	// When filtering, read all entries then filter, then apply limit
	readLimit := limit
	if !f.IsEmpty() {
		readLimit = 0 // read all, filter will narrow down
	}

	entries, err := oplog.Read(configPath, filename, readLimit)
	if err != nil {
		return fmt.Errorf("failed to read log: %w", err)
	}

	if !f.IsEmpty() {
		entries = oplog.FilterEntries(entries, f)
		if limit > 0 && len(entries) > limit {
			entries = entries[:limit]
		}
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

// logDetailPair represents a structured key-value for multi-line detail output.
type logDetailPair struct {
	key        string
	value      string
	isList     bool
	listValues []string
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

func formatLogDetailPairs(e oplog.Entry) []logDetailPair {
	if e.Args == nil {
		return nil
	}

	switch e.Command {
	case "sync":
		return formatSyncLogPairs(e.Args)
	case "install":
		return formatInstallLogPairs(e.Args)
	case "audit":
		return formatAuditLogPairs(e.Args)
	default:
		return formatGenericLogPairs(e.Args)
	}
}

func formatSyncLogPairs(args map[string]any) []logDetailPair {
	var pairs []logDetailPair

	if total, ok := logArgInt(args, "targets_total", "targets"); ok {
		pairs = append(pairs, logDetailPair{key: "targets", value: fmt.Sprintf("%d", total)})
	}
	if failed, ok := logArgInt(args, "targets_failed"); ok && failed > 0 {
		pairs = append(pairs, logDetailPair{key: "failed", value: fmt.Sprintf("%d", failed)})
	}
	if dryRun, ok := logArgBool(args, "dry_run"); ok && dryRun {
		pairs = append(pairs, logDetailPair{key: "dry-run", value: "yes"})
	}
	if force, ok := logArgBool(args, "force"); ok && force {
		pairs = append(pairs, logDetailPair{key: "force", value: "yes"})
	}
	if scope, ok := logArgString(args, "scope"); ok && scope != "" {
		pairs = append(pairs, logDetailPair{key: "scope", value: scope})
	}

	return pairs
}

func formatInstallLogPairs(args map[string]any) []logDetailPair {
	var pairs []logDetailPair

	if mode, ok := logArgString(args, "mode"); ok && mode != "" {
		pairs = append(pairs, logDetailPair{key: "mode", value: mode})
	}
	if threshold, ok := logArgString(args, "threshold"); ok && threshold != "" {
		pairs = append(pairs, logDetailPair{key: "threshold", value: strings.ToUpper(threshold)})
	}
	if skillCount, ok := logArgInt(args, "skill_count"); ok && skillCount > 0 {
		pairs = append(pairs, logDetailPair{key: "skills", value: fmt.Sprintf("%d", skillCount)})
	}
	if source, ok := logArgString(args, "source"); ok && source != "" {
		pairs = append(pairs, logDetailPair{key: "source", value: source})
	}
	if dryRun, ok := logArgBool(args, "dry_run"); ok && dryRun {
		pairs = append(pairs, logDetailPair{key: "dry-run", value: "yes"})
	}
	if tracked, ok := logArgBool(args, "tracked"); ok && tracked {
		pairs = append(pairs, logDetailPair{key: "tracked", value: "yes"})
	}
	if skipAudit, ok := logArgBool(args, "skip_audit"); ok && skipAudit {
		pairs = append(pairs, logDetailPair{key: "skip-audit", value: "yes"})
	}
	if installedSkills, ok := logArgStringSlice(args, "installed_skills"); ok && len(installedSkills) > 0 {
		pairs = append(pairs, logDetailPair{key: "installed", isList: true, listValues: installedSkills})
	}
	if failedSkills, ok := logArgStringSlice(args, "failed_skills"); ok && len(failedSkills) > 0 {
		pairs = append(pairs, logDetailPair{key: "failed", isList: true, listValues: failedSkills})
	}

	return pairs
}

func formatAuditLogPairs(args map[string]any) []logDetailPair {
	var pairs []logDetailPair

	scope, hasScope := logArgString(args, "scope")
	name, hasName := logArgString(args, "name")
	if hasScope && scope == "single" && hasName && name != "" {
		pairs = append(pairs, logDetailPair{key: "skill", value: name})
	} else if hasScope && scope == "all" {
		pairs = append(pairs, logDetailPair{key: "scope", value: "all-skills"})
	} else if hasName && name != "" {
		pairs = append(pairs, logDetailPair{key: "name", value: name})
	}
	if path, ok := logArgString(args, "path"); ok && path != "" {
		pairs = append(pairs, logDetailPair{key: "path", value: path})
	}

	if mode, ok := logArgString(args, "mode"); ok && mode != "" {
		pairs = append(pairs, logDetailPair{key: "mode", value: mode})
	}
	if threshold, ok := logArgString(args, "threshold"); ok && threshold != "" {
		pairs = append(pairs, logDetailPair{key: "threshold", value: strings.ToUpper(threshold)})
	}
	if scanned, ok := logArgInt(args, "scanned"); ok {
		pairs = append(pairs, logDetailPair{key: "scanned", value: fmt.Sprintf("%d", scanned)})
	}
	if passed, ok := logArgInt(args, "passed"); ok {
		pairs = append(pairs, logDetailPair{key: "passed", value: fmt.Sprintf("%d", passed)})
	}
	if warning, ok := logArgInt(args, "warning"); ok && warning > 0 {
		pairs = append(pairs, logDetailPair{key: "warning", value: fmt.Sprintf("%d", warning)})
	}
	if failed, ok := logArgInt(args, "failed"); ok && failed > 0 {
		pairs = append(pairs, logDetailPair{key: "failed", value: fmt.Sprintf("%d", failed)})
	}

	critical, hasCritical := logArgInt(args, "critical")
	high, hasHigh := logArgInt(args, "high")
	medium, hasMedium := logArgInt(args, "medium")
	low, hasLow := logArgInt(args, "low")
	info, hasInfo := logArgInt(args, "info")
	if (hasCritical && critical > 0) || (hasHigh && high > 0) || (hasMedium && medium > 0) || (hasLow && low > 0) || (hasInfo && info > 0) {
		pairs = append(pairs, logDetailPair{key: "severity(c/h/m/l/i)", value: fmt.Sprintf("%d/%d/%d/%d/%d", critical, high, medium, low, info)})
	}

	riskScore, hasRiskScore := logArgInt(args, "risk_score")
	riskLabel, hasRiskLabel := logArgString(args, "risk_label")
	if hasRiskScore {
		if hasRiskLabel && riskLabel != "" {
			pairs = append(pairs, logDetailPair{key: "risk", value: fmt.Sprintf("%s (%d/100)", strings.ToUpper(riskLabel), riskScore)})
		} else {
			pairs = append(pairs, logDetailPair{key: "risk", value: fmt.Sprintf("%d/100", riskScore)})
		}
	}

	if scanErrors, ok := logArgInt(args, "scan_errors"); ok && scanErrors > 0 {
		pairs = append(pairs, logDetailPair{key: "scan-errors", value: fmt.Sprintf("%d", scanErrors)})
	}

	if failedSkills, ok := logArgStringSlice(args, "failed_skills"); ok && len(failedSkills) > 0 {
		pairs = append(pairs, logDetailPair{key: "failed skills", isList: true, listValues: failedSkills})
	}
	if warningSkills, ok := logArgStringSlice(args, "warning_skills"); ok && len(warningSkills) > 0 {
		pairs = append(pairs, logDetailPair{key: "warning skills", isList: true, listValues: warningSkills})
	}
	if lowSkills, ok := logArgStringSlice(args, "low_skills"); ok && len(lowSkills) > 0 {
		pairs = append(pairs, logDetailPair{key: "low skills", isList: true, listValues: lowSkills})
	}
	if infoSkills, ok := logArgStringSlice(args, "info_skills"); ok && len(infoSkills) > 0 {
		pairs = append(pairs, logDetailPair{key: "info skills", isList: true, listValues: infoSkills})
	}

	return pairs
}

func formatGenericLogPairs(args map[string]any) []logDetailPair {
	var pairs []logDetailPair

	if source, ok := logArgString(args, "source"); ok {
		pairs = append(pairs, logDetailPair{key: "source", value: source})
	}
	if name, ok := logArgString(args, "name"); ok {
		pairs = append(pairs, logDetailPair{key: "name", value: name})
	}
	if target, ok := logArgString(args, "target"); ok {
		pairs = append(pairs, logDetailPair{key: "target", value: target})
	}
	if targets, ok := logArgInt(args, "targets"); ok {
		pairs = append(pairs, logDetailPair{key: "targets", value: fmt.Sprintf("%d", targets)})
	}
	if summary, ok := logArgString(args, "summary"); ok {
		pairs = append(pairs, logDetailPair{key: "summary", value: summary})
	}

	return pairs
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

func formatLogDetail(e oplog.Entry, truncate bool) string {
	detail := ""
	if e.Args != nil {
		switch e.Command {
		case "sync":
			detail = formatSyncLogDetail(e.Args)
		case "install":
			detail = formatInstallLogDetail(e.Args)
		case "audit":
			detail = formatAuditLogDetail(e.Args)
		default:
			detail = formatGenericLogDetail(e.Args)
		}
	}

	if e.Message != "" && detail != "" {
		return formatLogDetailValue(detail+" ("+e.Message+")", truncate)
	}
	if e.Message != "" {
		return formatLogDetailValue(e.Message, truncate)
	}
	if detail != "" {
		return formatLogDetailValue(detail, truncate)
	}
	return ""
}

func formatLogDetailValue(value string, truncate bool) string {
	if !truncate {
		return value
	}
	return truncateLogString(value, logDetailTruncateLen)
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

func formatInstallLogDetail(args map[string]any) string {
	parts := make([]string, 0, 8)

	if mode, ok := logArgString(args, "mode"); ok && mode != "" {
		parts = append(parts, "mode="+mode)
	}
	if threshold, ok := logArgString(args, "threshold"); ok && threshold != "" {
		parts = append(parts, "threshold="+strings.ToUpper(threshold))
	}
	if skillCount, ok := logArgInt(args, "skill_count"); ok && skillCount > 0 {
		parts = append(parts, fmt.Sprintf("skills=%d", skillCount))
	}
	if installedSkills, ok := logArgStringSlice(args, "installed_skills"); ok && len(installedSkills) > 0 {
		parts = append(parts, "installed="+strings.Join(installedSkills, ", "))
	}
	if failedSkills, ok := logArgStringSlice(args, "failed_skills"); ok && len(failedSkills) > 0 {
		parts = append(parts, "failed="+strings.Join(failedSkills, ", "))
	}
	if dryRun, ok := logArgBool(args, "dry_run"); ok && dryRun {
		parts = append(parts, "dry-run")
	}
	if tracked, ok := logArgBool(args, "tracked"); ok && tracked {
		parts = append(parts, "tracked")
	}
	if skipAudit, ok := logArgBool(args, "skip_audit"); ok && skipAudit {
		parts = append(parts, "skip-audit")
	}
	if source, ok := logArgString(args, "source"); ok && source != "" {
		parts = append(parts, "source="+source)
	}

	if len(parts) == 0 {
		return formatGenericLogDetail(args)
	}
	return strings.Join(parts, ", ")
}

func formatAuditLogDetail(args map[string]any) string {
	parts := make([]string, 0, 12)

	scope, hasScope := logArgString(args, "scope")
	name, hasName := logArgString(args, "name")
	if hasScope && scope == "single" && hasName && name != "" {
		parts = append(parts, "skill="+name)
	} else if hasScope && scope == "all" {
		parts = append(parts, "all-skills")
	} else if hasName && name != "" {
		parts = append(parts, name)
	}
	if path, ok := logArgString(args, "path"); ok && path != "" {
		parts = append(parts, "path="+path)
	}

	if mode, ok := logArgString(args, "mode"); ok && mode != "" {
		parts = append(parts, "mode="+mode)
	}
	if threshold, ok := logArgString(args, "threshold"); ok && threshold != "" {
		parts = append(parts, "threshold="+strings.ToUpper(threshold))
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
	low, hasLow := logArgInt(args, "low")
	info, hasInfo := logArgInt(args, "info")
	if (hasCritical && critical > 0) || (hasHigh && high > 0) || (hasMedium && medium > 0) || (hasLow && low > 0) || (hasInfo && info > 0) {
		parts = append(parts, fmt.Sprintf("sev(c/h/m/l/i)=%d/%d/%d/%d/%d", critical, high, medium, low, info))
	}

	if riskScore, ok := logArgInt(args, "risk_score"); ok {
		riskLabel, hasRiskLabel := logArgString(args, "risk_label")
		if hasRiskLabel && riskLabel != "" {
			parts = append(parts, fmt.Sprintf("risk=%s(%d/100)", strings.ToUpper(riskLabel), riskScore))
		} else {
			parts = append(parts, fmt.Sprintf("risk=%d/100", riskScore))
		}
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
  skillshare log -p                 Show project operations and audit logs`)
}

func isProjectLogConfig(configPath string) bool {
	return filepath.Base(filepath.Dir(configPath)) == ".skillshare"
}
