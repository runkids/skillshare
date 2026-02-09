package server

import (
	"net/http"
	"os"
	"path/filepath"
	"time"

	"skillshare/internal/audit"
	"skillshare/internal/sync"
	"skillshare/internal/utils"
)

type auditFindingResponse struct {
	Severity string `json:"severity"`
	Pattern  string `json:"pattern"`
	Message  string `json:"message"`
	File     string `json:"file"`
	Line     int    `json:"line"`
	Snippet  string `json:"snippet"`
}

type auditResultResponse struct {
	SkillName string                 `json:"skillName"`
	Findings  []auditFindingResponse `json:"findings"`
}

type auditSummary struct {
	Total   int `json:"total"`
	Passed  int `json:"passed"`
	Warning int `json:"warning"`
	Failed  int `json:"failed"`
}

func (s *Server) handleAuditAll(w http.ResponseWriter, r *http.Request) {
	start := time.Now()
	source := s.cfg.Source

	// Discover all skills
	discovered, err := sync.DiscoverSourceSkills(source)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	// Deduplicate and also pick up top-level dirs without SKILL.md
	seen := make(map[string]bool)
	type skillEntry struct {
		name string
		path string
	}
	var skills []skillEntry

	for _, d := range discovered {
		if seen[d.SourcePath] {
			continue
		}
		seen[d.SourcePath] = true
		skills = append(skills, skillEntry{d.FlatName, d.SourcePath})
	}

	entries, _ := os.ReadDir(source)
	for _, e := range entries {
		if !e.IsDir() || utils.IsHidden(e.Name()) {
			continue
		}
		p := filepath.Join(source, e.Name())
		if !seen[p] {
			seen[p] = true
			skills = append(skills, skillEntry{e.Name(), p})
		}
	}

	var results []auditResultResponse
	summary := auditSummary{Total: len(skills)}
	criticalCount := 0
	highCount := 0
	mediumCount := 0
	failedSkills := make([]string, 0)
	warningSkills := make([]string, 0)
	scanErrors := 0

	for _, sk := range skills {
		result, err := audit.ScanSkill(sk.path)
		if err != nil {
			scanErrors++
			continue
		}

		resp := toAuditResponse(result)
		results = append(results, resp)

		switch result.MaxSeverity() {
		case audit.SeverityCritical, audit.SeverityHigh:
			summary.Failed++
			failedSkills = append(failedSkills, result.SkillName)
		case audit.SeverityMedium:
			summary.Warning++
			warningSkills = append(warningSkills, result.SkillName)
		default:
			summary.Passed++
		}

		c, h, m := result.CountBySeverity()
		criticalCount += c
		highCount += h
		mediumCount += m
	}

	status := "ok"
	msg := ""
	if criticalCount > 0 {
		status = "blocked"
		msg = "critical findings detected"
	}
	args := map[string]any{
		"scope":       "all",
		"mode":        "ui",
		"scanned":     summary.Total,
		"passed":      summary.Passed,
		"warning":     summary.Warning,
		"failed":      summary.Failed,
		"critical":    criticalCount,
		"high":        highCount,
		"medium":      mediumCount,
		"scan_errors": scanErrors,
	}
	if len(failedSkills) > 0 {
		args["failed_skills"] = failedSkills
	}
	if len(warningSkills) > 0 {
		args["warning_skills"] = warningSkills
	}
	s.writeAuditLog(status, start, args, msg)

	writeJSON(w, map[string]any{
		"results": results,
		"summary": summary,
	})
}

func (s *Server) handleAuditSkill(w http.ResponseWriter, r *http.Request) {
	start := time.Now()
	name := r.PathValue("name")
	skillPath := filepath.Join(s.cfg.Source, name)

	if _, err := os.Stat(skillPath); os.IsNotExist(err) {
		writeError(w, http.StatusNotFound, "skill not found: "+name)
		return
	}

	result, err := audit.ScanSkill(skillPath)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	c, h, m := result.CountBySeverity()
	warningCount := 0
	failedCount := 0
	failedSkills := []string{}
	warningSkills := []string{}
	switch result.MaxSeverity() {
	case audit.SeverityCritical, audit.SeverityHigh:
		failedCount = 1
		failedSkills = append(failedSkills, result.SkillName)
	case audit.SeverityMedium:
		warningCount = 1
		warningSkills = append(warningSkills, result.SkillName)
	}

	status := "ok"
	msg := ""
	if result.HasCritical() {
		status = "blocked"
		msg = "critical findings detected"
	}
	args := map[string]any{
		"scope":    "single",
		"name":     name,
		"mode":     "ui",
		"scanned":  1,
		"passed":   boolToInt(len(result.Findings) == 0),
		"warning":  warningCount,
		"failed":   failedCount,
		"critical": c,
		"high":     h,
		"medium":   m,
	}
	if len(failedSkills) > 0 {
		args["failed_skills"] = failedSkills
	}
	if len(warningSkills) > 0 {
		args["warning_skills"] = warningSkills
	}
	s.writeAuditLog(status, start, args, msg)

	writeJSON(w, toAuditResponse(result))
}

func boolToInt(v bool) int {
	if v {
		return 1
	}
	return 0
}

func toAuditResponse(result *audit.Result) auditResultResponse {
	findings := make([]auditFindingResponse, 0, len(result.Findings))
	for _, f := range result.Findings {
		findings = append(findings, auditFindingResponse{
			Severity: f.Severity,
			Pattern:  f.Pattern,
			Message:  f.Message,
			File:     f.File,
			Line:     f.Line,
			Snippet:  f.Snippet,
		})
	}
	return auditResultResponse{
		SkillName: result.SkillName,
		Findings:  findings,
	}
}
