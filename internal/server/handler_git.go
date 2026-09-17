package server

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"

	"skillshare/internal/config"
	"skillshare/internal/git"
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
	// Ahead counts commits no remote-tracking branch has yet, i.e. what a push uploads.
	Ahead int `json:"ahead"`
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
	src, ok := s.gitSource(w)
	if !ok {
		s.mu.RUnlock()
		return
	}
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

	if resp.HasRemote {
		resp.Ahead = git.AheadCount(src)
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
	src, ok := s.gitSource(w)
	s.mu.RUnlock()
	if !ok {
		return
	}

	if !git.IsRepo(src) {
		writeError(w, http.StatusBadRequest, "source directory is not a git repository")
		return
	}

	// Optional: fetch from remote first to discover new branches
	if r.URL.Query().Get("fetch") == "true" && git.HasRemote(src) {
		if err := git.FetchWithEnv(src, git.AuthEnvForRepo(src)); err != nil {
			writeError(w, http.StatusInternalServerError, "git fetch failed: "+err.Error())
			return
		}
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

	src, ok := s.gitSource(w)
	if !ok {
		return
	}

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
		if err := git.FetchWithEnv(src, git.AuthEnvForRepo(src)); err != nil {
			s.writeOpsLog("checkout", "error", start, map[string]any{
				"branch": body.Branch,
				"scope":  "ui",
			}, err.Error())
			writeError(w, http.StatusInternalServerError, "git fetch failed: "+err.Error())
			return
		}
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

	src, ok := s.gitSource(w)
	if !ok {
		return
	}

	if !git.IsRepo(src) {
		writeError(w, http.StatusBadRequest, "source directory is not a git repository")
		return
	}

	if s.rootScopeGuard(w, src, body.DryRun) {
		return
	}

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

// handlePush commits pending changes, then pushes every commit the remote
// does not have yet. The first push sets upstream.
func (s *Server) handlePush(w http.ResponseWriter, r *http.Request) {
	start := time.Now()
	s.mu.Lock()
	defer s.mu.Unlock()

	var body pushRequest
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON body")
		return
	}

	src, ok := s.gitSource(w)
	if !ok {
		return
	}

	if !git.IsRepo(src) {
		writeError(w, http.StatusBadRequest, "source directory is not a git repository")
		return
	}
	if !git.HasRemote(src) {
		writeError(w, http.StatusBadRequest, "no git remote configured")
		return
	}

	if s.rootScopeGuard(w, src, body.DryRun) {
		return
	}

	status, err := git.GetStatus(src)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to get git status: "+err.Error())
		return
	}
	ahead := git.AheadCount(src)
	if status == "" && ahead == 0 {
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
		msg := "dry run: would stage, commit, and push changes"
		if status == "" {
			msg = fmt.Sprintf("dry run: would push %d commit(s)", ahead)
		}
		writeJSON(w, pushResponse{Success: true, Message: msg, DryRun: true})
		return
	}

	msg := body.Message
	if msg == "" {
		msg = "Update skills"
	}
	args := map[string]any{"message": msg, "dry_run": false, "scope": "ui"}
	fail := func(err error) {
		s.writeOpsLog("push", "error", start, args, err.Error())
		writeError(w, http.StatusInternalServerError, err.Error())
	}

	if status != "" {
		if err := git.StageAll(src); err != nil {
			fail(fmt.Errorf("failed to stage changes: %w", err))
			return
		}
		if err := git.Commit(src, msg); err != nil {
			fail(err)
			return
		}
	} else {
		args["message"] = "" // nothing new was committed
	}

	if err := git.PushRemoteWithAuth(src); err != nil {
		fail(err)
		return
	}

	s.writeOpsLog("push", "ok", start, args, "")
	writeJSON(w, pushResponse{Success: true, Message: "pushed successfully"})
}

// gitSource validates the configured git_root scope and returns the directory
// git operations run on. On an invalid scope it writes a 400 and returns
// ok=false — mirroring the CLI's resolveGitRoot, so a hand-edited bad git_root
// can't silently fall back to the skills scope (ScopeDir's default). Callers
// must hold at least s.mu.RLock.
func (s *Server) gitSource(w http.ResponseWriter) (string, bool) {
	if !config.ValidGitRoot(s.cfg.GitRoot) {
		writeError(w, http.StatusBadRequest,
			"invalid git_root \""+s.cfg.GitRoot+"\" (valid: "+strings.Join(config.ValidGitRoots, ", ")+")")
		return "", false
	}
	return s.cfg.EffectiveGitRoot(), true
}

// rootScopeGuard enforces root-scope safety before staging (server-side parity
// with the CLI push sweep). It blocks the request with a 400 when nested git
// repos would upload as empty submodules, and keeps config.yaml untracked. The
// config mutation runs only on a real request — on dry-run the guard is
// strictly read-only so a preview never writes .gitignore or rewrites the
// index. No-op off the root scope. Returns blocked=true once it has written an
// error response.
func (s *Server) rootScopeGuard(w http.ResponseWriter, src string, dryRun bool) (blocked bool) {
	if s.cfg.GitRoot != "root" {
		return false
	}
	if nested, err := git.NestedRepos(src); err == nil && len(nested) > 0 {
		writeError(w, http.StatusBadRequest,
			"nested git repositories must be disabled before committing (they upload as empty submodules): "+strings.Join(nested, ", "))
		return true
	}
	if !dryRun {
		_, _ = git.EnsureConfigUntracked(src)
	}
	return false
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
	Warnings    []string           `json:"warnings,omitempty"`
}

// handlePull pulls changes and syncs to targets
func (s *Server) handlePull(w http.ResponseWriter, r *http.Request) {
	start := time.Now()
	s.mu.Lock()
	defer s.mu.Unlock()

	var body struct {
		DryRun bool `json:"dryRun"`
		// Force replaces local files with the remote on a first pull instead of merging.
		Force bool `json:"force"`
	}
	json.NewDecoder(r.Body).Decode(&body)

	src, ok := s.gitSource(w)
	if !ok {
		return
	}

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

	// A branch without upstream (e.g. a repo created here with a remote added
	// later) is attached to the remote default branch first, like the CLI.
	var info *git.UpdateInfo
	if git.HasUpstream(src) {
		info, err = git.PullWithAuth(src)
	} else {
		info, err = git.FirstPull(src, body.Force)
	}
	if errors.Is(err, git.ErrNoRemoteBranches) {
		writeError(w, http.StatusBadRequest, "the remote has no branches yet; push first")
		return
	}
	if err != nil {
		s.writeOpsLog("pull", "error", start, map[string]any{"dry_run": false, "force": body.Force, "scope": "ui"}, err.Error())
		if errors.Is(err, git.ErrMergeFailed) {
			// The UI offers a force pull for this code.
			writeCodedError(w, http.StatusConflict, "merge_failed", "git pull failed: "+err.Error(), nil)
			return
		}
		writeError(w, http.StatusInternalServerError, "git pull failed: "+err.Error())
		return
	}

	resp := pullResponse{
		Success:     true,
		UpToDate:    info.UpToDate,
		Commits:     info.Commits,
		Stats:       info.Stats,
		SyncResults: make([]syncTargetResult, 0),
	}
	if resp.Commits == nil {
		resp.Commits = make([]git.CommitInfo, 0)
	}

	// Sync what the pulled scope holds, as the CLI does. Skills always sync
	// from the skills source, whatever directory git_root points at.
	switch scope := s.cfg.GitRoot; {
	case info.UpToDate:
	case scope == "extras":
		for _, extra := range s.syncExtras("", false, false) {
			for _, t := range extra.Targets {
				for _, msg := range append([]string{t.Error}, t.Errors...) {
					if msg != "" {
						resp.Warnings = append(resp.Warnings, fmt.Sprintf("extras sync failed for %s (%s): %s", extra.Name, t.Target, msg))
					}
				}
			}
		}
	default:
		kind := "" // root holds both
		if scope == "" || scope == "skills" {
			kind = kindSkill
		} else if scope == "agents" {
			kind = kindAgent
		}
		if out, _, err := s.syncResources(start, false, false, kind); err != nil {
			resp.Warnings = append(resp.Warnings, "sync after pull failed: "+err.Error())
		} else {
			resp.SyncResults = out.results
			resp.Warnings = append(resp.Warnings, out.warnings...)
		}
	}

	s.writeOpsLog("pull", "ok", start, map[string]any{
		"dry_run":      false,
		"up_to_date":   resp.UpToDate,
		"commits":      len(resp.Commits),
		"force":        body.Force,
		"targets_sync": len(resp.SyncResults),
		"scope":        "ui",
	}, "")

	writeJSON(w, resp)
}
