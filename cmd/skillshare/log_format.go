package main

import (
	"fmt"
	"strconv"
	"strings"
	"time"

	"skillshare/internal/oplog"
)

// logDetailPair represents a structured key-value for multi-line detail output.
type logDetailPair struct {
	key        string
	value      string
	isList     bool
	listValues []string
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
	case "update":
		return formatUpdateLogPairs(e.Args)
	case "audit":
		return formatAuditLogPairs(e.Args)
	case "check":
		return formatCheckLogPairs(e.Args)
	case "diff":
		return formatDiffLogPairs(e.Args)
	case "trash":
		return formatTrashLogPairs(e.Args)
	case "init":
		return formatInitLogPairs(e.Args)
	case "upgrade":
		return formatUpgradeLogPairs(e.Args)
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

func formatUpdateLogPairs(args map[string]any) []logDetailPair {
	var pairs []logDetailPair

	if mode, ok := logArgString(args, "mode"); ok && mode != "" {
		pairs = append(pairs, logDetailPair{key: "mode", value: mode})
	}
	if all, ok := logArgBool(args, "all"); ok && all {
		pairs = append(pairs, logDetailPair{key: "scope", value: "all"})
	}
	if name, ok := logArgString(args, "name"); ok && name != "" {
		pairs = append(pairs, logDetailPair{key: "name", value: name})
	}
	if names, ok := logArgStringSlice(args, "names"); ok && len(names) > 0 {
		pairs = append(pairs, logDetailPair{key: "names", isList: true, listValues: names})
	}
	if threshold, ok := logArgString(args, "threshold"); ok && threshold != "" {
		pairs = append(pairs, logDetailPair{key: "threshold", value: strings.ToUpper(threshold)})
	}
	if force, ok := logArgBool(args, "force"); ok && force {
		pairs = append(pairs, logDetailPair{key: "force", value: "yes"})
	}
	if dryRun, ok := logArgBool(args, "dry_run"); ok && dryRun {
		pairs = append(pairs, logDetailPair{key: "dry-run", value: "yes"})
	}
	if skipAudit, ok := logArgBool(args, "skip_audit"); ok && skipAudit {
		pairs = append(pairs, logDetailPair{key: "skip-audit", value: "yes"})
	}
	if diff, ok := logArgBool(args, "diff"); ok && diff {
		pairs = append(pairs, logDetailPair{key: "diff", value: "yes"})
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

func formatCheckLogPairs(args map[string]any) []logDetailPair {
	var pairs []logDetailPair

	if repos, ok := logArgInt(args, "repos_checked"); ok && repos > 0 {
		pairs = append(pairs, logDetailPair{key: "repos", value: fmt.Sprintf("%d", repos)})
	}
	if skills, ok := logArgInt(args, "skills_checked"); ok && skills > 0 {
		pairs = append(pairs, logDetailPair{key: "skills", value: fmt.Sprintf("%d", skills)})
	}
	if updates, ok := logArgInt(args, "updates_available"); ok && updates > 0 {
		pairs = append(pairs, logDetailPair{key: "updates", value: fmt.Sprintf("%d", updates)})
	}
	if errors, ok := logArgInt(args, "errors"); ok && errors > 0 {
		pairs = append(pairs, logDetailPair{key: "errors", value: fmt.Sprintf("%d", errors)})
	}
	if scope, ok := logArgString(args, "scope"); ok && scope != "" {
		pairs = append(pairs, logDetailPair{key: "scope", value: scope})
	}

	return pairs
}

func formatDiffLogPairs(args map[string]any) []logDetailPair {
	var pairs []logDetailPair

	if target, ok := logArgString(args, "target"); ok && target != "" {
		pairs = append(pairs, logDetailPair{key: "target", value: target})
	}
	if targetsShown, ok := logArgInt(args, "targets_shown"); ok && targetsShown > 0 {
		pairs = append(pairs, logDetailPair{key: "targets", value: fmt.Sprintf("%d", targetsShown)})
	}
	if scope, ok := logArgString(args, "scope"); ok && scope != "" {
		pairs = append(pairs, logDetailPair{key: "scope", value: scope})
	}

	return pairs
}

func formatTrashLogPairs(args map[string]any) []logDetailPair {
	var pairs []logDetailPair

	if action, ok := logArgString(args, "action"); ok && action != "" {
		pairs = append(pairs, logDetailPair{key: "action", value: action})
	}
	if items, ok := logArgInt(args, "items"); ok && items > 0 {
		pairs = append(pairs, logDetailPair{key: "items", value: fmt.Sprintf("%d", items)})
	}
	if name, ok := logArgString(args, "name"); ok && name != "" {
		pairs = append(pairs, logDetailPair{key: "name", value: name})
	}

	return pairs
}

func formatInitLogPairs(args map[string]any) []logDetailPair {
	var pairs []logDetailPair

	if targetsAdded, ok := logArgInt(args, "targets_added"); ok && targetsAdded > 0 {
		pairs = append(pairs, logDetailPair{key: "targets", value: fmt.Sprintf("%d", targetsAdded)})
	}
	if sourceCreated, ok := logArgBool(args, "source_created"); ok && sourceCreated {
		pairs = append(pairs, logDetailPair{key: "source", value: "created"})
	}
	if gitInit, ok := logArgBool(args, "git_init"); ok && gitInit {
		pairs = append(pairs, logDetailPair{key: "git", value: "yes"})
	}
	if skillInstalled, ok := logArgBool(args, "skill_installed"); ok && skillInstalled {
		pairs = append(pairs, logDetailPair{key: "skill", value: "installed"})
	}

	return pairs
}

func formatUpgradeLogPairs(args map[string]any) []logDetailPair {
	var pairs []logDetailPair

	if cli, ok := logArgBool(args, "cli"); ok && cli {
		pairs = append(pairs, logDetailPair{key: "cli", value: "yes"})
	}
	if skill, ok := logArgBool(args, "skill"); ok && skill {
		pairs = append(pairs, logDetailPair{key: "skill", value: "yes"})
	}
	if from, ok := logArgString(args, "from_version"); ok && from != "" {
		pairs = append(pairs, logDetailPair{key: "from", value: from})
	}
	if to, ok := logArgString(args, "to_version"); ok && to != "" {
		pairs = append(pairs, logDetailPair{key: "to", value: to})
	}

	return pairs
}

func formatCheckLogDetail(args map[string]any) string {
	parts := make([]string, 0, 5)

	if repos, ok := logArgInt(args, "repos_checked"); ok && repos > 0 {
		parts = append(parts, fmt.Sprintf("repos=%d", repos))
	}
	if skills, ok := logArgInt(args, "skills_checked"); ok && skills > 0 {
		parts = append(parts, fmt.Sprintf("skills=%d", skills))
	}
	if updates, ok := logArgInt(args, "updates_available"); ok && updates > 0 {
		parts = append(parts, fmt.Sprintf("updates=%d", updates))
	}
	if errors, ok := logArgInt(args, "errors"); ok && errors > 0 {
		parts = append(parts, fmt.Sprintf("errors=%d", errors))
	}
	if scope, ok := logArgString(args, "scope"); ok && scope != "" {
		parts = append(parts, "scope="+scope)
	}

	if len(parts) == 0 {
		return formatGenericLogDetail(args)
	}
	return strings.Join(parts, ", ")
}

func formatDiffLogDetail(args map[string]any) string {
	parts := make([]string, 0, 3)

	if target, ok := logArgString(args, "target"); ok && target != "" {
		parts = append(parts, "target="+target)
	}
	if targetsShown, ok := logArgInt(args, "targets_shown"); ok && targetsShown > 0 {
		parts = append(parts, fmt.Sprintf("targets=%d", targetsShown))
	}
	if scope, ok := logArgString(args, "scope"); ok && scope != "" {
		parts = append(parts, "scope="+scope)
	}

	if len(parts) == 0 {
		return formatGenericLogDetail(args)
	}
	return strings.Join(parts, ", ")
}

func formatTrashLogDetail(args map[string]any) string {
	parts := make([]string, 0, 3)

	if action, ok := logArgString(args, "action"); ok && action != "" {
		parts = append(parts, action)
	}
	if items, ok := logArgInt(args, "items"); ok && items > 0 {
		parts = append(parts, fmt.Sprintf("items=%d", items))
	}
	if name, ok := logArgString(args, "name"); ok && name != "" {
		parts = append(parts, name)
	}

	if len(parts) == 0 {
		return formatGenericLogDetail(args)
	}
	return strings.Join(parts, ", ")
}

func formatInitLogDetail(args map[string]any) string {
	parts := make([]string, 0, 4)

	if targetsAdded, ok := logArgInt(args, "targets_added"); ok && targetsAdded > 0 {
		parts = append(parts, fmt.Sprintf("targets=%d", targetsAdded))
	}
	if sourceCreated, ok := logArgBool(args, "source_created"); ok && sourceCreated {
		parts = append(parts, "source-created")
	}
	if gitInit, ok := logArgBool(args, "git_init"); ok && gitInit {
		parts = append(parts, "git")
	}
	if skillInstalled, ok := logArgBool(args, "skill_installed"); ok && skillInstalled {
		parts = append(parts, "skill-installed")
	}

	if len(parts) == 0 {
		return formatGenericLogDetail(args)
	}
	return strings.Join(parts, ", ")
}

func formatUpgradeLogDetail(args map[string]any) string {
	parts := make([]string, 0, 4)

	if cli, ok := logArgBool(args, "cli"); ok && cli {
		parts = append(parts, "cli")
	}
	if skill, ok := logArgBool(args, "skill"); ok && skill {
		parts = append(parts, "skill")
	}
	if from, ok := logArgString(args, "from_version"); ok && from != "" {
		parts = append(parts, "from="+from)
	}
	if to, ok := logArgString(args, "to_version"); ok && to != "" {
		parts = append(parts, "to="+to)
	}

	if len(parts) == 0 {
		return formatGenericLogDetail(args)
	}
	return strings.Join(parts, ", ")
}

func formatGenericLogPairs(args map[string]any) []logDetailPair {
	var pairs []logDetailPair

	if source, ok := logArgString(args, "source"); ok {
		pairs = append(pairs, logDetailPair{key: "source", value: source})
	}
	if name, ok := logArgString(args, "name"); ok {
		pairs = append(pairs, logDetailPair{key: "name", value: name})
	}
	if names, ok := logArgStringSlice(args, "names"); ok && len(names) > 0 {
		pairs = append(pairs, logDetailPair{key: "names", isList: true, listValues: names})
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
		case "update":
			detail = formatUpdateLogDetail(e.Args)
		case "audit":
			detail = formatAuditLogDetail(e.Args)
		case "check":
			detail = formatCheckLogDetail(e.Args)
		case "diff":
			detail = formatDiffLogDetail(e.Args)
		case "trash":
			detail = formatTrashLogDetail(e.Args)
		case "init":
			detail = formatInitLogDetail(e.Args)
		case "upgrade":
			detail = formatUpgradeLogDetail(e.Args)
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

func formatUpdateLogDetail(args map[string]any) string {
	parts := make([]string, 0, 8)

	if mode, ok := logArgString(args, "mode"); ok && mode != "" {
		parts = append(parts, "mode="+mode)
	}
	if all, ok := logArgBool(args, "all"); ok && all {
		parts = append(parts, "all")
	}
	if name, ok := logArgString(args, "name"); ok && name != "" {
		parts = append(parts, name)
	}
	if names, ok := logArgStringSlice(args, "names"); ok && len(names) > 0 {
		parts = append(parts, strings.Join(names, ", "))
	}
	if threshold, ok := logArgString(args, "threshold"); ok && threshold != "" {
		parts = append(parts, "threshold="+strings.ToUpper(threshold))
	}
	if force, ok := logArgBool(args, "force"); ok && force {
		parts = append(parts, "force")
	}
	if dryRun, ok := logArgBool(args, "dry_run"); ok && dryRun {
		parts = append(parts, "dry-run")
	}
	if skipAudit, ok := logArgBool(args, "skip_audit"); ok && skipAudit {
		parts = append(parts, "skip-audit")
	}
	if diff, ok := logArgBool(args, "diff"); ok && diff {
		parts = append(parts, "diff")
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
	if names, ok := logArgStringSlice(args, "names"); ok && len(names) > 0 {
		parts = append(parts, strings.Join(names, ", "))
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

func formatLogDuration(ms int64) string {
	if ms <= 0 {
		return ""
	}
	if ms < 1000 {
		return fmt.Sprintf("%dms", ms)
	}
	return fmt.Sprintf("%.1fs", float64(ms)/1000)
}

func truncateLogString(s string, maxLen int) string { return truncateStr(s, maxLen) }

// logArg* helpers extract typed values from oplog.Entry.Args.

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
