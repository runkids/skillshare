package server

import (
	"encoding/json"
	"io"
	"maps"
	"net/http"
	"os"
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
	Resource   string   `json:"resource"`
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
		DryRun    bool     `json:"dryRun"`
		Force     bool     `json:"force"`
		Kind      string   `json:"kind"`
		Resources []string `json:"resources"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		if err != io.EOF {
			writeError(w, http.StatusBadRequest, "invalid request body: "+err.Error())
			return
		}
	}

	if body.Kind != "" && body.Kind != kindSkill && body.Kind != kindAgent {
		writeError(w, http.StatusBadRequest, "invalid kind: must be 'skill', 'agent', or empty")
		return
	}
	if body.Kind != "" && len(body.Resources) > 0 {
		writeError(w, http.StatusBadRequest, "kind and resources cannot be combined")
		return
	}

	var resources serverSyncResources
	switch body.Kind {
	case kindAgent:
		resources = serverSyncResources{}
	case kindSkill:
		resources = serverSyncResources{skills: true}
	default:
		parsed, err := parseServerSyncResources(body.Resources)
		if err != nil {
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}
		resources = parsed
	}

	globalMode := s.cfg.Mode
	if globalMode == "" {
		globalMode = "merge"
	}

	warnings := []string{}
	if resources.skills || body.Kind == kindAgent {
		configWarnings, validErr := config.ValidateConfig(s.cfg)
		if validErr != nil {
			writeError(w, http.StatusBadRequest, validErr.Error())
			return
		}
		warnings = append(warnings, configWarnings...)
	}

	results := make([]syncTargetResult, 0)
	var ignoreStats *skillignore.IgnoreStats
	var allSkills []ssync.DiscoveredSkill

	if resources.skills {
		var err error
		allSkills, ignoreStats, err = ssync.DiscoverSourceSkillsWithStats(s.cfg.Source)
		if err != nil {
			writeError(w, http.StatusInternalServerError, "failed to discover skills: "+err.Error())
			return
		}
		if len(allSkills) == 0 {
			warnings = append(warnings, "source directory is empty (0 skills)")
		}
	}

	if body.Kind == kindAgent {
		s.syncAgentsForUI(&results, &warnings, body.DryRun, body.Force)
	} else {
		for name, target := range s.cfg.Targets {
			syncErrArgs := map[string]any{
				"targets_total":  len(s.cfg.Targets),
				"targets_failed": 1,
				"target":         name,
				"dry_run":        body.DryRun,
				"force":          body.Force,
				"scope":          "ui",
			}

			if resources.skills {
				sc := target.SkillsConfig()
				mode := sc.Mode
				if mode == "" {
					mode = globalMode
				}

				res := newSyncTargetResult(name, "skills")
				switch mode {
				case "merge":
					mergeResult, err := ssync.SyncTargetMergeWithSkills(name, target, allSkills, s.cfg.Source, body.DryRun, body.Force, s.projectRoot)
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
						TargetPath: sc.Path, SourcePath: s.cfg.Source, Skills: allSkills,
						Include: sc.Include, Exclude: sc.Exclude, TargetNaming: sc.TargetNaming, TargetName: name,
						DryRun: body.DryRun, Force: body.Force,
					})
					if err == nil {
						res.Pruned = pruneResult.Removed
					}

				case "copy":
					copyResult, err := ssync.SyncTargetCopyWithSkills(name, target, allSkills, s.cfg.Source, body.DryRun, body.Force, nil)
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
					err := ssync.SyncTarget(name, target, s.cfg.Source, body.DryRun, s.projectRoot)
					if err != nil {
						s.writeOpsLog("sync", "error", start, syncErrArgs, err.Error())
						writeError(w, http.StatusInternalServerError, "sync failed for "+name+": "+err.Error())
						return
					}
					res.Linked = []string{"(symlink mode)"}
				}
				results = append(results, res)
			}

			if resources.rules {
				res, err := s.syncManagedRulesForTarget(name, target, body.DryRun)
				if err != nil {
					s.writeOpsLog("sync", "error", start, syncErrArgs, err.Error())
					writeError(w, http.StatusInternalServerError, "sync failed for "+name+": "+err.Error())
					return
				}
				results = append(results, res)
			}

			if resources.hooks {
				res, err := s.syncManagedHooksForTarget(name, target, body.DryRun)
				if err != nil {
					s.writeOpsLog("sync", "error", start, syncErrArgs, err.Error())
					writeError(w, http.StatusInternalServerError, "sync failed for "+name+": "+err.Error())
					return
				}
				results = append(results, res)
			}
		}
	}

	args := map[string]any{
		"targets_total":  len(s.cfg.Targets),
		"targets_failed": 0,
		"dry_run":        body.DryRun,
		"force":          body.Force,
		"scope":          "ui",
	}
	if body.Kind != "" {
		args["kind"] = body.Kind
	}
	if body.Kind == "" {
		args["resources"] = body.Resources
	}
	s.writeOpsLog("sync", "ok", start, args, "")

	resp := map[string]any{
		"results":  results,
		"warnings": warnings,
	}
	maps.Copy(resp, ignorePayload(ignoreStats))
	maps.Copy(resp, agentIgnorePayload(s.agentsSource(), nil))
	writeJSON(w, resp)
}

func (s *Server) syncAgentsForUI(results *[]syncTargetResult, warnings *[]string, dryRun, force bool) {
	agentsSource := s.agentsSource()
	info, err := os.Stat(agentsSource)
	if err != nil || !info.IsDir() {
		return
	}

	agents := discoverActiveAgents(agentsSource)
	builtinAgents := s.builtinAgentTargets()

	for name, target := range s.cfg.Targets {
		agentPath := resolveAgentPath(target, builtinAgents, name)
		if agentPath == "" {
			continue
		}

		agentMode := target.AgentsConfig().Mode
		if agentMode == "" {
			agentMode = "merge"
		}

		agentResult, err := ssync.SyncAgents(agents, agentsSource, agentPath, agentMode, dryRun, force)
		if err != nil {
			*warnings = append(*warnings, "agent sync failed for "+name+": "+err.Error())
			continue
		}

		var pruned []string
		if agentMode == "merge" {
			pruned, _ = ssync.PruneOrphanAgentLinks(agentPath, agents, dryRun)
		} else if agentMode == "copy" {
			pruned, _ = ssync.PruneOrphanAgentCopies(agentPath, agents, dryRun)
		}

		if len(agentResult.Linked) == 0 && len(agentResult.Updated) == 0 && len(agentResult.Skipped) == 0 && len(pruned) == 0 {
			continue
		}

		res := newSyncTargetResult(name, "agents")
		res.Linked = append(res.Linked, agentResult.Linked...)
		res.Updated = append(res.Updated, agentResult.Updated...)
		res.Skipped = append(res.Skipped, agentResult.Skipped...)
		res.Pruned = append(res.Pruned, pruned...)
		*results = append(*results, res)
	}
}

func newSyncTargetResult(target, resource string) syncTargetResult {
	return syncTargetResult{
		Resource: resource,
		Target:   target,
		Linked:   make([]string, 0),
		Updated:  make([]string, 0),
		Skipped:  make([]string, 0),
		Pruned:   make([]string, 0),
	}
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
	source := s.cfg.Source
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
