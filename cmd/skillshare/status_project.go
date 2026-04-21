package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"skillshare/internal/audit"
	"skillshare/internal/config"
	"skillshare/internal/git"
	hookpkg "skillshare/internal/hooks"
	pluginpkg "skillshare/internal/plugins"
	"skillshare/internal/resource"
	"skillshare/internal/skillignore"
	"skillshare/internal/sync"
	"skillshare/internal/ui"
)

func cmdStatusProject(root string) error {
	if !projectConfigExists(root) {
		if err := performProjectInit(root, projectInitOptions{}); err != nil {
			return err
		}
	}

	runtime, err := loadProjectRuntime(root)
	if err != nil {
		return err
	}

	sp := ui.StartSpinner("Discovering skills...")
	discovered, stats, discoverErr := sync.DiscoverSourceSkillsWithStats(runtime.sourcePath)
	if discoverErr != nil {
		discovered = nil
	}
	trackedRepos := extractTrackedRepos(discovered)
	sp.Stop()

	printProjectSourceStatus(runtime.sourcePath, runtime.agentsSourcePath, len(discovered), stats)
	printProjectTrackedReposStatus(runtime.sourcePath, discovered, trackedRepos)
	if err := printProjectTargetsStatus(runtime, discovered); err != nil {
		return err
	}

	// Extras
	if len(runtime.config.Extras) > 0 {
		ui.Header("Extras")
		printExtrasStatus(runtime.config.Extras, func(extra config.ExtraConfig) string {
			return config.ExtrasSourceDirProject(root, extra.Name)
		})
	}
	if bundles, bundleErr := pluginpkg.Discover(config.PluginsSourceDirProject(root)); bundleErr == nil && len(bundles) > 0 {
		ui.Header("Plugins")
		for _, bundle := range bundles {
			ui.Status(bundle.Name, "plugin", fmt.Sprintf("claude=%t codex=%t", bundle.HasClaude, bundle.HasCodex))
		}
	}
	if bundles, bundleErr := hookpkg.Discover(config.HooksSourceDirProject(root)); bundleErr == nil && len(bundles) > 0 {
		ui.Header("Hooks")
		for _, bundle := range bundles {
			ui.Status(bundle.Name, "hook", fmt.Sprintf("claude=%d codex=%d", bundle.Targets["claude"], bundle.Targets["codex"]))
		}
	}

	printAuditStatus(runtime.config.Audit)

	return nil
}

func cmdStatusProjectJSON(root string) error {
	if !projectConfigExists(root) {
		if err := performProjectInit(root, projectInitOptions{}); err != nil {
			return writeJSONError(err)
		}
	}

	runtime, err := loadProjectRuntime(root)
	if err != nil {
		return writeJSONError(err)
	}

	output := statusJSONOutput{
		Version: version,
	}

	discovered, stats, _ := sync.DiscoverSourceSkillsWithStats(runtime.sourcePath)
	trackedRepos := extractTrackedRepos(discovered)

	output.Source = statusJSONSource{
		Path:        runtime.sourcePath,
		Exists:      dirExists(runtime.sourcePath),
		Skillignore: buildSkillignoreJSON(stats),
	}
	output.SkillCount = len(discovered)
	output.TrackedRepos = buildTrackedRepoJSON(runtime.sourcePath, trackedRepos, discovered)

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
	output.Plugins, _ = pluginpkg.Discover(config.PluginsSourceDirProject(root))
	output.Hooks, _ = hookpkg.Discover(config.HooksSourceDirProject(root))

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

		linked := countLinkedAgents(agentPath)
		result.Targets = append(result.Targets, statusJSONAgentTarget{
			Name:     entry.Name,
			Path:     agentPath,
			Expected: len(agents),
			Linked:   linked,
			Drift:    linked != len(agents) && len(agents) > 0,
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
	return ""
}

func printProjectSourceStatus(sourcePath, agentsSourcePath string, skillCount int, stats *skillignore.IgnoreStats) {
	ui.Header("Source (project)")
	info, err := os.Stat(sourcePath)
	if err != nil {
		ui.Error(".skillshare/skills/ (not found)")
		return
	}

	ui.Success(".skillshare/skills/ (%d skills, %s)", skillCount, info.ModTime().Format("2006-01-02 15:04"))
	printSkillignoreLine(stats)

	// Agents source
	if agentsInfo, agentsErr := os.Stat(agentsSourcePath); agentsErr == nil {
		agentCount := 0
		if agents, discoverErr := (resource.AgentKind{}).Discover(agentsSourcePath); discoverErr == nil {
			agentCount = len(agents)
		}
		ui.Success(".skillshare/agents/ (%d agents, %s)", agentCount, agentsInfo.ModTime().Format("2006-01-02 15:04"))
	}
}

func printProjectTrackedReposStatus(sourcePath string, discovered []sync.DiscoveredSkill, trackedRepos []string) {
	if len(trackedRepos) == 0 {
		return
	}

	ui.Header("Tracked Repositories")
	for _, repoName := range trackedRepos {
		repoPath := filepath.Join(sourcePath, repoName)

		skillCount := 0
		for _, d := range discovered {
			if d.IsInRepo && strings.HasPrefix(d.RelPath, repoName+"/") {
				skillCount++
			}
		}

		statusStr := "up-to-date"
		statusIcon := "✓"
		if isDirty, _ := git.IsDirty(repoPath); isDirty {
			statusStr = "has uncommitted changes"
			statusIcon = "!"
		}

		ui.Status(repoName, statusIcon, fmt.Sprintf("%d skills, %s", skillCount, statusStr))
	}
}

func printProjectTargetsStatus(runtime *projectRuntime, discovered []sync.DiscoveredSkill) error {
	ui.Header("Targets")

	builtinAgents := config.ProjectAgentTargets()
	agentsExist := dirExists(runtime.agentsSourcePath)
	var agentCount int
	if agentsExist {
		agents, _ := (resource.AgentKind{}).Discover(runtime.agentsSourcePath)
		agentCount = len(agents)
	}

	driftTotal := 0
	for _, entry := range runtime.config.Targets {
		target, ok := runtime.targets[entry.Name]
		if !ok {
			ui.Error("%s: target not found", entry.Name)
			continue
		}

		// Target name header
		fmt.Printf("%s%s%s\n", ui.Bold, entry.Name, ui.Reset)

		// Skills sub-item
		sc := target.SkillsConfig()
		mode := sc.Mode
		if mode == "" {
			mode = "merge"
		}

		res := getTargetStatusDetail(target, runtime.sourcePath, mode)
		printTargetSubItem("skills", res.statusStr, res.detail)

		if mode == "merge" || mode == "copy" {
			filtered, err := sync.FilterSkills(discovered, sc.Include, sc.Exclude)
			if err != nil {
				return fmt.Errorf("target %s has invalid include/exclude config: %w", entry.Name, err)
			}
			filtered = sync.FilterSkillsByTarget(filtered, entry.Name)
			expectedCount := len(filtered)

			if res.syncedCount < expectedCount {
				drift := expectedCount - res.syncedCount
				if drift > driftTotal {
					driftTotal = drift
				}
			}
		} else if len(sc.Include) > 0 || len(sc.Exclude) > 0 {
			ui.Warning("%s: include/exclude ignored in symlink mode", entry.Name)
		}

		// Agents sub-item
		if agentsExist {
			agentPath := resolveProjectAgentTargetPath(entry, builtinAgents, runtime.root)
			if agentPath != "" {
				linked := countLinkedAgents(agentPath)
				agentStatus := "merged"
				driftLabel := ""
				if linked != agentCount && agentCount > 0 {
					agentStatus = "drift"
					driftLabel = ui.Yellow + " (drift)" + ui.Reset
				}
				printTargetSubItem("agents", agentStatus, fmt.Sprintf("[merge] %d/%d linked%s", linked, agentCount, driftLabel))
			}
		}
	}
	if driftTotal > 0 {
		ui.Warning("%d skill(s) not synced — run 'skillshare sync'", driftTotal)
	}
	return nil
}
