package server

import (
	"errors"
	"net/http"
	"os"
	"path/filepath"
	"time"

	"skillshare/internal/git"
)

func (s *Server) handleGitDiscard(w http.ResponseWriter, r *http.Request) {
	start := time.Now()
	if s.IsProjectMode() {
		writeError(w, http.StatusBadRequest, "Git Sync is only available in global mode")
		return
	}
	var body struct {
		DryRun bool `json:"dryRun"`
	}
	if err := decodeJSON(w, r, &body, defaultJSONBodyLimit); err != nil {
		if !errors.Is(err, errBodyTooLarge) {
			writeError(w, http.StatusBadRequest, "invalid JSON body")
		}
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	src, ok := s.gitSource(w)
	if !ok {
		return
	}
	// Never discover an ancestor repo and discard outside the configured source.
	if _, err := os.Stat(filepath.Join(src, ".git")); err != nil || !git.IsRepo(src) {
		writeError(w, http.StatusBadRequest, "source directory is not a git repository root")
		return
	}
	if _, err := git.GetCurrentHash(src); err != nil {
		writeError(w, http.StatusBadRequest, "cannot discard changes before the first commit")
		return
	}
	args := map[string]any{"dry_run": body.DryRun, "scope": "ui", "git_root": s.cfg.GitRoot}
	if body.DryRun {
		s.writeOpsLog("discard", "ok", start, args, "")
		writeJSON(w, pushResponse{Success: true, Message: "dry run: would restore tracked files to HEAD and remove untracked files", DryRun: true})
		return
	}
	if err := git.DiscardChanges(src, s.cfg.GitRoot == "root"); err != nil {
		s.writeOpsLog("discard", "error", start, args, err.Error())
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	s.writeOpsLog("discard", "ok", start, args, "")
	writeJSON(w, pushResponse{Success: true, Message: "changes discarded"})
}
