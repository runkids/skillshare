package server

import (
	"errors"
	"fmt"
	"log"
	"net/http"
	"path"
	"path/filepath"
	"strings"
	"time"

	"skillshare/internal/config"
	"skillshare/internal/install"
	"skillshare/internal/sourcewalk"
	"skillshare/internal/sync"
	"skillshare/internal/uninstall"
	"skillshare/internal/utils"
)

type batchUninstallRequest struct {
	Names []string `json:"names"`
	Kind  string   `json:"kind,omitempty"`
	Force bool     `json:"force"`
}

type batchUninstallItemResult struct {
	Name         string `json:"name"`
	Kind         string `json:"kind,omitempty"`
	Success      bool   `json:"success"`
	MovedToTrash bool   `json:"movedToTrash,omitempty"`
	Error        string `json:"error,omitempty"`
}

type batchUninstallSummary struct {
	Succeeded int `json:"succeeded"`
	Failed    int `json:"failed"`
}

func (s *Server) handleBatchUninstall(w http.ResponseWriter, r *http.Request) {
	start := time.Now()
	s.mu.Lock()
	defer s.mu.Unlock()

	var body batchUninstallRequest
	if err := decodeJSON(w, r, &body, defaultJSONBodyLimit); err != nil {
		if !errors.Is(err, errBodyTooLarge) {
			writeError(w, http.StatusBadRequest, "invalid JSON body")
		}
		return
	}
	if len(body.Names) == 0 {
		writeError(w, http.StatusBadRequest, "names array is required and must not be empty")
		return
	}
	if body.Kind != "" && body.Kind != "skill" && body.Kind != "agent" {
		writeError(w, http.StatusBadRequest, "invalid kind: "+body.Kind)
		return
	}

	// Agent-mode batch uninstall
	if body.Kind == "agent" {
		s.handleBatchUninstallAgents(w, body, start)
		return
	}

	// Skill-mode (default) batch uninstall
	s.handleBatchUninstallSkills(w, body, start)
}

func (s *Server) handleBatchUninstallAgents(w http.ResponseWriter, body batchUninstallRequest, start time.Time) {
	agentsSource := s.agentsSource()
	if agentsSource == "" {
		writeError(w, http.StatusInternalServerError, "agents source not configured")
		return
	}

	results := make([]batchUninstallItemResult, len(body.Names))
	var agents []uninstall.Agent
	var slots []int // results index of each resolved agent
	for i, name := range body.Names {
		results[i] = batchUninstallItemResult{Name: name, Kind: "agent"}
		agent, err := resolveAgentResource(agentsSource, name)
		if err != nil {
			results[i].Error = "agent not found: " + name
			continue
		}
		agents = append(agents, uninstall.Agent{Name: agentMetaKey(agent.RelPath), File: agent.SourcePath})
		slots = append(slots, i)
	}

	for j, err := range s.uninstallAgents(agentsSource, agents) {
		res := &results[slots[j]]
		if err != nil {
			res.Error = fmt.Sprintf("failed to trash agent: %v", err)
			continue
		}
		res.Success, res.MovedToTrash = true, true
	}

	summary, status, firstErr := summarizeBatchUninstall(results)
	s.writeOpsLog("uninstall", status, start, map[string]any{
		"names": body.Names,
		"kind":  "agent",
		"scope": "ui",
		"count": summary.Succeeded,
	}, firstErr)

	writeJSON(w, map[string]any{"results": results, "summary": summary})
}

func (s *Server) handleBatchUninstallSkills(w http.ResponseWriter, body batchUninstallRequest, start time.Time) {
	// Use DiscoverSourceSkillsAll (not DiscoverSourceSkills) so disabled skills
	// — those listed in .skillignore — are also resolvable. The list handler
	// shows disabled skills, so uninstall must be able to find them too (#190).
	source := s.skillsSource()
	walk := s.skillsWalk()
	discovered, err := sync.DiscoverSourceSkillsAll(source, walk)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to discover skills: "+err.Error())
		return
	}

	results := make([]batchUninstallItemResult, len(body.Names))
	var items []uninstall.Item
	var slots []int // results index of each resolved item
	for i, name := range body.Names {
		results[i] = batchUninstallItemResult{Name: name, Kind: "skill"}
		item, err := s.resolveBatchUninstallItem(discovered, source, walk, name)
		if err != nil {
			results[i].Error = err.Error()
			continue
		}
		items = append(items, item)
		slots = append(slots, i)
	}

	for j, r := range s.uninstallSkills(items, body.Force) {
		res := &results[slots[j]]
		if r.Err != nil {
			res.Error = uninstallErrorMessage(r)
			continue
		}
		res.Success, res.MovedToTrash = true, true
	}

	summary, status, firstErr := summarizeBatchUninstall(results)
	if summary.Succeeded > 0 {
		s.reconcileSkillsConfig(s.cfg.EffectiveSkillsSource())
	}
	s.writeOpsLog("uninstall", status, start, map[string]any{
		"names": body.Names,
		"force": body.Force,
		"scope": "ui",
		"count": summary.Succeeded,
	}, firstErr)

	writeJSON(w, map[string]any{"results": results, "summary": summary})
}

// summarizeBatchUninstall counts the results and derives the oplog status and
// message from them.
func summarizeBatchUninstall(results []batchUninstallItemResult) (summary batchUninstallSummary, status, firstErr string) {
	for _, res := range results {
		if res.Success {
			summary.Succeeded++
			continue
		}
		summary.Failed++
		if firstErr == "" {
			firstErr = res.Error
		}
	}
	status = "ok"
	if summary.Failed > 0 && summary.Succeeded > 0 {
		status = "partial"
	} else if summary.Failed > 0 {
		status = "error"
	}
	return summary, status, firstErr
}

// resolveBatchUninstallItem resolves a flat name from the dashboard to a
// tracked repo or a skill.
func (s *Server) resolveBatchUninstallItem(discovered []sync.DiscoveredSkill, source string, walk sourcewalk.Options, name string) (uninstall.Item, error) {
	if !validSourceName(name) {
		return uninstall.Item{}, errors.New("invalid skill name: " + name)
	}

	repoPath := filepath.Join(source, name)
	_, repoFollowed := walk.Follow.Resolve(repoPath)
	// A repo installed with --into is named by its path, e.g. org/_team. --into
	// folders cannot start with "_" (validate.IntoPath), so a deeper _ git dir
	// such as _team/vendor/_lib is part of a tracked repo, not a repo of its own.
	parent, base := path.Split(name)
	repoName := utils.IsTrackedRepoDir(base) && !strings.Contains("/"+parent, "/_")
	managedRepo := repoName && !repoFollowed && install.IsGitRepo(repoPath)
	var skill *sync.DiscoveredSkill
	if !managedRepo {
		var err error
		if skill, err = resolveUninstallSkill(discovered, name); err != nil {
			return uninstall.Item{}, err
		}
	}
	followed := false
	if skill != nil {
		_, followed = walk.Follow.Resolve(skill.SourcePath)
	}
	// An explicit managed repo takes precedence over a skill's basename.
	if managedRepo || (repoName && !followed) {
		if !install.IsGitRepo(repoPath) {
			return uninstall.Item{}, errors.New("not a tracked repository: " + name)
		}
		return s.repoUninstallItem(name, repoPath), nil
	}
	if skill == nil {
		return uninstall.Item{}, errors.New("skill not found: " + name)
	}
	if skill.IsInRepo && !followed {
		return uninstall.Item{}, errors.New("skill is inside a tracked repo; uninstall the repo instead")
	}
	return skillUninstallItem(skill, followed), nil
}

// repoUninstallItem describes a tracked repo by its source-relative name.
func (s *Server) repoUninstallItem(name, repoPath string) uninstall.Item {
	entry := name
	if s.IsProjectMode() {
		entry = s.projectGitignorePrefix() + "/" + name
	}
	return uninstall.Item{Name: name, Path: repoPath, TrashName: name, Repo: true, Gitignore: entry}
}

// skillUninstallItem describes a discovered skill. A skill below a followed
// source link keeps its logical path in the trash so restore goes back through
// the same link.
func skillUninstallItem(skill *sync.DiscoveredSkill, followed bool) uninstall.Item {
	trashName := filepath.Base(skill.SourcePath)
	if followed {
		trashName = skill.RelPath
	}
	return uninstall.Item{Name: skill.RelPath, Path: skill.SourcePath, TrashName: trashName}
}

// uninstallSkills runs the shared uninstall for resolved skills and repos.
func (s *Server) uninstallSkills(items []uninstall.Item, force bool) []uninstall.Result {
	out := uninstall.Run(items, uninstall.Options{
		SourceDir:    s.skillsSource(),
		Follow:       s.skillsWalk().Follow,
		TrashDir:     s.trashBase(),
		Store:        s.skillsStore,
		GitignoreDir: s.gitignoreDir(),
		Force:        force,
	})
	if out.GitignoreErr != nil {
		log.Printf("warning: failed to clean .gitignore: %v", out.GitignoreErr)
	}
	if out.SaveErr != nil {
		log.Printf("warning: failed to save metadata: %v", out.SaveErr)
	}
	if s.IsProjectMode() {
		var removed []string
		for _, r := range out.Results {
			if r.Err == nil {
				removed = append(removed, r.Item.Name)
			}
		}
		if err := config.ForgetProjectSkills(s.projectRoot, s.projectCfg, removed); err != nil {
			log.Printf("warning: failed to update project config after uninstall: %v", err)
		}
	}
	return out.Results
}

// uninstallAgents runs the shared agent uninstall; errs follows agents.
func (s *Server) uninstallAgents(agentsSource string, agents []uninstall.Agent) []error {
	errs, saveErr := uninstall.Agents(agents, agentsSource, s.agentTrashBase(), s.agentsStore)
	if saveErr != nil {
		log.Printf("warning: failed to save agent metadata after uninstall: %v", saveErr)
	}
	return errs
}

// uninstallErrorStatus is the HTTP status of a failed single uninstall: a
// refusal is a conflict, a failed move is the server's fault.
func uninstallErrorStatus(err error) int {
	var trashErr *uninstall.TrashError
	if errors.As(err, &trashErr) {
		return http.StatusInternalServerError
	}
	return http.StatusConflict
}

// uninstallForceCode names the refusals force overrides, so the dashboard
// offers the override for these and nothing else; "" for any other error.
func uninstallForceCode(err error) string {
	var statusErr *uninstall.StatusError
	switch {
	case errors.Is(err, uninstall.ErrDirty):
		return "repo_dirty"
	case errors.As(err, &statusErr):
		return "repo_status_failed"
	}
	return ""
}

// writeUninstallConflict preserves the dashboard's existing refusal codes.
// A restored checkout requires refresh/reselection, never a force retry.
func writeUninstallConflict(w http.ResponseWriter, res uninstall.Result) bool {
	code := uninstallForceCode(res.Err)
	if errors.Is(res.Err, uninstall.ErrCheckoutRestored) {
		code = "repo_checkout_restored"
	}
	if code == "" {
		return false
	}
	writeCodedError(w, http.StatusConflict, code, uninstallErrorMessage(res), nil)
	return true
}

// uninstallErrorMessage words a failed result for the dashboard.
func uninstallErrorMessage(r uninstall.Result) string {
	var trashErr *uninstall.TrashError
	switch {
	case errors.Is(r.Err, uninstall.ErrDirty):
		return "uncommitted changes (use force to override)"
	case errors.As(r.Err, &trashErr):
		if r.Item.Repo {
			return fmt.Sprintf("failed to trash repo: %v", trashErr.Err)
		}
		return fmt.Sprintf("failed to trash skill: %v", trashErr.Err)
	}
	return r.Err.Error()
}

// ponytail: scan per requested name; index discovery if large batches become slow.
func resolveUninstallSkill(discovered []sync.DiscoveredSkill, name string) (*sync.DiscoveredSkill, error) {
	for i := range discovered {
		if discovered[i].FlatName == name {
			return &discovered[i], nil
		}
	}
	var match *sync.DiscoveredSkill
	var candidates []string
	for i := range discovered {
		if filepath.Base(discovered[i].SourcePath) == name {
			match = &discovered[i]
			candidates = append(candidates, match.FlatName)
		}
	}
	if len(candidates) > 1 {
		return nil, fmt.Errorf("ambiguous skill name %q: use one of %s", name, strings.Join(candidates, ", "))
	}
	return match, nil
}
