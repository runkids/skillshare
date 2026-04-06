package main

import (
	"os"
	"strings"
	"time"

	"skillshare/internal/config"
	"skillshare/internal/resource"
	"skillshare/internal/ui"
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
		return diffOutputJSON(results, start)
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
		return diffOutputJSON(results, start)
	}

	if len(results) == 0 {
		ui.Info("No agent-capable targets found")
		return nil
	}

	renderGroupedDiffs(results, opts)
	return nil
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
	for flatName := range expected {
		if _, ok := existing[flatName]; !ok {
			r.items = append(r.items, copyDiffEntry{
				action: "add",
				name:   flatName,
				kind:   "agent",
				reason: "not in target",
				isSync: true,
			})
			r.synced = false
			r.syncCount++
		}
	}

	// Extra in target (orphans)
	for name, fileType := range existing {
		if _, ok := expected[name]; !ok {
			if fileType&os.ModeSymlink != 0 {
				r.items = append(r.items, copyDiffEntry{
					action: "remove",
					name:   name,
					kind:   "agent",
					reason: "orphan symlink",
					isSync: true,
				})
				r.synced = false
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

	return r
}
