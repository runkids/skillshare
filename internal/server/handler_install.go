package server

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"time"

	"skillshare/internal/config"
	"skillshare/internal/install"
)

// handleDiscover clones a git repo to a temp dir, discovers skills, then cleans up.
// Returns whether the caller needs to present a selection UI.
func (s *Server) handleDiscover(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Source string `json:"source"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON body")
		return
	}
	if body.Source == "" {
		writeError(w, http.StatusBadRequest, "source is required")
		return
	}

	source, err := install.ParseSource(body.Source)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid source: "+err.Error())
		return
	}

	// Only git sources without a subdir can contain multiple skills
	if !source.IsGit() || source.HasSubdir() {
		writeJSON(w, map[string]any{
			"needsSelection": false,
			"skills":         []any{},
		})
		return
	}

	discovery, err := install.DiscoverFromGit(source)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	defer install.CleanupDiscovery(discovery)

	skills := make([]map[string]string, len(discovery.Skills))
	for i, sk := range discovery.Skills {
		skills[i] = map[string]string{"name": sk.Name, "path": sk.Path}
	}

	writeJSON(w, map[string]any{
		"needsSelection": len(discovery.Skills) > 1,
		"skills":         skills,
	})
}

// handleInstallBatch re-clones a repo and installs each selected skill.
func (s *Server) handleInstallBatch(w http.ResponseWriter, r *http.Request) {
	start := time.Now()
	s.mu.Lock()
	defer s.mu.Unlock()

	var body struct {
		Source string `json:"source"`
		Skills []struct {
			Name string `json:"name"`
			Path string `json:"path"`
		} `json:"skills"`
		Force     bool   `json:"force"`
		SkipAudit bool   `json:"skipAudit"`
		Into      string `json:"into"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON body")
		return
	}
	if body.Source == "" || len(body.Skills) == 0 {
		writeError(w, http.StatusBadRequest, "source and skills are required")
		return
	}

	source, err := install.ParseSource(body.Source)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid source: "+err.Error())
		return
	}

	discovery, err := install.DiscoverFromGit(source)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "discovery failed: "+err.Error())
		return
	}
	defer install.CleanupDiscovery(discovery)

	type batchResultItem struct {
		Name     string   `json:"name"`
		Action   string   `json:"action,omitempty"`
		Warnings []string `json:"warnings,omitempty"`
		Error    string   `json:"error,omitempty"`
	}

	// Ensure Into directory exists
	if body.Into != "" {
		if err := os.MkdirAll(filepath.Join(s.cfg.Source, body.Into), 0755); err != nil {
			writeError(w, http.StatusInternalServerError, "failed to create into directory: "+err.Error())
			return
		}
	}

	results := make([]batchResultItem, 0, len(body.Skills))
	installOpts := install.InstallOptions{
		Force:          body.Force,
		SkipAudit:      body.SkipAudit,
		AuditThreshold: s.auditThreshold(),
	}
	if s.IsProjectMode() {
		installOpts.AuditProjectRoot = s.projectRoot
	}
	for _, sel := range body.Skills {
		destPath := filepath.Join(s.cfg.Source, body.Into, sel.Name)
		res, err := install.InstallFromDiscovery(discovery, install.SkillInfo{
			Name: sel.Name,
			Path: sel.Path,
		}, destPath, installOpts)
		if err != nil {
			results = append(results, batchResultItem{
				Name:  sel.Name,
				Error: err.Error(),
			})
			continue
		}
		results = append(results, batchResultItem{
			Name:     sel.Name,
			Action:   res.Action,
			Warnings: res.Warnings,
		})
	}

	// Summary for toast
	installed := 0
	installedSkills := make([]string, 0, len(results))
	failedSkills := make([]string, 0, len(results))
	var firstErr string
	for _, r := range results {
		if r.Error == "" {
			installed++
			installedSkills = append(installedSkills, r.Name)
		} else if firstErr == "" {
			firstErr = r.Error
			failedSkills = append(failedSkills, r.Name)
		} else {
			failedSkills = append(failedSkills, r.Name)
		}
	}
	summary := fmt.Sprintf("Installed %d of %d skills", installed, len(body.Skills))
	if firstErr != "" {
		summary += " (some errors)"
	}

	status := "ok"
	if installed < len(body.Skills) {
		status = "partial"
	}
	args := map[string]any{
		"source":      body.Source,
		"mode":        s.installLogMode(),
		"force":       body.Force,
		"scope":       "ui",
		"threshold":   s.auditThreshold(),
		"skill_count": installed,
	}
	if body.SkipAudit {
		args["skip_audit"] = true
	}
	if body.Into != "" {
		args["into"] = body.Into
	}
	if len(installedSkills) > 0 {
		args["installed_skills"] = installedSkills
	}
	if len(failedSkills) > 0 {
		args["failed_skills"] = failedSkills
	}
	s.writeOpsLog("install", status, start, args, firstErr)

	// Reconcile project config after install
	if s.IsProjectMode() && installed > 0 {
		_ = config.ReconcileProjectSkills(s.projectRoot, s.projectCfg, s.cfg.Source)
	}

	writeJSON(w, map[string]any{
		"results": results,
		"summary": summary,
	})
}

func (s *Server) handleInstall(w http.ResponseWriter, r *http.Request) {
	start := time.Now()
	s.mu.Lock()
	defer s.mu.Unlock()

	var body struct {
		Source    string `json:"source"`
		Name      string `json:"name"`
		Force     bool   `json:"force"`
		SkipAudit bool   `json:"skipAudit"`
		Track     bool   `json:"track"`
		Into      string `json:"into"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON body")
		return
	}

	if body.Source == "" {
		writeError(w, http.StatusBadRequest, "source is required")
		return
	}

	source, err := install.ParseSource(body.Source)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid source: "+err.Error())
		return
	}

	if body.Name != "" {
		source.Name = body.Name
	}

	// Tracked repo install
	if body.Track {
		installOpts := install.InstallOptions{
			Name:           body.Name,
			Force:          body.Force,
			SkipAudit:      body.SkipAudit,
			Into:           body.Into,
			AuditThreshold: s.auditThreshold(),
		}
		if s.IsProjectMode() {
			installOpts.AuditProjectRoot = s.projectRoot
		}
		result, err := install.InstallTrackedRepo(source, s.cfg.Source, install.InstallOptions{
			Name:             installOpts.Name,
			Force:            installOpts.Force,
			SkipAudit:        installOpts.SkipAudit,
			Into:             installOpts.Into,
			AuditThreshold:   installOpts.AuditThreshold,
			AuditProjectRoot: installOpts.AuditProjectRoot,
		})
		if err != nil {
			s.writeOpsLog("install", "error", start, map[string]any{
				"source":        body.Source,
				"mode":          s.installLogMode(),
				"tracked":       true,
				"force":         body.Force,
				"threshold":     s.auditThreshold(),
				"scope":         "ui",
				"failed_skills": []string{source.Name},
			}, err.Error())
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}
		// Reconcile project config after tracked repo install
		if s.IsProjectMode() {
			_ = config.ReconcileProjectSkills(s.projectRoot, s.projectCfg, s.cfg.Source)
		}

		args := map[string]any{
			"source":      body.Source,
			"mode":        s.installLogMode(),
			"tracked":     true,
			"force":       body.Force,
			"threshold":   s.auditThreshold(),
			"scope":       "ui",
			"skill_count": result.SkillCount,
		}
		if body.SkipAudit {
			args["skip_audit"] = true
		}
		if body.Into != "" {
			args["into"] = body.Into
		}
		if len(result.Skills) > 0 {
			args["installed_skills"] = result.Skills
		}
		s.writeOpsLog("install", "ok", start, args, "")

		writeJSON(w, map[string]any{
			"repoName":   result.RepoName,
			"skillCount": result.SkillCount,
			"skills":     result.Skills,
			"action":     result.Action,
			"warnings":   result.Warnings,
		})
		return
	}

	// Regular install
	destPath := filepath.Join(s.cfg.Source, body.Into, source.Name)
	if body.Into != "" {
		if err := os.MkdirAll(filepath.Join(s.cfg.Source, body.Into), 0755); err != nil {
			writeError(w, http.StatusInternalServerError, "failed to create into directory: "+err.Error())
			return
		}
	}

	result, err := install.Install(source, destPath, install.InstallOptions{
		Name:           body.Name,
		Force:          body.Force,
		SkipAudit:      body.SkipAudit,
		AuditThreshold: s.auditThreshold(),
		AuditProjectRoot: func() string {
			if s.IsProjectMode() {
				return s.projectRoot
			}
			return ""
		}(),
	})
	if err != nil {
		s.writeOpsLog("install", "error", start, map[string]any{
			"source":        body.Source,
			"mode":          s.installLogMode(),
			"force":         body.Force,
			"threshold":     s.auditThreshold(),
			"scope":         "ui",
			"failed_skills": []string{source.Name},
		}, err.Error())
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	// Reconcile project config after single install
	if s.IsProjectMode() {
		_ = config.ReconcileProjectSkills(s.projectRoot, s.projectCfg, s.cfg.Source)
	}

	okArgs := map[string]any{
		"source":           body.Source,
		"mode":             s.installLogMode(),
		"force":            body.Force,
		"threshold":        s.auditThreshold(),
		"scope":            "ui",
		"skill_count":      1,
		"installed_skills": []string{result.SkillName},
	}
	if body.SkipAudit {
		okArgs["skip_audit"] = true
	}
	if body.Into != "" {
		okArgs["into"] = body.Into
	}
	s.writeOpsLog("install", "ok", start, okArgs, "")

	writeJSON(w, map[string]any{
		"skillName": result.SkillName,
		"action":    result.Action,
		"warnings":  result.Warnings,
	})
}

func (s *Server) installLogMode() string {
	if s.IsProjectMode() {
		return "project"
	}
	return "global"
}
