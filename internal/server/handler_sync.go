package server

import (
	"encoding/json"
	"fmt"
	"maps"
	"net/http"
	"os"
	"sort"
	"time"

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
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		// Default to non-dry-run, non-force, empty kind (both)
	}

	if body.Kind != "" && body.Kind != kindSkill && body.Kind != kindAgent {
		writeError(w, http.StatusBadRequest, "invalid kind: must be 'skill', 'agent', or empty")
		return
	}

	globalMode := s.cfg.Mode
	if globalMode == "" {
		globalMode = "merge"
	}

	// Pre-check warnings via shared config validation
	warnings, validErr := config.ValidateConfig(s.cfg)
	if validErr != nil {
		writeError(w, http.StatusBadRequest, validErr.Error())
		return
	}

	if involved := config.DetectPathOverlap(s.cfg.Targets, s.IsProjectMode()); len(involved) > 0 {
		warnings = append(warnings, fmt.Sprintf("Skill path overlap across %d target(s) — see Health Check for details", len(involved)))
	}

	results := make([]syncTargetResult, 0)

	var ignoreStats *skillignore.IgnoreStats
	var allSkills []ssync.DiscoveredSkill

	// Skill sync (skip when kind == "agent")
	if body.Kind != kindAgent {
		var err error
		allSkills, ignoreStats, err = ssync.DiscoverSourceSkillsWithStatsAndContext(s.cfg.EffectiveSkillsSource())
		if err != nil {
			writeError(w, http.StatusInternalServerError, "failed to discover skills: "+err.Error())
			return
		}

		if len(allSkills) == 0 {
			warnings = append(warnings, "source directory is empty (0 skills)")
		}

		// Registry entries are managed by install/uninstall, not sync.
		// Sync only manages symlinks — it must not prune registry entries
		// for installed skills whose files may be missing from disk.

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

			syncErrArgs := map[string]any{
				"targets_total":  len(s.cfg.Targets),
				"targets_failed": 1,
				"target":         name,
				"dry_run":        body.DryRun,
				"force":          body.Force,
				"scope":          "ui",
			}

			switch mode {
			case "merge":
				mergeResult, err := ssync.SyncTargetMergeWithSkills(name, target, allSkills, s.cfg.EffectiveSkillsSource(), body.DryRun, body.Force, s.projectRoot)
				if err != nil {
					s.writeOpsLog("sync", "error", start, syncErrArgs, err.Error())
					writeError(w, http.StatusInternalServerError, "sync failed for "+name+": "+err.Error())
					return
				}
				res.Linked = mergeResult.Linked
				res.Updated = mergeResult.Updated
				res.Skipped = mergeResult.Skipped
				res.DirCreated = mergeResult.DirCreated

				pruneResult, err := ssync.PruneOrphanLinksWithSkills(ssync.PruneOptions{
					TargetPath: sc.Path, SourcePath: s.cfg.EffectiveSkillsSource(), Skills: allSkills,
					Include: sc.Include, Exclude: sc.Exclude, TargetNaming: sc.TargetNaming, TargetName: name,
					DryRun: body.DryRun, Force: body.Force,
				})
				if err == nil {
					res.Pruned = pruneResult.Removed
				}

			case "copy":
				copyResult, err := ssync.SyncTargetCopyWithSkills(name, target, allSkills, s.cfg.EffectiveSkillsSource(), body.DryRun, body.Force, nil)
				if err != nil {
					s.writeOpsLog("sync", "error", start, syncErrArgs, err.Error())
					writeError(w, http.StatusInternalServerError, "sync failed for "+name+": "+err.Error())
					return
				}
				res.Linked = copyResult.Copied
				res.Updated = copyResult.Updated
				res.Skipped = copyResult.Skipped
				res.DirCreated = copyResult.DirCreated

				pruneResult, err := ssync.PruneOrphanCopiesWithSkills(sc.Path, allSkills, sc.Include, sc.Exclude, name, sc.TargetNaming, body.DryRun)
				if err == nil {
					res.Pruned = pruneResult.Removed
				}

			default:
				err := ssync.SyncTarget(name, target, s.cfg.EffectiveSkillsSource(), body.DryRun, s.projectRoot)
				if err != nil {
					s.writeOpsLog("sync", "error", start, syncErrArgs, err.Error())
					writeError(w, http.StatusInternalServerError, "sync failed for "+name+": "+err.Error())
					return
				}
				res.Linked = []string{"(symlink mode)"}
			}

			results = append(results, res)
		}
	}

	// Agent sync (skip when kind == "skill")
	if body.Kind != kindSkill {
		agentsSource := s.agentsSource()
		if info, err := os.Stat(agentsSource); err == nil && info.IsDir() {
			agents := discoverActiveAgents(agentsSource)
			builtinAgents := s.builtinAgentTargets()

			for name, target := range s.cfg.Targets {
				agentPath := resolveAgentPath(target, builtinAgents, name)
				if agentPath == "" {
					continue
				}

				ac := target.AgentsConfig()
				agentMode := ac.Mode
				if agentMode == "" {
					agentMode = "merge"
				}

				filtered, filterErr := ssync.FilterAgents(agents, ac.Include, ac.Exclude)
				if filterErr != nil {
					warnings = append(warnings, "invalid agent filter for "+name+": "+filterErr.Error())
					continue
				}

				agentResult, err := ssync.SyncAgents(filtered, agentsSource, agentPath, agentMode, body.DryRun, body.Force)
				if err != nil {
					warnings = append(warnings, "agent sync failed for "+name+": "+err.Error())
					continue
				}

				// Prune orphan agents even when the source is empty so uninstall-all
				// matches skills and clears previously synced target entries.
				var pruned []string
				if agentMode == "merge" {
					pruned, _ = ssync.PruneOrphanAgentLinks(agentPath, filtered, body.DryRun)
				} else if agentMode == "copy" {
					pruned, _ = ssync.PruneOrphanAgentCopies(agentPath, filtered, body.DryRun)
				}

				// Find or create result entry for this target
				idx := -1
				for i := range results {
					if results[i].Target == name {
						idx = i
						break
					}
				}
				if idx < 0 && (len(agentResult.Linked) > 0 || len(agentResult.Updated) > 0 || len(agentResult.Skipped) > 0 || len(pruned) > 0) {
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
					results[idx].Linked = append(results[idx].Linked, agentResult.Linked...)
					results[idx].Updated = append(results[idx].Updated, agentResult.Updated...)
					results[idx].Skipped = append(results[idx].Skipped, agentResult.Skipped...)
					results[idx].Pruned = append(results[idx].Pruned, pruned...)
				}
			}
		}
	}

	// Log the sync operation
	s.writeOpsLog("sync", "ok", start, map[string]any{
		"targets_total":  len(results),
		"targets_failed": 0,
		"dry_run":        body.DryRun,
		"force":          body.Force,
		"kind":           body.Kind,
		"scope":          "ui",
	}, "")

	var contextCost map[string]any
	if len(allSkills) > 0 {
		contextCost = s.computeContextCost(allSkills)
	}

	resp := map[string]any{
		"results":  results,
		"warnings": warnings,
	}
	if contextCost != nil {
		resp["context_cost"] = contextCost
	}
	maps.Copy(resp, ignorePayload(ignoreStats))
	maps.Copy(resp, agentIgnorePayload(s.agentsSource(), nil))
	writeJSON(w, resp)
}

func (s *Server) computeContextCost(skills []ssync.DiscoveredSkill) map[string]any {
	const charsPerToken = 4

	type tokenPair struct{ always, onDemand int }
	groupMap := make(map[tokenPair][]string)

	type skillToken struct {
		name       string
		descTokens int
		bodyTokens int
	}

	var maxAlways, maxOnDemand int
	var worstAlwaysSkills, worstOnDemandSkills []skillToken

	cfgMode := s.cfg.Mode
	if cfgMode == "" {
		cfgMode = "merge"
	}

	for name, target := range s.cfg.Targets {
		sc := target.SkillsConfig()
		mode := sc.Mode
		if mode == "" {
			mode = cfgMode
		}

		var filtered []ssync.DiscoveredSkill
		if mode == "symlink" {
			filtered = skills
		} else {
			var fErr error
			filtered, fErr = ssync.FilterSkills(skills, sc.Include, sc.Exclude)
			if fErr != nil {
				filtered = skills
			}
			filtered = ssync.FilterSkillsByTarget(filtered, name)
		}

		var alwaysChars, onDemandChars int
		var perSkill []skillToken
		for _, sk := range filtered {
			alwaysChars += sk.DescChars
			onDemandChars += sk.BodyChars
			perSkill = append(perSkill, skillToken{
				name:       sk.FlatName,
				descTokens: sk.DescChars / charsPerToken,
				bodyTokens: sk.BodyChars / charsPerToken,
			})
		}
		at := alwaysChars / charsPerToken
		ot := onDemandChars / charsPerToken

		groupMap[tokenPair{at, ot}] = append(groupMap[tokenPair{at, ot}], name)

		if at > maxAlways {
			maxAlways = at
			worstAlwaysSkills = perSkill
		}
		if ot > maxOnDemand {
			maxOnDemand = ot
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
			Type: "always_loaded", Actual: maxAlways, Budget: t,
			TopOffenders: topN(worstAlwaysSkills, 3, true),
		})
	}
	if t := budget.OnDemandThreshold(); t > 0 && maxOnDemand > t {
		warns = append(warns, warning{
			Type: "on_demand", Actual: maxOnDemand, Budget: t,
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
	Action string `json:"action"` // "link", "update", "skip", "prune", "local"
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
	targets := s.cloneTargets()
	s.mu.RUnlock()

	if globalMode == "" {
		globalMode = "merge"
	}

	filterTarget := r.URL.Query().Get("target")

	discovered, ignoreStats, err := ssync.DiscoverSourceSkillsWithStats(source)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	diffs := make([]diffTarget, 0)
	for name, target := range targets {
		if filterTarget != "" && filterTarget != name {
			continue
		}
		diffs = append(diffs, s.computeTargetDiff(name, target, discovered, globalMode, source))
	}

	diffs = s.appendAgentDiffs(diffs, targets, agentsSource, filterTarget)

	resp := map[string]any{"diffs": diffs}
	maps.Copy(resp, ignorePayload(ignoreStats))
	maps.Copy(resp, agentIgnorePayload(agentsSource, nil))
	writeJSON(w, resp)
}
