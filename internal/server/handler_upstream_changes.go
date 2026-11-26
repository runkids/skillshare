package server

import (
	"errors"
	"net/http"
	"path/filepath"
	"slices"

	"skillshare/internal/check"
	"skillshare/internal/git"
	"skillshare/internal/install"
)

// upstreamCommitLimit caps the commits listed for a tracked repo.
const upstreamCommitLimit = 50

// handleSkillUpstreamChanges diffs the version a skill was installed at with
// the latest commit of its remote, as updating would change it. name is the
// skill's relative path.
func (s *Server) handleSkillUpstreamChanges(w http.ResponseWriter, r *http.Request) {
	name := r.PathValue("name")
	s.mu.RLock()
	parseOpts := s.parseOpts()
	var entry *install.MetadataEntry
	if e := s.skillsStore.GetExact(name); e != nil && filepath.IsLocal(name) {
		copied := *e
		entry = &copied
	}
	s.mu.RUnlock()

	if entry == nil || entry.Tracked || entry.RepoURL == "" {
		writeError(w, http.StatusNotFound, name+": "+install.ErrNoInstallRecord.Error())
		return
	}
	// The same lookup the update check uses, so the preview shows the update it reported.
	latest, _, err := check.GitRemote{ParseOptions: parseOpts}.Hash(entry.RepoURL, entry.Branch)
	if err != nil {
		writeError(w, http.StatusBadGateway, "resolve the latest commit: "+err.Error())
		return
	}
	diff, err := install.UpstreamChanges(entry, parseOpts, latest)
	if errors.Is(err, install.ErrNoInstallRecord) {
		writeError(w, http.StatusNotFound, name+": "+err.Error())
		return
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, diff)
}

// handleRepoUpstreamChanges lists what pulling a tracked repo would bring in,
// comparing HEAD with the upstream as the last update check left it. It does
// not fetch.
func (s *Server) handleRepoUpstreamChanges(w http.ResponseWriter, r *http.Request) {
	name := r.PathValue("name")
	s.mu.RLock()
	source := s.skillsSource()
	walk := s.skillsWalk()
	s.mu.RUnlock()

	repos, _ := dashboardRepos(source, walk)
	if !slices.Contains(repos, name) {
		writeError(w, http.StatusNotFound, "tracked repo not found: "+name)
		return
	}
	repoPath := filepath.Join(source, filepath.FromSlash(name))
	if !git.HasUpstream(repoPath) {
		writeError(w, http.StatusConflict, name+": the current branch has no upstream to compare with")
		return
	}
	preview, err := git.PreviewUpstream(repoPath, upstreamCommitLimit)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, map[string]any{
		"from":          preview.From,
		"to":            preview.To,
		"total_commits": preview.TotalCommits,
		"commits":       preview.Commits,
		"files":         preview.Files,
	})
}
