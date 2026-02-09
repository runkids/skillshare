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

	if mode == modeProject {
		err := cmdAuditProject(cwd, rest)
		logAuditOp(config.ProjectConfigPath(cwd), rest, start, err)
		return err
	}

	cfg, err := config.Load()
	if err != nil {
		return err
	}

	// Determine what to scan
	var specificSkill string
	for _, a := range rest {
		if a == "--help" || a == "-h" {
			printAuditHelp()
			return nil
		}
		if specificSkill == "" {
			specificSkill = a
		}
	}

	if specificSkill != "" {
		err = auditSingleSkill(cfg.Source, specificSkill)
		logAuditOp(config.ConfigPath(), rest, start, err)
		return err
	}

	err = auditAllSkills(cfg.Source)
	logAuditOp(config.ConfigPath(), rest, start, err)
	return err
}

func logAuditOp(cfgPath string, args []string, start time.Time, cmdErr error) {
	e := oplog.NewEntry("audit", statusFromErr(cmdErr), time.Since(start))
	if len(args) > 0 {
		e.Args = map[string]any{"name": args[0]}
	}
	if cmdErr != nil {
		e.Message = cmdErr.Error()
	}
	oplog.Write(cfgPath, oplog.AuditFile, e) //nolint:errcheck
}

func auditSingleSkill(sourcePath, name string) error {
	skillPath := filepath.Join(sourcePath, name)
	if _, err := os.Stat(skillPath); os.IsNotExist(err) {
		return fmt.Errorf("skill not found: %s", name)
	}

	ui.HeaderBox("skillshare audit", fmt.Sprintf("Scanning skill: %s", name))

	start := time.Now()
	result, err := audit.ScanSkill(skillPath)
	if err != nil {
		return fmt.Errorf("scan error: %w", err)
	}
	elapsed := time.Since(start)

	printSkillResult(result, elapsed)
	printAuditSummary(1, []*audit.Result{result})

	if result.HasCritical() {
		os.Exit(1)
	}
	return nil
}

func auditAllSkills(sourcePath string) error {
	// Discover all skills
	discovered, err := sync.DiscoverSourceSkills(sourcePath)
	if err != nil {
		return fmt.Errorf("failed to discover skills: %w", err)
	}

	if len(discovered) == 0 {
		ui.Info("No skills found in source directory")
		return nil
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
	for i, sp := range skillPaths {
		start := time.Now()
		result, err := audit.ScanSkill(sp.path)
		elapsed := time.Since(start)

		if err != nil {
			ui.ListItem("error", sp.name, fmt.Sprintf("scan error: %v", err))
			continue
		}

		results = append(results, result)
		printSkillResultLine(i+1, total, result, elapsed)
	}

	fmt.Println()
	printAuditSummary(total, results)

	// Exit with code 1 if any critical findings
	for _, r := range results {
		if r.HasCritical() {
			os.Exit(1)
		}
	}

	return nil
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
	passed := 0
	warning := 0
	failed := 0
	var totalCritical, totalHigh, totalMedium int

	for _, r := range results {
		c, h, m := r.CountBySeverity()
		totalCritical += c
		totalHigh += h
		totalMedium += m

		switch r.MaxSeverity() {
		case audit.SeverityCritical, audit.SeverityHigh:
			failed++
		case audit.SeverityMedium:
			warning++
		default:
			passed++
		}
	}

	var lines []string
	lines = append(lines, fmt.Sprintf("  Scanned:  %d skills", total))
	lines = append(lines, fmt.Sprintf("  Passed:   %d", passed))

	if warning > 0 {
		lines = append(lines, fmt.Sprintf("  Warning:  %d (%d medium)", warning, totalMedium))
	} else {
		lines = append(lines, fmt.Sprintf("  Warning:  %d", warning))
	}

	if failed > 0 {
		parts := []string{}
		if totalCritical > 0 {
			parts = append(parts, fmt.Sprintf("%d critical", totalCritical))
		}
		if totalHigh > 0 {
			parts = append(parts, fmt.Sprintf("%d high", totalHigh))
		}
		lines = append(lines, fmt.Sprintf("  Failed:   %d (%s)", failed, joinParts(parts)))
	} else {
		lines = append(lines, fmt.Sprintf("  Failed:   %d", failed))
	}

	ui.Box("Summary", lines...)
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
