package sync

import (
	"context"
	"errors"
	"fmt"
	"maps"
	"slices"
	"strings"

	"skillshare/internal/config"
	"skillshare/internal/resource"
	"skillshare/internal/skillignore"
)

// TargetResult records completed skill and agent operations for a target.
type TargetResult struct {
	Target     string   `json:"target"`
	Linked     []string `json:"linked"`
	Updated    []string `json:"updated"`
	Skipped    []string `json:"skipped"`
	Pruned     []string `json:"pruned"`
	DirCreated string   `json:"dir_created,omitempty"`
}

// TargetFailure reports one failed resource without discarding completed work.
type TargetFailure struct {
	Target   string `json:"target"`
	Part     string `json:"part"` // "skill", "agent", or adapter-supplied "config"
	Error    string `json:"error"`
	Message  string `json:"message"`
	Conflict bool   `json:"conflict,omitempty"`
}

// TargetUnmatchedInclude describes include patterns that selected no skill.
type TargetUnmatchedInclude struct {
	Target      string   `json:"target"`
	Root        string   `json:"root,omitempty"`
	Patterns    []string `json:"patterns"`
	Suggestions []string `json:"suggestions,omitempty"`
	All         bool     `json:"all"` // no include pattern selects any skill
}

// TargetDiscovery is a prepared source snapshot, shared by every target in a run.
type TargetDiscovery struct {
	Skills           []DiscoveredSkill
	IgnoreStats      *skillignore.IgnoreStats
	Agents           []resource.DiscoveredResource
	AllAgents        []resource.DiscoveredResource // Agents plus .agentignore'd ones
	AgentsAvailable  bool
	SourceIncomplete bool
	Warnings         []string
}

// ExecuteOptions supplies caller-owned preparation and execution policy.
// Kinds accepts "skill" and "agent"; empty syncs both. Preparation, backups and logging are
// owned by adapters; Discovery must describe the resources selected by Kinds.
type ExecuteOptions struct {
	Kinds                      []string
	Force, DryRun, ProjectMode bool
	ProjectRoot                string
	AgentsSource               string
	Discovery                  TargetDiscovery
	BuiltinAgentTargets        map[string]config.TargetConfig
	ResolveExtension           func(string) (*ExtensionSpec, error)
	// DiagnosticsTargets preserves whole-config diagnostics when only a project
	// subset runs. Nil uses the selected targets.
	DiagnosticsTargets map[string]config.TargetConfig
	MovedTargets       []MovedTarget
}

// ExecuteResult retains completed rows even when one or every target fails.
type ExecuteResult struct {
	Results         []TargetResult
	Warnings        []string
	Unmatched       []TargetUnmatchedInclude
	Failed          []TargetFailure
	FolderConflicts []config.SkillsFolderConflict
	PathOverlap     int
	Skills          []DiscoveredSkill
	IgnoreStats     *skillignore.IgnoreStats
	AllAgents       []resource.DiscoveredResource
}

// ExecuteTargets syncs prepared resources directly, without HTTP, prompts or logs.
func ExecuteTargets(ctx context.Context, cfg *config.Config, targets map[string]config.TargetConfig, opts ExecuteOptions) (ExecuteResult, error) {
	globalMode := cfg.Mode
	if globalMode == "" {
		globalMode = "merge"
	}
	results := make([]TargetResult, 0)
	failed := make([]TargetFailure, 0)
	unmatched := make([]TargetUnmatchedInclude, 0)
	warnings := append([]string(nil), opts.Discovery.Warnings...)
	allSkills := opts.Discovery.Skills
	sourceIncomplete := opts.Discovery.SourceIncomplete
	ignorePatterns := EffectiveFileIgnorePatterns(cfg.Ignore)
	runTargets := targets
	dryRun, force := opts.DryRun, opts.Force
	if opts.ResolveExtension == nil {
		opts.ResolveExtension = func(ext string) (*ExtensionSpec, error) {
			if ext != "" {
				return nil, fmt.Errorf("no extension resolver for %q", ext)
			}
			return nil, nil
		}
	}
	if runsKind(opts.Kinds, "skill") {
		// A failed target adds a warning and no results row; the rest still sync.
		failTarget := func(name string, err error) {
			msg := name + ": sync failed: " + err.Error()
			warnings = append(warnings, msg)
			failed = append(failed, TargetFailure{Target: name, Part: "skill", Error: err.Error(), Message: msg, Conflict: errors.Is(err, ErrSymlinkConflict)})
		}
		runOpts := SkillRunOptions{
			Source: cfg.EffectiveSkillsSource(), ProjectRoot: opts.ProjectRoot, IgnorePatterns: ignorePatterns,
			DryRun: dryRun, Force: force, SourceIncomplete: sourceIncomplete,
		}
		for _, name := range slices.Sorted(maps.Keys(runTargets)) {
			if err := ctx.Err(); err != nil {
				return ExecuteResult{Results: results, Warnings: warnings, Failed: failed, Unmatched: unmatched, Skills: allSkills, IgnoreStats: opts.Discovery.IgnoreStats, AllAgents: opts.Discovery.AllAgents}, err
			}
			target := runTargets[name]
			sc := target.SkillsConfig()
			if !sc.IsEnabled() {
				continue // skills off: agents below still run
			}
			mode := sc.Mode
			if mode == "" {
				mode = globalMode
			}

			res := TargetResult{
				Target:  name,
				Linked:  make([]string, 0),
				Updated: make([]string, 0),
				Skipped: make([]string, 0),
				Pruned:  make([]string, 0),
			}

			run := SyncSkillTarget(SkillTarget{Name: name, Target: target, Mode: mode}, allSkills, runOpts)
			if run.Err != nil {
				failTarget(name, run.Err)
				continue
			}
			warnings = append(warnings, run.Warnings...)
			if len(run.UnmatchedIncludes) > 0 {
				u := TargetUnmatchedInclude{Target: name, Root: target.ProjectRoot(), Patterns: []string{}, All: AllIncludesUnmatched(sc.Include, run.UnmatchedIncludes)}
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
		if opts.ProjectMode {
			warnings = append(warnings, CleanMovedProjectDirs(
				opts.ProjectRoot, cfg.EffectiveSkillsSource(),
				opts.MovedTargets, dryRun)...)
		}
	}

	// Agent sync (skip when kind == "skill")
	if runsKind(opts.Kinds, "agent") {
		agentsSource := opts.AgentsSource
		if opts.Discovery.AgentsAvailable {
			agents := opts.Discovery.Agents
			builtinAgents := opts.BuiltinAgentTargets

			agentTargets := make([]AgentTarget, 0, len(runTargets))
			for _, name := range slices.Sorted(maps.Keys(runTargets)) {
				if err := ctx.Err(); err != nil {
					return ExecuteResult{Results: results, Warnings: warnings, Failed: failed, Unmatched: unmatched, Skills: allSkills, IgnoreStats: opts.Discovery.IgnoreStats, AllAgents: opts.Discovery.AllAgents}, err
				}
				target := runTargets[name]
				agentTargets = append(agentTargets, AgentTarget{
					Name:   name,
					Path:   ResolveAgentTargetPath(target, builtinAgents, name, opts.ProjectMode),
					Config: target.AgentsConfig(),
				})
			}
			runs := RunAgentSync(agentTargets, agents, AgentRunOptions{
				Source:           agentsSource,
				ProjectRoot:      opts.ProjectRoot,
				DryRun:           dryRun,
				Force:            force,
				ResolveExtension: opts.ResolveExtension,
			})

			for _, run := range runs {
				name := run.Name
				if run.Path == "" {
					continue
				}
				failAgent := func(err error) {
					msg := "agent sync failed for " + name + ": " + err.Error()
					warnings = append(warnings, msg)
					failed = append(failed, TargetFailure{Target: name, Part: "agent", Error: err.Error(), Message: msg})
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
					results = append(results, TargetResult{
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

	diagnostics := opts.DiagnosticsTargets
	if diagnostics == nil {
		diagnostics = targets
	}
	conflicts, overlap := TargetFolderConflicts(diagnostics, opts.ProjectMode, globalMode, HarmlessOverlap(diagnostics, globalMode, allSkills))
	slices.SortFunc(unmatched, func(a, b TargetUnmatchedInclude) int { return strings.Compare(a.Target, b.Target) })
	return ExecuteResult{Results: results, Warnings: warnings, Failed: failed, Unmatched: unmatched, FolderConflicts: conflicts, PathOverlap: overlap, Skills: allSkills, IgnoreStats: opts.Discovery.IgnoreStats, AllAgents: opts.Discovery.AllAgents}, nil
}

func runsKind(kinds []string, kind string) bool {
	return len(kinds) == 0 || slices.Contains(kinds, kind)
}

// TargetFolderConflicts reports conflicting skill folders and unexplained overlaps.
func TargetFolderConflicts(targets map[string]config.TargetConfig, isProject bool, defaultMode string, harmless func(scanner, writer string) bool) ([]config.SkillsFolderConflict, int) {
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
