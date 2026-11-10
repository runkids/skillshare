package main

import (
	"fmt"
	"path"
	"path/filepath"
	"strings"

	"skillshare/internal/audit"
	"skillshare/internal/sourcewalk"
	"skillshare/internal/sync"
	"skillshare/internal/ui"
)

// Use the audit structure analyzer and its normal rule loading in both modes.
func checkSkillReferences(result *doctorResult, skills []sync.DiscoveredSkill, projectRoot string, follow *sourcewalk.Follow) {
	disabled := crossSkillDisabledIDs(projectRoot)
	if disabled["dangling-link"] && disabled["reference-escapes-root"] && disabled["reference-check-incomplete"] {
		if projectRoot != "" {
			if _, err := audit.RulesWithProject(projectRoot); err != nil {
				result.addWarning()
				result.addCheck("bundled_references", checkWarning, "References: could not load rules", []string{err.Error()})
				return
			}
		}
		result.addCheck("bundled_references", checkInfo, "Reference checks disabled", nil)
		return
	}
	registry := audit.DefaultRegistry().ForPolicy(audit.Policy{EnabledAnalyzers: []string{audit.AnalyzerStructure}})
	var projectScans []audit.ScanOutput
	var projectErr error
	if projectRoot != "" {
		paths := make([]string, len(skills))
		for i, skill := range skills {
			paths[i] = skill.SourcePath
		}
		projectScans, projectErr = audit.ScanSkillsFilteredForProject(paths, projectRoot, registry, follow)
	} else {
		inputs := make([]audit.SkillInput, len(skills))
		for i, skill := range skills {
			inputs[i] = audit.SkillInput{Name: skill.RelPath, Path: skill.SourcePath}
		}
		projectScans = audit.ParallelScan(inputs, "", nil, registry, follow)
	}
	if projectErr != nil && len(skills) == 0 {
		result.addWarning()
		result.addCheck("bundled_references", checkWarning, "References: could not load rules", []string{projectErr.Error()})
		return
	}
	checksBefore := len(result.checks)
	// A nested skill is scanned on its own and again inside its parent.
	reported := map[string]bool{}
	for i, skill := range skills {
		var scan *audit.Result
		var err error
		if projectErr != nil {
			err = projectErr
		} else {
			scan, err = projectScans[i].Result, projectScans[i].Err
		}
		if err != nil {
			result.addWarning()
			result.addCheck("bundled_references", checkWarning, skill.RelPath+": references could not be checked", []string{err.Error()})
			ui.Row(ui.MarkWarn, "References", skill.RelPath+": could not check", doctorWidth)
			continue
		}
		for _, f := range scan.Findings {
			if f.Analyzer != audit.AnalyzerStructure {
				continue
			}
			key := fmt.Sprintf("%s:%d:%s:%s", path.Join(filepath.ToSlash(skill.RelPath), filepath.ToSlash(f.File)), f.Line, f.RuleID, f.Message)
			if reported[key] {
				continue
			}
			reported[key] = true
			status, mark := checkWarning, ui.MarkWarn
			if f.Severity == audit.SeverityHigh {
				status, mark = checkError, ui.MarkFail
				result.addError()
			} else {
				result.addWarning()
			}
			message := fmt.Sprintf("%s/%s:%d: %s: %s", skill.RelPath, f.File, f.Line, f.RuleID, f.Message)
			result.addCheck("bundled_references", status, message, []string{strings.TrimSpace(f.Snippet)})
			ui.Row(mark, "References", message, doctorWidth)
		}
	}
	if len(result.checks) == checksBefore {
		result.addCheck("bundled_references", checkPass, "No broken bundled references", nil)
		ui.Row(ui.MarkOK, "References", "No broken bundled references", doctorWidth)
	}
}
