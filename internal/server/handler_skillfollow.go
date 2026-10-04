package server

import (
	"errors"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"skillshare/internal/sourcefs"
	"skillshare/internal/sourcewalk"
)

// skillfollowFile is one declaration file as the editor shows it.
type skillfollowFile struct {
	Path    string `json:"path"`
	Exists  bool   `json:"exists"`
	Content string `json:"content"`
}

// skillfollowResponse carries both declaration files and the states that
// status --json reports for them, computed from one follow snapshot.
type skillfollowResponse struct {
	Base        skillfollowFile    `json:"base"`
	Local       skillfollowFile    `json:"local"`
	Active      bool               `json:"active"`
	LocalActive bool               `json:"local_active"`
	Entries     []sourcewalk.Entry `json:"entries"`
	Warnings    []string           `json:"warnings"`
	PrunePaused []string           `json:"prune_paused"`
}

var skillfollowFiles = map[string]string{"base": ".skillfollow", "local": ".skillfollow.local"}

func readSkillfollowFile(path string) skillfollowFile {
	raw, err := os.ReadFile(path)
	return skillfollowFile{Path: path, Exists: err == nil, Content: string(raw)}
}

// buildSkillfollowResponse lists declared entries only, like status --json:
// undeclared links stay invisible to this view.
func buildSkillfollowResponse(source string, follow *sourcewalk.FollowSet) skillfollowResponse {
	resp := skillfollowResponse{
		Base:        readSkillfollowFile(filepath.Join(source, ".skillfollow")),
		Local:       readSkillfollowFile(filepath.Join(source, ".skillfollow.local")),
		Entries:     []sourcewalk.Entry{},
		Warnings:    []string{},
		PrunePaused: []string{},
	}
	if follow == nil {
		return resp
	}
	resp.Active, resp.LocalActive = follow.Active(), follow.HasLocal()
	for _, entry := range follow.Entries() {
		if entry.State != sourcewalk.UndeclaredLink {
			resp.Entries = append(resp.Entries, entry)
		}
	}
	resp.Warnings = append(resp.Warnings, follow.Warnings()...)
	// Same recovery sentence as status and doctor print.
	resp.PrunePaused = append(resp.PrunePaused, follow.PrunePauses(source)...)
	return resp
}

func (s *Server) handleGetSkillfollow(w http.ResponseWriter, r *http.Request) {
	s.mu.RLock()
	source := s.skillsSource()
	follow := s.skillFollowSet()
	s.mu.RUnlock()

	writeJSON(w, buildSkillfollowResponse(source, follow))
}

// handlePutSkillfollow edits one declaration file. It changes no links,
// .gitignore entries, or targets; the next sync applies the declarations.
func (s *Server) handlePutSkillfollow(w http.ResponseWriter, r *http.Request) {
	start := time.Now()
	s.mu.Lock()
	defer s.mu.Unlock()

	var body struct {
		File    string `json:"file"`
		Content string `json:"content"`
		Delete  bool   `json:"delete"`
	}
	if err := decodeJSON(w, r, &body, defaultJSONBodyLimit); err != nil {
		if !errors.Is(err, errBodyTooLarge) {
			writeError(w, http.StatusBadRequest, "invalid JSON body")
		}
		return
	}
	name, ok := skillfollowFiles[body.File]
	if !ok {
		writeError(w, http.StatusBadRequest, `file must be "base" or "local"`)
		return
	}
	if body.Delete && body.Content != "" {
		writeError(w, http.StatusBadRequest, "delete requires empty content")
		return
	}
	for i, line := range strings.Split(body.Content, "\n") {
		entry := strings.TrimSpace(line)
		if entry == "" || strings.HasPrefix(entry, "#") {
			continue
		}
		if !sourcewalk.ValidEntryName(entry) {
			writeError(w, http.StatusBadRequest, fmt.Sprintf("%s:%d: invalid first-level entry %q; use one direct child name per line", name, i+1, entry))
			return
		}
	}

	source := s.skillsSource()
	// The source handle refuses a linked declaration file instead of writing through it.
	src, err := sourcefs.Open(source)
	if err == nil {
		defer src.Close()
		if body.Delete {
			err = src.Remove(name)
			if errors.Is(err, os.ErrNotExist) {
				err = nil
			}
		} else {
			err = src.WriteFileAtomic(name, []byte(body.Content), 0644)
		}
	}
	if err != nil {
		s.writeOpsLog("skillfollow", "error", start, map[string]any{"scope": "ui", "file": name}, err.Error())
		writeError(w, http.StatusInternalServerError, "failed to write "+name+": "+err.Error())
		return
	}

	s.writeOpsLog("skillfollow", "ok", start, map[string]any{"scope": "ui", "file": name, "delete": body.Delete}, "")
	writeJSON(w, buildSkillfollowResponse(source, s.skillFollowSet()))
}
