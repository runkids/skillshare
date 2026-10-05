package server

import (
	"errors"
	"net/http"
	"os"
	"path/filepath"
	"time"

	"skillshare/internal/sync"
)

type skillignoreStats struct {
	PatternCount  int      `json:"pattern_count"`
	IgnoredCount  int      `json:"ignored_count"`
	Patterns      []string `json:"patterns"`
	IgnoredSkills []string `json:"ignored_skills"`
}

type skillignoreResponse struct {
	Exists bool              `json:"exists"`
	Path   string            `json:"path"`
	Raw    string            `json:"raw"`
	Local  *ignoreLocalFile  `json:"local,omitempty"`
	Stats  *skillignoreStats `json:"stats,omitempty"`
}

// ignoreLocalFile is the machine-only .local companion of an ignore file.
// Its rules apply after the shared file and can override it.
type ignoreLocalFile struct {
	Path string `json:"path"`
	Raw  string `json:"raw"`
}

// readIgnoreLocal returns ignorePath's .local companion, or nil when absent.
func readIgnoreLocal(ignorePath string) *ignoreLocalFile {
	localPath := ignorePath + ".local"
	raw, err := os.ReadFile(localPath)
	if err != nil {
		return nil
	}
	return &ignoreLocalFile{Path: localPath, Raw: string(raw)}
}

func (s *Server) handleGetSkillignore(w http.ResponseWriter, r *http.Request) {
	// Snapshot source path under RLock, then release before I/O.
	s.mu.RLock()
	source := s.skillsSource()
	s.mu.RUnlock()

	ignorePath := filepath.Join(source, ".skillignore")

	raw, err := os.ReadFile(ignorePath)

	resp := skillignoreResponse{
		Exists: err == nil,
		Path:   ignorePath,
		Raw:    string(raw), // empty if file doesn't exist
		Local:  readIgnoreLocal(ignorePath),
	}

	// Always discover stats — tracked repos may have their own .skillignore
	_, stats, discoverErr := sync.DiscoverSourceSkillsWithStats(source)
	if discoverErr == nil && stats != nil {
		patterns := stats.Patterns
		if patterns == nil {
			patterns = []string{}
		}
		ignoredSkills := stats.IgnoredSkills
		if ignoredSkills == nil {
			ignoredSkills = []string{}
		}
		resp.Stats = &skillignoreStats{
			PatternCount:  stats.PatternCount(),
			IgnoredCount:  stats.IgnoredCount(),
			Patterns:      patterns,
			IgnoredSkills: ignoredSkills,
		}
	}

	writeJSON(w, resp)
}

func (s *Server) handlePutSkillignore(w http.ResponseWriter, r *http.Request) {
	start := time.Now()
	s.mu.Lock()
	defer s.mu.Unlock()

	var body struct {
		Raw string `json:"raw"`
	}
	if err := decodeJSON(w, r, &body, defaultJSONBodyLimit); err != nil {
		if !errors.Is(err, errBodyTooLarge) {
			writeError(w, http.StatusBadRequest, "invalid JSON body")
		}
		return
	}

	source := s.skillsSource()
	ignorePath := filepath.Join(source, ".skillignore")

	if body.Raw == "" {
		if err := os.Remove(ignorePath); err != nil && !errors.Is(err, os.ErrNotExist) {
			writeError(w, http.StatusInternalServerError, "failed to delete .skillignore: "+err.Error())
			return
		}
	} else {
		if err := os.WriteFile(ignorePath, []byte(body.Raw), 0644); err != nil {
			writeError(w, http.StatusInternalServerError, "failed to write .skillignore: "+err.Error())
			return
		}
	}

	s.writeOpsLog("skillignore", "ok", start, map[string]any{
		"scope": "ui",
	}, "")

	writeJSON(w, map[string]any{"success": true})
}
