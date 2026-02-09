package server

import (
	"net/http"
	"path/filepath"

	"skillshare/internal/git"
	"skillshare/internal/install"
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
}

func (s *Server) handleCheck(w http.ResponseWriter, r *http.Request) {
	sourceDir := s.cfg.Source
	if s.IsProjectMode() {
		sourceDir = filepath.Join(s.projectRoot, ".skillshare", "skills")
	}

	repos, _ := install.GetTrackedRepos(sourceDir)
	skills, _ := install.GetUpdatableSkills(sourceDir)

	var repoResults []repoCheckResult
	for _, repo := range repos {
		repoPath := filepath.Join(sourceDir, repo)
		result := repoCheckResult{Name: repo}

		if isDirty, _ := git.IsDirty(repoPath); isDirty {
			result.Status = "dirty"
			result.Message = "has uncommitted changes"
		} else if behind, err := git.GetBehindCount(repoPath); err != nil {
			result.Status = "error"
			result.Message = err.Error()
		} else if behind == 0 {
			result.Status = "up_to_date"
		} else {
			result.Status = "behind"
			result.Behind = behind
		}

		repoResults = append(repoResults, result)
	}

	var skillResults []skillCheckResult
	for _, skill := range skills {
		skillPath := filepath.Join(sourceDir, skill)
		result := skillCheckResult{Name: skill}

		meta, err := install.ReadMeta(skillPath)
		if err != nil || meta == nil || meta.RepoURL == "" {
			result.Status = "local"
			skillResults = append(skillResults, result)
			continue
		}

		result.Source = meta.Source
		result.Version = meta.Version
		if !meta.InstalledAt.IsZero() {
			result.InstalledAt = meta.InstalledAt.Format("2006-01-02")
		}

		remoteHash, err := git.GetRemoteHeadHash(meta.RepoURL)
		if err != nil {
			result.Status = "error"
		} else if meta.Version == remoteHash {
			result.Status = "up_to_date"
		} else {
			result.Status = "update_available"
		}

		skillResults = append(skillResults, result)
	}

	if repoResults == nil {
		repoResults = []repoCheckResult{}
	}
	if skillResults == nil {
		skillResults = []skillCheckResult{}
	}

	writeJSON(w, map[string]any{
		"tracked_repos": repoResults,
		"skills":        skillResults,
	})
}
