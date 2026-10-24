package server

import (
	"context"
	"net/http"
	"path/filepath"

	"skillshare/internal/check"
	"skillshare/internal/install"
	"skillshare/internal/sourcewalk"
)

type repoCheckResult struct {
	Name    string `json:"name"`
	Status  string `json:"status"`
	Behind  int    `json:"behind"`
	Message string `json:"message,omitempty"`
}

type skillCheckResult struct {
	Name        string `json:"name"`
	Source      string `json:"source"`
	Version     string `json:"version"`
	Status      string `json:"status"`
	InstalledAt string `json:"installed_at,omitempty"`
	Kind        string `json:"kind,omitempty"`
	Message     string `json:"message,omitempty"`
}

// planSkillCheck plans the update check of every installed skill outside a
// followed checkout. It takes s.mu.RLock, so the caller must not hold s.mu.
func (s *Server) planSkillCheck(sourceDir, projectRoot string, walk sourcewalk.Options) *check.Resolution {
	names, _ := install.GetUpdatableSkills(sourceDir)
	var skills []check.Skill
	for _, name := range names {
		if followedCheckout(filepath.Join(sourceDir, name), walk.Follow) != "" {
			continue
		}
		skills = append(skills, check.Skill{Name: name, Entry: s.skillEntry(name), Dir: filepath.Join(sourceDir, name), Follow: walk.Follow})
	}
	return check.Plan(skills, projectRoot)
}

// dashboardSkillResults converts resolved statuses to the dashboard payload,
// which is narrower than the CLI's: a skill without a remote carries only its
// name, status and message.
func dashboardSkillResults(results []check.SkillResult) []skillCheckResult {
	out := make([]skillCheckResult, 0, len(results))
	for _, r := range results {
		if r.Local {
			out = append(out, skillCheckResult{Name: r.Name, Status: r.Status, Message: r.Message})
			continue
		}
		out = append(out, skillCheckResult{
			Name:        r.Name,
			Source:      r.Source,
			Version:     r.Version,
			Status:      r.Status,
			InstalledAt: r.InstalledAt,
			Message:     r.Message,
		})
	}
	return out
}

// checkTrackedRepo checks one tracked repo with the same logic as the CLI:
// auth-aware fetch and a reported dirty-check error.
func checkTrackedRepo(name, repoPath string) repoCheckResult {
	out := check.ParallelCheckRepos([]check.RepoCheckInput{{Name: name, RepoPath: repoPath}}, nil)[0]
	return repoCheckResult{Name: out.Name, Status: out.Status, Behind: out.Behind, Message: out.Message}
}

func (s *Server) handleCheck(w http.ResponseWriter, r *http.Request) {
	// Snapshot config under RLock, then release before I/O.
	s.mu.RLock()
	sourceDir := s.skillsSource()
	projectRoot := s.projectRoot
	walk := s.skillsWalk()
	s.mu.RUnlock()

	repos, linked := dashboardRepos(sourceDir, walk)
	plan := s.planSkillCheck(sourceDir, projectRoot, walk)

	var repoResults []repoCheckResult
	for _, repo := range repos {
		repoResults = append(repoResults, checkTrackedRepo(repo, filepath.Join(sourceDir, repo)))
	}

	// Without a context the check runs to completion, so the error is always nil.
	resolved, _ := plan.Run(context.Background(), check.Options{})
	skillResults := dashboardSkillResults(resolved)

	if repoResults == nil {
		repoResults = []repoCheckResult{}
	}
	writeJSON(w, map[string]any{
		"tracked_repos": repoResults,
		"linked_repos":  linked,
		"skills":        skillResults,
	})
}
