package server

import (
	"os"
	"path/filepath"

	"skillshare/internal/config"
	"skillshare/internal/resource"
	"skillshare/internal/utils"
)

// Kind constants for diff/sync operations.
const (
	kindSkill = "skill"
	kindAgent = "agent"
)

// discoverAllAgents discovers all agents (including disabled) from the source directory.
func discoverAllAgents(agentsSource string) []resource.DiscoveredResource {
	if agentsSource == "" {
		return nil
	}
	discovered, _ := resource.AgentKind{}.Discover(agentsSource)
	return discovered
}

// discoverActiveAgents discovers agents from the given source directory,
// returning only non-disabled agents. Returns nil if source is empty.
func discoverActiveAgents(agentsSource string) []resource.DiscoveredResource {
	return resource.ActiveAgents(discoverAllAgents(agentsSource))
}

// agentIgnorePayload returns JSON-serialisable fields describing .agentignore state.
// Pass pre-discovered agents to avoid duplicate filesystem traversal; nil triggers discovery.
func agentIgnorePayload(agentsSource string, all []resource.DiscoveredResource) map[string]any {
	root := ""
	ignoredNames := []string{}
	if agentsSource != "" {
		resolved := utils.ResolveSymlink(agentsSource)
		ignoreFile := filepath.Join(resolved, ".agentignore")
		if _, err := os.Stat(ignoreFile); err == nil {
			root = ignoreFile
		}
		if all == nil {
			all = discoverAllAgents(agentsSource)
		}
		for _, a := range all {
			if a.Disabled {
				ignoredNames = append(ignoredNames, a.RelPath)
			}
		}
	}
	return map[string]any{
		"agent_ignore_root":    root,
		"agent_ignored_count":  len(ignoredNames),
		"agent_ignored_skills": ignoredNames,
	}
}

// resolveAgentPath returns the expanded agent target path for a target,
// checking user config first, then builtin defaults. Returns "" if no path.
func resolveAgentPath(target config.TargetConfig, builtinAgents map[string]config.TargetConfig, name string, projectMode bool) string {
	if ac := target.AgentsConfig(); ac.Path != "" {
		return config.ExpandPath(ac.Path)
	}
	if builtin, ok := builtinAgents[name]; ok {
		return config.ExpandPath(builtin.Path)
	}
	if projectMode {
		if builtin, ok := config.LookupProjectAgentTarget(name); ok {
			return config.ExpandPath(builtin.Path)
		}
	} else if builtin, ok := config.LookupGlobalAgentTarget(name); ok {
		return config.ExpandPath(builtin.Path)
	}
	return ""
}

// builtinAgentTargets returns the builtin agent target map for the server's mode.
func (s *Server) builtinAgentTargets() map[string]config.TargetConfig {
	if s.IsProjectMode() {
		return config.ProjectAgentTargets()
	}
	return config.DefaultAgentTargets()
}

// mergeAgentDiffItems appends agent diff items into the existing diffs slice,
// merging with an existing target entry or creating a new one.
func mergeAgentDiffItems(diffs []diffTarget, name string, items []diffItem) []diffTarget {
	for i := range diffs {
		if diffs[i].Target == name {
			diffs[i].Items = append(diffs[i].Items, items...)
			return diffs
		}
	}
	return append(diffs, diffTarget{
		Target: name,
		Items:  items,
	})
}

// appendAgentDiffs merges agent diff items for every agent-capable target.
// Unlike sync-matrix, diff must still inspect targets when the source is empty
// so orphaned synced agents surface as prune drift, matching skills behavior.
func (s *Server) appendAgentDiffs(diffs []diffTarget, targets map[string]config.TargetConfig, agentsSource, filterTarget string) []diffTarget {
	agents := discoverActiveAgents(agentsSource)
	builtinAgents := s.builtinAgentTargets()

	for name, target := range targets {
		if filterTarget != "" && filterTarget != name {
			continue
		}
		agentPath := resolveAgentPath(target, builtinAgents, name, s.IsProjectMode())
		if agentPath == "" {
			continue
		}
		if items := computeAgentTargetDiff(agentPath, agents); len(items) > 0 {
			diffs = mergeAgentDiffItems(diffs, name, items)
		}
	}

	return diffs
}
