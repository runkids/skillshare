package server

import (
	"encoding/json"
	"net/http"
	"strings"
	"time"

	"skillshare/internal/config"
	"skillshare/internal/git"
	ssync "skillshare/internal/sync"
)

type gitStatusResponse struct {
	GitInstalled   bool     `json:"gitInstalled"`
	IsRepo         bool     `json:"isRepo"`
	HasRemote      bool     `json:"hasRemote"`
	Branch         string   `json:"branch"`
	IsDirty        bool     `json:"isDirty"`
	Files          []string `json:"files"`
	SourceDir      string   `json:"sourceDir"`
	Scope          string   `json:"scope"`
	ScopeMismatch  bool     `json:"scopeMismatch"`
	MismatchScope  string   `json:"mismatchScope,omitempty"`
	MismatchDir    string   `json:"mismatchDir,omitempty"`
	RemoteURL      string   `json:"remoteURL,omitempty"`
	HeadHash       string   `json:"headHash,omitempty"`
	HeadMessage    string   `json:"headMessage,omitempty"`
	TrackingBranch string   `json:"trackingBranch,omitempty"`
	// Root-scope hazards (populated only when scope == "root"): NestedRepos are
	// subdirectories with their own .git that commit as empty submodules;
	// ConfigTracked means config.yaml leaked into version control.
	NestedRepos   []string `json:"nestedRepos"`
	ConfigTracked bool     `json:"configTracked"`
}

// handleGitStatus returns the git status of the source directory
func (s *Server) handleGitStatus(w http.ResponseWriter, r *http.Request) {
	// Snapshot config under RLock, then release before I/O.
	s.mu.RLock()
	src := s.cfg.EffectiveGitRoot()
	scope := s.cfg.GitRoot
	mScope, mDir, mismatch := s.cfg.GitRootMismatch()
	s.mu.RUnlock()
	if scope == "" {
		scope = "skills"
	}
	resp := gitStatusResponse{
		GitInstalled:  git.IsInstalled(),
		SourceDir:     src,
		Scope:         scope,
		ScopeMismatch: mismatch,
		MismatchScope: mScope,
		MismatchDir:   mDir,
		Files:         make([]string, 0),
		NestedRepos:   make([]string, 0),
	}

	// Without git on PATH every other probe is a raw exec failure; report it
	// plainly and stop so the UI can show a clear "git not installed" notice.
	if !resp.GitInstalled {
		writeJSON(w, resp)
		return
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

	if url, err := git.GetRemoteURL(src); err == nil {
		resp.RemoteURL = url
	}

	if hash, err := git.GetCurrentHash(src); err == nil {
		resp.HeadHash = hash
	}

	if msg, err := git.GetHeadMessage(src); err == nil {
		resp.HeadMessage = msg
	}

	if tb, err := git.GetTrackingBranch(src); err == nil {
		resp.TrackingBranch = tb
	}

	// Root-scope hazards: nested submodule traps and a leaked config.yaml.
	if scope == "root" {
		if nested, err := git.NestedRepos(src); err == nil {
			resp.NestedRepos = nested
		}
		resp.ConfigTracked = git.IsConfigTracked(src)
	}

	writeJSON(w, resp)
}

type setGitRootRequest struct {
	Scope     string `json:"scope"`
	RemoteURL string `json:"remoteURL,omitempty"`
}

// handleSetGitRoot changes the git_root scope: it initializes a git repo at the
// new scope directory if absent (with a scope-aware .gitignore), optionally
// wires the "origin" remote on that repo, persists the scope to config, and
// returns the updated git status. It does NOT relocate an existing repo —
// switching scope creates/uses a repo at the target directory, mirroring the
// CLI's `skillshare init --git-root <scope> [--remote <url>]` behavior.
func (s *Server) handleSetGitRoot(w http.ResponseWriter, r *http.Request) {
	start := time.Now()

	if s.IsProjectMode() {
		writeError(w, http.StatusBadRequest, "git_root is only available in global mode")
		return
	}
	if !git.IsInstalled() {
		writeError(w, http.StatusBadRequest, "git is not installed or not in PATH")
		return
	}

	var body setGitRootRequest
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON body")
		return
	}
	if !config.ValidGitRoot(body.Scope) || body.Scope == "" {
		writeError(w, http.StatusBadRequest, "invalid scope (want: skills, agents, extras, or root)")
		return
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	dir := config.ScopeDir(s.cfg, body.Scope)
	if _, err := git.InitScopeRepo(dir, body.Scope); err != nil {
		writeError(w, http.StatusInternalServerError, "failed to initialize git at scope: "+err.Error())
		return
	}

	// Optionally wire the remote on the just-initialized scope repo.
	if remote := strings.TrimSpace(body.RemoteURL); remote != "" {
		if err := git.SetOrAddRemote(dir, remote); err != nil {
			writeError(w, http.StatusInternalServerError, "failed to set git remote: "+err.Error())
			return
		}
	}

	// Persist the scope. "skills" is the default — store it as empty to keep
	// config.yaml clean and consistent with the CLI.
	if body.Scope == "skills" {
		s.cfg.GitRoot = ""
	} else {
		s.cfg.GitRoot = body.Scope
	}
	if err := s.saveAndReloadConfig(); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	s.writeOpsLog("git-root", "ok", start, map[string]any{"scope": body.Scope}, "")

	writeJSON(w, map[string]any{
		"success": true,
		"scope":   body.Scope,
		"gitRoot": dir,
	})
}

type gitBranchesResponse struct {
	Current    string   `json:"current"`
	Local      []string `json:"local"`
	Remote     []string `json:"remote"`
	IsDirty    bool     `json:"isDirty"`
	DirtyFiles []string `json:"dirtyFiles"`
}

// handleGitBranches returns local/remote branches for the source directory.
// Pass ?fetch=true to run git fetch first (discovers new remote branches).
func (s *Server) handleGitBranches(w http.ResponseWriter, r *http.Request) {
	// Snapshot config under RLock, then release before I/O.
	s.mu.RLock()
	src := s.cfg.EffectiveGitRoot()
	s.mu.RUnlock()

	if !git.IsRepo(src) {
		writeError(w, http.StatusBadRequest, "source directory is not a git repository")
		return
	}

	// Optional: fetch from remote first to discover new branches
	if r.URL.Query().Get("fetch") == "true" && git.HasRemote(src) {
		_ = git.FetchWithEnv(src, git.AuthEnvForRepo(src))
	}

	resp := gitBranchesResponse{
		Local:      make([]string, 0),
		Remote:     make([]string, 0),
		DirtyFiles: make([]string, 0),
	}

	if branch, err := git.GetCurrentBranch(src); err == nil {
		resp.Current = branch
	}

	if local, err := git.ListLocalBranches(src); err == nil && len(local) > 0 {
		resp.Local = local
	}

	if remote, err := git.ListRemoteBranches(src); err == nil && len(remote) > 0 {
		resp.Remote = remote
	}

	if dirty, err := git.IsDirty(src); err == nil {
		resp.IsDirty = dirty
	}

	if resp.IsDirty {
		if files, err := git.GetDirtyFiles(src); err == nil && len(files) > 0 {
			resp.DirtyFiles = files
		}
	}

	writeJSON(w, resp)
}

type checkoutRequest struct {
	Branch string `json:"branch"`
}

type checkoutResponse struct {
	Success bool   `json:"success"`
	Branch  string `json:"branch"`
	Message string `json:"message"`
}

// handleGitCheckout switches to a different branch
func (s *Server) handleGitCheckout(w http.ResponseWriter, r *http.Request) {
	start := time.Now()
	s.mu.Lock()
	defer s.mu.Unlock()

	var body checkoutRequest
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON body")
		return
	}
	if body.Branch == "" {
		writeError(w, http.StatusBadRequest, "branch is required")
		return
	}

	src := s.cfg.EffectiveGitRoot()

	if !git.IsRepo(src) {
		writeError(w, http.StatusBadRequest, "source directory is not a git repository")
		return
	}

	// Dirty check
	dirty, err := git.IsDirty(src)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to check git status: "+err.Error())
		return
	}
	if dirty {
		files, _ := git.GetDirtyFiles(src)
		resp := map[string]any{
			"error":      "working tree has uncommitted changes — commit or stash before switching branches",
			"isDirty":    true,
			"dirtyFiles": files,
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(resp)
		return
	}

	// Fetch before checkout to ensure remote refs are up to date
	if git.HasRemote(src) {
		_ = git.FetchWithEnv(src, git.AuthEnvForRepo(src))
	}

	// Checkout
	if err := git.Checkout(src, body.Branch); err != nil {
		s.writeOpsLog("checkout", "error", start, map[string]any{
			"branch": body.Branch,
			"scope":  "ui",
		}, err.Error())
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	s.writeOpsLog("checkout", "ok", start, map[string]any{
		"branch": body.Branch,
		"scope":  "ui",
	}, "")

	writeJSON(w, checkoutResponse{
		Success: true,
		Branch:  body.Branch,
		Message: "switched to branch " + body.Branch,
	})
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

// handleGitCommit stages and commits changes without pushing.
func (s *Server) handleGitCommit(w http.ResponseWriter, r *http.Request) {
	start := time.Now()
	s.mu.Lock()
	defer s.mu.Unlock()

	var body pushRequest
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON body")
		return
	}

	src := s.cfg.EffectiveGitRoot()

	if !git.IsRepo(src) {
		writeError(w, http.StatusBadRequest, "source directory is not a git repository")
		return
	}

	s.protectRootScopeConfig(src)

	status, err := git.GetStatus(src)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to get git status: "+err.Error())
		return
	}
	if status == "" {
		s.writeOpsLog("commit", "ok", start, map[string]any{
			"summary": "nothing to commit",
			"dry_run": body.DryRun,
			"scope":   "ui",
		}, "")
		writeJSON(w, pushResponse{Success: true, Message: "nothing to commit (working tree clean)", DryRun: body.DryRun})
		return
	}

	if body.DryRun {
		s.writeOpsLog("commit", "ok", start, map[string]any{
			"summary": "dry run",
			"dry_run": true,
			"scope":   "ui",
		}, "")
		writeJSON(w, pushResponse{Success: true, Message: "dry run: would stage and commit changes", DryRun: true})
		return
	}

	if err := git.StageAll(src); err != nil {
		writeError(w, http.StatusInternalServerError, "failed to stage changes: "+err.Error())
		return
	}

	msg := body.Message
	if msg == "" {
		msg = "Update skills"
	}
	if err := git.Commit(src, msg); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	s.writeOpsLog("commit", "ok", start, map[string]any{
		"message": msg,
		"dry_run": false,
		"scope":   "ui",
	}, "")

	writeJSON(w, pushResponse{Success: true, Message: "committed successfully"})
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

	src := s.cfg.EffectiveGitRoot()

	if !git.IsRepo(src) {
		writeError(w, http.StatusBadRequest, "source directory is not a git repository")
		return
	}
	if !git.HasRemote(src) {
		writeError(w, http.StatusBadRequest, "no git remote configured")
		return
	}

	s.protectRootScopeConfig(src)

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

// protectRootScopeConfig keeps config.yaml out of a root-scope repo before
// staging (server-side parity with the CLI push sweep). No-op off the root
// scope; best-effort so a failure never blocks the commit/push.
func (s *Server) protectRootScopeConfig(src string) {
	if s.cfg.GitRoot != "root" {
		return
	}
	_, _ = git.EnsureConfigUntracked(src)
}

type absorbNestedRequest struct {
	Subdirs []string `json:"subdirs"`
}

// handleAbsorbNested disables nested git repos under the root scope by renaming
// each <sub>/.git to <sub>/.git.disabled, so the directory's files get tracked
// normally instead of as an empty submodule. Global mode + root scope only; each
// subdir must match a currently-detected nested repo (this also blocks path
// traversal).
func (s *Server) handleAbsorbNested(w http.ResponseWriter, r *http.Request) {
	start := time.Now()

	if s.IsProjectMode() {
		writeError(w, http.StatusBadRequest, "git_root is only available in global mode")
		return
	}

	var body absorbNestedRequest
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON body")
		return
	}
	if len(body.Subdirs) == 0 {
		writeError(w, http.StatusBadRequest, "no subdirectories specified")
		return
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	if s.cfg.GitRoot != "root" {
		writeError(w, http.StatusBadRequest, "nested repos can only be disabled at the root scope")
		return
	}
	src := s.cfg.EffectiveGitRoot()

	allowed := make(map[string]bool)
	if nested, err := git.NestedRepos(src); err == nil {
		for _, n := range nested {
			allowed[n] = true
		}
	}

	disabled := make([]string, 0, len(body.Subdirs))
	for _, sub := range body.Subdirs {
		if !allowed[sub] {
			writeError(w, http.StatusBadRequest, "not a detected nested repository: "+sub)
			return
		}
		if err := git.DisableNestedRepo(src, sub); err != nil {
			writeError(w, http.StatusInternalServerError, "failed to disable "+sub+": "+err.Error())
			return
		}
		disabled = append(disabled, sub)
	}

	s.writeOpsLog("git-absorb-nested", "ok", start, map[string]any{"subdirs": disabled}, "")

	writeJSON(w, map[string]any{"success": true, "disabled": disabled})
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

	src := s.cfg.EffectiveGitRoot()

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
		allSkills, discoverErr := ssync.DiscoverSourceSkills(src)

		for name, target := range s.cfg.Targets {
			sc := target.SkillsConfig()
			mode := sc.Mode
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
				mergeResult, err := ssync.SyncTargetMergeWithSkills(name, target, allSkills, src, false, false, s.projectRoot)
				if err == nil {
					res.Linked = mergeResult.Linked
					res.Updated = mergeResult.Updated
					res.Skipped = mergeResult.Skipped
				}
				pruneResult, err := ssync.PruneOrphanLinksWithSkills(ssync.PruneOptions{
					TargetPath: sc.Path, SourcePath: src, Skills: allSkills,
					Include: sc.Include, Exclude: sc.Exclude, TargetNaming: sc.TargetNaming, TargetName: name,
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
				pruneResult, err := ssync.PruneOrphanCopiesWithSkills(sc.Path, allSkills, sc.Include, sc.Exclude, name, sc.TargetNaming, false)
				if err == nil {
					res.Pruned = pruneResult.Removed
				}
			default:
				ssync.SyncTarget(name, target, src, false, s.projectRoot)
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
