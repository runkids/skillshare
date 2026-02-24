package server

import (
	"net/http"
	"path/filepath"

	"skillshare/internal/check"
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

	// Group skills by repo URL for efficient checking
	type skillWithMeta struct {
		name string
		meta *install.SkillMeta
	}
	urlGroups := make(map[string][]skillWithMeta)
	var localResults []skillCheckResult

	for _, skill := range skills {
		skillPath := filepath.Join(sourceDir, skill)
		meta, err := install.ReadMeta(skillPath)
		if err != nil || meta == nil || meta.RepoURL == "" {
			localResults = append(localResults, skillCheckResult{
				Name:   skill,
				Status: "local",
			})
			continue
		}
		urlGroups[meta.RepoURL] = append(urlGroups[meta.RepoURL], skillWithMeta{
			name: skill,
			meta: meta,
		})
	}

	skillResults := append([]skillCheckResult{}, localResults...)

	for url, group := range urlGroups {
		// Get remote HEAD hash
		remoteHash, err := git.GetRemoteHeadHash(url)

		if err != nil {
			for _, sw := range group {
				r := skillCheckResult{
					Name:    sw.name,
					Source:  sw.meta.Source,
					Version: sw.meta.Version,
					Status:  "error",
				}
				if !sw.meta.InstalledAt.IsZero() {
					r.InstalledAt = sw.meta.InstalledAt.Format("2006-01-02")
				}
				skillResults = append(skillResults, r)
			}
			continue
		}

		// Fast path: check if all skills match by commit hash
		allMatch := true
		for _, sw := range group {
			if sw.meta.Version != remoteHash {
				allMatch = false
				break
			}
		}
		if allMatch {
			for _, sw := range group {
				r := skillCheckResult{
					Name:    sw.name,
					Source:  sw.meta.Source,
					Version: sw.meta.Version,
					Status:  "up_to_date",
				}
				if !sw.meta.InstalledAt.IsZero() {
					r.InstalledAt = sw.meta.InstalledAt.Format("2006-01-02")
				}
				skillResults = append(skillResults, r)
			}
			continue
		}

		// Slow path: HEAD moved — try tree hash comparison
		var hasTreeHash bool
		for _, sw := range group {
			if sw.meta.TreeHash != "" && sw.meta.Subdir != "" {
				hasTreeHash = true
				break
			}
		}

		var remoteTreeHashes map[string]string
		if hasTreeHash {
			remoteTreeHashes = check.FetchRemoteTreeHashes(url)
		}

		for _, sw := range group {
			r := skillCheckResult{
				Name:    sw.name,
				Source:  sw.meta.Source,
				Version: sw.meta.Version,
			}
			if !sw.meta.InstalledAt.IsZero() {
				r.InstalledAt = sw.meta.InstalledAt.Format("2006-01-02")
			}

			if sw.meta.Version == remoteHash {
				r.Status = "up_to_date"
			} else if sw.meta.TreeHash != "" && sw.meta.Subdir != "" && remoteTreeHashes != nil {
				if rh, ok := remoteTreeHashes[sw.meta.Subdir]; ok && sw.meta.TreeHash == rh {
					r.Status = "up_to_date"
				} else {
					r.Status = "update_available"
				}
			} else {
				r.Status = "update_available"
			}

			skillResults = append(skillResults, r)
		}
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
