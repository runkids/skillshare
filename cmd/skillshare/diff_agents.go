package main

import (
	"os"
	"path/filepath"
	"strings"
	"time"

	"skillshare/internal/config"
	"skillshare/internal/resource"
	"skillshare/internal/ui"
	"skillshare/internal/utils"
)

// diffProjectAgents computes agent diffs for project mode.
func diffProjectAgents(root, targetName string, opts diffRenderOpts, start time.Time) error {
	if !projectConfigExists(root) {
		if err := performProjectInit(root, projectInitOptions{}); err != nil {
			return err
		}
	}

	rt, err := loadProjectRuntime(root)
	if err != nil {
		return err
	}

	agentsSource := rt.agentsSourcePath
	agents, _ := resource.AgentKind{}.Discover(agentsSource)

	builtinAgents := config.ProjectAgentTargets()
	var results []targetDiffResult

	for _, entry := range rt.config.Targets {
		if targetName != "" && entry.Name != targetName {
			continue
		}
		agentPath := resolveProjectAgentTargetPath(entry, builtinAgents, root)
		if agentPath == "" {
			continue
		}

		r := computeAgentDiff(entry.Name, agentPath, agents)
		results = append(results, r)
	}

	if opts.jsonOutput {
		return diffOutputJSON(results, nil, nil, start)
	}

	if len(results) == 0 {
		ui.Info("No agent-capable targets found")
		return nil
	}

	renderGroupedDiffs(results, opts)
	return nil
}

// diffGlobalAgents computes agent diffs for global mode.
func diffGlobalAgents(cfg *config.Config, targetName string, opts diffRenderOpts, start time.Time) error {
	agentsSource := cfg.EffectiveAgentsSource()
	agents, _ := resource.AgentKind{}.Discover(agentsSource)

	builtinAgents := config.DefaultAgentTargets()
	var results []targetDiffResult

	for name := range cfg.Targets {
		if targetName != "" && name != targetName {
			continue
		}
		agentPath := resolveAgentTargetPath(cfg.Targets[name], builtinAgents, name)
		if agentPath == "" {
			continue
		}

		r := computeAgentDiff(name, agentPath, agents)
		results = append(results, r)
	}

	if opts.jsonOutput {
		return diffOutputJSON(results, nil, nil, start)
	}

	if len(results) == 0 {
		ui.Info("No agent-capable targets found")
		return nil
	}

	renderGroupedDiffs(results, opts)
	return nil
}

// mergeAgentDiffsGlobal computes agent diffs for all targets and merges them
// into existing skill diff results. Targets with agent diffs get their items
// appended; targets without a skill result get a new entry.
func mergeAgentDiffsGlobal(cfg *config.Config, results []targetDiffResult, targetName string) []targetDiffResult {
	agentsSource := cfg.EffectiveAgentsSource()
	agents, _ := resource.AgentKind{}.Discover(agentsSource)

	builtinAgents := config.DefaultAgentTargets()
	var agentResults []targetDiffResult
	for name := range cfg.Targets {
		if targetName != "" && name != targetName {
			continue
		}
		agentPath := resolveAgentTargetPath(cfg.Targets[name], builtinAgents, name)
		if agentPath == "" {
			continue
		}
		agentResults = append(agentResults, computeAgentDiff(name, agentPath, agents))
	}

	return mergeAgentResults(results, agentResults)
}

// mergeAgentDiffsProject computes agent diffs for project targets and merges
// them into existing skill diff results.
func mergeAgentDiffsProject(root string, results []targetDiffResult, targetName string) []targetDiffResult {
	if !projectConfigExists(root) {
		return results
	}
	rt, err := loadProjectRuntime(root)
	if err != nil {
		return results
	}

	agentsSource := rt.agentsSourcePath
	agents, _ := resource.AgentKind{}.Discover(agentsSource)

	builtinAgents := config.ProjectAgentTargets()
	var agentResults []targetDiffResult
	for _, entry := range rt.config.Targets {
		if targetName != "" && entry.Name != targetName {
			continue
		}
		agentPath := resolveProjectAgentTargetPath(entry, builtinAgents, root)
		if agentPath == "" {
			continue
		}
		agentResults = append(agentResults, computeAgentDiff(entry.Name, agentPath, agents))
	}

	return mergeAgentResults(results, agentResults)
}

// mergeAgentResults merges agent diff results into skill results by target name.
func mergeAgentResults(skillResults, agentResults []targetDiffResult) []targetDiffResult {
	if len(agentResults) == 0 {
		return skillResults
	}

	idx := make(map[string]int, len(skillResults))
	for i, r := range skillResults {
		idx[r.name] = i
	}

	for _, ar := range agentResults {
		if len(ar.items) == 0 {
			continue
		}
		if i, ok := idx[ar.name]; ok {
			skillResults[i].items = append(skillResults[i].items, ar.items...)
			skillResults[i].syncCount += ar.syncCount
			skillResults[i].localCount += ar.localCount
			if !ar.synced {
				skillResults[i].synced = false
			}
		} else {
			skillResults = append(skillResults, ar)
		}
	}
	return skillResults
}

// computeAgentDiff compares source agents against a target directory.
func computeAgentDiff(targetName, targetDir string, agents []resource.DiscoveredResource) targetDiffResult {
	r := targetDiffResult{
		name:   targetName,
		mode:   "merge",
		synced: true,
	}

	// Build map of expected agents
	expected := make(map[string]resource.DiscoveredResource, len(agents))
	for _, a := range agents {
		expected[a.FlatName] = a
	}

	// Check what exists in target (store type for symlink detection)
	existing := make(map[string]os.FileMode) // key=filename, value=type bits
	if entries, err := os.ReadDir(targetDir); err == nil {
		for _, e := range entries {
			if e.IsDir() || !strings.HasSuffix(strings.ToLower(e.Name()), ".md") {
				continue
			}
			existing[e.Name()] = e.Type()
		}
	}

	// Missing in target (need sync)
	for flatName, agent := range expected {
		fileType, ok := existing[flatName]
		if !ok {
			r.items = append(r.items, copyDiffEntry{
				action: "add",
				name:   flatName,
				kind:   "agent",
				reason: "not in target",
				isSync: true,
			})
			r.synced = false
			r.syncCount++
			continue
		}

		targetPath := filepath.Join(targetDir, flatName)
		if fileType&os.ModeSymlink != 0 || utils.IsSymlinkOrJunction(targetPath) {
			absLink, err := utils.ResolveLinkTarget(targetPath)
			if err != nil {
				r.items = append(r.items, copyDiffEntry{
					action: "modify",
					name:   flatName,
					kind:   "agent",
					reason: "link target unreadable",
					isSync: true,
				})
				r.synced = false
				r.syncCount++
				continue
			}
			absSource, _ := filepath.Abs(agent.AbsPath)
			if !utils.PathsEqual(absLink, absSource) {
				r.items = append(r.items, copyDiffEntry{
					action: "modify",
					name:   flatName,
					kind:   "agent",
					reason: "symlink points elsewhere",
					isSync: true,
				})
				r.synced = false
				r.syncCount++
			}
			continue
		}

		r.items = append(r.items, copyDiffEntry{
			action: "modify",
			name:   flatName,
			kind:   "agent",
			reason: "local copy (sync --force to replace)",
			isSync: true,
		})
		r.synced = false
		r.syncCount++
	}

	// Extra in target (orphans)
	for name, fileType := range existing {
		if _, ok := expected[name]; !ok {
			targetPath := filepath.Join(targetDir, name)
			if fileType&os.ModeSymlink != 0 || utils.IsSymlinkOrJunction(targetPath) {
				r.items = append(r.items, copyDiffEntry{
					action: "remove",
					name:   name,
					kind:   "agent",
					reason: "orphan symlink",
					isSync: true,
				})
				r.synced = false
				r.syncCount++
			} else {
				r.items = append(r.items, copyDiffEntry{
					action: "local",
					name:   name,
					kind:   "agent",
					reason: "local file",
				})
				r.localCount++
			}
		}
	}

	r.synced = r.syncCount == 0 && r.localCount == 0
	return r
}
