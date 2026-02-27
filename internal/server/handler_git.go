package server

import (
	"encoding/json"
	"net/http"
	"time"

	"skillshare/internal/git"
	ssync "skillshare/internal/sync"
)

type gitStatusResponse struct {
	IsRepo    bool     `json:"isRepo"`
	HasRemote bool     `json:"hasRemote"`
	Branch    string   `json:"branch"`
	IsDirty   bool     `json:"isDirty"`
	Files     []string `json:"files"`
	SourceDir string   `json:"sourceDir"`
}

// handleGitStatus returns the git status of the source directory
func (s *Server) handleGitStatus(w http.ResponseWriter, r *http.Request) {
	src := s.cfg.Source
	resp := gitStatusResponse{
		SourceDir: src,
		Files:     make([]string, 0),
	}

	resp.IsRepo = git.IsRepo(src)
	if !resp.IsRepo {
		writeJSON(w, resp)
		return
	}

	resp.HasRemote = git.HasRemote(src)

	if branch, err := git.GetCurrentBranch(src); err == nil {
		resp.Branch = branch
	}

	if dirty, err := git.IsDirty(src); err == nil {
		resp.IsDirty = dirty
	}

	if files, err := git.GetDirtyFiles(src); err == nil && len(files) > 0 {
		resp.Files = files
	}

	writeJSON(w, resp)
}

type pushRequest struct {
	Message string `json:"message"`
	DryRun  bool   `json:"dryRun"`
}

type pushResponse struct {
	Success bool   `json:"success"`
	Message string `json:"message"`
	DryRun  bool   `json:"dryRun"`
}

// handlePush stages, commits, and pushes changes
func (s *Server) handlePush(w http.ResponseWriter, r *http.Request) {
	start := time.Now()
	s.mu.Lock()
	defer s.mu.Unlock()

	var body pushRequest
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON body")
		return
	}

	src := s.cfg.Source

	if !git.IsRepo(src) {
		writeError(w, http.StatusBadRequest, "source directory is not a git repository")
		return
	}
	if !git.HasRemote(src) {
		writeError(w, http.StatusBadRequest, "no git remote configured")
		return
	}

	// Check for changes
	status, err := git.GetStatus(src)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to get git status: "+err.Error())
		return
	}
	if status == "" {
		s.writeOpsLog("push", "ok", start, map[string]any{
			"summary": "nothing to push",
			"dry_run": body.DryRun,
			"scope":   "ui",
		}, "")
		writeJSON(w, pushResponse{Success: true, Message: "nothing to push (working tree clean)", DryRun: body.DryRun})
		return
	}

	if body.DryRun {
		s.writeOpsLog("push", "ok", start, map[string]any{
			"summary": "dry run",
			"dry_run": true,
			"scope":   "ui",
		}, "")
		writeJSON(w, pushResponse{Success: true, Message: "dry run: would stage, commit, and push changes", DryRun: true})
		return
	}

	// Stage all
	if err := git.StageAll(src); err != nil {
		writeError(w, http.StatusInternalServerError, "failed to stage changes: "+err.Error())
		return
	}

	// Commit
	msg := body.Message
	if msg == "" {
		msg = "Update skills"
	}
	if err := git.Commit(src, msg); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	// Push
	if err := git.PushRemoteWithAuth(src); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	s.writeOpsLog("push", "ok", start, map[string]any{
		"message": msg,
		"dry_run": false,
		"scope":   "ui",
	}, "")

	writeJSON(w, pushResponse{Success: true, Message: "pushed successfully"})
}

type pullResponse struct {
	Success     bool               `json:"success"`
	UpToDate    bool               `json:"upToDate"`
	Commits     []git.CommitInfo   `json:"commits"`
	Stats       git.DiffStats      `json:"stats"`
	SyncResults []syncTargetResult `json:"syncResults"`
	DryRun      bool               `json:"dryRun"`
	Message     string             `json:"message,omitempty"`
}

// handlePull pulls changes and syncs to targets
func (s *Server) handlePull(w http.ResponseWriter, r *http.Request) {
	start := time.Now()
	s.mu.Lock()
	defer s.mu.Unlock()

	var body struct {
		DryRun bool `json:"dryRun"`
	}
	json.NewDecoder(r.Body).Decode(&body)

	src := s.cfg.Source

	if !git.IsRepo(src) {
		writeError(w, http.StatusBadRequest, "source directory is not a git repository")
		return
	}
	if !git.HasRemote(src) {
		writeError(w, http.StatusBadRequest, "no git remote configured")
		return
	}

	// Check dirty
	dirty, err := git.IsDirty(src)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to check git status: "+err.Error())
		return
	}
	if dirty {
		writeError(w, http.StatusBadRequest, "working tree has uncommitted changes — commit or stash before pulling")
		return
	}

	if body.DryRun {
		s.writeOpsLog("pull", "ok", start, map[string]any{
			"summary": "dry run",
			"dry_run": true,
			"scope":   "ui",
		}, "")
		writeJSON(w, pullResponse{Success: true, DryRun: true, Message: "dry run: would pull and sync"})
		return
	}

	// Pull
	info, err := git.PullWithAuth(src)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "git pull failed: "+err.Error())
		return
	}

	resp := pullResponse{
		Success:  true,
		UpToDate: info.UpToDate,
		Commits:  info.Commits,
		Stats:    info.Stats,
	}

	if resp.Commits == nil {
		resp.Commits = make([]git.CommitInfo, 0)
	}

	// Auto-sync to targets (same logic as handleSync)
	if !info.UpToDate {
		globalMode := s.cfg.Mode
		if globalMode == "" {
			globalMode = "merge"
		}

		// Discover skills once for all targets
		allSkills, discoverErr := s.cache.Discover(src)

		for name, target := range s.cfg.Targets {
			mode := target.Mode
			if mode == "" {
				mode = globalMode
			}

			res := syncTargetResult{
				Target:  name,
				Linked:  make([]string, 0),
				Updated: make([]string, 0),
				Skipped: make([]string, 0),
				Pruned:  make([]string, 0),
			}

			if discoverErr != nil {
				resp.SyncResults = append(resp.SyncResults, res)
				continue
			}

			switch mode {
			case "merge":
				mergeResult, err := ssync.SyncTargetMergeWithSkills(name, target, allSkills, src, false, false)
				if err == nil {
					res.Linked = mergeResult.Linked
					res.Updated = mergeResult.Updated
					res.Skipped = mergeResult.Skipped
				}
				pruneResult, err := ssync.PruneOrphanLinksWithSkills(ssync.PruneOptions{
					TargetPath: target.Path, SourcePath: src, Skills: allSkills,
					Include: target.Include, Exclude: target.Exclude, TargetName: name,
				})
				if err == nil {
					res.Pruned = pruneResult.Removed
				}
			case "copy":
				copyResult, err := ssync.SyncTargetCopyWithSkills(name, target, allSkills, src, false, false, nil)
				if err == nil {
					res.Linked = copyResult.Copied
					res.Updated = copyResult.Updated
					res.Skipped = copyResult.Skipped
				}
				pruneResult, err := ssync.PruneOrphanCopiesWithSkills(target.Path, allSkills, target.Include, target.Exclude, name, false)
				if err == nil {
					res.Pruned = pruneResult.Removed
				}
			default:
				ssync.SyncTarget(name, target, src, false)
				res.Linked = []string{"(symlink mode)"}
			}

			resp.SyncResults = append(resp.SyncResults, res)
		}
	}

	if resp.SyncResults == nil {
		resp.SyncResults = make([]syncTargetResult, 0)
	}

	s.writeOpsLog("pull", "ok", start, map[string]any{
		"dry_run":      false,
		"up_to_date":   resp.UpToDate,
		"commits":      len(resp.Commits),
		"targets_sync": len(resp.SyncResults),
		"scope":        "ui",
	}, "")

	writeJSON(w, resp)
}
