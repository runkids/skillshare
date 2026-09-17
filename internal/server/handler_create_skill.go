package server

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"time"
	"unicode/utf8"

	"skillshare/internal/skill"
	"skillshare/internal/utils"
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
	Description  string   `json:"description"`
	Into         string   `json:"into"`
	ScaffoldDirs []string `json:"scaffoldDirs"`
}

// maxDescriptionLen is the longest description every target accepts (Codex rejects longer ones).
const maxDescriptionLen = 1024

// validate checks the fields shared by create and preview and returns an error message.
func (req *createSkillRequest) validate() string {
	if !skill.ValidNameRe.MatchString(req.Name) {
		return "invalid skill name: use lowercase letters, numbers, hyphens, underscores; must start with letter or underscore"
	}
	if skill.FindPattern(req.Pattern) == nil {
		return fmt.Sprintf("unknown pattern: %s", req.Pattern)
	}
	if utf8.RuneCountInString(req.Description) > maxDescriptionLen {
		return fmt.Sprintf("description is longer than %d characters", maxDescriptionLen)
	}
	if req.Into != "" && !filepath.IsLocal(req.Into) {
		return "invalid folder: must be a relative path inside the source directory"
	}
	return ""
}

func (req *createSkillRequest) content() string {
	return skill.WithDescription(skill.GenerateContent(req.Name, req.Pattern, req.Category), req.Description)
}

// handlePreviewSkill returns the SKILL.md that create would write, so the form can show it before creating.
func (s *Server) handlePreviewSkill(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	req := createSkillRequest{Name: q.Get("name"), Pattern: q.Get("pattern"), Category: q.Get("category"), Description: q.Get("description"), Into: q.Get("into")}
	if msg := req.validate(); msg != "" {
		writeError(w, http.StatusBadRequest, msg)
		return
	}
	writeJSON(w, map[string]any{
		"content": req.content(),
		"path":    filepath.Join(s.cfg.EffectiveSkillsSource(), req.Into, req.Name, "SKILL.md"),
	})
}

func (s *Server) handleCreateSkill(w http.ResponseWriter, r *http.Request) {
	start := time.Now()

	var req createSkillRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	if msg := req.validate(); msg != "" {
		writeError(w, http.StatusBadRequest, msg)
		return
	}
	pattern := skill.FindPattern(req.Pattern)

	// Validate scaffoldDirs against the pattern's dirs. A blank template may use any
	// pattern's dirs; anything else would let a request create folders outside the skill.
	allowed := map[string]bool{}
	for _, p := range skill.Patterns {
		if p.Name == pattern.Name || len(pattern.ScaffoldDirs) == 0 {
			for _, d := range p.ScaffoldDirs {
				allowed[d] = true
			}
		}
	}
	for _, d := range req.ScaffoldDirs {
		if !allowed[d] {
			writeError(w, http.StatusBadRequest, fmt.Sprintf("scaffold dir %q not valid for pattern %q", d, req.Pattern))
			return
		}
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	source := s.cfg.EffectiveSkillsSource()
	relPath := filepath.ToSlash(filepath.Join(req.Into, req.Name))
	skillDir := filepath.Join(source, req.Into, req.Name)

	// Check if skill already exists
	if _, err := os.Stat(skillDir); err == nil {
		writeError(w, http.StatusConflict, fmt.Sprintf("skill '%s' already exists", req.Name))
		return
	}

	// Generate SKILL.md content
	content := req.content()

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
	args := map[string]any{
		"name":     req.Name,
		"pattern":  req.Pattern,
		"category": req.Category,
		"scope":    "ui",
	}
	if req.Into != "" {
		args["into"] = req.Into
	}
	s.writeOpsLog("create-skill", "ok", start, args, "")

	w.WriteHeader(http.StatusCreated)
	writeJSON(w, map[string]any{
		"skill": map[string]any{
			"name":       req.Name,
			"flatName":   utils.PathToFlatName(relPath),
			"relPath":    relPath,
			"sourcePath": skillDir,
		},
		"createdFiles": createdFiles,
	})
}
