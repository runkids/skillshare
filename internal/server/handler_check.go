package server

import (
	"net/http"
	"path/filepath"
	"strings"

	"skillshare/internal/check"
	"skillshare/internal/git"
	"skillshare/internal/install"
	"skillshare/internal/sourcewalk"
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
	Message     string `json:"message,omitempty"`
}

// localCheckResult checks a skill without a remote: local installs are
// compared against their recorded source path, the rest stay "local".
// projectRoot is the base of relative sources; "" in global mode.
func localCheckResult(name string, entry *install.MetadataEntry, projectRoot string) skillCheckResult {
	status, message := check.LocalSourceStatus(entry, projectRoot)
	return skillCheckResult{Name: name, Status: status, Message: message}
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
	follow := s.skillFollowSet()
	s.mu.RUnlock()

	repos, err := install.GetTrackedReposWithOptions(sourceDir, sourcewalk.Options{Follow: follow})
	if err != nil && follow != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	skills, _ := install.GetUpdatableSkillsWithOptions(sourceDir, sourcewalk.Options{Follow: follow})

	var repoResults []repoCheckResult
	for _, repo := range repos {
		repoResults = append(repoResults, checkTrackedRepo(repo, filepath.Join(sourceDir, repo)))
	}

	// Group skills by repo URL+branch for efficient checking
	urlGroups := make(map[urlBranchGroup][]skillWithMetaEntry)
	var localResults []skillCheckResult

	for _, skill := range skills {
		entry := s.skillEntry(skill)
		if entry == nil || entry.RepoURL == "" {
			localResults = append(localResults, localCheckResult(skill, entry, projectRoot))
			continue
		}
		key := urlBranchGroup{url: entry.RepoURL, branch: entry.Branch}
		urlGroups[key] = append(urlGroups[key], skillWithMetaEntry{
			name:  skill,
			entry: entry,
		})
	}

	skillResults := append([]skillCheckResult{}, localResults...)

	for key, group := range urlGroups {
		remoteHash, err := key.remoteHash()

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
			remoteTreeHashes = check.FetchRemoteTreeHashesForRef(key.url, key.branch)
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

// urlBranchGroup keys skills that share a clone URL and installed branch, so a
// skill installed from a non-default branch is checked against that branch
// rather than the remote HEAD.
type urlBranchGroup struct {
	url    string
	branch string
}

func (g urlBranchGroup) remoteHash() (string, error) {
	if g.branch != "" {
		return git.GetRemoteRefHashWithAuth(g.url, g.branch)
	}
	return git.GetRemoteHeadHashWithAuth(g.url)
}
