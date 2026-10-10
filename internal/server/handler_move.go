package server

import (
	"errors"
	"net/http"
	"time"

	"skillshare/internal/skillmove"
	ssync "skillshare/internal/sync"
)

type batchMoveRequest struct {
	Names  []string `json:"names"`
	Dest   string   `json:"dest"`
	Kind   string   `json:"kind,omitempty"`
	Force  bool     `json:"force"`
	DryRun bool     `json:"dryRun"`
}

type batchMoveItemResult struct {
	Name      string `json:"name"`
	Success   bool   `json:"success"`
	From      string `json:"from,omitempty"`
	To        string `json:"to,omitempty"`
	FlatName  string `json:"flatName,omitempty"` // the new flat name of a skill; the dashboard route is keyed by it
	Record    bool   `json:"record,omitempty"`   // an install record moved along
	Skills    int    `json:"skills,omitempty"`   // the count below a folder
	Error     string `json:"error,omitempty"`
	ErrorCode string `json:"error_code,omitempty"`
}

// handleBatchMove moves skills and folders to another folder of the source,
// with their install records. It does not sync: the old target links stay
// until the dashboard runs sync.
func (s *Server) handleBatchMove(w http.ResponseWriter, r *http.Request) {
	start := time.Now()
	s.mu.Lock()
	defer s.mu.Unlock()

	var body batchMoveRequest
	if err := decodeJSON(w, r, &body, defaultJSONBodyLimit); err != nil {
		if !errors.Is(err, errBodyTooLarge) {
			writeCodedError(w, http.StatusBadRequest, "invalid_body", "invalid JSON body", nil)
		}
		return
	}
	if len(body.Names) == 0 {
		writeCodedError(w, http.StatusBadRequest, "invalid_body", "names array is required and must not be empty", nil)
		return
	}
	if body.Kind == "agent" {
		writeCodedError(w, http.StatusBadRequest, "unsupported_kind", "agents cannot be moved", nil)
		return
	}
	if body.Kind != "" && body.Kind != "skill" {
		writeCodedError(w, http.StatusBadRequest, "invalid_body", "invalid kind: "+body.Kind, nil)
		return
	}

	source := s.skillsSource()
	walk := s.skillsWalk()
	opts := skillmove.Options{
		SourceDir: source,
		Follow:    walk.Follow,
		Store:     s.skillsStore,
		Targets:   skillmove.EnabledTargets(s.cfg.Targets),
		Force:     body.Force,
		DryRun:    body.DryRun,
		Reconcile: func() error { return s.reconcileSkills(source, true) },
	}
	if s.IsProjectMode() {
		opts.ProjectRoot = s.projectRoot
		opts.GitignoreDir, opts.GitignorePrefix = s.gitignoreDir(), s.projectGitignorePrefix()
	}
	// A malformed destination fails the request; the other refusals are per name.
	if _, refusal := skillmove.CheckDest(body.Dest, opts); refusal != nil && refusal.Code == skillmove.CodeInvalidDest {
		writeCodedError(w, http.StatusBadRequest, string(refusal.Code), refusal.Error(), nil)
		return
	}

	discovered, err := ssync.DiscoverSourceSkillsAll(source, walk)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to discover skills: "+err.Error())
		return
	}
	out := skillmove.Run(skillmove.Plan(discovered, body.Names, body.Dest, opts), opts)

	results := make([]batchMoveItemResult, len(out.Items))
	warnings := []string{}
	var summary batchUninstallSummary
	var firstErr string
	for i, item := range out.Items {
		res := batchMoveItemResult{Name: item.Name}
		if item.Skipped() {
			// Already in place: nothing to do and nothing wrong; the code says why.
			res.Success, res.From, res.To, res.FlatName = true, item.From, item.To, item.FlatName()
			res.Error, res.ErrorCode = item.Err.Error(), string(item.Err.Code)
			summary.Succeeded++
		} else if item.Moved() {
			res.Success, res.From, res.To = true, item.From, item.To
			res.Record, res.FlatName = item.Records > 0, item.FlatName()
			if item.Folder || len(item.Skills) > 1 {
				res.Skills = len(item.Skills)
			}
			summary.Succeeded++
		} else {
			res.Error, res.ErrorCode = item.Err.Error(), string(item.Err.Code)
			summary.Failed++
			if firstErr == "" {
				firstErr = res.Error
			}
		}
		warnings = append(warnings, item.Warnings...)
		results[i] = res
	}
	if out.Err != nil {
		warnings = append(warnings, "moved, but a follow-up step failed: "+out.Err.Error())
		if firstErr == "" {
			firstErr = out.Err.Error()
		}
	}

	if !body.DryRun {
		status := "ok"
		switch {
		case firstErr != "" && summary.Succeeded > 0:
			status = "partial"
		case firstErr != "":
			status = "error"
		}
		s.writeOpsLog("move", status, start, map[string]any{
			"names": body.Names,
			"dest":  body.Dest,
			"force": body.Force,
			"count": summary.Succeeded,
			"scope": "ui",
		}, firstErr)
	}

	writeJSON(w, map[string]any{
		"results":  results,
		"summary":  summary,
		"warnings": warnings,
		"dryRun":   body.DryRun,
	})
}
