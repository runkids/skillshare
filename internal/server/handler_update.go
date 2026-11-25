package server

import (
	"errors"
	"fmt"
	"log"
	"net/http"
	"os"
	"path"
	"path/filepath"
	"time"

	"skillshare/internal/audit"
	"skillshare/internal/config"
	"skillshare/internal/install"
	"skillshare/internal/sourcewalk"
	"skillshare/internal/update"
	"skillshare/internal/utils"
)

type updateRequest struct {
	Name  string `json:"name"`
	Kind  string `json:"kind,omitempty"`
	Force bool   `json:"force"`
	// Merge 3-way merges upstream changes into a skill or agent with local
	// edits instead of skipping it.
	Merge     bool `json:"merge"`
	All       bool `json:"all"`
	SkipAudit bool `json:"skipAudit"`
	// Branch reinstalls one skill from this branch of its recorded source.
	Branch string `json:"branch,omitempty"`
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

// locallyModifiedMessage is the reason a skill or agent edited since install is skipped;
// the dashboard offers Force Retry on its "(try force update)" hint.
const locallyModifiedMessage = "has local changes since install (try force update)"

// updateFailure reports a failed skill or agent update. A merge conflict keeps
// the force hint: discarding the local edits is the other way to update.
func updateFailure(name, kind string, err error) updateResultItem {
	message := err.Error()
	var conflict *install.MergeConflictError
	if errors.As(err, &conflict) {
		message += "; kept the local version (try force update)"
	}
	return updateResultItem{Name: name, Kind: kind, Action: "error", Message: message}
}

// updatedMessage describes a successful reinstall or merge.
func updatedMessage(result *install.InstallResult) string {
	if result != nil && result.Action == "merged" {
		return "merged local changes"
	}
	return "reinstalled from source"
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
	defer s.followRepairedSources()()

	var body updateRequest
	if err := decodeJSON(w, r, &body, defaultJSONBodyLimit); err != nil {
		if !errors.Is(err, errBodyTooLarge) {
			writeError(w, http.StatusBadRequest, "invalid JSON body")
		}
		return
	}

	if body.Force && body.Merge {
		writeError(w, http.StatusBadRequest, "force and merge cannot be combined")
		return
	}
	if body.Branch != "" && (body.All || body.Kind == "agent") {
		writeError(w, http.StatusBadRequest, "branch applies to a single skill only")
		return
	}

	if body.All {
		results := s.updateAll(body.Force, body.SkipAudit, body.Merge)
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
			"merge":           body.Merge,
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

	var result updateResultItem
	if body.Branch != "" {
		result = s.switchSkillBranch(body.Name, body.Branch, body.Force, body.SkipAudit)
	} else {
		result = s.updateSingleByKind(body.Name, body.Kind, body.Force, body.SkipAudit, body.Merge)
	}
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
	logArgs := map[string]any{
		"name":       body.Name,
		"force":      body.Force,
		"merge":      body.Merge,
		"skip_audit": body.SkipAudit,
		"scope":      "ui",
	}
	if body.Branch != "" {
		logArgs["branch"] = body.Branch
	}
	s.writeOpsLog("update", status, start, logArgs, msg)
	writeJSON(w, map[string]any{"results": []updateResultItem{result}})
}

func (s *Server) updateSingle(name string, force, skipAudit bool) updateResultItem {
	return s.updateSingleByKind(name, "", force, skipAudit, false)
}

func (s *Server) updateSingleByKind(name, kind string, force, skipAudit, merge bool) updateResultItem {
	name = path.Clean(filepath.ToSlash(name)) // "org/team/" names the same item as "org/team"
	if kind == "agent" {
		return s.updateAgent(name, force, skipAudit, merge)
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
		return s.updateRegularSkill(name, skillPath, force, skipAudit, merge, walk.Follow)
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

func (s *Server) updateAgent(name string, force, skipAudit, merge bool) updateResultItem {
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

	source, err := install.AgentRepoSource(entry, s.parseOpts())
	if err != nil {
		return updateResultItem{Name: metaKey, Kind: "agent", Action: "error", Message: "invalid source: " + err.Error()}
	}

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

	// Checked only once there is a new version to write over the local edits.
	modified := !force && entry.AgentLocallyModified(localAgent.SourcePath)
	if modified && !merge {
		return updateResultItem{Name: metaKey, Kind: "agent", Action: "skipped", Message: locallyModifiedMessage}
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
	if modified {
		opts.MergeBase = entry
	}
	res, err := install.UpdateAgentFromDiscovery(discovery, *target, destDir, opts)
	if err != nil {
		return updateFailure(metaKey, "agent", err)
	}

	if st, loadErr := install.LoadMetadataWithMigration(agentsSource, install.MetadataKindAgent); loadErr == nil && st != nil {
		s.agentsStore = st
	}

	if res.Action == "up-to-date" {
		return updateResultItem{Name: metaKey, Kind: "agent", Action: "up-to-date"}
	}
	message := res.Action
	if message == "merged" {
		message = updatedMessage(res)
	} else if message == "" {
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
		ParseOptions: s.parseOpts(),
		SourceDir:    sourceDir,
		Force:        force,
		SkipAudit:    skipAudit,
		Threshold:    s.updateAuditThreshold(),
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

// switchSkillBranch reinstalls a skill from another branch of its recorded
// source. Tracked repos change branch with git checkout instead.
func (s *Server) switchSkillBranch(name, branch string, force, skipAudit bool) updateResultItem {
	name = path.Clean(filepath.ToSlash(name))
	skillPath := filepath.Join(s.cfg.EffectiveSkillsSource(), name)
	if entry := s.skillsStore.GetByPath(name); entry == nil || entry.Source == "" || entry.Tracked || install.IsTrackedCheckout(skillPath) {
		return updateResultItem{Name: name, Action: "error", Message: "only a skill installed from a remote source can switch branch"}
	}
	walk := s.skillsWalk()
	result := s.reinstallRegularSkill(name, skillPath, branch, force, skipAudit, false, walk.Follow)
	if result.Action == "updated" && s.IsProjectMode() {
		// config.yaml declares the branch teammates install from, so it follows
		// the switch; other declarations stay as the shared file has them.
		for i := range s.projectCfg.Skills {
			if skill := &s.projectCfg.Skills[i]; skill.FullName() == name {
				skill.Branch = branch
				if err := s.projectCfg.Save(s.projectRoot); err != nil {
					result.Message += "; failed to record the branch in config.yaml: " + err.Error()
				}
				break
			}
		}
	}
	return result
}

func (s *Server) updateRegularSkill(name, skillPath string, force, skipAudit, merge bool, follow ...*sourcewalk.Follow) updateResultItem {
	return s.reinstallRegularSkill(name, skillPath, "", force, skipAudit, merge, follow...)
}

// reinstallRegularSkill reinstalls a skill from its recorded source, from
// branch when set and from the recorded branch otherwise.
func (s *Server) reinstallRegularSkill(name, skillPath, branch string, force, skipAudit, merge bool, follow ...*sourcewalk.Follow) updateResultItem {
	if refusal := s.refuseFollowedCheckout(name, skillPath, follow); refusal != nil {
		return *refusal
	}
	entry := s.skillsStore.GetByPath(name)
	if entry == nil {
		return updateResultItem{Name: name, Action: "error", Message: "no metadata found"}
	}
	modified := !force && entry.LocallyModified(skillPath, follow...)
	if modified && !merge {
		return updateResultItem{Name: name, Action: "skipped", Message: locallyModifiedMessage}
	}
	source, err := install.RecordedSource(entry, s.parseOpts())
	if err != nil {
		return updateResultItem{
			Name:    name,
			Action:  "error",
			Message: "invalid source: " + err.Error(),
		}
	}
	if branch == "" {
		branch = entry.Branch
	}
	source.ApplyRecordedBranch(branch)

	sourceDir := s.cfg.EffectiveSkillsSource()
	opts := s.skillUpdateOpts(force, skipAudit)
	if len(follow) > 0 {
		opts.SourceFollow = follow[0]
	}
	if modified {
		opts.MergeBase = entry
	}
	result, err := install.Install(source, skillPath, opts)
	if err != nil {
		return updateFailure(name, "", err)
	}

	if st, loadErr := install.LoadMetadataWithMigration(sourceDir, ""); loadErr == nil && st != nil {
		s.skillsStore = st
	}

	if result != nil && result.Action == "up-to-date" {
		return updateResultItem{Name: name, Action: "up-to-date"}
	}
	item := updateResultItem{
		Name:    name,
		Action:  "updated",
		Message: updatedMessage(result),
	}
	if result != nil && result.AuditRiskLabel != "" {
		item.AuditRiskScore = result.AuditRiskScore
		item.AuditRiskLabel = result.AuditRiskLabel
	}
	return item
}

func (s *Server) updateAll(force, skipAudit, merge bool) []updateResultItem {
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
		batch := s.newSkillBatch(skills, force, skipAudit, merge, walk.Follow)
		for _, skill := range skills {
			results = append(results, batch.update(skill))
		}
	}

	return results
}

// skillUpdateOpts returns the install options every regular-skill update uses.
func (s *Server) skillUpdateOpts(force, skipAudit bool) install.InstallOptions {
	opts := install.InstallOptions{
		ParseOptions:   s.parseOpts(),
		Force:          true,
		AuditOverride:  force,
		Reinstall:      force,
		Update:         true,
		SkipAudit:      skipAudit,
		AuditThreshold: s.updateAuditThreshold(),
		SourceDir:      s.cfg.EffectiveSkillsSource(),
	}
	if s.IsProjectMode() {
		opts.AuditProjectRoot = s.projectRoot
	}
	return opts
}

// skillBatch updates the regular skills of one request. Skills installed from
// the same repository and branch share one fetch, and a skill that already
// matches the fetched state is reported up to date instead of reinstalled, as
// `skillshare update` does. Callers hold s.mu for every update call.
type skillBatch struct {
	s         *Server
	names     []string // every regular skill the request updates
	force     bool
	skipAudit bool
	merge     bool
	follow    *sourcewalk.Follow
	done      map[string]updateResultItem // results of the repositories already fetched
}

func (s *Server) newSkillBatch(names []string, force, skipAudit, merge bool, follow *sourcewalk.Follow) *skillBatch {
	return &skillBatch{s: s, names: names, force: force, skipAudit: skipAudit, merge: merge, follow: follow, done: make(map[string]updateResultItem)}
}

// update returns the result for one skill. The first skill of a repository
// fetches it and settles every other skill of the request that shares it.
func (b *skillBatch) update(name string) updateResultItem {
	if item, ok := b.done[name]; ok {
		return item
	}
	entry := b.s.skillsStore.GetByPath(name)
	if entry == nil || entry.RepoURL == "" {
		skillPath := filepath.Join(b.s.cfg.EffectiveSkillsSource(), name)
		return b.s.updateRegularSkill(name, skillPath, b.force, b.skipAudit, b.merge, b.follow)
	}
	b.updateRepo(entry.RepoURL, entry.Branch)
	return b.done[name]
}

// updateRepo fetches one repository and updates the request's skills recorded
// from it at branch.
func (b *skillBatch) updateRepo(repoURL, branch string) {
	s := b.s
	sourceDir := s.cfg.EffectiveSkillsSource()
	targets := make(map[string]string) // repo subdir → skill path
	owners := make(map[string]string)  // repo subdir → skill name
	sources := make(map[string]string) // skill name → the source its record keeps
	for _, name := range b.names {
		if _, ok := b.done[name]; ok {
			continue
		}
		entry := s.skillsStore.GetByPath(name)
		if entry == nil || entry.RepoURL != repoURL || entry.Branch != branch {
			continue
		}
		skillPath := filepath.Join(sourceDir, name)
		if install.IsGitRepo(skillPath) {
			// A skill kept as a checkout updates by pulling; a copy from the
			// shared fetch would replace its repository.
			b.done[name] = s.updateRegularSkill(name, skillPath, b.force, b.skipAudit, b.merge, b.follow)
			continue
		}
		if refusal := s.refuseFollowedCheckout(name, skillPath, []*sourcewalk.Follow{b.follow}); refusal != nil {
			b.done[name] = *refusal
			continue
		}
		if !b.force && entry.LocallyModified(skillPath, b.follow) {
			// A merge needs this skill's own base, not the shared fetch.
			b.done[name] = b.s.updateRegularSkill(name, skillPath, b.force, b.skipAudit, b.merge, b.follow)
			continue
		}
		subdir := entry.Subdir
		if subdir == "" {
			subdir = "."
		}
		if owners[subdir] == name {
			continue // named twice in the request
		}
		if _, taken := targets[subdir]; taken {
			// A second local copy of the same repo path updates on its own.
			b.done[name] = s.updateRegularSkill(name, skillPath, b.force, b.skipAudit, b.merge, b.follow)
			continue
		}
		targets[subdir] = skillPath
		owners[subdir] = name
		sources[name] = install.RecordedSourceString(entry, s.parseOpts())
	}
	if len(targets) == 0 {
		return
	}

	opts := s.skillUpdateOpts(b.force, b.skipAudit)
	opts.SourceFollow = b.follow
	res, err := install.UpdateSkillsFromRepo(repoURL, branch, targets, opts)
	for subdir, name := range owners {
		item := updateResultItem{Name: name}
		switch {
		case err != nil:
			item.Action, item.Message = "error", err.Error()
		case res.Errors[subdir] != nil:
			item.Action, item.Message = "error", res.Errors[subdir].Error()
		case res.Results[subdir] != nil:
			item.Action, item.Message = "updated", "reinstalled from source"
			if label := res.Results[subdir].AuditRiskLabel; label != "" {
				item.AuditRiskScore = res.Results[subdir].AuditRiskScore
				item.AuditRiskLabel = label
			}
		case res.Current[subdir]:
			item.Action = "up-to-date"
		default:
			item.Action = "skipped"
		}
		b.done[name] = item
	}
	if err != nil || s.reloadSkillsStore() != nil {
		return
	}
	// The shared fetch records each source in the repository's canonical
	// spelling. Declarations match the spelling a skill was installed with, so
	// the record keeps it, as updating one skill does.
	respelled := false
	for name, source := range sources {
		if entry := s.skillsStore.GetByPath(name); entry != nil && entry.Source != source && b.done[name].Action == "updated" {
			entry.Source = source
			respelled = true
		}
	}
	if respelled {
		if saveErr := s.skillsStore.Save(sourceDir); saveErr != nil {
			log.Printf("warning: failed to keep recorded sources: %v", saveErr)
		}
	}
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

// followRepairedSources returns a function for defer: the project config
// follows the records the request rewrote because their source did not parse
// (config.FollowRepairedSources). Defer it after trackProjectLock, which pins
// by the config's source.
func (s *Server) followRepairedSources() func() {
	if !s.IsProjectMode() {
		return func() {}
	}
	sourceDir := s.cfg.EffectiveSkillsSource()
	before := install.LoadMetadataOrNew(sourceDir)
	return func() {
		if err := config.FollowRepairedSources(s.projectRoot, s.projectCfg, before, install.LoadMetadataOrNew(sourceDir), s.parseOpts()); err != nil {
			log.Printf("warning: failed to update project config: %v", err)
		}
	}
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
