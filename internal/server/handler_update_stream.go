package server

import (
	"fmt"
	"net/http"
	"path/filepath"
	"strings"
	"time"

	"skillshare/internal/install"
)

// handleUpdateStream serves an SSE endpoint that streams update progress in real time.
// Events:
//   - "start"  → {"total": N}                    after collecting updatable items
//   - "result" → updateResultItem                 after each item is updated
//   - "done"   → {"results":…,"summary":…}       final payload
//
// Query params: names (comma-separated), force, skipAudit
func (s *Server) handleUpdateStream(w http.ResponseWriter, r *http.Request) {
	safeSend, ok := initSSE(w)
	if !ok {
		return
	}

	start := time.Now()
	ctx := r.Context()

	force := r.URL.Query().Get("force") == "true"
	skipAudit := r.URL.Query().Get("skipAudit") == "true"

	// Snapshot source under RLock, then release before slow I/O.
	s.mu.RLock()
	source := s.cfg.Source
	s.mu.RUnlock()

	// Collect items to update based on "names" query param.
	// If names is empty, update all updatable items.
	namesParam := r.URL.Query().Get("names")

	type updateItem struct {
		name   string
		isRepo bool
		path   string
	}

	var items []updateItem

	if namesParam != "" {
		// Update specific items by name
		for name := range strings.SplitSeq(namesParam, ",") {
			name = strings.TrimSpace(name)
			if name == "" {
				continue
			}
			// Check if it's a tracked repo
			repoName := name
			if !strings.HasPrefix(repoName, "_") {
				repoName = "_" + name
			}
			repoPath := filepath.Join(source, repoName)
			if install.IsGitRepo(repoPath) {
				items = append(items, updateItem{name: repoName, isRepo: true, path: repoPath})
				continue
			}
			// Check original name as repo
			origPath := filepath.Join(source, name)
			if install.IsGitRepo(origPath) {
				items = append(items, updateItem{name: name, isRepo: true, path: origPath})
				continue
			}
			// Regular skill
			skillPath := filepath.Join(source, name)
			items = append(items, updateItem{name: name, isRepo: false, path: skillPath})
		}
	} else {
		// Update all: tracked repos + regular skills
		repos, err := install.GetTrackedRepos(source)
		if err == nil {
			for _, repo := range repos {
				items = append(items, updateItem{
					name:   repo,
					isRepo: true,
					path:   filepath.Join(source, repo),
				})
			}
		}
		skills, err := getServerUpdatableSkills(source)
		if err == nil {
			for _, skill := range skills {
				items = append(items, updateItem{
					name:   skill,
					isRepo: false,
					path:   filepath.Join(source, skill),
				})
			}
		}
	}

	safeSend("start", map[string]int{"total": len(items)})

	var results []updateResultItem
	summary := struct {
		Updated  int `json:"updated"`
		UpToDate int `json:"upToDate"`
		Blocked  int `json:"blocked"`
		Errors   int `json:"errors"`
		Skipped  int `json:"skipped"`
	}{}

	for _, item := range items {
		// Check if client disconnected
		select {
		case <-ctx.Done():
			return
		default:
		}

		var result updateResultItem

		// Lock per-item for write operations
		s.mu.Lock()
		if item.isRepo {
			result = s.updateTrackedRepo(item.name, item.path, force, skipAudit)
		} else {
			result = s.updateRegularSkill(item.name, item.path, skipAudit)
		}
		s.mu.Unlock()

		results = append(results, result)

		switch result.Action {
		case "updated":
			summary.Updated++
		case "up-to-date":
			summary.UpToDate++
		case "blocked":
			summary.Blocked++
		case "error":
			summary.Errors++
		case "skipped":
			summary.Skipped++
		}

		safeSend("result", result)
	}

	// Write ops log
	s.mu.Lock()
	status := "ok"
	msg := ""
	if summary.Blocked > 0 {
		status = "partial"
		msg = fmt.Sprintf("%d update(s) blocked by security audit", summary.Blocked)
	}
	if summary.Errors > 0 {
		status = "partial"
		if msg != "" {
			msg += fmt.Sprintf(", %d update(s) failed", summary.Errors)
		} else {
			msg = fmt.Sprintf("%d update(s) failed", summary.Errors)
		}
	}
	s.writeOpsLog("update", status, start, map[string]any{
		"name":            "--all-stream",
		"force":           force,
		"skip_audit":      skipAudit,
		"results_total":   len(results),
		"results_failed":  summary.Errors,
		"results_blocked": summary.Blocked,
		"scope":           "ui",
	}, msg)
	s.mu.Unlock()

	safeSend("done", map[string]any{
		"results": results,
		"summary": summary,
	})
}
