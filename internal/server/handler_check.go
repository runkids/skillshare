package server

import (
	"net/http"
	"path/filepath"
	"strings"

	"skillshare/internal/check"
	"skillshare/internal/git"
	"skillshare/internal/install"
)

// skillWithMetaEntry holds a skill name paired with its centralized metadata entry.
type skillWithMetaEntry struct {
	name  string
	entry *install.MetadataEntry
}

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
}

func (s *Server) handleCheck(w http.ResponseWriter, r *http.Request) {
	// Snapshot config under RLock, then release before I/O.
	s.mu.RLock()
	sourceDir := s.skillsSource()
	s.mu.RUnlock()

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
	urlGroups := make(map[string][]skillWithMetaEntry)
	var localResults []skillCheckResult

	for _, skill := range skills {
		entry := s.skillsStore.GetByPath(skill)
		if entry == nil || entry.RepoURL == "" {
			localResults = append(localResults, skillCheckResult{
				Name:   skill,
				Status: "local",
			})
			continue
		}
		urlGroups[entry.RepoURL] = append(urlGroups[entry.RepoURL], skillWithMetaEntry{
			name:  skill,
			entry: entry,
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
					Source:  sw.entry.Source,
					Version: sw.entry.Version,
					Status:  "error",
				}
				if !sw.entry.InstalledAt.IsZero() {
					r.InstalledAt = sw.entry.InstalledAt.Format("2006-01-02")
				}
				skillResults = append(skillResults, r)
			}
			continue
		}

		// Fast path: check if all skills match by commit hash
		allMatch := true
		for _, sw := range group {
			if sw.entry.Version != remoteHash {
				allMatch = false
				break
			}
		}
		if allMatch {
			for _, sw := range group {
				r := skillCheckResult{
					Name:    sw.name,
					Source:  sw.entry.Source,
					Version: sw.entry.Version,
					Status:  "up_to_date",
				}
				if !sw.entry.InstalledAt.IsZero() {
					r.InstalledAt = sw.entry.InstalledAt.Format("2006-01-02")
				}
				skillResults = append(skillResults, r)
			}
			continue
		}

		// Slow path: HEAD moved — try tree hash comparison
		var hasTreeHash bool
		for _, sw := range group {
			if sw.entry.TreeHash != "" && sw.entry.Subdir != "" {
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
				Source:  sw.entry.Source,
				Version: sw.entry.Version,
			}
			if !sw.entry.InstalledAt.IsZero() {
				r.InstalledAt = sw.entry.InstalledAt.Format("2006-01-02")
			}

			if sw.entry.Version == remoteHash {
				r.Status = "up_to_date"
			} else if sw.entry.TreeHash != "" && sw.entry.Subdir != "" && remoteTreeHashes != nil {
				normalizedSubdir := strings.TrimPrefix(sw.entry.Subdir, "/")
				if rh, ok := remoteTreeHashes[normalizedSubdir]; ok && sw.entry.TreeHash == rh {
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
