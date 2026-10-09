package main

import (
	"fmt"
	"path/filepath"
	"strings"

	"skillshare/internal/audit"
	"skillshare/internal/config"
	"skillshare/internal/resource"
	"skillshare/internal/sync"
	"skillshare/internal/ui"
	"skillshare/internal/utils"
)

func cmdStatusProject(root string) error {
	if err := ensureProjectConfig(root); err != nil {
		return err
	}

	runtime, err := loadProjectRuntime(root)
	if err != nil {
		return err
	}

	walk := runtime.skillsWalk()
	sp := ui.StartSpinner("Discovering skills...")
	discovered, stats, discoverErr := sync.DiscoverSourceSkillsWithStats(runtime.sourcePath, walk)
	if discoverErr != nil {
		discovered = nil
	}
	trackedRepos := extractTrackedRepos(runtime.sourcePath, walk)
	sp.Stop()
	printSkippedSourceLinkWarnings(walk, false)

	agentCount := -1
	if agents, err := (resource.AgentKind{}).Discover(runtime.agentsSourcePath); err == nil && dirExists(runtime.agentsSourcePath) {
		agentCount = len(agents)
	}
	show := func(path string) string { return projectStatusPath(runtime.root, path) }
	printSourceStatus(runtime.sourcePath, runtime.agentsSourcePath, show, len(discovered), agentCount, stats)
	printTrackedReposStatus(runtime.sourcePath, discovered, trackedRepos)
	if err := printProjectTargetsStatus(runtime, discovered); err != nil {
		return err
	}

	if len(runtime.config.Extras) > 0 {
		printExtrasStatus(runtime.config.Extras, func(extra config.ExtraConfig) string {
			return config.ResolveExtrasSourceDirProject(extra, runtime.config.EffectiveExtrasSource(root), root)
		})
	}

	printAuditStatus(runtime.config.Audit)

	return nil
}

func cmdStatusProjectJSON(root string) error {
	if err := ensureProjectConfig(root); err != nil {
		return writeJSONError(err)
	}

	runtime, err := loadProjectRuntime(root)
	if err != nil {
		return writeJSONError(err)
	}

	walk := runtime.skillsWalk()
	output := statusJSONOutput{
		Version: version,
	}

	discovered, stats, _ := sync.DiscoverSourceSkillsWithStats(runtime.sourcePath, walk)
	trackedRepos := extractTrackedRepos(runtime.sourcePath, walk)

	printSkippedSourceLinkWarnings(walk, true)
	output.Source = statusJSONSource{
		Path:        runtime.sourcePath,
		Exists:      dirExists(runtime.sourcePath),
		Skillignore: buildSkillignoreJSON(stats),
	}
	output.SkillCount = len(discovered)
	output.TrackedRepos = buildTrackedRepoJSON(runtime.sourcePath, trackedRepos, discovered, walk)

	for _, entry := range runtime.config.Targets {
		target, ok := runtime.targets[entry.Name]
		if !ok {
			continue
		}
		sc := target.SkillsConfig()
		mode := sc.Mode
		if mode == "" {
			mode = "merge"
		}
		res := getTargetStatusDetail(target, runtime.sourcePath, mode)
		output.Targets = append(output.Targets, statusJSONTarget{
			Name:        entry.Name,
			Path:        sc.Path,
			Mode:        mode,
			Status:      res.statusStr,
			SyncedCount: res.syncedCount,
			Include:     sc.Include,
			Exclude:     sc.Exclude,

			SkillsEnabled: sc.IsEnabled(),
		})
	}

	policy := audit.ResolvePolicy(audit.PolicyInputs{
		ConfigProfile:   runtime.config.Audit.Profile,
		ConfigThreshold: runtime.config.Audit.BlockThreshold,
		ConfigDedupe:    runtime.config.Audit.DedupeMode,
		ConfigAnalyzers: runtime.config.Audit.EnabledAnalyzers,
	})
	output.Audit = statusJSONAudit{
		Profile:   string(policy.Profile),
		Threshold: policy.Threshold,
		Dedupe:    string(policy.DedupeMode),
		Analyzers: policy.EffectiveAnalyzers(),
	}

	output.Agents = buildProjectAgentStatusJSON(runtime)

	return writeJSON(&output)
}

// buildProjectAgentStatusJSON builds the agents section for project status --json.
func buildProjectAgentStatusJSON(rt *projectRuntime) *statusJSONAgents {
	exists := dirExists(rt.agentsSourcePath)
	result := &statusJSONAgents{
		Source: rt.agentsSourcePath,
		Exists: exists,
	}

	if !exists {
		return result
	}

	agents, _ := resource.AgentKind{}.Discover(rt.agentsSourcePath)
	result.Count = len(agents)

	builtinAgents := config.ProjectAgentTargets()
	for _, entry := range rt.config.Targets {
		agentPath := resolveProjectAgentTargetPath(entry, builtinAgents, rt.root)
		if agentPath == "" {
			continue
		}

		ac := entry.AgentsConfig()
		expected, err := expectedAgentsForTarget(ac, entry.Name, agents)
		if err != nil {
			expected = resource.ActiveAgents(agents) // invalid include/exclude: keep .agentignore at least
		}
		linked := countLinkedAgents(ac, agentPath, expected)
		result.Targets = append(result.Targets, statusJSONAgentTarget{
			Name:     entry.Name,
			Path:     agentPath,
			Expected: len(expected),
			Linked:   linked,
			Drift:    linked != len(expected) && len(expected) > 0,
		})
	}

	return result
}

// resolveProjectAgentTargetPath resolves the agent path for a project target entry.
func resolveProjectAgentTargetPath(entry config.ProjectTargetEntry, builtinAgents map[string]config.TargetConfig, projectRoot string) string {
	ac := entry.AgentsConfig()
	if ac.Path != "" {
		return resolveProjectPath(projectRoot, ac.Path)
	}
	if builtin, ok := builtinAgents[entry.Name]; ok {
		return resolveProjectPath(projectRoot, builtin.Path)
	}
	if builtin, ok := config.LookupProjectAgentTarget(entry.Name); ok {
		return resolveProjectPath(projectRoot, builtin.Path)
	}
	return ""
}

func printProjectTargetsStatus(runtime *projectRuntime, discovered []sync.DiscoveredSkill) error {
	builtinAgents := config.ProjectAgentTargets()
	agentsExist := dirExists(runtime.agentsSourcePath)
	var agents []resource.DiscoveredResource
	if agentsExist {
		agents, _ = (resource.AgentKind{}).Discover(runtime.agentsSourcePath)
	}

	var rows []statusTarget
	var warnings []string
	notSynced := 0
	for _, entry := range runtime.config.Targets {
		target, ok := runtime.targets[entry.Name]
		if !ok {
			rows = append(rows, statusTarget{name: entry.Name, skills: statusCell{mark: ui.MarkFail, text: "target not found"}})
			continue
		}

		sc := target.SkillsConfig()
		mode := sc.Mode
		if mode == "" {
			mode = "merge"
		}
		res := getTargetStatusDetail(target, runtime.sourcePath, mode)

		// A target with skills off expects nothing, so it has no drift.
		expected := 0
		if sc.IsEnabled() && (mode == "merge" || mode == "copy") {
			var err error
			expected, err = sync.ExpectedSkillCount(entry.Name, sc, discovered)
			if err != nil {
				return fmt.Errorf("target %s has invalid include/exclude config: %w", entry.Name, err)
			}
			notSynced = max(notSynced, expected-res.syncedCount)
		} else if sc.IsEnabled() && (len(sc.Include) > 0 || len(sc.Exclude) > 0) {
			warnings = append(warnings, entry.Name+": include/exclude ignored in symlink mode")
		}

		row := statusTarget{name: entry.Name, path: projectStatusPath(runtime.root, sc.Path), skills: skillsCell(res, sc.Path, mode, expected)}
		if sc.IsEnabled() {
			row.mode = mode
		}
		if agentsExist {
			if agentPath := resolveProjectAgentTargetPath(entry, builtinAgents, runtime.root); agentPath != "" {
				ac := entry.AgentsConfig()
				expected, err := expectedAgentsForTarget(ac, entry.Name, agents)
				if err != nil {
					return err
				}
				preserved := 0
				linked := countLinkedAgents(ac, agentPath, expected, &preserved)
				row.agents = agentsCell(linked, len(expected), preserved)
			}
		}
		rows = append(rows, row)
	}
	printStatusTargets(rows, agentsExist, warnings, notSynced)
	return nil
}

// projectStatusPath shows a target folder relative to the project root when
// it is inside the project.
func projectStatusPath(root, path string) string {
	if rel, err := filepath.Rel(root, path); err == nil && !strings.HasPrefix(rel, "..") {
		return rel
	}
	return utils.FoldHomePath(path)
}
