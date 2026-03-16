package server

import (
	"net/http"
	"sync"
	"sync/atomic"
	"time"

	"skillshare/internal/audit"
)

// handleAuditStream serves an SSE endpoint that streams audit progress in real time.
// Events:
//   - "start"    → {"total": N}           after skill discovery
//   - "progress" → {"scanned": N}         every 200ms during scan
//   - "done"     → {"results":…,"summary":…}  final payload (same shape as GET /api/audit)
func (s *Server) handleAuditStream(w http.ResponseWriter, r *http.Request) {
	safeSend, ok := initSSE(w)
	if !ok {
		return
	}

	start := time.Now()

	// Snapshot config under RLock, then release before slow I/O.
	s.mu.RLock()
	source := s.cfg.Source
	projectRoot := s.projectRoot
	policy := s.auditPolicy()
	s.mu.RUnlock()

	// 1. Discover skills
	skills, err := discoverAuditSkills(source)
	if err != nil {
		safeSend("error", map[string]string{"error": err.Error()})
		return
	}

	safeSend("start", map[string]int{"total": len(skills)})

	// 2. Atomic counter + ticker for progress events
	var scanned atomic.Int64
	ctx := r.Context()
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
				safeSend("progress", map[string]int64{"scanned": scanned.Load()})
			case <-done:
				return
			case <-ctx.Done():
				return
			}
		}
	}()

	onDone := func() { scanned.Add(1) }

	// 3. Parallel scan (blocks until all skills are scanned)
	outputs := audit.ParallelScan(skillsToAuditInputs(skills), projectRoot, onDone, nil)
	close(done) // signal ticker goroutine to stop
	wg.Wait()   // wait for it to fully exit before writing to w

	// 4. Process results
	agg := processAuditResults(skills, outputs, policy)
	s.writeAuditLog(agg.Status, start, agg.LogArgs, agg.Message)

	// 5. Send final result (no concurrent writers at this point)
	safeSend("done", map[string]any{
		"results": agg.Results,
		"summary": agg.Summary,
	})
}
