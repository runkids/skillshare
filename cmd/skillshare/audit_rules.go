package main

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"skillshare/internal/audit"
	"skillshare/internal/ui"
)

type auditRulesOptions struct {
	Pattern  string
	Severity string
	Disabled bool
	Format   string
	NoTUI    bool
}

func cmdAuditRules(mode runMode, args []string) error {
	if len(args) > 0 {
		switch args[0] {
		case "disable":
			return cmdAuditRulesDisable(mode, args[1:])
		case "enable":
			return cmdAuditRulesEnable(mode, args[1:])
		case "severity":
			return cmdAuditRulesSeverity(mode, args[1:])
		case "reset":
			return cmdAuditRulesReset(mode)
		case "init":
			return cmdAuditRulesInit(mode)
		case "--help", "-h":
			printAuditRulesHelp()
			return nil
		}
	}
	return cmdAuditRulesList(mode, args)
}

func cmdAuditRulesList(mode runMode, args []string) error {
	opts, err := parseAuditRulesListArgs(args)
	if err != nil {
		return err
	}

	cwd, _ := os.Getwd()
	var rules []audit.CompiledRule
	if mode == modeProject {
		rules, err = audit.ListRulesWithProject(cwd)
	} else {
		rules, err = audit.ListRules()
	}
	if err != nil {
		return fmt.Errorf("list rules: %w", err)
	}

	rules = filterCompiledRules(rules, opts)

	if opts.Format == "json" {
		patterns := audit.PatternSummary(rules)
		out, _ := json.MarshalIndent(struct {
			Rules    []audit.CompiledRule `json:"rules"`
			Patterns []audit.PatternGroup `json:"patterns"`
		}{rules, patterns}, "", "  ")
		fmt.Println(string(out))
		return nil
	}

	if !opts.NoTUI && ui.IsTTY() && len(rules) > 0 {
		return runAuditRulesTUI(rules, mode)
	}

	return printAuditRulesTable(rules)
}

func filterCompiledRules(rules []audit.CompiledRule, opts auditRulesOptions) []audit.CompiledRule {
	var filtered []audit.CompiledRule
	for _, r := range rules {
		if opts.Pattern != "" && r.Pattern != opts.Pattern {
			continue
		}
		if opts.Severity != "" && audit.SeverityRank(r.Severity) > audit.SeverityRank(strings.ToUpper(opts.Severity)) {
			continue
		}
		if opts.Disabled && r.Enabled {
			continue
		}
		filtered = append(filtered, r)
	}
	return filtered
}

func printAuditRulesTable(rules []audit.CompiledRule) error {
	if len(rules) == 0 {
		ui.Info("No rules match the filter.")
		return nil
	}

	fmt.Printf("%-24s %-40s %-10s %s\n", "Pattern", "ID", "Severity", "Status")
	fmt.Println(strings.Repeat("\u2500", 90))

	enabledCount := 0
	disabledCount := 0
	for _, r := range rules {
		status := "enabled"
		if !r.Enabled {
			status = "disabled"
			disabledCount++
		} else {
			enabledCount++
		}
		sevLabel := r.Severity
		if ui.IsTTY() {
			sevLabel = ui.Colorize(ui.SeverityColor(r.Severity), r.Severity)
		}
		fmt.Printf("%-24s %-40s %-10s %s\n", r.Pattern, r.ID, sevLabel, status)
	}
	fmt.Println(strings.Repeat("\u2500", 90))
	fmt.Printf("(%d rules: %d enabled, %d disabled)\n", len(rules), enabledCount, disabledCount)
	return nil
}

func parseAuditRulesListArgs(args []string) (auditRulesOptions, error) {
	var opts auditRulesOptions
	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "--pattern":
			if i+1 >= len(args) {
				return opts, fmt.Errorf("--pattern requires a value")
			}
			i++
			opts.Pattern = args[i]
		case "--severity":
			if i+1 >= len(args) {
				return opts, fmt.Errorf("--severity requires a value")
			}
			i++
			opts.Severity = args[i]
		case "--disabled":
			opts.Disabled = true
		case "--format":
			if i+1 >= len(args) {
				return opts, fmt.Errorf("--format requires a value")
			}
			i++
			opts.Format = args[i]
		case "--no-tui":
			opts.NoTUI = true
		case "--help", "-h":
			printAuditRulesHelp()
			return opts, nil
		}
	}
	return opts, nil
}

func cmdAuditRulesDisable(mode runMode, args []string) error {
	return cmdAuditRulesToggle(mode, args, false)
}

func cmdAuditRulesEnable(mode runMode, args []string) error {
	return cmdAuditRulesToggle(mode, args, true)
}

func cmdAuditRulesToggle(mode runMode, args []string, enabled bool) error {
	cwd, _ := os.Getwd()
	rulesPath := auditRulesPathForMode(mode, cwd)

	verb := "Enabled"
	if !enabled {
		verb = "Disabled"
	}

	var pattern string
	var ids []string
	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "--pattern":
			if i+1 >= len(args) {
				return fmt.Errorf("--pattern requires a value")
			}
			i++
			pattern = args[i]
		default:
			if !strings.HasPrefix(args[i], "-") {
				ids = append(ids, args[i])
			}
		}
	}

	if pattern != "" {
		if err := audit.TogglePattern(rulesPath, pattern, enabled); err != nil {
			return err
		}
		ui.Success("%s all rules in pattern: %s", verb, pattern)
		return nil
	}

	if len(ids) == 0 {
		return fmt.Errorf("specify a rule ID or --pattern <name>")
	}
	if err := audit.ToggleRules(rulesPath, ids, enabled); err != nil {
		return err
	}
	for _, id := range ids {
		ui.Success("%s rule: %s", verb, id)
	}
	return nil
}

func cmdAuditRulesSeverity(mode runMode, args []string) error {
	cwd, _ := os.Getwd()
	rulesPath := auditRulesPathForMode(mode, cwd)

	var pattern string
	var ids []string
	var severity string
	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "--pattern":
			if i+1 >= len(args) {
				return fmt.Errorf("--pattern requires a value")
			}
			i++
			pattern = args[i]
		default:
			if !strings.HasPrefix(args[i], "-") {
				// Last positional arg is the severity level
				ids = append(ids, args[i])
			}
		}
	}

	// The last positional arg is the severity level
	if pattern != "" {
		if len(ids) < 1 {
			return fmt.Errorf("usage: audit rules severity --pattern <name> <level>")
		}
		severity = ids[len(ids)-1]
		if err := audit.SetPatternSeverity(rulesPath, pattern, severity); err != nil {
			return err
		}
		ui.Success("Set severity for pattern %s → %s", pattern, strings.ToUpper(severity))
		return nil
	}

	if len(ids) < 2 {
		return fmt.Errorf("usage: audit rules severity <id> <level>")
	}
	severity = ids[len(ids)-1]
	ruleIDs := ids[:len(ids)-1]
	if err := audit.SetSeverityBatch(rulesPath, ruleIDs, severity); err != nil {
		return err
	}
	for _, id := range ruleIDs {
		ui.Success("Set severity for %s → %s", id, strings.ToUpper(severity))
	}
	return nil
}

func cmdAuditRulesReset(mode runMode) error {
	cwd, _ := os.Getwd()
	rulesPath := auditRulesPathForMode(mode, cwd)

	if err := audit.ResetRules(rulesPath); err != nil {
		return err
	}
	audit.ResetGlobalCache()
	ui.Success("All custom audit rules have been reset to defaults.")
	return nil
}

func cmdAuditRulesInit(mode runMode) error {
	cwd, _ := os.Getwd()
	path := auditRulesPathForMode(mode, cwd)
	return initAuditRules(path)
}

func auditRulesPathForMode(mode runMode, cwd string) string {
	if mode == modeProject {
		return audit.ProjectAuditRulesPath(cwd)
	}
	return audit.GlobalAuditRulesPath()
}

func printAuditRulesHelp() {
	fmt.Println(`Usage: skillshare audit rules [action] [options]

Browse, enable, disable, and configure audit rules.

Actions:
  (none)               List all rules (default, opens TUI if available)
  disable <id>         Disable a single rule by ID
  disable --pattern <p> Disable all rules in a pattern group
  enable <id>          Re-enable a single rule
  enable --pattern <p> Re-enable all rules in a pattern group
  severity <id> <level>           Override severity for a rule
  severity --pattern <p> <level>  Override severity for a pattern group
  reset                Remove all custom rules (restore built-in defaults)
  init                 Create a starter audit-rules.yaml

Options:
  --pattern <name>     Filter by pattern name
  --severity <level>   Filter by minimum severity (critical/high/medium/low/info)
  --disabled           Only show disabled rules
  --format json        Output as JSON
  --no-tui             Plain text table (no interactive TUI)
  -p, --project        Use project-level rules
  -g, --global         Use global rules
  -h, --help           Show this help

Severity levels: critical (c), high (h), medium (m), low (l), info (i)`)
}
