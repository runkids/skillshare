package server

import (
	"net/http"
	"path/filepath"
	"slices"
	"sync"
	"sync/atomic"
	"time"

	"skillshare/internal/check"
)

// handleCheckStream serves an SSE endpoint that streams check progress in real time.
// Events:
//   - "discovering" → {"phase":"..."}                    immediately on connect
//   - "start"       → {"total": N, "repos": R, "sources": S}  after discovery (N = work units)
//   - "progress"    → {"checked": N}                     every 200ms
//   - "done"        → {"tracked_repos":…,"skills":…,"agents":…}  final payload (GET /api/check plus agents)
//
// "total" counts actual work units (repos + remote URL groups), NOT individual skills.
// This ensures the progress bar advances evenly — one tick per network call.
//
// Query params: names (comma-separated) limits the check to those tracked repos
// and skills; the events then cover only them, and no agent is checked.
func (s *Server) handleCheckStream(w http.ResponseWriter, r *http.Request) {
	safeSend, ok := initSSE(w)
	if !ok {
		return
	}

	ctx := r.Context()

	// Snapshot config under RLock, then release before slow I/O.
	s.mu.RLock()
	sourceDir := s.skillsSource()
	agentsDir := s.agentsSource()
	projectRoot := s.projectRoot
	walk := s.skillsWalk()
	parseOpts := s.parseOpts()
	s.mu.RUnlock()

	// Immediate feedback before the potentially slow discovery walk.
	safeSend("discovering", map[string]string{"phase": "scanning source directory"})

	only := checkNames(r)
	repos, linked := dashboardRepos(sourceDir, walk)
	if only != nil {
		repos = slices.DeleteFunc(repos, func(repo string) bool { return !only[repo] })
	}
	// Group skills by remote (fast, local only).
	plan := s.planSkillCheck(sourceDir, projectRoot, walk, only)
	// Names are tracked repos and skills, so only a full check covers agents.
	var agents []check.AgentCheckResult
	if only == nil {
		agents = check.CheckAgents(agentsDir)
	}

	// Total = repos + URL groups (the actual network-bound work units).
	sources := plan.Remotes() + check.AgentRemotes(agents)
	safeSend("start", map[string]any{
		"total":   len(repos) + sources,
		"repos":   len(repos),
		"sources": sources,
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

		repoResults = append(repoResults, checkTrackedRepo(repo, filepath.Join(sourceDir, repo), parseOpts))
		checked.Add(1)
	}

	// --- Phase 2: Check skills by remote (1 work unit per remote) ---
	resolved, err := plan.Run(ctx, check.Options{ParseOptions: parseOpts, OnRemoteDone: func(int) { checked.Add(1) }})

	// --- Phase 3: Check agents by remote (1 work unit per remote) ---
	if err == nil {
		check.EnrichAgentResultsWithRemote(agents, func() { checked.Add(1) }, parseOpts)
	}

	// Stop ticker
	close(done)
	wg.Wait()
	if err != nil {
		return
	}

	if repoResults == nil {
		repoResults = []repoCheckResult{}
	}

	safeSend("done", map[string]any{
		"tracked_repos": repoResults,
		"linked_repos":  linked,
		"skills":        dashboardSkillResults(resolved),
		"agents":        dashboardAgentResults(agents),
	})
}
