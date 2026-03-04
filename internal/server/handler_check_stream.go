package server

import (
	"net/http"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"skillshare/internal/check"
	"skillshare/internal/git"
	"skillshare/internal/install"
)

// handleCheckStream serves an SSE endpoint that streams check progress in real time.
// Events:
//   - "discovering" → {"phase":"..."}                    immediately on connect
//   - "start"       → {"total": N, "repos": R, "sources": S}  after discovery (N = work units)
//   - "progress"    → {"checked": N}                     every 200ms
//   - "done"        → {"tracked_repos":…,"skills":…}     final payload (same shape as GET /api/check)
//
// "total" counts actual work units (repos + remote URL groups), NOT individual skills.
// This ensures the progress bar advances evenly — one tick per network call.
func (s *Server) handleCheckStream(w http.ResponseWriter, r *http.Request) {
	safeSend, ok := initSSE(w)
	if !ok {
		return
	}

	ctx := r.Context()

	sourceDir := s.cfg.Source
	if s.IsProjectMode() {
		sourceDir = filepath.Join(s.projectRoot, ".skillshare", "skills")
	}

	// Immediate feedback before the potentially slow discovery walk.
	safeSend("discovering", map[string]string{"phase": "scanning source directory"})

	repos, _ := install.GetTrackedRepos(sourceDir)
	skills, _ := install.GetUpdatableSkills(sourceDir)

	// --- Pre-process: group skills by URL (fast, local only) ---
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

	// Total = repos + URL groups (the actual network-bound work units).
	total := len(repos) + len(urlGroups)
	safeSend("start", map[string]any{
		"total":   total,
		"repos":   len(repos),
		"sources": len(urlGroups),
	})

	// Atomic counter + ticker for progress events
	var checked atomic.Int64
	done := make(chan struct{})
	var wg sync.WaitGroup
	wg.Add(1)

	go func() {
		defer wg.Done()
		ticker := time.NewTicker(200 * time.Millisecond)
		defer ticker.Stop()
		for {
			select {
			case <-ticker.C:
				safeSend("progress", map[string]int64{"checked": checked.Load()})
			case <-done:
				return
			case <-ctx.Done():
				return
			}
		}
	}()

	// --- Phase 1: Check tracked repos (1 work unit per repo) ---
	var repoResults []repoCheckResult
	for _, repo := range repos {
		select {
		case <-ctx.Done():
			close(done)
			wg.Wait()
			return
		default:
		}

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
		checked.Add(1)
	}

	// --- Phase 2: Check skills by URL group (1 work unit per URL) ---
	skillResults := append([]skillCheckResult{}, localResults...)

	for url, group := range urlGroups {
		select {
		case <-ctx.Done():
			close(done)
			wg.Wait()
			return
		default:
		}

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
			checked.Add(1)
			continue
		}

		// Fast path: all commit hashes match
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
			checked.Add(1)
			continue
		}

		// Slow path: tree hash comparison
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
				normalizedSubdir := strings.TrimPrefix(sw.meta.Subdir, "/")
				if rh, ok := remoteTreeHashes[normalizedSubdir]; ok && sw.meta.TreeHash == rh {
					r.Status = "up_to_date"
				} else {
					r.Status = "update_available"
				}
			} else {
				r.Status = "update_available"
			}

			skillResults = append(skillResults, r)
		}
		checked.Add(1)
	}

	// Stop ticker
	close(done)
	wg.Wait()

	if repoResults == nil {
		repoResults = []repoCheckResult{}
	}
	if skillResults == nil {
		skillResults = []skillCheckResult{}
	}

	safeSend("done", map[string]any{
		"tracked_repos": repoResults,
		"skills":        skillResults,
	})
}
