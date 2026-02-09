package main

import (
	"fmt"
	"os"
	"path/filepath"
	"time"

	"skillshare/internal/audit"
	"skillshare/internal/config"
	"skillshare/internal/oplog"
	"skillshare/internal/sync"
	"skillshare/internal/ui"
	"skillshare/internal/utils"
)

type auditRunSummary struct {
	Scope      string
	Skill      string
	Scanned    int
	Passed     int
	Warning    int
	Failed     int
	Critical   int
	High       int
	Medium     int
	WarnSkills []string
	FailSkills []string
	ScanErrors int
	Mode       string
}

func cmdAudit(args []string) error {
	start := time.Now()

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

	specificSkill := ""
	for _, a := range rest {
		if a == "--help" || a == "-h" {
			printAuditHelp()
			return nil
		}
		if specificSkill == "" {
			specificSkill = a
		}
	}

	var (
		summary auditRunSummary
		blocked bool
	)

	if mode == modeProject {
		summary, blocked, err = cmdAuditProject(cwd, specificSkill)
		logAuditOp(config.ProjectConfigPath(cwd), rest, summary, start, err, blocked)
		if blocked && err == nil {
			os.Exit(1)
		}
		return err
	}

	cfg, err := config.Load()
	if err != nil {
		return err
	}

	summary, blocked, err = runAudit(cfg.Source, specificSkill)
	summary.Mode = "global"
	logAuditOp(config.ConfigPath(), rest, summary, start, err, blocked)
	if blocked && err == nil {
		os.Exit(1)
	}
	return err
}

func runAudit(sourcePath, specificSkill string) (auditRunSummary, bool, error) {
	if specificSkill != "" {
		return auditSingleSkill(sourcePath, specificSkill)
	}
	return auditAllSkills(sourcePath)
}

func logAuditOp(cfgPath string, args []string, summary auditRunSummary, start time.Time, cmdErr error, blocked bool) {
	status := statusFromErr(cmdErr)
	if blocked && cmdErr == nil {
		status = "blocked"
	}

	e := oplog.NewEntry("audit", status, time.Since(start))
	fields := map[string]any{}

	if summary.Scope != "" {
		fields["scope"] = summary.Scope
	}
	if summary.Skill != "" {
		fields["name"] = summary.Skill
	}
	if summary.Mode != "" {
		fields["mode"] = summary.Mode
	}
	if summary.Scanned > 0 {
		fields["scanned"] = summary.Scanned
		fields["passed"] = summary.Passed
		fields["warning"] = summary.Warning
		fields["failed"] = summary.Failed
		fields["critical"] = summary.Critical
		fields["high"] = summary.High
		fields["medium"] = summary.Medium
		if len(summary.WarnSkills) > 0 {
			fields["warning_skills"] = summary.WarnSkills
		}
		if len(summary.FailSkills) > 0 {
			fields["failed_skills"] = summary.FailSkills
		}
	}
	if summary.ScanErrors > 0 {
		fields["scan_errors"] = summary.ScanErrors
	}
	if len(fields) == 0 && len(args) > 0 {
		fields["name"] = args[0]
	}
	if len(fields) > 0 {
		e.Args = fields
	}
	if cmdErr != nil {
		e.Message = cmdErr.Error()
	} else if blocked {
		e.Message = "critical findings detected"
	}
	oplog.Write(cfgPath, oplog.AuditFile, e) //nolint:errcheck
}

func auditSingleSkill(sourcePath, name string) (auditRunSummary, bool, error) {
	summary := auditRunSummary{
		Scope: "single",
		Skill: name,
	}

	skillPath := filepath.Join(sourcePath, name)
	if _, err := os.Stat(skillPath); os.IsNotExist(err) {
		return summary, false, fmt.Errorf("skill not found: %s", name)
	}

	ui.HeaderBox("skillshare audit", fmt.Sprintf("Scanning skill: %s", name))

	start := time.Now()
	result, err := audit.ScanSkill(skillPath)
	if err != nil {
		return summary, false, fmt.Errorf("scan error: %w", err)
	}
	elapsed := time.Since(start)

	printSkillResult(result, elapsed)
	printAuditSummary(1, []*audit.Result{result})

	summary = summarizeAuditResults(1, []*audit.Result{result})
	summary.Scope = "single"
	summary.Skill = name
	return summary, result.HasCritical(), nil
}

func auditAllSkills(sourcePath string) (auditRunSummary, bool, error) {
	// Discover all skills
	discovered, err := sync.DiscoverSourceSkills(sourcePath)
	if err != nil {
		return auditRunSummary{Scope: "all"}, false, fmt.Errorf("failed to discover skills: %w", err)
	}

	if len(discovered) == 0 {
		ui.Info("No skills found in source directory")
		return auditRunSummary{Scope: "all"}, false, nil
	}

	// Deduplicate by SourcePath — DiscoverSourceSkills may walk nested repos
	seen := make(map[string]bool)
	var skillPaths []struct {
		name string
		path string
	}
	for _, d := range discovered {
		if seen[d.SourcePath] {
			continue
		}
		seen[d.SourcePath] = true
		skillPaths = append(skillPaths, struct {
			name string
			path string
		}{d.FlatName, d.SourcePath})
	}

	// Also include top-level dirs without SKILL.md (might have .sh or other scannable files)
	entries, _ := os.ReadDir(sourcePath)
	for _, e := range entries {
		if !e.IsDir() || utils.IsHidden(e.Name()) {
			continue
		}
		p := filepath.Join(sourcePath, e.Name())
		if !seen[p] {
			seen[p] = true
			skillPaths = append(skillPaths, struct {
				name string
				path string
			}{e.Name(), p})
		}
	}

	total := len(skillPaths)
	ui.HeaderBox("skillshare audit", fmt.Sprintf("Scanning %d skills for threats", total))

	var results []*audit.Result
	scanErrors := 0
	for i, sp := range skillPaths {
		start := time.Now()
		result, err := audit.ScanSkill(sp.path)
		elapsed := time.Since(start)

		if err != nil {
			ui.ListItem("error", sp.name, fmt.Sprintf("scan error: %v", err))
			scanErrors++
			continue
		}

		results = append(results, result)
		printSkillResultLine(i+1, total, result, elapsed)
	}

	fmt.Println()
	printAuditSummary(total, results)

	summary := summarizeAuditResults(total, results)
	summary.Scope = "all"
	summary.ScanErrors = scanErrors

	for _, r := range results {
		if r.HasCritical() {
			return summary, true, nil
		}
	}

	return summary, false, nil
}

// printSkillResultLine prints a single-line result for a skill during batch scan.
func printSkillResultLine(index, total int, result *audit.Result, elapsed time.Duration) {
	prefix := fmt.Sprintf("[%d/%d]", index, total)
	name := result.SkillName
	timeStr := fmt.Sprintf("%.1fs", elapsed.Seconds())

	if len(result.Findings) == 0 {
		if ui.IsTTY() {
			fmt.Printf("%s \033[32m✓\033[0m %s %s%s%s\n", prefix, name, ui.Gray, timeStr, ui.Reset)
		} else {
			fmt.Printf("%s ✓ %s %s\n", prefix, name, timeStr)
		}
		return
	}

	sev := result.MaxSeverity()
	if sev == audit.SeverityCritical || sev == audit.SeverityHigh {
		if ui.IsTTY() {
			fmt.Printf("%s \033[31m✗\033[0m %s %s%s%s\n", prefix, name, ui.Gray, timeStr, ui.Reset)
		} else {
			fmt.Printf("%s ✗ %s %s\n", prefix, name, timeStr)
		}
	} else {
		if ui.IsTTY() {
			fmt.Printf("%s \033[33m!\033[0m %s %s%s%s\n", prefix, name, ui.Gray, timeStr, ui.Reset)
		} else {
			fmt.Printf("%s ! %s %s\n", prefix, name, timeStr)
		}
	}

	// Print finding details as tree
	for i, f := range result.Findings {
		var branch string
		if i < len(result.Findings)-1 {
			branch = "├─"
		} else {
			branch = "└─"
		}

		sevLabel := formatSeverity(f.Severity)
		loc := fmt.Sprintf("%s:%d", f.File, f.Line)
		if ui.IsTTY() {
			fmt.Printf("       %s %s: %s (%s)\n", branch, sevLabel, f.Message, loc)
			fmt.Printf("       %s  \033[90m\"%s\"\033[0m\n", continuationPrefix(i, len(result.Findings)), f.Snippet)
		} else {
			fmt.Printf("       %s %s: %s (%s)\n", branch, f.Severity, f.Message, loc)
			fmt.Printf("       %s  \"%s\"\n", continuationPrefix(i, len(result.Findings)), f.Snippet)
		}
	}
}

// printSkillResult prints detailed results for a single-skill audit.
func printSkillResult(result *audit.Result, elapsed time.Duration) {
	if len(result.Findings) == 0 {
		ui.Success("No issues found in %s (%.1fs)", result.SkillName, elapsed.Seconds())
		return
	}

	for _, f := range result.Findings {
		sevLabel := formatSeverity(f.Severity)
		loc := fmt.Sprintf("%s:%d", f.File, f.Line)
		if ui.IsTTY() {
			fmt.Printf("  %s: %s (%s)\n", sevLabel, f.Message, loc)
			fmt.Printf("  \033[90m\"%s\"\033[0m\n\n", f.Snippet)
		} else {
			fmt.Printf("  %s: %s (%s)\n", f.Severity, f.Message, loc)
			fmt.Printf("  \"%s\"\n\n", f.Snippet)
		}
	}
}

func printAuditSummary(total int, results []*audit.Result) {
	summary := summarizeAuditResults(total, results)

	var lines []string
	lines = append(lines, fmt.Sprintf("  Scanned:  %d skills", summary.Scanned))
	lines = append(lines, fmt.Sprintf("  Passed:   %d", summary.Passed))

	if summary.Warning > 0 {
		lines = append(lines, fmt.Sprintf("  Warning:  %d (%d medium)", summary.Warning, summary.Medium))
	} else {
		lines = append(lines, fmt.Sprintf("  Warning:  %d", summary.Warning))
	}

	if summary.Failed > 0 {
		parts := []string{}
		if summary.Critical > 0 {
			parts = append(parts, fmt.Sprintf("%d critical", summary.Critical))
		}
		if summary.High > 0 {
			parts = append(parts, fmt.Sprintf("%d high", summary.High))
		}
		lines = append(lines, fmt.Sprintf("  Failed:   %d (%s)", summary.Failed, joinParts(parts)))
	} else {
		lines = append(lines, fmt.Sprintf("  Failed:   %d", summary.Failed))
	}

	ui.Box("Summary", lines...)
}

func summarizeAuditResults(total int, results []*audit.Result) auditRunSummary {
	summary := auditRunSummary{Scanned: total}

	for _, r := range results {
		c, h, m := r.CountBySeverity()
		summary.Critical += c
		summary.High += h
		summary.Medium += m

		switch r.MaxSeverity() {
		case audit.SeverityCritical, audit.SeverityHigh:
			summary.Failed++
			summary.FailSkills = append(summary.FailSkills, r.SkillName)
		case audit.SeverityMedium:
			summary.Warning++
			summary.WarnSkills = append(summary.WarnSkills, r.SkillName)
		default:
			summary.Passed++
		}
	}

	return summary
}

func formatSeverity(sev string) string {
	if !ui.IsTTY() {
		return sev
	}
	switch sev {
	case audit.SeverityCritical:
		return "\033[31;1mCRITICAL\033[0m"
	case audit.SeverityHigh:
		return "\033[33;1mHIGH\033[0m"
	case audit.SeverityMedium:
		return "\033[36mMEDIUM\033[0m"
	}
	return sev
}

func continuationPrefix(index, total int) string {
	if index < total-1 {
		return "│ "
	}
	return "  "
}

func joinParts(parts []string) string {
	result := ""
	for i, p := range parts {
		if i > 0 {
			result += ", "
		}
		result += p
	}
	return result
}

func printAuditHelp() {
	fmt.Println("Usage: skillshare audit [name] [options]")
	fmt.Println()
	fmt.Println("Scan installed skills for security threats.")
	fmt.Println()
	fmt.Println("Arguments:")
	fmt.Println("  name              Scan a specific skill (optional)")
	fmt.Println()
	fmt.Println("Options:")
	fmt.Println("  -p, --project     Use project-level skills")
	fmt.Println("  -g, --global      Use global skills")
	fmt.Println("  -h, --help        Show this help")
	fmt.Println()
	fmt.Println("Examples:")
	fmt.Println("  skillshare audit                  Scan all installed skills")
	fmt.Println("  skillshare audit react-patterns    Scan a specific skill")
	fmt.Println("  skillshare audit -p                Scan project skills")
}
