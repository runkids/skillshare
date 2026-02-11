package server

import (
	"encoding/json"
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
	SkillName  string                 `json:"skillName"`
	Findings   []auditFindingResponse `json:"findings"`
	RiskScore  int                    `json:"riskScore"`
	RiskLabel  string                 `json:"riskLabel"`
	Threshold  string                 `json:"threshold"`
	IsBlocked  bool                   `json:"isBlocked"`
	ScanTarget string                 `json:"scanTarget,omitempty"`
}

type auditSummary struct {
	Total      int    `json:"total"`
	Passed     int    `json:"passed"`
	Warning    int    `json:"warning"`
	Failed     int    `json:"failed"`
	Critical   int    `json:"critical"`
	High       int    `json:"high"`
	Medium     int    `json:"medium"`
	Low        int    `json:"low"`
	Info       int    `json:"info"`
	Threshold  string `json:"threshold"`
	RiskScore  int    `json:"riskScore"`
	RiskLabel  string `json:"riskLabel"`
	ScanErrors int    `json:"scanErrors,omitempty"`
}

func (s *Server) handleAuditAll(w http.ResponseWriter, r *http.Request) {
	start := time.Now()
	source := s.cfg.Source
	threshold := s.auditThreshold()

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
	summary := auditSummary{
		Total:     len(skills),
		Threshold: threshold,
	}
	criticalCount := 0
	highCount := 0
	mediumCount := 0
	lowCount := 0
	infoCount := 0
	failedSkills := make([]string, 0)
	warningSkills := make([]string, 0)
	lowSkills := make([]string, 0)
	infoSkills := make([]string, 0)
	scanErrors := 0
	maxRisk := 0

	for _, sk := range skills {
		var result *audit.Result
		if s.IsProjectMode() {
			result, err = audit.ScanSkillForProject(sk.path, s.projectRoot)
		} else {
			result, err = audit.ScanSkill(sk.path)
		}
		if err != nil {
			scanErrors++
			continue
		}

		result.Threshold = threshold
		result.IsBlocked = result.HasSeverityAtOrAbove(threshold)

		resp := toAuditResponse(result)
		results = append(results, resp)

		if len(result.Findings) == 0 {
			summary.Passed++
		} else if result.IsBlocked {
			summary.Failed++
			failedSkills = append(failedSkills, result.SkillName)
		} else {
			summary.Warning++
			warningSkills = append(warningSkills, result.SkillName)
		}

		c, h, m, l, i := result.CountBySeverityAll()
		criticalCount += c
		highCount += h
		mediumCount += m
		lowCount += l
		infoCount += i
		if l > 0 {
			lowSkills = append(lowSkills, result.SkillName)
		}
		if i > 0 {
			infoSkills = append(infoSkills, result.SkillName)
		}
		if result.RiskScore > maxRisk {
			maxRisk = result.RiskScore
		}
	}

	status := "ok"
	msg := ""
	if summary.Failed > 0 {
		status = "blocked"
		msg = "findings at/above threshold detected"
	}
	summary.Critical = criticalCount
	summary.High = highCount
	summary.Medium = mediumCount
	summary.Low = lowCount
	summary.Info = infoCount
	summary.ScanErrors = scanErrors
	summary.RiskScore = maxRisk
	summary.RiskLabel = audit.RiskLabelFromScore(maxRisk)

	args := map[string]any{
		"scope":       "all",
		"mode":        "ui",
		"threshold":   threshold,
		"scanned":     summary.Total,
		"passed":      summary.Passed,
		"warning":     summary.Warning,
		"failed":      summary.Failed,
		"critical":    criticalCount,
		"high":        highCount,
		"medium":      mediumCount,
		"low":         lowCount,
		"info":        infoCount,
		"risk_score":  summary.RiskScore,
		"risk_label":  summary.RiskLabel,
		"scan_errors": scanErrors,
	}
	if len(failedSkills) > 0 {
		args["failed_skills"] = failedSkills
	}
	if len(warningSkills) > 0 {
		args["warning_skills"] = warningSkills
	}
	if len(lowSkills) > 0 {
		args["low_skills"] = lowSkills
	}
	if len(infoSkills) > 0 {
		args["info_skills"] = infoSkills
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
	threshold := s.auditThreshold()
	skillPath := filepath.Join(s.cfg.Source, name)

	if _, err := os.Stat(skillPath); os.IsNotExist(err) {
		writeError(w, http.StatusNotFound, "skill not found: "+name)
		return
	}

	var (
		result *audit.Result
		err    error
	)
	if s.IsProjectMode() {
		result, err = audit.ScanSkillForProject(skillPath, s.projectRoot)
	} else {
		result, err = audit.ScanSkill(skillPath)
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	result.Threshold = threshold
	result.IsBlocked = result.HasSeverityAtOrAbove(threshold)

	c, h, m, l, i := result.CountBySeverityAll()
	warningCount := 0
	failedCount := 0
	failedSkills := []string{}
	warningSkills := []string{}
	lowSkills := []string{}
	infoSkills := []string{}
	if len(result.Findings) == 0 {
		// no-op
	} else if result.IsBlocked {
		failedCount = 1
		failedSkills = append(failedSkills, result.SkillName)
	} else {
		warningCount = 1
		warningSkills = append(warningSkills, result.SkillName)
	}
	if l > 0 {
		lowSkills = append(lowSkills, result.SkillName)
	}
	if i > 0 {
		infoSkills = append(infoSkills, result.SkillName)
	}

	status := "ok"
	msg := ""
	if result.IsBlocked {
		status = "blocked"
		msg = "findings at/above threshold detected"
	}
	args := map[string]any{
		"scope":      "single",
		"name":       name,
		"mode":       "ui",
		"threshold":  threshold,
		"scanned":    1,
		"passed":     boolToInt(len(result.Findings) == 0),
		"warning":    warningCount,
		"failed":     failedCount,
		"critical":   c,
		"high":       h,
		"medium":     m,
		"low":        l,
		"info":       i,
		"risk_score": result.RiskScore,
		"risk_label": result.RiskLabel,
	}
	if len(failedSkills) > 0 {
		args["failed_skills"] = failedSkills
	}
	if len(warningSkills) > 0 {
		args["warning_skills"] = warningSkills
	}
	if len(lowSkills) > 0 {
		args["low_skills"] = lowSkills
	}
	if len(infoSkills) > 0 {
		args["info_skills"] = infoSkills
	}
	s.writeAuditLog(status, start, args, msg)

	writeJSON(w, map[string]any{
		"result": toAuditResponse(result),
		"summary": auditSummary{
			Total:     1,
			Passed:    boolToInt(len(result.Findings) == 0),
			Warning:   warningCount,
			Failed:    failedCount,
			Critical:  c,
			High:      h,
			Medium:    m,
			Low:       l,
			Info:      i,
			Threshold: threshold,
			RiskScore: result.RiskScore,
			RiskLabel: result.RiskLabel,
		},
	})
}

func boolToInt(v bool) int {
	if v {
		return 1
	}
	return 0
}

func (s *Server) auditThreshold() string {
	if s.IsProjectMode() && s.projectCfg != nil {
		if threshold, err := audit.NormalizeThreshold(s.projectCfg.Audit.BlockThreshold); err == nil {
			return threshold
		}
	}
	if threshold, err := audit.NormalizeThreshold(s.cfg.Audit.BlockThreshold); err == nil {
		return threshold
	}
	return audit.DefaultThreshold()
}

// auditRulesPath returns the correct audit-rules.yaml path for the current mode.
func (s *Server) auditRulesPath() string {
	if s.IsProjectMode() {
		return audit.ProjectAuditRulesPath(s.projectRoot)
	}
	return audit.GlobalAuditRulesPath()
}

func (s *Server) handleGetAuditRules(w http.ResponseWriter, r *http.Request) {
	path := s.auditRulesPath()
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			writeJSON(w, map[string]any{
				"exists": false,
				"raw":    "",
				"path":   path,
			})
			return
		}
		writeError(w, http.StatusInternalServerError, "failed to read rules: "+err.Error())
		return
	}

	writeJSON(w, map[string]any{
		"exists": true,
		"raw":    string(data),
		"path":   path,
	})
}

func (s *Server) handlePutAuditRules(w http.ResponseWriter, r *http.Request) {
	s.mu.Lock()
	defer s.mu.Unlock()

	var body struct {
		Raw string `json:"raw"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON body")
		return
	}

	if err := audit.ValidateRulesYAML(body.Raw); err != nil {
		writeError(w, http.StatusBadRequest, "invalid rules: "+err.Error())
		return
	}

	path := s.auditRulesPath()
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		writeError(w, http.StatusInternalServerError, "create directory: "+err.Error())
		return
	}
	if err := os.WriteFile(path, []byte(body.Raw), 0644); err != nil {
		writeError(w, http.StatusInternalServerError, "failed to write rules: "+err.Error())
		return
	}

	writeJSON(w, map[string]any{"success": true})
}

func (s *Server) handleInitAuditRules(w http.ResponseWriter, r *http.Request) {
	s.mu.Lock()
	defer s.mu.Unlock()

	path := s.auditRulesPath()
	if err := audit.InitRulesFile(path); err != nil {
		if os.IsNotExist(err) {
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}
		// File already exists → 409 Conflict
		if _, statErr := os.Stat(path); statErr == nil {
			writeError(w, http.StatusConflict, "rules file already exists: "+path)
			return
		}
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	writeJSON(w, map[string]any{
		"success": true,
		"path":    path,
	})
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
		SkillName:  result.SkillName,
		Findings:   findings,
		RiskScore:  result.RiskScore,
		RiskLabel:  result.RiskLabel,
		Threshold:  result.Threshold,
		IsBlocked:  result.IsBlocked,
		ScanTarget: result.ScanTarget,
	}
}
