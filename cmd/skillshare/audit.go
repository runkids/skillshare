package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"skillshare/internal/audit"
	"skillshare/internal/config"
	"skillshare/internal/oplog"
	"skillshare/internal/sync"
	"skillshare/internal/ui"
	"skillshare/internal/utils"
)

type auditOptions struct {
	Targets   []string
	Groups    []string
	InitRules bool
	JSON      bool
	Threshold string
}

type auditRunSummary struct {
	Scope       string   `json:"scope,omitempty"`
	Skill       string   `json:"skill,omitempty"`
	Path        string   `json:"path,omitempty"`
	Scanned     int      `json:"scanned"`
	Passed      int      `json:"passed"`
	Warning     int      `json:"warning"`
	Failed      int      `json:"failed"`
	Critical    int      `json:"critical"`
	High        int      `json:"high"`
	Medium      int      `json:"medium"`
	Low         int      `json:"low"`
	Info        int      `json:"info"`
	WarnSkills  []string `json:"warningSkills,omitempty"`
	FailSkills  []string `json:"failedSkills,omitempty"`
	LowSkills   []string `json:"lowSkills,omitempty"`
	InfoSkills  []string `json:"infoSkills,omitempty"`
	ScanErrors  int      `json:"scanErrors"`
	Mode        string   `json:"mode,omitempty"`
	Threshold   string   `json:"threshold,omitempty"`
	MaxSeverity string   `json:"maxSeverity,omitempty"`
	RiskScore   int      `json:"riskScore"`
	RiskLabel   string   `json:"riskLabel,omitempty"`
}

type auditJSONOutput struct {
	Results []*audit.Result `json:"results"`
	Summary auditRunSummary `json:"summary"`
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

	opts, showHelp, err := parseAuditArgs(rest)
	if showHelp {
		return err
	}
	if err != nil {
		return err
	}
	if opts.InitRules {
		if mode == modeProject {
			return initAuditRules(audit.ProjectAuditRulesPath(cwd))
		}
		return initAuditRules(audit.GlobalAuditRulesPath())
	}

	var (
		sourcePath       string
		projectRoot      string
		defaultThreshold string
		cfgPath          string
	)

	// Path mode: exactly 1 target that is an existing file/directory — no config needed.
	isSinglePath := len(opts.Targets) == 1 && len(opts.Groups) == 0 && pathExists(opts.Targets[0])
	if isSinglePath {
		if mode == modeProject {
			projectRoot = cwd
			cfgPath = config.ProjectConfigPath(cwd)
		} else {
			cfgPath = config.ConfigPath()
		}
	} else if mode == modeProject {
		rt, err := loadProjectRuntime(cwd)
		if err != nil {
			return err
		}
		sourcePath = rt.sourcePath
		projectRoot = cwd
		defaultThreshold = rt.config.Audit.BlockThreshold
		cfgPath = config.ProjectConfigPath(cwd)
	} else {
		cfg, err := config.Load()
		if err != nil {
			return err
		}
		sourcePath = cfg.Source
		defaultThreshold = cfg.Audit.BlockThreshold
		cfgPath = config.ConfigPath()
	}

	threshold := defaultThreshold
	if opts.Threshold != "" {
		threshold = opts.Threshold
	}
	threshold, err = audit.NormalizeThreshold(threshold)
	if err != nil {
		return err
	}

	var (
		results []*audit.Result
		summary auditRunSummary
	)

	hasTargets := len(opts.Targets) > 0 || len(opts.Groups) > 0
	isSingleName := len(opts.Targets) == 1 && len(opts.Groups) == 0 && !pathExists(opts.Targets[0])

	switch {
	case !hasTargets:
		results, summary, err = auditInstalled(sourcePath, modeString(mode), projectRoot, threshold, opts.JSON)
	case isSinglePath:
		results, summary, err = auditPath(opts.Targets[0], modeString(mode), projectRoot, threshold, opts.JSON)
	case isSingleName:
		results, summary, err = auditSkillByName(sourcePath, opts.Targets[0], modeString(mode), projectRoot, threshold, opts.JSON)
	default:
		results, summary, err = auditFiltered(sourcePath, opts.Targets, opts.Groups, modeString(mode), projectRoot, threshold, opts.JSON)
	}
	if err != nil {
		logAuditOp(cfgPath, rest, summary, start, err, false)
		return err
	}

	blocked := summary.Failed > 0
	logAuditOp(cfgPath, rest, summary, start, nil, blocked)

	if opts.JSON {
		out, _ := json.MarshalIndent(auditJSONOutput{
			Results: results,
			Summary: summary,
		}, "", "  ")
		fmt.Println(string(out))
		if blocked {
			os.Exit(1)
		}
		return nil
	}

	if blocked {
		os.Exit(1)
	}
	return nil
}

func modeString(mode runMode) string {
	if mode == modeProject {
		return "project"
	}
	return "global"
}

func parseAuditArgs(args []string) (auditOptions, bool, error) {
	opts := auditOptions{}
	for i := 0; i < len(args); i++ {
		arg := args[i]
		switch arg {
		case "--help", "-h":
			printAuditHelp()
			return opts, true, nil
		case "--init-rules":
			opts.InitRules = true
		case "--json":
			opts.JSON = true
		case "--threshold", "-T":
			if i+1 >= len(args) {
				return opts, false, fmt.Errorf("%s requires a value", arg)
			}
			i++
			threshold, err := normalizeInstallAuditThreshold(args[i])
			if err != nil {
				return opts, false, err
			}
			opts.Threshold = threshold
		case "--group", "-G":
			if i+1 >= len(args) {
				return opts, false, fmt.Errorf("--group requires a value")
			}
			i++
			opts.Groups = append(opts.Groups, args[i])
		default:
			if strings.HasPrefix(arg, "-") {
				return opts, false, fmt.Errorf("unknown option: %s", arg)
			}
			opts.Targets = append(opts.Targets, arg)
		}
	}
	return opts, false, nil
}

func pathExists(path string) bool {
	if path == "" {
		return false
	}
	_, err := os.Stat(path)
	return err == nil
}

func auditHeaderSubtitle(scanLine, mode, sourcePath, threshold string) string {
	displayPath := sourcePath
	if abs, err := filepath.Abs(sourcePath); err == nil {
		displayPath = abs
	}
	return fmt.Sprintf("%s\nmode: %s\npath: %s\nblock rule: finding severity >= %s", scanLine, mode, displayPath, threshold)
}

func collectInstalledSkillPaths(sourcePath string) ([]struct {
	name string
	path string
}, error) {
	discovered, err := sync.DiscoverSourceSkills(sourcePath)
	if err != nil {
		return nil, fmt.Errorf("failed to discover skills: %w", err)
	}

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

	return skillPaths, nil
}

// resolveSkillPath searches installed skills for a match by flat name or basename.
// Returns the full path if found, empty string otherwise.
func resolveSkillPath(sourcePath, name string) string {
	skills, err := collectInstalledSkillPaths(sourcePath)
	if err != nil {
		return ""
	}
	for _, sp := range skills {
		if sp.name == name || filepath.Base(sp.path) == name {
			return sp.path
		}
	}
	return ""
}

func scanSkillPath(skillPath, projectRoot string) (*audit.Result, error) {
	if projectRoot != "" {
		return audit.ScanSkillForProject(skillPath, projectRoot)
	}
	return audit.ScanSkill(skillPath)
}

func toAuditInputs(skills []struct {
	name string
	path string
}) []audit.SkillInput {
	inputs := make([]audit.SkillInput, len(skills))
	for i, s := range skills {
		inputs[i] = audit.SkillInput{Name: s.name, Path: s.path}
	}
	return inputs
}

func scanPathTarget(targetPath, projectRoot string) (*audit.Result, error) {
	info, err := os.Stat(targetPath)
	if err != nil {
		return nil, err
	}
	if info.IsDir() {
		return scanSkillPath(targetPath, projectRoot)
	}
	if projectRoot != "" {
		return audit.ScanFileForProject(targetPath, projectRoot)
	}
	return audit.ScanFile(targetPath)
}

func auditInstalled(sourcePath, mode, projectRoot, threshold string, jsonOutput bool) ([]*audit.Result, auditRunSummary, error) {
	base := auditRunSummary{
		Scope:     "all",
		Mode:      mode,
		Threshold: threshold,
	}

	skillPaths, err := collectInstalledSkillPaths(sourcePath)
	if err != nil {
		return nil, base, err
	}
	if len(skillPaths) == 0 {
		if !jsonOutput {
			ui.Info("No skills found in source directory")
		}
		return []*audit.Result{}, base, nil
	}

	if !jsonOutput {
		ui.HeaderBox("skillshare audit", auditHeaderSubtitle(fmt.Sprintf("Scanning %d skills for threats", len(skillPaths)), mode, sourcePath, threshold))
	}

	// Phase 1: parallel scan with bounded workers.
	scanResults := audit.ParallelScan(toAuditInputs(skillPaths), projectRoot)

	// Phase 2: sequential output and result collection.
	results := make([]*audit.Result, 0, len(skillPaths))
	scanErrors := 0
	for i, sp := range skillPaths {
		sr := scanResults[i]
		if sr.Err != nil {
			scanErrors++
			if !jsonOutput {
				ui.ListItem("error", sp.name, fmt.Sprintf("scan error: %v", sr.Err))
			}
			continue
		}
		sr.Result.Threshold = threshold
		sr.Result.IsBlocked = sr.Result.HasSeverityAtOrAbove(threshold)
		results = append(results, sr.Result)
		if !jsonOutput {
			printSkillResultLine(i+1, len(skillPaths), sr.Result, sr.Elapsed)
		}
	}

	if !jsonOutput {
		fmt.Println()
	}

	summary := summarizeAuditResults(len(skillPaths), results, threshold)
	summary.Scope = "all"
	summary.Mode = mode
	summary.ScanErrors = scanErrors

	if !jsonOutput {
		printAuditSummary(summary)
	}

	return results, summary, nil
}

func auditFiltered(sourcePath string, names, groups []string, mode, projectRoot, threshold string, jsonOutput bool) ([]*audit.Result, auditRunSummary, error) {
	base := auditRunSummary{
		Scope:     "filtered",
		Mode:      mode,
		Threshold: threshold,
	}

	allSkills, err := collectInstalledSkillPaths(sourcePath)
	if err != nil {
		return nil, base, err
	}

	// Build match sets for O(1) lookup.
	nameSet := make(map[string]bool, len(names))
	for _, n := range names {
		nameSet[n] = true
	}

	// Filter skills by names and groups.
	seen := make(map[string]bool)
	var matched []struct {
		name string
		path string
	}
	resolvedNames := make(map[string]bool)

	for _, sp := range allSkills {
		// Name match: flat name or basename.
		if nameSet[sp.name] || nameSet[filepath.Base(sp.path)] {
			if !seen[sp.path] {
				seen[sp.path] = true
				matched = append(matched, sp)
			}
			resolvedNames[sp.name] = true
			resolvedNames[filepath.Base(sp.path)] = true
			continue
		}

		// Group match: flat name starts with group+"__".
		for _, g := range groups {
			if strings.HasPrefix(sp.name, g+"__") {
				if !seen[sp.path] {
					seen[sp.path] = true
					matched = append(matched, sp)
				}
				break
			}
		}
	}

	// Warn about unresolved names.
	var warnings []string
	for _, n := range names {
		if !resolvedNames[n] {
			warnings = append(warnings, n)
		}
	}
	for _, w := range warnings {
		if !jsonOutput {
			ui.Warning("skill not found: %s", w)
		}
	}

	if len(matched) == 0 {
		return nil, base, fmt.Errorf("no skills matched the given names/groups")
	}

	if !jsonOutput {
		ui.HeaderBox("skillshare audit", auditHeaderSubtitle(fmt.Sprintf("Scanning %d skills for threats", len(matched)), mode, sourcePath, threshold))
	}

	// Phase 1: parallel scan.
	scanResults := audit.ParallelScan(toAuditInputs(matched), projectRoot)

	// Phase 2: sequential output and result collection.
	results := make([]*audit.Result, 0, len(matched))
	scanErrors := 0
	for i, sp := range matched {
		sr := scanResults[i]
		if sr.Err != nil {
			scanErrors++
			if !jsonOutput {
				ui.ListItem("error", sp.name, fmt.Sprintf("scan error: %v", sr.Err))
			}
			continue
		}
		sr.Result.Threshold = threshold
		sr.Result.IsBlocked = sr.Result.HasSeverityAtOrAbove(threshold)
		results = append(results, sr.Result)
		if !jsonOutput {
			printSkillResultLine(i+1, len(matched), sr.Result, sr.Elapsed)
		}
	}

	if !jsonOutput {
		fmt.Println()
	}

	summary := summarizeAuditResults(len(matched), results, threshold)
	summary.Scope = "filtered"
	summary.Mode = mode
	summary.ScanErrors = scanErrors

	if !jsonOutput {
		printAuditSummary(summary)
	}

	return results, summary, nil
}

func auditSkillByName(sourcePath, name, mode, projectRoot, threshold string, jsonOutput bool) ([]*audit.Result, auditRunSummary, error) {
	summary := auditRunSummary{
		Scope:     "single",
		Skill:     name,
		Mode:      mode,
		Threshold: threshold,
	}

	skillPath := filepath.Join(sourcePath, name)
	if _, err := os.Stat(skillPath); os.IsNotExist(err) {
		// Short-name fallback: search installed skills by flat name or basename.
		resolved := resolveSkillPath(sourcePath, name)
		if resolved == "" {
			return nil, summary, fmt.Errorf("skill not found: %s", name)
		}
		skillPath = resolved
	}

	if !jsonOutput {
		ui.HeaderBox("skillshare audit", auditHeaderSubtitle(fmt.Sprintf("Scanning skill: %s", name), mode, sourcePath, threshold))
	}

	start := time.Now()
	result, err := scanSkillPath(skillPath, projectRoot)
	if err != nil {
		return nil, summary, fmt.Errorf("scan error: %w", err)
	}
	elapsed := time.Since(start)
	result.Threshold = threshold
	result.IsBlocked = result.HasSeverityAtOrAbove(threshold)

	if !jsonOutput {
		printSkillResult(result, elapsed)
	}

	summary = summarizeAuditResults(1, []*audit.Result{result}, threshold)
	summary.Scope = "single"
	summary.Skill = name
	summary.Mode = mode
	if !jsonOutput {
		printAuditSummary(summary)
	}

	return []*audit.Result{result}, summary, nil
}

func auditPath(rawPath, mode, projectRoot, threshold string, jsonOutput bool) ([]*audit.Result, auditRunSummary, error) {
	absPath, err := filepath.Abs(rawPath)
	if err != nil {
		absPath = rawPath
	}

	summary := auditRunSummary{
		Scope:     "path",
		Path:      absPath,
		Mode:      mode,
		Threshold: threshold,
	}

	if !jsonOutput {
		ui.HeaderBox("skillshare audit", fmt.Sprintf("Scanning path target\nmode: %s\npath: %s\nblock rule: finding severity >= %s", mode, absPath, threshold))
	}

	start := time.Now()
	result, err := scanPathTarget(absPath, projectRoot)
	if err != nil {
		return nil, summary, fmt.Errorf("scan error: %w", err)
	}
	elapsed := time.Since(start)
	result.ScanTarget = absPath
	result.Threshold = threshold
	result.IsBlocked = result.HasSeverityAtOrAbove(threshold)

	if !jsonOutput {
		printSkillResult(result, elapsed)
	}

	summary = summarizeAuditResults(1, []*audit.Result{result}, threshold)
	summary.Scope = "path"
	summary.Path = absPath
	summary.Mode = mode
	if !jsonOutput {
		printAuditSummary(summary)
	}
	return []*audit.Result{result}, summary, nil
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
	if summary.Path != "" {
		fields["path"] = summary.Path
	}
	if summary.Mode != "" {
		fields["mode"] = summary.Mode
	}
	if summary.Threshold != "" {
		fields["threshold"] = summary.Threshold
	}
	if summary.MaxSeverity != "" {
		fields["max_severity"] = summary.MaxSeverity
	}
	if summary.Scanned > 0 {
		fields["scanned"] = summary.Scanned
		fields["passed"] = summary.Passed
		fields["warning"] = summary.Warning
		fields["failed"] = summary.Failed
		fields["critical"] = summary.Critical
		fields["high"] = summary.High
		fields["medium"] = summary.Medium
		fields["low"] = summary.Low
		fields["info"] = summary.Info
		fields["risk_score"] = summary.RiskScore
		fields["risk_label"] = summary.RiskLabel
		if len(summary.WarnSkills) > 0 {
			fields["warning_skills"] = summary.WarnSkills
		}
		if len(summary.FailSkills) > 0 {
			fields["failed_skills"] = summary.FailSkills
		}
		if len(summary.LowSkills) > 0 {
			fields["low_skills"] = summary.LowSkills
		}
		if len(summary.InfoSkills) > 0 {
			fields["info_skills"] = summary.InfoSkills
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
		e.Message = fmt.Sprintf("findings at/above %s detected", summary.Threshold)
	}
	oplog.Write(cfgPath, oplog.AuditFile, e) //nolint:errcheck
}

func summarizeAuditResults(total int, results []*audit.Result, threshold string) auditRunSummary {
	summary := auditRunSummary{
		Scanned:   total,
		Threshold: threshold,
	}

	maxRisk := 0
	maxSeverity := ""
	for _, r := range results {
		c, h, m, l, i := r.CountBySeverityAll()
		summary.Critical += c
		summary.High += h
		summary.Medium += m
		summary.Low += l
		summary.Info += i

		if containsSeverity(r.Findings, audit.SeverityLow) {
			summary.LowSkills = append(summary.LowSkills, r.SkillName)
		}
		if containsSeverity(r.Findings, audit.SeverityInfo) {
			summary.InfoSkills = append(summary.InfoSkills, r.SkillName)
		}

		if len(r.Findings) == 0 {
			summary.Passed++
		} else if r.HasSeverityAtOrAbove(threshold) {
			summary.Failed++
			summary.FailSkills = append(summary.FailSkills, r.SkillName)
		} else {
			summary.Warning++
			summary.WarnSkills = append(summary.WarnSkills, r.SkillName)
		}

		if r.RiskScore > maxRisk {
			maxRisk = r.RiskScore
		}
		if rs := r.MaxSeverity(); rs != "" {
			if maxSeverity == "" || audit.SeverityRank(rs) < audit.SeverityRank(maxSeverity) {
				maxSeverity = rs
			}
		}
	}
	summary.RiskScore = maxRisk
	summary.RiskLabel = audit.RiskLabelFromScoreAndMaxSeverity(maxRisk, maxSeverity)
	summary.MaxSeverity = maxSeverity
	return summary
}

func containsSeverity(findings []audit.Finding, severity string) bool {
	for _, f := range findings {
		if f.Severity == severity {
			return true
		}
	}
	return false
}

// riskColor maps a risk label to an ANSI color, aligned with formatSeverity.
func riskColor(label string) string {
	if c := ui.SeverityColor(label); c != "" {
		return c
	}
	return ui.Gray
}

// printSkillResultLine prints a single-line result for a skill during batch scan.
func printSkillResultLine(index, total int, result *audit.Result, elapsed time.Duration) {
	prefix := fmt.Sprintf("[%d/%d]", index, total)
	name := result.SkillName
	showTime := elapsed >= time.Second
	timeStr := fmt.Sprintf("%.1fs", elapsed.Seconds())

	if len(result.Findings) == 0 {
		if ui.IsTTY() {
			if showTime {
				fmt.Printf("%s \033[32m✓\033[0m %s %s%s%s\n", prefix, name, ui.Gray, timeStr, ui.Reset)
			} else {
				fmt.Printf("%s \033[32m✓\033[0m %s\n", prefix, name)
			}
		} else {
			if showTime {
				fmt.Printf("%s ✓ %s %s\n", prefix, name, timeStr)
			} else {
				fmt.Printf("%s ✓ %s\n", prefix, name)
			}
		}
		return
	}

	color := riskColor(result.RiskLabel)
	symbol := "!"
	if result.IsBlocked {
		symbol = "✗"
	}
	maxSeverity := result.MaxSeverity()
	if maxSeverity == "" {
		maxSeverity = "NONE"
	}
	riskText := fmt.Sprintf("AGG %s %d/100, max %s", strings.ToUpper(result.RiskLabel), result.RiskScore, maxSeverity)

	if ui.IsTTY() {
		if showTime {
			fmt.Printf("%s %s%s%s %s  %s(%s)%s  %s%s%s\n", prefix, color, symbol, ui.Reset, name, color, riskText, ui.Reset, ui.Gray, timeStr, ui.Reset)
		} else {
			fmt.Printf("%s %s%s%s %s  %s(%s)%s\n", prefix, color, symbol, ui.Reset, name, color, riskText, ui.Reset)
		}
	} else {
		if showTime {
			fmt.Printf("%s %s %s  (%s)  %s\n", prefix, symbol, name, riskText, timeStr)
		} else {
			fmt.Printf("%s %s %s  (%s)\n", prefix, symbol, name, riskText)
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

	color := riskColor(result.RiskLabel)
	threshold := result.Threshold
	if threshold == "" {
		threshold = audit.DefaultThreshold()
	}
	maxSeverity := result.MaxSeverity()
	if maxSeverity == "" {
		maxSeverity = "NONE"
	}
	decision := "ALLOW"
	compare := "<"
	if result.IsBlocked {
		decision = "BLOCK"
		compare = ">="
	}
	if ui.IsTTY() {
		fmt.Printf("%s→%s Aggregate risk: %s%s (%d/100)%s\n", ui.Cyan, ui.Reset, color, strings.ToUpper(result.RiskLabel), result.RiskScore, ui.Reset)
		fmt.Printf("%s→%s Block decision: %s (max severity %s %s threshold %s)\n", ui.Cyan, ui.Reset, decision, maxSeverity, compare, threshold)
	} else {
		fmt.Printf("→ Aggregate risk: %s (%d/100)\n", strings.ToUpper(result.RiskLabel), result.RiskScore)
		fmt.Printf("→ Block decision: %s (max severity %s %s threshold %s)\n", decision, maxSeverity, compare, threshold)
	}
}

func printAuditSummary(summary auditRunSummary) {
	var lines []string
	maxSeverity := summary.MaxSeverity
	if maxSeverity == "" {
		maxSeverity = "NONE"
	}
	lines = append(lines, fmt.Sprintf("  Block:     severity >= %s", summary.Threshold))
	lines = append(lines, fmt.Sprintf("  Max sev:   %s", maxSeverity))
	lines = append(lines, fmt.Sprintf("  Scanned:   %d skill(s)", summary.Scanned))
	lines = append(lines, fmt.Sprintf("  Passed:    %s", ui.Colorize(ui.Green, fmt.Sprintf("%d", summary.Passed))))
	if summary.Warning > 0 {
		lines = append(lines, fmt.Sprintf("  Warning:   %s", ui.Colorize(ui.Yellow, fmt.Sprintf("%d", summary.Warning))))
	} else {
		lines = append(lines, fmt.Sprintf("  Warning:   %d", summary.Warning))
	}
	if summary.Failed > 0 {
		lines = append(lines, fmt.Sprintf("  Failed:    %s", ui.Colorize(ui.Red, fmt.Sprintf("%d", summary.Failed))))
	} else {
		lines = append(lines, fmt.Sprintf("  Failed:    %d", summary.Failed))
	}
	lines = append(lines, fmt.Sprintf("  Severity:  c/h/m/l/i = %s/%s/%s/%s/%d",
		ui.Colorize(ui.Red, fmt.Sprintf("%d", summary.Critical)),
		ui.Colorize(ui.Orange, fmt.Sprintf("%d", summary.High)),
		ui.Colorize(ui.Yellow, fmt.Sprintf("%d", summary.Medium)),
		ui.Colorize(ui.Gray, fmt.Sprintf("%d", summary.Low)),
		summary.Info))
	riskLabel := strings.ToUpper(summary.RiskLabel)
	riskText := fmt.Sprintf("%s (%d/100)", riskLabel, summary.RiskScore)
	lines = append(lines, fmt.Sprintf("  Aggregate: %s", ui.Colorize(riskColor(summary.RiskLabel), riskText)))
	lines = append(lines, "  Note:      Failed uses severity gate; aggregate is informational")
	if summary.ScanErrors > 0 {
		lines = append(lines, fmt.Sprintf("  Scan errs: %d", summary.ScanErrors))
	}
	ui.Box("Summary", lines...)
}

func formatSeverity(sev string) string {
	return ui.Colorize(ui.SeverityColor(sev), strings.ToUpper(sev))
}

func initAuditRules(path string) error {
	if err := audit.InitRulesFile(path); err != nil {
		return err
	}
	ui.Success("Created %s", path)
	return nil
}

func printAuditHelp() {
	fmt.Println(`Usage: skillshare audit [name...] [options]
       skillshare audit --group <group> [options]
       skillshare audit <path> [options]

Scan installed skills (or a specific skill/path) for security threats.

If no names or groups are specified, all installed skills are scanned.
Block decisions use severity threshold; aggregate risk score is reported separately.

Arguments:
  name...              Skill name(s) to scan (optional)
  path                 Existing file/directory path to scan (optional)

Options:
  --group, -G <name>   Scan all skills in a group (repeatable)
  -p, --project        Use project-level skills
  -g, --global         Use global skills
  --threshold, -T <t>  Block by severity at/above: critical|high|medium|low|info
                       (also supports c|h|m|l|i)
  --json               Output JSON
  --init-rules         Create a starter audit-rules.yaml
  -h, --help           Show this help

Examples:
  skillshare audit                           # Scan all installed skills
  skillshare audit react-patterns            # Scan a specific skill
  skillshare audit a b c                     # Scan multiple skills
  skillshare audit --group frontend          # Scan all skills in frontend/
  skillshare audit x -G backend              # Mix names and groups
  skillshare audit ./skills/my-skill         # Scan a directory path
  skillshare audit ./skills/foo/SKILL.md     # Scan a single file
  skillshare audit --threshold high          # Block on HIGH+ findings
  skillshare audit -T h                      # Same, with shorthand alias
  skillshare audit --json                    # Output machine-readable results
  skillshare audit -p --init-rules           # Create project custom rules file`)
}
