package server

import (
	"errors"
	"fmt"
	"log"
	"net/http"
	"os"
	"path"
	"path/filepath"
	"strings"
	"time"

	"skillshare/internal/audit"
	"skillshare/internal/config"
	"skillshare/internal/install"
	"skillshare/internal/sourcewalk"
	"skillshare/internal/update"
	"skillshare/internal/utils"
)

type updateRequest struct {
	Name      string `json:"name"`
	Kind      string `json:"kind,omitempty"`
	Force     bool   `json:"force"`
	All       bool   `json:"all"`
	SkipAudit bool   `json:"skipAudit"`
}

type updateResultItem struct {
	Name           string `json:"name"`
	Action         string `json:"action"` // "updated", "up-to-date", "skipped", "error", "blocked"
	Message        string `json:"message,omitempty"`
	IsRepo         bool   `json:"isRepo"`
	AuditRiskScore int    `json:"auditRiskScore,omitempty"`
	AuditRiskLabel string `json:"auditRiskLabel,omitempty"`
	Kind           string `json:"kind,omitempty"`
}

func (s *Server) updateAuditThreshold() string {
	if s.IsProjectMode() && s.projectCfg != nil {
		if threshold, err := audit.NormalizeThreshold(s.projectCfg.Audit.BlockThreshold); err == nil {
			return threshold
		}
		return audit.DefaultThreshold()
	}
	if s.cfg != nil {
		if threshold, err := audit.NormalizeThreshold(s.cfg.Audit.BlockThreshold); err == nil {
			return threshold
		}
	}
	return audit.DefaultThreshold()
}

func (s *Server) handleUpdate(w http.ResponseWriter, r *http.Request) {
	start := time.Now()
	s.mu.Lock()
	defer s.mu.Unlock()
	defer s.trackProjectLock()()

	var body updateRequest
	if err := decodeJSON(w, r, &body, defaultJSONBodyLimit); err != nil {
		if !errors.Is(err, errBodyTooLarge) {
			writeError(w, http.StatusBadRequest, "invalid JSON body")
		}
		return
	}

	if body.All {
		results := s.updateAll(body.Force, body.SkipAudit)
		total := len(results)
		failed := 0
		blocked := 0
		for _, item := range results {
			if item.Action == "error" {
				failed++
			} else if item.Action == "blocked" {
				blocked++
			}
		}
		status := "ok"
		msg := ""
		if blocked > 0 {
			status = "partial"
			msg = fmt.Sprintf("%d update(s) blocked by security audit", blocked)
		}
		if failed > 0 {
			status = "partial"
			if msg != "" {
				msg += fmt.Sprintf(", %d update(s) failed", failed)
			} else {
				msg = fmt.Sprintf("%d update(s) failed", failed)
			}
		}
		s.writeOpsLog("update", status, start, map[string]any{
			"name":            "--all",
			"force":           body.Force,
			"skip_audit":      body.SkipAudit,
			"results_total":   total,
			"results_failed":  failed,
			"results_blocked": blocked,
			"scope":           "ui",
		}, msg)
		writeJSON(w, map[string]any{"results": results})
		return
	}

	if body.Name == "" {
		writeError(w, http.StatusBadRequest, "name is required (or use all: true)")
		return
	}
	if body.Kind != "" && body.Kind != "skill" && body.Kind != "agent" {
		writeError(w, http.StatusBadRequest, "invalid kind: "+body.Kind)
		return
	}
	if body.Kind != "agent" && !validSourceName(body.Name) {
		writeError(w, http.StatusBadRequest, "invalid skill name: "+body.Name)
		return
	}

	result := s.updateSingleByKind(body.Name, body.Kind, body.Force, body.SkipAudit)
	status := "ok"
	msg := ""
	if result.Action == "error" {
		status = "error"
		msg = result.Message
	} else if result.Action == "blocked" {
		status = "error"
		msg = result.Message
	} else if result.Action == "skipped" {
		status = "partial"
		msg = result.Message
	}
	s.writeOpsLog("update", status, start, map[string]any{
		"name":       body.Name,
		"force":      body.Force,
		"skip_audit": body.SkipAudit,
		"scope":      "ui",
	}, msg)
	writeJSON(w, map[string]any{"results": []updateResultItem{result}})
}

func (s *Server) updateSingle(name string, force, skipAudit bool) updateResultItem {
	return s.updateSingleByKind(name, "", force, skipAudit)
}

func (s *Server) updateSingleByKind(name, kind string, force, skipAudit bool) updateResultItem {
	name = path.Clean(filepath.ToSlash(name)) // "org/team/" names the same item as "org/team"
	if kind == "agent" {
		return s.updateAgent(name, force, skipAudit)
	}
	walk := s.skillsWalk()
	// Try exact skill path first (prevents basename collision with nested repos)
	skillPath := filepath.Join(s.cfg.EffectiveSkillsSource(), name)
	if refusal := s.refuseFollowedCheckout(name, skillPath, []*sourcewalk.Follow{walk.Follow}); refusal != nil {
		return *refusal
	}
	// A tracked repo's own entry has a Source too; a reinstall would drop its
	// tracked state, so checkouts go to the tracked-repo update. A tracked repo
	// whose checkout is gone is not reinstalled, nor matched by basename.
	if entry := s.skillsStore.GetByPath(name); entry != nil && entry.Source != "" && !install.IsTrackedCheckout(skillPath) {
		if entry.Tracked && utils.IsTrackedRepoDir(filepath.Base(name)) {
			return updateResultItem{Name: name, Action: "error", Message: fmt.Sprintf("tracked repo '%s' is declared in metadata but missing on disk", name)}
		}
		return s.updateRegularSkill(name, skillPath, force, skipAudit, walk.Follow)
	}

	// Try tracked repo (flat, nested, or basename fallback)
	repoName, repoPath, err := s.resolveTrackedRepo(name, walk)
	if err != nil {
		return updateResultItem{Name: name, Action: "error", Message: err.Error()}
	}
	if repoPath != "" {
		return s.updateTrackedRepo(repoName, repoPath, s.cfg.EffectiveSkillsSource(), force, skipAudit, walk.Follow)
	}

	return updateResultItem{
		Name:    name,
		Action:  "error",
		Message: fmt.Sprintf("'%s' not found as tracked repo or updatable skill", name),
	}
}

func (s *Server) updateAgent(name string, force, skipAudit bool) updateResultItem {
	agentsSource := s.agentsSource()
	if agentsSource == "" {
		return updateResultItem{Name: name, Kind: "agent", Action: "error", Message: "agents source is not configured"}
	}

	// The dashboard names an agent repo by its folder.
	if validSourceName(name) {
		if repoPath := filepath.Join(agentsSource, filepath.FromSlash(name)); install.IsTrackedCheckout(repoPath) {
			item := s.updateTrackedRepo(name, repoPath, agentsSource, force, skipAudit)
			item.Kind = "agent"
			return item
		}
	}

	localAgent, err := resolveAgentResource(agentsSource, name)
	if err != nil {
		return updateResultItem{Name: name, Kind: "agent", Action: "error", Message: err.Error()}
	}

	if localAgent.RepoRelPath != "" {
		repoPath := filepath.Join(agentsSource, filepath.FromSlash(localAgent.RepoRelPath))
		item := s.updateTrackedRepo(agentMetaKey(localAgent.RelPath), repoPath, agentsSource, force, skipAudit)
		item.Kind = "agent"
		return item
	}

	metaKey := agentMetaKey(localAgent.RelPath)
	entry := s.agentsStore.GetByPath(metaKey)
	if entry == nil || entry.Source == "" {
		return updateResultItem{
			Name:    metaKey,
			Kind:    "agent",
			Action:  "skipped",
			Message: "agent is local and has no update source",
		}
	}

	source, err := install.ParseSourceWithOptions(entry.Source, s.parseOpts())
	if err != nil {
		return updateResultItem{Name: metaKey, Kind: "agent", Action: "error", Message: "invalid source: " + err.Error()}
	}
	source.ApplyRecordedBranch(entry.Branch)

	repoSubdir := strings.TrimSuffix(source.Subdir, entry.Subdir)
	repoSubdir = strings.TrimRight(repoSubdir, "/")
	source.Subdir = repoSubdir

	var discovery *install.DiscoveryResult
	if source.HasSubdir() {
		discovery, err = install.DiscoverFromGitSubdir(source)
	} else {
		discovery, err = install.DiscoverFromGit(source)
	}
	if err != nil {
		return updateResultItem{Name: metaKey, Kind: "agent", Action: "error", Message: err.Error()}
	}
	defer install.CleanupDiscovery(discovery)

	if discovery.CommitHash != "" && discovery.CommitHash == entry.Version {
		return updateResultItem{Name: metaKey, Kind: "agent", Action: "up-to-date"}
	}

	var target *install.AgentInfo
	for i := range discovery.Agents {
		candidate := discovery.Agents[i]
		if candidate.Path == entry.Subdir ||
			candidate.FileName == filepath.Base(localAgent.RelPath) ||
			candidate.Name == filepath.Base(metaKey) {
			target = &discovery.Agents[i]
			break
		}
	}
	if target == nil {
		return updateResultItem{
			Name:    metaKey,
			Kind:    "agent",
			Action:  "error",
			Message: fmt.Sprintf("agent path %q not found in repository", entry.Subdir),
		}
	}

	destDir := agentsSource
	opts := install.InstallOptions{
		Kind:           "agent",
		Force:          force,
		Update:         true,
		SkipAudit:      skipAudit,
		AuditThreshold: s.updateAuditThreshold(),
		SourceDir:      agentsSource,
	}
	if s.IsProjectMode() {
		opts.AuditProjectRoot = s.projectRoot
	}
	res, err := install.UpdateAgentFromDiscovery(discovery, *target, destDir, opts)
	if err != nil {
		return updateResultItem{Name: metaKey, Kind: "agent", Action: "error", Message: err.Error()}
	}

	if st, loadErr := install.LoadMetadataWithMigration(agentsSource, install.MetadataKindAgent); loadErr == nil && st != nil {
		s.agentsStore = st
	}

	message := res.Action
	if message == "" {
		message = "updated"
	}
	return updateResultItem{
		Name:    metaKey,
		Kind:    "agent",
		Action:  "updated",
		Message: message,
	}
}

// updateTrackedRepo runs the shared tracked-repo update and reports it as a
// result item. sourceDir is the skills or agents source root that holds the
// repo's metadata and accepted findings, as the CLI passes it.
func (s *Server) updateTrackedRepo(name, repoPath, sourceDir string, force, skipAudit bool, follow ...*sourcewalk.Follow) updateResultItem {
	if refusal := s.refuseFollowedCheckout(name, repoPath, follow); refusal != nil {
		return *refusal
	}
	opts := update.TrackedRepoOptions{
		SourceDir: sourceDir,
		Force:     force,
		SkipAudit: skipAudit,
		Threshold: s.updateAuditThreshold(),
	}
	if len(follow) > 0 {
		opts.Follow = follow[0]
	}
	if s.IsProjectMode() {
		opts.ProjectRoot = s.projectRoot
	}
	res, err := update.TrackedRepo(repoPath, opts)

	item := updateResultItem{Name: name, IsRepo: true}
	var blocked *install.AuditGateError
	var opErr *update.Error
	switch {
	case errors.As(err, &blocked):
		item.Action = "blocked"
		item.Message = trackedBlockMessage(blocked)
		return item
	case errors.As(err, &opErr) && opErr.Stage == update.StagePull:
		item.Action = "error"
		item.Message = opErr.Err.Error()
		if !force {
			item.Message += " (try force update)"
		}
		return item
	case err != nil:
		item.Action = "error"
		item.Message = err.Error()
		return item
	}

	if res.MetadataErr != nil {
		log.Printf("warning: failed to refresh metadata for %s: %v", name, res.MetadataErr)
	}
	if res.MetadataChanged {
		if sourceDir == s.agentsSource() {
			if st, loadErr := install.LoadMetadataWithMigration(sourceDir, install.MetadataKindAgent); loadErr == nil {
				s.agentsStore = st
			}
		} else {
			s.reloadSkillsStore()
		}
	}
	switch res.Status {
	case update.StatusDirty:
		item.Action = "skipped"
		item.Message = "has uncommitted changes (use force to discard)"
	case update.StatusUpToDate:
		item.Action = "up-to-date"
	default:
		item.Action = "updated"
		item.Message = fmt.Sprintf("%d commits, %d files changed", len(res.Info.Commits), res.Info.Stats.FilesChanged)
		for _, w := range res.Warnings {
			item.Message += "; " + w
		}
		if res.RecordErr != nil {
			item.Message += "; failed to record accepted findings: " + res.RecordErr.Error()
		}
		if res.Audit != nil {
			item.AuditRiskScore = res.Audit.RiskScore
			item.AuditRiskLabel = res.Audit.RiskLabel
		}
	}
	return item
}

// trackedBlockMessage words a blocked update for the dashboard, which offers
// Force Retry only for messages containing "blocked by security audit".
func trackedBlockMessage(e *install.AuditGateError) string {
	switch {
	case e.Rollback == install.RollbackUnavailable:
		return "security audit failed (" + e.RollbackNote() + ")"
	case e.ScanErr != nil:
		return "security audit failed: " + e.ScanErr.Error() + " (" + e.RollbackNote() + ")"
	case e.Rollback == install.RolledBack:
		return fmt.Sprintf("blocked by security audit — findings at/above %s detected, rolled back", e.Threshold)
	}
	return fmt.Sprintf("blocked by security audit — findings at/above %s detected (%s)", e.Threshold, e.RollbackNote())
}

func (s *Server) updateRegularSkill(name, skillPath string, force, skipAudit bool, follow ...*sourcewalk.Follow) updateResultItem {
	if refusal := s.refuseFollowedCheckout(name, skillPath, follow); refusal != nil {
		return *refusal
	}
	entry := s.skillsStore.GetByPath(name)
	if entry == nil {
		return updateResultItem{Name: name, Action: "error", Message: "no metadata found"}
	}
	if !force && entry.LocallyModified(skillPath, follow...) {
		return updateResultItem{Name: name, Action: "skipped", Message: "has local changes since install (try force update)"}
	}
	source, err := install.ParseSourceWithOptions(entry.Source, s.parseOpts())
	if err != nil {
		return updateResultItem{
			Name:    name,
			Action:  "error",
			Message: "invalid source: " + err.Error(),
		}
	}
	source.ApplyRecordedBranch(entry.Branch)

	sourceDir := s.cfg.EffectiveSkillsSource()
	opts := install.InstallOptions{
		Force:          true,
		AuditOverride:  force,
		Reinstall:      force,
		Update:         true,
		SkipAudit:      skipAudit,
		AuditThreshold: s.updateAuditThreshold(),
		SourceDir:      sourceDir,
	}
	if len(follow) > 0 {
		opts.SourceFollow = follow[0]
	}
	if s.IsProjectMode() {
		opts.AuditProjectRoot = s.projectRoot
	}
	result, err := install.Install(source, skillPath, opts)
	if err != nil {
		return updateResultItem{
			Name:    name,
			Action:  "error",
			Message: err.Error(),
		}
	}

	if st, loadErr := install.LoadMetadataWithMigration(sourceDir, ""); loadErr == nil && st != nil {
		s.skillsStore = st
	}

	item := updateResultItem{
		Name:    name,
		Action:  "updated",
		Message: "reinstalled from source",
	}
	if result != nil && result.AuditRiskLabel != "" {
		item.AuditRiskScore = result.AuditRiskScore
		item.AuditRiskLabel = result.AuditRiskLabel
	}
	return item
}

func (s *Server) updateAll(force, skipAudit bool) []updateResultItem {
	var results []updateResultItem

	// Update tracked repos
	walk := s.skillsWalk()
	repos, err := install.GetTrackedRepos(s.cfg.EffectiveSkillsSource(), walk)
	if err == nil {
		for _, repo := range repos {
			repoPath := filepath.Join(s.cfg.EffectiveSkillsSource(), repo)
			results = append(results, s.updateTrackedRepo(repo, repoPath, s.cfg.EffectiveSkillsSource(), force, skipAudit, walk.Follow))
		}
	}

	// Update regular skills with source metadata
	skills, err := getServerUpdatableSkills(s.cfg.EffectiveSkillsSource(), s.skillsStore, walk)
	if err == nil {
		for _, skill := range skills {
			skillPath := filepath.Join(s.cfg.EffectiveSkillsSource(), skill)
			results = append(results, s.updateRegularSkill(skill, skillPath, force, skipAudit, walk.Follow))
		}
	}

	return results
}

// getServerUpdatableSkills returns relative paths of skills that have metadata with a remote source.
// It walks the source directory recursively to find nested skills (e.g. utils/ascii-box-check).
func getServerUpdatableSkills(sourceDir string, store *install.MetadataStore, walk sourcewalk.Options) ([]string, error) {
	var skills []string
	walkRoot := utils.ResolveSymlink(sourceDir)
	err := sourcewalk.WalkDir(walkRoot, walk, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		if path == walkRoot {
			return nil
		}
		if !d.IsDir() {
			return nil
		}
		name := d.Name()
		// Skip hidden directories and .git
		if name == ".git" || (len(name) > 0 && name[0] == '.') {
			return filepath.SkipDir
		}
		// Skip tracked repos (_ prefix with .git inside)
		if len(name) > 0 && name[0] == '_' {
			return filepath.SkipDir
		}
		// Check if this directory has updatable metadata
		relName := filepath.Base(path)
		if relP, relErr2 := filepath.Rel(walkRoot, path); relErr2 == nil {
			relName = filepath.ToSlash(relP)
		}
		entry := store.GetByPath(relName)
		if entry == nil || entry.Source == "" {
			return nil // continue walking into subdirectories
		}
		relPath, relErr := filepath.Rel(walkRoot, path)
		if relErr == nil {
			skills = append(skills, filepath.ToSlash(relPath))
		}
		return filepath.SkipDir // don't recurse into skill directories
	})
	if err != nil {
		return nil, err
	}
	return skills, nil
}

// trackProjectLock returns a function for defer: it moves the lockfile pins of
// the skills the request changed. Global mode has no lockfile.
func (s *Server) trackProjectLock() func() {
	if !s.IsProjectMode() {
		return func() {}
	}
	done := config.TrackProjectLock(s.projectRoot, s.projectCfg, s.cfg.EffectiveSkillsSource())
	return func() {
		if err := done(); err != nil {
			log.Printf("warning: failed to update %s: %v", install.LockFileName, err)
		}
	}
}
