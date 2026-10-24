package server

import (
	"errors"
	"net/http"
	"path/filepath"

	"skillshare/internal/install"
)

// handleSkillLocalChanges diffs the files of a skill installed from a repo
// against the version it was installed at. name is the skill's relative path.
func (s *Server) handleSkillLocalChanges(w http.ResponseWriter, r *http.Request) {
	name := r.PathValue("name")
	s.mu.RLock()
	source := s.skillsSource()
	walk := s.skillsWalk()
	parseOpts := s.parseOpts()
	var entry *install.MetadataEntry
	if e := s.skillsStore.GetExact(name); e != nil && filepath.IsLocal(name) {
		copied := *e
		entry = &copied
	}
	s.mu.RUnlock()

	changes, err := install.LocalChanges(entry, parseOpts, filepath.Join(source, filepath.FromSlash(name)), walk.Follow)
	if errors.Is(err, install.ErrNoInstallRecord) {
		writeError(w, http.StatusNotFound, name+": "+err.Error())
		return
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	if changes == nil {
		changes = []install.FileChange{}
	}
	writeJSON(w, map[string]any{"files": changes})
}
