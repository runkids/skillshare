package server

import (
	"encoding/json"
	"fmt"
	"net/http"
	"path/filepath"

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
	s.mu.Lock()
	defer s.mu.Unlock()

	var body struct {
		Source string `json:"source"`
		Skills []struct {
			Name string `json:"name"`
			Path string `json:"path"`
		} `json:"skills"`
		Force bool `json:"force"`
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

	results := make([]batchResultItem, 0, len(body.Skills))
	for _, sel := range body.Skills {
		destPath := filepath.Join(s.cfg.Source, sel.Name)
		res, err := install.InstallFromDiscovery(discovery, install.SkillInfo{
			Name: sel.Name,
			Path: sel.Path,
		}, destPath, install.InstallOptions{Force: body.Force})
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
	var firstErr string
	for _, r := range results {
		if r.Error == "" {
			installed++
		} else if firstErr == "" {
			firstErr = r.Error
		}
	}
	summary := fmt.Sprintf("Installed %d of %d skills", installed, len(body.Skills))
	if firstErr != "" {
		summary += " (some errors)"
	}

	writeJSON(w, map[string]any{
		"results": results,
		"summary": summary,
	})
}

func (s *Server) handleInstall(w http.ResponseWriter, r *http.Request) {
	s.mu.Lock()
	defer s.mu.Unlock()

	var body struct {
		Source string `json:"source"`
		Name   string `json:"name"`
		Force  bool   `json:"force"`
		Track  bool   `json:"track"`
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
		result, err := install.InstallTrackedRepo(source, s.cfg.Source, install.InstallOptions{
			Name:  body.Name,
			Force: body.Force,
		})
		if err != nil {
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}
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
	destPath := filepath.Join(s.cfg.Source, source.Name)

	result, err := install.Install(source, destPath, install.InstallOptions{
		Name:  body.Name,
		Force: body.Force,
	})
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	writeJSON(w, map[string]any{
		"skillName": result.SkillName,
		"action":    result.Action,
		"warnings":  result.Warnings,
	})
}
