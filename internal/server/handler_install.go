package server

import (
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"time"

	"skillshare/internal/config"
	"skillshare/internal/install"
)

// reloadSkillsStore refreshes the in-memory skills metadata store from disk.
// install.Install writes metadata to the file via its own store instance, so the
// server's cached s.skillsStore stays stale after an install — a freshly
// installed skill would then render with an empty source/type (shown as "Local"
// with a blank source in the UI) until the server restarts.
func (s *Server) reloadSkillsStore() {
	if st, err := install.LoadMetadataWithMigration(s.cfg.EffectiveSkillsSource(), ""); err == nil && st != nil {
		s.skillsStore = st
	}
}

func discoverInstallSource(source *install.Source) (*install.DiscoveryResult, error) {
	if source.IsGit() {
		if source.HasSubdir() {
			return install.DiscoverFromGitSubdir(source)
		}
		return install.DiscoverFromGit(source)
	}
	return install.DiscoverLocal(source)
}

// handleDiscover clones a git repo to a temp dir, discovers skills, then cleans up.
// Returns whether the caller needs to present a selection UI.
func (s *Server) handleDiscover(w http.ResponseWriter, r *http.Request) {
	// Snapshot config under RLock, then release before I/O.
	s.mu.RLock()
	parseOpts := s.parseOpts()
	s.mu.RUnlock()

	var body struct {
		Source string `json:"source"`
		Branch string `json:"branch"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON body")
		return
	}
	if body.Source == "" {
		writeError(w, http.StatusBadRequest, "source is required")
		return
	}

	source, err := install.ParseSourceWithOptions(body.Source, parseOpts)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid source: "+err.Error())
		return
	}
	source.Branch = body.Branch

	discovery, err := discoverInstallSource(source)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	defer install.CleanupDiscovery(discovery)

	skills := make([]map[string]string, len(discovery.Skills))
	for i, sk := range discovery.Skills {
		skills[i] = map[string]string{"name": sk.Name, "path": sk.Path, "description": sk.Description, "kind": "skill"}
	}

	agents := make([]map[string]string, len(discovery.Agents))
	for i, ag := range discovery.Agents {
		agents[i] = map[string]string{"name": ag.Name, "path": ag.Path, "fileName": ag.FileName, "kind": "agent"}
	}

	writeJSON(w, map[string]any{
		"needsSelection": len(discovery.Skills) > 1 || len(discovery.Agents) > 0,
		"skills":         skills,
		"agents":         agents,
	})
}

// handleInstallBatch re-clones a repo and installs each selected skill.
func (s *Server) handleInstallBatch(w http.ResponseWriter, r *http.Request) {
	start := time.Now()
	s.mu.Lock()
	defer s.mu.Unlock()

	var body struct {
		Source string `json:"source"`
		Branch string `json:"branch"`
		Skills []struct {
			Name string `json:"name"`
			Path string `json:"path"`
		} `json:"skills"`
		Force     bool   `json:"force"`
		SkipAudit bool   `json:"skipAudit"`
		Into      string `json:"into"`
		Name      string `json:"name"`
		Kind      string `json:"kind,omitempty"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON body")
		return
	}
	if body.Source == "" || len(body.Skills) == 0 {
		writeError(w, http.StatusBadRequest, "source and skills are required")
		return
	}

	source, err := install.ParseSourceWithOptions(body.Source, s.parseOpts())
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid source: "+err.Error())
		return
	}
	source.Branch = body.Branch

	if s.IsProjectMode() {
		if err := install.RejectProjectRootLocalInstall(source, s.projectRoot); err != nil {
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}
	}

	discovery, err := discoverInstallSource(source)
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
		baseDir := s.cfg.EffectiveSkillsSource()
		if body.Kind == "agent" {
			baseDir = s.agentsSource()
		}
		if err := os.MkdirAll(filepath.Join(baseDir, body.Into), 0755); err != nil {
			writeError(w, http.StatusInternalServerError, "failed to create into directory: "+err.Error())
			return
		}
	}

	// Note: no cross-path duplicate check for batch — user explicitly selected
	// specific skills from discovery, so installing to a different path is intentional.

	results := make([]batchResultItem, 0, len(body.Skills))
	installOpts := install.InstallOptions{
		Force:          body.Force,
		SkipAudit:      body.SkipAudit,
		AuditThreshold: s.auditThreshold(),
		Branch:         body.Branch,
		SourceDir:      s.cfg.EffectiveSkillsSource(),
	}
	if s.IsProjectMode() {
		installOpts.AuditProjectRoot = s.projectRoot
	}
	isAgent := body.Kind == "agent"
	if isAgent {
		installOpts.SourceDir = s.agentsSource()
	}

	for _, sel := range body.Skills {
		skillName := sel.Name
		if body.Name != "" && len(body.Skills) == 1 {
			skillName = body.Name
		}

		if isAgent {
			// Agent install: copy single .md file to agents source
			agentsDir := s.agentsSource()
			if body.Into != "" {
				agentsDir = filepath.Join(agentsDir, body.Into)
			}
			agentInfo := install.AgentInfo{
				Name:     sel.Name,
				Path:     sel.Path,
				FileName: filepath.Base(sel.Path),
			}
			res, err := install.InstallAgentFromDiscovery(discovery, agentInfo, agentsDir, installOpts)
			if err != nil {
				results = append(results, batchResultItem{
					Name:  skillName,
					Error: err.Error(),
				})
				continue
			}
			results = append(results, batchResultItem{
				Name:     skillName,
				Action:   res.Action,
				Warnings: res.Warnings,
			})
		} else {
			// Skill install: copy directory to skills source
			destPath := filepath.Join(s.cfg.EffectiveSkillsSource(), body.Into, skillName)
			res, err := install.InstallFromDiscovery(discovery, install.SkillInfo{
				Name: sel.Name,
				Path: sel.Path,
			}, destPath, installOpts)
			if err != nil {
				results = append(results, batchResultItem{
					Name:  skillName,
					Error: err.Error(),
				})
				continue
			}
			results = append(results, batchResultItem{
				Name:     skillName,
				Action:   res.Action,
				Warnings: res.Warnings,
			})
		}
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
	kindLabel := "skills"
	if isAgent {
		kindLabel = "agents"
	}
	summary := fmt.Sprintf("Installed %d of %d %s", installed, len(body.Skills), kindLabel)
	if firstErr != "" {
		summary += " (some errors)"
	}
	if isAgent && installed > 0 && !s.cfg.HasAgentTarget() {
		summary += ". Warning: none of your configured targets support agents"
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

	// Reconcile config after install
	if installed > 0 {
		if isAgent {
			if st, loadErr := install.LoadMetadataWithMigration(s.agentsSource(), install.MetadataKindAgent); loadErr == nil && st != nil {
				s.agentsStore = st
			}
		} else {
			s.reloadSkillsStore()
		}
		if s.IsProjectMode() {
			if rErr := config.ReconcileProjectSkills(s.projectRoot, s.projectCfg, s.skillsStore, s.cfg.EffectiveSkillsSource()); rErr != nil {
				log.Printf("warning: failed to reconcile project skills config: %v", rErr)
			}
		} else {
			if rErr := config.ReconcileGlobalSkills(s.cfg, s.skillsStore); rErr != nil {
				log.Printf("warning: failed to reconcile global skills config: %v", rErr)
			}
		}
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
		Branch    string `json:"branch"`
		Name      string `json:"name"`
		Force     bool   `json:"force"`
		SkipAudit bool   `json:"skipAudit"`
		Track     bool   `json:"track"`
		Into      string `json:"into"`
		Kind      string `json:"kind,omitempty"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON body")
		return
	}

	if body.Source == "" {
		writeError(w, http.StatusBadRequest, "source is required")
		return
	}

	source, err := install.ParseSourceWithOptions(body.Source, s.parseOpts())
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid source: "+err.Error())
		return
	}
	source.Branch = body.Branch

	if s.IsProjectMode() {
		if err := install.RejectProjectRootLocalInstall(source, s.projectRoot); err != nil {
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}
	}

	if body.Name != "" {
		source.Name = body.Name
	}

	// Tracked repo install
	if body.Track {
		trackedKind, err := install.InferTrackedKind(source, body.Kind)
		if err != nil {
			var ambig *install.TrackKindAmbiguousError
			if errors.As(err, &ambig) {
				writeCodedError(w, http.StatusBadRequest,
					"install.track_kind_ambiguous", err.Error(),
					map[string]string{
						"skills": strconv.Itoa(ambig.Skills),
						"agents": strconv.Itoa(ambig.Agents),
					})
				return
			}
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}

		trackSourceDir := s.cfg.EffectiveSkillsSource()
		if trackedKind == "agent" {
			trackSourceDir = s.agentsSource()
		}

		installOpts := install.InstallOptions{
			Name:           body.Name,
			Kind:           trackedKind,
			Force:          body.Force,
			SkipAudit:      body.SkipAudit,
			Into:           body.Into,
			Branch:         body.Branch,
			AuditThreshold: s.auditThreshold(),
			SourceDir:      trackSourceDir,
		}
		if s.IsProjectMode() {
			installOpts.AuditProjectRoot = s.projectRoot
		}
		result, err := install.InstallTrackedRepo(source, trackSourceDir, install.InstallOptions{
			Name:             installOpts.Name,
			Kind:             installOpts.Kind,
			Force:            installOpts.Force,
			SkipAudit:        installOpts.SkipAudit,
			Into:             installOpts.Into,
			Branch:           installOpts.Branch,
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
		// Reconcile config after tracked repo install
		if trackedKind == "skill" {
			s.reloadSkillsStore()
			if s.IsProjectMode() {
				if rErr := config.ReconcileProjectSkills(s.projectRoot, s.projectCfg, s.skillsStore, s.cfg.EffectiveSkillsSource()); rErr != nil {
					log.Printf("warning: failed to reconcile project skills config: %v", rErr)
				}
			} else {
				if rErr := config.ReconcileGlobalSkills(s.cfg, s.skillsStore); rErr != nil {
					log.Printf("warning: failed to reconcile global skills config: %v", rErr)
				}
			}
		}

		args := map[string]any{
			"source":      body.Source,
			"mode":        s.installLogMode(),
			"tracked":     true,
			"kind":        trackedKind,
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
			"agentCount": result.AgentCount,
			"skills":     result.Skills,
			"agents":     result.Agents,
			"action":     result.Action,
			"warnings":   result.Warnings,
		})
		return
	}

	// Cross-path duplicate detection
	if !body.Force && source.CloneURL != "" {
		if err := install.CheckCrossPathDuplicate(s.cfg.EffectiveSkillsSource(), source.CloneURL, body.Into); err != nil {
			writeError(w, http.StatusConflict, err.Error())
			return
		}
	}

	// Regular install
	destPath := filepath.Join(s.cfg.EffectiveSkillsSource(), body.Into, source.Name)
	if body.Into != "" {
		if err := os.MkdirAll(filepath.Join(s.cfg.EffectiveSkillsSource(), body.Into), 0755); err != nil {
			writeError(w, http.StatusInternalServerError, "failed to create into directory: "+err.Error())
			return
		}
	}

	result, err := install.Install(source, destPath, install.InstallOptions{
		Name:           body.Name,
		Force:          body.Force,
		SkipAudit:      body.SkipAudit,
		Branch:         body.Branch,
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

	// Reconcile config after single install
	s.reloadSkillsStore()
	if s.IsProjectMode() {
		if rErr := config.ReconcileProjectSkills(s.projectRoot, s.projectCfg, s.skillsStore, s.cfg.EffectiveSkillsSource()); rErr != nil {
			log.Printf("warning: failed to reconcile project skills config: %v", rErr)
		}
	} else {
		if rErr := config.ReconcileGlobalSkills(s.cfg, s.skillsStore); rErr != nil {
			log.Printf("warning: failed to reconcile global skills config: %v", rErr)
		}
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
