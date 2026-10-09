package server

import (
	"errors"
	"fmt"
	"maps"
	"net/http"
	"os"
	"path/filepath"
	"slices"
	"sort"
	"strings"
	"time"

	"skillshare/internal/backup"
	"skillshare/internal/config"
	"skillshare/internal/skillignore"
	ssync "skillshare/internal/sync"
)

// ignorePayload builds the common ignored-skills fields for JSON responses.
func ignorePayload(stats *skillignore.IgnoreStats) map[string]any {
	skills := []string{}
	rootFile := ""
	repoFiles := []string{}
	if stats != nil {
		if len(stats.IgnoredSkills) > 0 {
			skills = stats.IgnoredSkills
		}
		rootFile = stats.RootFile
		if stats.RepoFiles != nil {
			repoFiles = stats.RepoFiles
		}
	}
	return map[string]any{
		"ignored_count":  len(skills),
		"ignored_skills": skills,
		"ignore_root":    rootFile,
		"ignore_repos":   repoFiles,
	}
}

// syncFailure is a target that failed to sync while the others went ahead.
// Message is the same text the failure adds to warnings, so a client can show
// failures apart from the other warnings.
type syncFailure struct {
	Target   string `json:"target"`
	Part     string `json:"part"` // "skill", "agent", or "config"
	Error    string `json:"error"`
	Message  string `json:"message"`
	Conflict bool   `json:"conflict,omitempty"` // a symlink points elsewhere; force replaces it
}

// unmatchedInclude is a target whose include filter selects no skill, so the
// dashboard can explain it and link to where the filter is edited.
type unmatchedInclude struct {
	Target      string   `json:"target"`
	Root        string   `json:"root,omitempty"` // the project folder of a project target
	Patterns    []string `json:"patterns"`
	Suggestions []string `json:"suggestions,omitempty"` // source path names the patterns likely meant
	// All means no include pattern selects anything, so the target gets no skills.
	All bool `json:"all"`
}

type syncTargetResult struct {
	Target     string   `json:"target"`
	Linked     []string `json:"linked"`
	Updated    []string `json:"updated"`
	Skipped    []string `json:"skipped"`
	Pruned     []string `json:"pruned"`
	DirCreated string   `json:"dir_created,omitempty"`
}

func (s *Server) handleSync(w http.ResponseWriter, r *http.Request) {
	start := time.Now()
	s.mu.Lock()
	defer s.mu.Unlock()

	var body struct {
		DryRun bool   `json:"dryRun"`
		Force  bool   `json:"force"`
		Kind   string `json:"kind"`
		// Project limits the sync to one declared project's targets.
		Project string `json:"project"`
	}
	if err := decodeJSON(w, r, &body, defaultJSONBodyLimit); errors.Is(err, errBodyTooLarge) {
		return
	}
	// Any other decode error keeps the defaults: non-dry-run, non-force, empty kind (both)

	if body.Kind != "" && body.Kind != kindSkill && body.Kind != kindAgent {
		writeError(w, http.StatusBadRequest, "invalid kind: must be 'skill', 'agent', or empty")
		return
	}

	if body.Project != "" {
		root := s.declaredProjectRoot(body.Project)
		if _, ok := s.cfg.Projects[root]; !ok {
			writeError(w, http.StatusBadRequest, "unknown project: "+body.Project)
			return
		}
		body.Project = root
	}

	out, code, err := s.syncResources(start, body.DryRun, body.Force, body.Kind, body.Project)
	if err != nil {
		writeError(w, code, err.Error())
		return
	}

	resp := map[string]any{
		"results":          out.results,
		"warnings":         out.warnings,
		"unmatched":        out.unmatched,
		"failed":           out.failed,
		"folder_conflicts": out.folderConflicts,
		"path_overlap":     out.pathOverlap,
	}
	if len(out.skills) > 0 {
		if contextCost := s.computeContextCost(out.skills); contextCost != nil {
			resp["context_cost"] = contextCost
		}
	}
	maps.Copy(resp, ignorePayload(out.ignoreStats))
	maps.Copy(resp, agentIgnorePayload(s.agentsSource(), nil))
	writeJSON(w, resp)
}

type syncOutcome struct {
	results         []syncTargetResult
	warnings        []string
	unmatched       []unmatchedInclude
	failed          []syncFailure
	folderConflicts []config.SkillsFolderConflict
	pathOverlap     int // targets whose path overlap the folder conflicts don't explain
	skills          []ssync.DiscoveredSkill
	ignoreStats     *skillignore.IgnoreStats
}

// folderConflicts returns the skills folders whose targets undo each other's
// sync, never nil, plus how many targets overlap in a way they don't explain.
// harmless is passed on to config.DetectPathOverlap.
func folderConflicts(targets map[string]config.TargetConfig, isProject bool, defaultMode string, harmless func(scanner, writer string) bool) ([]config.SkillsFolderConflict, int) {
	conflicts := config.SkillsFolderConflicts(targets, defaultMode)
	explained := map[string]bool{}
	for _, c := range conflicts {
		for _, name := range c.Targets {
			explained[name] = true
		}
	}
	rest := 0
	for _, name := range config.DetectPathOverlap(targets, isProject, harmless) {
		if !explained[name] {
			rest++
		}
	}
	if conflicts == nil {
		conflicts = []config.SkillsFolderConflict{}
	}
	return conflicts, rest
}

// projectTargets returns the targets of the project declared under root.
func (s *Server) projectTargets(root string) map[string]config.TargetConfig {
	abs := filepath.Clean(config.ExpandPath(root))
	targets := map[string]config.TargetConfig{}
	for name, target := range s.cfg.Targets {
		if target.ProjectRoot() != "" && filepath.Clean(target.ProjectRoot()) == abs {
			targets[name] = target
		}
	}
	return targets
}

// syncResources links skills and agents (kind "" means both) into every target,
// or only the targets of the project declared under project, and logs the sync.
// A failed target, even when every target fails, is reported in failed and
// warnings; an error, with the HTTP status to report, means nothing could run.
// Callers must hold s.mu.
func (s *Server) syncResources(start time.Time, dryRun, force bool, kind, project string) (*syncOutcome, int, error) {
	targets := s.cfg.Targets
	if project != "" {
		targets = s.projectTargets(project)
	}

	globalMode := s.cfg.Mode
	if globalMode == "" {
		globalMode = "merge"
	}

	// Pre-check warnings via shared config validation
	warnings, invalid, validErr := config.ValidateConfigForSync(s.cfg)
	if validErr != nil {
		return nil, http.StatusBadRequest, validErr
	}

	// A project target that cannot resolve (no path) is not in s.cfg.Targets;
	// it fails like one with invalid settings.
	maps.Copy(invalid, s.unresolvedTargets)
	total := len(targets) + len(s.unresolvedTargets)

	results := make([]syncTargetResult, 0)
	failed := make([]syncFailure, 0)
	unmatched := make([]unmatchedInclude, 0)

	// A target with invalid settings fails alone: it is skipped for skills and agents.
	runTargets := maps.Clone(targets)
	for _, name := range slices.Sorted(maps.Keys(invalid)) {
		if _, ok := targets[name]; !ok && s.unresolvedTargets[name] == nil {
			continue
		}
		delete(runTargets, name)
		err := invalid[name]
		msg := name + ": invalid config: " + err.Error()
		warnings = append(warnings, msg)
		failed = append(failed, syncFailure{Target: name, Part: "config", Error: err.Error(), Message: msg})
	}

	if !dryRun {
		warnings = append(warnings, s.backupBeforeSync(runTargets, kind != kindAgent, kind != kindSkill)...)
	}

	var ignoreStats *skillignore.IgnoreStats
	var allSkills []ssync.DiscoveredSkill
	ignorePatterns := ssync.EffectiveFileIgnorePatterns(s.cfg.Ignore)

	// Skill sync (skip when kind == "agent")
	if kind != kindAgent {
		var err error
		walk := s.skillsWalk()
		allSkills, ignoreStats, err = ssync.DiscoverSourceSkillsWithStatsAndContext(s.cfg.EffectiveSkillsSource(), walk)
		if err != nil {
			return nil, http.StatusInternalServerError, fmt.Errorf("failed to discover skills: %w", err)
		}
		sourceIncomplete := walk.Follow.Incomplete()
		warnings = append(warnings, ssync.SourceLinkWarnings(walk, sourceIncomplete)...)

		if len(allSkills) == 0 {
			warnings = append(warnings, "source directory is empty (0 skills)")
		}

		// Registry entries are managed by install/uninstall, not sync.
		// Sync only manages symlinks — it must not prune registry entries
		// for installed skills whose files may be missing from disk.

		// A failed target adds a warning and no results row; the rest still sync.
		failTarget := func(name string, err error) {
			msg := name + ": sync failed: " + err.Error()
			warnings = append(warnings, msg)
			failed = append(failed, syncFailure{Target: name, Part: "skill", Error: err.Error(), Message: msg, Conflict: errors.Is(err, ssync.ErrSymlinkConflict)})
		}
		runOpts := ssync.SkillRunOptions{
			Source: s.cfg.EffectiveSkillsSource(), ProjectRoot: s.projectRoot, IgnorePatterns: ignorePatterns,
			DryRun: dryRun, Force: force, SourceIncomplete: sourceIncomplete,
		}
		for name, target := range runTargets {
			sc := target.SkillsConfig()
			if !sc.IsEnabled() {
				continue // skills off: agents, extras and MCP below still run
			}
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

			run := ssync.SyncSkillTarget(ssync.SkillTarget{Name: name, Target: target, Mode: mode}, allSkills, runOpts)
			if run.Err != nil {
				failTarget(name, run.Err)
				continue
			}
			warnings = append(warnings, run.Warnings...)
			if len(run.UnmatchedIncludes) > 0 {
				u := unmatchedInclude{Target: name, Root: target.ProjectRoot(), Patterns: []string{}, All: ssync.AllIncludesUnmatched(sc.Include, run.UnmatchedIncludes)}
				for _, m := range run.UnmatchedIncludes {
					u.Patterns = append(u.Patterns, m.Pattern)
					u.Suggestions = append(u.Suggestions, m.Suggestions...)
				}
				unmatched = append(unmatched, u)
			}
			switch mode {
			case "merge", "copy":
				res.Linked, res.Updated, res.Skipped, res.DirCreated = run.Linked, run.Updated, run.Skipped, run.DirCreated
				if run.Pruned != nil {
					res.Pruned = run.Pruned
				}
			default:
				res.Linked = []string{"(symlink mode)"}
			}

			results = append(results, res)
		}

		// Clean skillshare entries left in also_scans dirs a target no longer
		// writes to, so the runtime stops seeing every skill twice.
		if s.IsProjectMode() {
			warnings = append(warnings, ssync.CleanMovedProjectDirs(
				s.projectRoot, s.cfg.EffectiveSkillsSource(),
				ssync.MovedProjectTargets(s.projectCfg, s.cfg.Targets), dryRun)...)
		}
	}

	// Agent sync (skip when kind == "skill")
	if kind != kindSkill {
		agentsSource := s.agentsSource()
		if info, err := os.Stat(agentsSource); err == nil && info.IsDir() {
			agents := discoverActiveAgents(agentsSource)
			builtinAgents := s.builtinAgentTargets()

			agentTargets := make([]ssync.AgentTarget, 0, len(runTargets))
			for name, target := range runTargets {
				agentTargets = append(agentTargets, ssync.AgentTarget{
					Name:   name,
					Path:   resolveAgentPath(target, builtinAgents, name, s.IsProjectMode()),
					Config: target.AgentsConfig(),
				})
			}
			runs := ssync.RunAgentSync(agentTargets, agents, ssync.AgentRunOptions{
				Source:           agentsSource,
				ProjectRoot:      s.projectRoot,
				DryRun:           dryRun,
				Force:            force,
				ResolveExtension: s.resolveExtensionSpec,
			})

			for _, run := range runs {
				name := run.Name
				if run.Path == "" {
					continue
				}
				failAgent := func(err error) {
					msg := "agent sync failed for " + name + ": " + err.Error()
					warnings = append(warnings, msg)
					failed = append(failed, syncFailure{Target: name, Part: "agent", Error: err.Error(), Message: msg})
				}
				if run.Err != nil {
					failAgent(run.Err)
					continue
				}
				warnings = append(warnings, run.Warnings...)
				if run.SyncErr != nil {
					failAgent(run.SyncErr)
				}
				if !run.Synced {
					continue
				}

				// Find or create result entry for this target
				idx := -1
				for i := range results {
					if results[i].Target == name {
						idx = i
						break
					}
				}
				if idx < 0 && (len(run.Linked) > 0 || len(run.Updated) > 0 || len(run.Skipped) > 0 || len(run.Pruned) > 0) {
					results = append(results, syncTargetResult{
						Target:  name,
						Linked:  make([]string, 0),
						Updated: make([]string, 0),
						Skipped: make([]string, 0),
						Pruned:  make([]string, 0),
					})
					idx = len(results) - 1
				}

				if idx >= 0 {
					results[idx].Linked = append(results[idx].Linked, run.Linked...)
					results[idx].Updated = append(results[idx].Updated, run.Updated...)
					results[idx].Skipped = append(results[idx].Skipped, run.Skipped...)
					results[idx].Pruned = append(results[idx].Pruned, run.Pruned...)
				}
			}
		}
	}

	// Log the sync operation. A target counts once however many parts failed.
	failedNames := failedTargetNames(failed)
	logArgs := map[string]any{
		"targets_total":  total,
		"targets_failed": len(failedNames),
		"dry_run":        dryRun,
		"force":          force,
		"kind":           kind,
		"scope":          "ui",
	}
	if len(failedNames) > 0 {
		logArgs["failed_targets"] = failedNames
	}
	if project != "" {
		logArgs["project"] = project
	}
	status := "ok"
	switch {
	case len(failedNames) >= total && len(failedNames) > 0:
		status = "error"
	case len(failedNames) > 0:
		status = "partial"
	}
	s.writeOpsLog("sync", status, start, logArgs, "")

	conflicts, overlap := folderConflicts(s.cfg.Targets, s.IsProjectMode(), globalMode,
		ssync.HarmlessOverlap(s.cfg.Targets, globalMode, allSkills))
	// Targets sync from a map; a fixed order keeps the dashboard's notices still.
	slices.SortFunc(unmatched, func(a, b unmatchedInclude) int { return strings.Compare(a.Target, b.Target) })
	return &syncOutcome{results: results, warnings: warnings, unmatched: unmatched, failed: failed, folderConflicts: conflicts, pathOverlap: overlap, skills: allSkills, ignoreStats: ignoreStats}, 0, nil
}

// failedTargetNames returns the distinct targets in failed, sorted.
func failedTargetNames(failed []syncFailure) []string {
	names := make([]string, 0, len(failed))
	for _, f := range failed {
		names = append(names, f.Target)
	}
	slices.Sort(names)
	return slices.Compact(names)
}

// backupBeforeSync snapshots the target folders a sync may overwrite, as the
// CLI does, then trims old snapshots. Project mode backs up agents only, like
// `sync -p`. Failures come back as warnings. Callers must hold s.mu.
func (s *Server) backupBeforeSync(targets map[string]config.TargetConfig, skills, agents bool) []string {
	var warnings []string
	dir := backup.BackupDir()
	if s.IsProjectMode() {
		dir, skills = backup.ProjectBackupDir(s.projectRoot), false
	}
	snapshot := func(entry, path string) {
		if _, err := backup.CreateInDir(dir, entry, path); err != nil {
			warnings = append(warnings, "backup failed for "+entry+": "+err.Error())
		}
	}
	builtinAgents := s.builtinAgentTargets()
	for name, target := range targets {
		if skills && target.SkillsConfig().IsEnabled() {
			snapshot(name, target.SkillsConfig().Path)
		}
		if p := resolveAgentPath(target, builtinAgents, name, s.IsProjectMode()); agents && p != "" {
			snapshot(name+"-agents", resolveExtrasTargetPath(s.projectRoot, p))
		}
	}
	if _, err := backup.CleanupInDir(dir, s.backupRetention()); err != nil {
		warnings = append(warnings, "backup cleanup failed: "+err.Error())
	}
	return warnings
}

func (s *Server) computeContextCost(skills []ssync.DiscoveredSkill) map[string]any {
	type tokenPair struct{ always, onDemand int }
	groupMap := make(map[tokenPair][]string)

	type skillToken struct {
		name       string
		descTokens int
		bodyTokens int
	}

	var maxAlways, maxOnDemand int
	var worstAlwaysSkills, worstOnDemandSkills []skillToken
	var worstAlwaysTarget, worstOnDemandTarget string

	cfgMode := s.cfg.Mode
	if cfgMode == "" {
		cfgMode = "merge"
	}

	for name, target := range s.cfg.Targets {
		filtered, fErr := ssync.TargetSkills(name, target, cfgMode, s.cfg.EffectiveSkillsSource(), skills)
		if fErr != nil {
			filtered = skills
		}

		var at, ot int
		var perSkill []skillToken
		for _, sk := range filtered {
			at += sk.DescTokens
			ot += sk.BodyTokens
			perSkill = append(perSkill, skillToken{
				name:       sk.FlatName,
				descTokens: sk.DescTokens,
				bodyTokens: sk.BodyTokens,
			})
		}

		groupMap[tokenPair{at, ot}] = append(groupMap[tokenPair{at, ot}], name)

		if at > maxAlways {
			maxAlways = at
			worstAlwaysSkills = perSkill
			worstAlwaysTarget = name
		} else if at > 0 && at == maxAlways && name < worstAlwaysTarget {
			worstAlwaysTarget = name
			worstAlwaysSkills = perSkill
		}
		if ot > maxOnDemand {
			maxOnDemand = ot
			worstOnDemandSkills = perSkill
			worstOnDemandTarget = name
		} else if ot > 0 && ot == maxOnDemand && name < worstOnDemandTarget {
			worstOnDemandTarget = name
			worstOnDemandSkills = perSkill
		}
	}

	type group struct {
		Targets            []string `json:"targets"`
		AlwaysLoadedTokens int      `json:"always_loaded_tokens"`
		OnDemandTokens     int      `json:"on_demand_tokens"`
	}

	groups := make([]group, 0, len(groupMap))
	for pair, names := range groupMap {
		sort.Strings(names)
		groups = append(groups, group{
			Targets:            names,
			AlwaysLoadedTokens: pair.always,
			OnDemandTokens:     pair.onDemand,
		})
	}
	sort.Slice(groups, func(i, j int) bool { return groups[i].Targets[0] < groups[j].Targets[0] })

	budget := s.cfg.ContextBudget
	if s.IsProjectMode() && s.projectCfg != nil {
		budget = s.projectCfg.ContextBudget
	}

	result := map[string]any{"groups": groups}

	type offender struct {
		Name   string `json:"name"`
		Tokens int    `json:"tokens"`
	}
	type warning struct {
		Type         string     `json:"type"`
		Target       string     `json:"target"`
		Actual       int        `json:"actual"`
		Budget       int        `json:"budget"`
		TopOffenders []offender `json:"top_offenders"`
	}

	topN := func(perSkill []skillToken, n int, byDesc bool) []offender {
		type pair struct {
			name   string
			tokens int
		}
		pairs := make([]pair, 0, len(perSkill))
		for _, sk := range perSkill {
			t := sk.bodyTokens
			if byDesc {
				t = sk.descTokens
			}
			if t > 0 {
				pairs = append(pairs, pair{sk.name, t})
			}
		}
		sort.Slice(pairs, func(i, j int) bool { return pairs[i].tokens > pairs[j].tokens })
		if len(pairs) > n {
			pairs = pairs[:n]
		}
		out := make([]offender, len(pairs))
		for i, p := range pairs {
			out[i] = offender{Name: p.name, Tokens: p.tokens}
		}
		return out
	}

	var warns []warning
	if t := budget.AlwaysLoadedThreshold(); t > 0 && maxAlways > t {
		warns = append(warns, warning{
			Type: "always_loaded", Target: worstAlwaysTarget,
			Actual: maxAlways, Budget: t,
			TopOffenders: topN(worstAlwaysSkills, 3, true),
		})
	}
	if t := budget.OnDemandThreshold(); t > 0 && maxOnDemand > t {
		warns = append(warns, warning{
			Type: "on_demand", Target: worstOnDemandTarget,
			Actual: maxOnDemand, Budget: t,
			TopOffenders: topN(worstOnDemandSkills, 3, false),
		})
	}
	if len(warns) > 0 {
		result["warnings"] = warns
	}
	return result
}

type diffItem struct {
	Skill  string `json:"skill"`
	Action string `json:"action"` // "link", "update", "skip", "prune", "local", "kept" (the skill stays at a legacy entry; force changes nothing)
	Reason string `json:"reason"` // human-readable description
	Kind   string `json:"kind,omitempty"`
}

type diffTarget struct {
	Target         string     `json:"target"`
	Items          []diffItem `json:"items"`
	SkippedCount   int        `json:"skippedCount,omitempty"`
	CollisionCount int        `json:"collisionCount,omitempty"`
}

func (s *Server) handleDiff(w http.ResponseWriter, r *http.Request) {
	// Snapshot config under RLock, then release before slow I/O.
	s.mu.RLock()
	source := s.cfg.EffectiveSkillsSource()
	agentsSource := s.agentsSource()
	globalMode := s.cfg.Mode
	ignorePatterns := ssync.EffectiveFileIgnorePatterns(s.cfg.Ignore)
	targets := s.cloneTargets()
	conflicts, _ := folderConflicts(targets, s.IsProjectMode(), globalMode, nil)
	s.mu.RUnlock()

	if globalMode == "" {
		globalMode = "merge"
	}

	filterTarget := r.URL.Query().Get("target")

	discovered, ignoreStats, err := ssync.DiscoverSourceSkillsWithStats(source, s.skillsWalk())
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	diffs := make([]diffTarget, 0)
	for name, target := range targets {
		if filterTarget != "" && filterTarget != name {
			continue
		}
		diffs = append(diffs, s.computeTargetDiff(name, target, discovered, globalMode, source, ignorePatterns))
	}

	diffs = s.appendAgentDiffs(diffs, targets, agentsSource, filterTarget)

	resp := map[string]any{"diffs": diffs, "folder_conflicts": conflicts}
	maps.Copy(resp, ignorePayload(ignoreStats))
	maps.Copy(resp, agentIgnorePayload(agentsSource, nil))
	writeJSON(w, resp)
}
