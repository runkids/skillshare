package server

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"time"

	"skillshare/internal/skill"
)

func (s *Server) handleGetTemplates(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, map[string]any{
		"patterns":   skill.Patterns,
		"categories": skill.Categories,
	})
}

type createSkillRequest struct {
	Name         string   `json:"name"`
	Pattern      string   `json:"pattern"`
	Category     string   `json:"category"`
	ScaffoldDirs []string `json:"scaffoldDirs"`
}

func (s *Server) handleCreateSkill(w http.ResponseWriter, r *http.Request) {
	start := time.Now()

	var req createSkillRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	// Validate name
	if !skill.ValidNameRe.MatchString(req.Name) {
		writeError(w, http.StatusBadRequest, "invalid skill name: use lowercase letters, numbers, hyphens, underscores; must start with letter or underscore")
		return
	}

	// Validate pattern
	pattern := skill.FindPattern(req.Pattern)
	if pattern == nil {
		writeError(w, http.StatusBadRequest, fmt.Sprintf("unknown pattern: %s", req.Pattern))
		return
	}

	// Validate scaffoldDirs against pattern's allowed dirs
	if len(req.ScaffoldDirs) > 0 && len(pattern.ScaffoldDirs) > 0 {
		allowed := make(map[string]bool, len(pattern.ScaffoldDirs))
		for _, d := range pattern.ScaffoldDirs {
			allowed[d] = true
		}
		for _, d := range req.ScaffoldDirs {
			if !allowed[d] {
				writeError(w, http.StatusBadRequest, fmt.Sprintf("scaffold dir %q not valid for pattern %q", d, req.Pattern))
				return
			}
		}
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	source := s.cfg.Source
	skillDir := filepath.Join(source, req.Name)

	// Check if skill already exists
	if _, err := os.Stat(skillDir); err == nil {
		writeError(w, http.StatusConflict, fmt.Sprintf("skill '%s' already exists", req.Name))
		return
	}

	// Generate SKILL.md content
	content := skill.GenerateContent(req.Name, req.Pattern, req.Category)

	// Create directory
	if err := os.MkdirAll(skillDir, 0755); err != nil {
		writeError(w, http.StatusInternalServerError, "failed to create directory: "+err.Error())
		return
	}

	createdFiles := []string{"SKILL.md"}

	// Write SKILL.md
	skillFile := filepath.Join(skillDir, "SKILL.md")
	if err := os.WriteFile(skillFile, []byte(content), 0644); err != nil {
		os.RemoveAll(skillDir)
		writeError(w, http.StatusInternalServerError, "failed to write SKILL.md: "+err.Error())
		return
	}

	// Create scaffold directories
	for _, dir := range req.ScaffoldDirs {
		dirPath := filepath.Join(skillDir, dir)
		if err := os.MkdirAll(dirPath, 0755); err != nil {
			os.RemoveAll(skillDir)
			writeError(w, http.StatusInternalServerError, fmt.Sprintf("failed to create %s: %s", dir, err.Error()))
			return
		}
		gitkeep := filepath.Join(dirPath, ".gitkeep")
		if err := os.WriteFile(gitkeep, []byte{}, 0644); err != nil {
			os.RemoveAll(skillDir)
			writeError(w, http.StatusInternalServerError, fmt.Sprintf("failed to create %s/.gitkeep: %s", dir, err.Error()))
			return
		}
		createdFiles = append(createdFiles, dir+"/.gitkeep")
	}

	// Ops log
	s.writeOpsLog("create-skill", "ok", start, map[string]any{
		"name":     req.Name,
		"pattern":  req.Pattern,
		"category": req.Category,
		"scope":    "ui",
	}, "")

	w.WriteHeader(http.StatusCreated)
	writeJSON(w, map[string]any{
		"skill": map[string]any{
			"name":       req.Name,
			"flatName":   req.Name,
			"relPath":    req.Name,
			"sourcePath": skillDir,
		},
		"createdFiles": createdFiles,
	})
}
