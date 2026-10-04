package main

import (
	"fmt"
	"skillshare/internal/config"
	"skillshare/internal/resource"
	"skillshare/internal/sync"
)

// statusJSONAgents is the agent section of status --json output.
type statusJSONAgents struct {
	Source  string                  `json:"source"`
	Exists  bool                    `json:"exists"`
	Count   int                     `json:"count"`
	Targets []statusJSONAgentTarget `json:"targets,omitempty"`
}

type statusJSONAgentTarget struct {
	Name     string `json:"name"`
	Path     string `json:"path"`
	Expected int    `json:"expected"`
	Linked   int    `json:"linked"`
	Drift    bool   `json:"drift"`
}

// buildAgentStatusJSON builds the agents section for status --json output.
func buildAgentStatusJSON(cfg *config.Config) *statusJSONAgents {
	agentsSource := cfg.EffectiveAgentsSource()
	exists := dirExists(agentsSource)

	result := &statusJSONAgents{
		Source: agentsSource,
		Exists: exists,
	}

	if !exists {
		return result
	}

	agents, _ := resource.AgentKind{}.Discover(agentsSource)
	result.Count = len(agents)

	builtinAgents := config.DefaultAgentTargets()
	for name := range cfg.Targets {
		target := cfg.Targets[name]
		agentPath := resolveAgentTargetPath(target, builtinAgents, name)
		if agentPath == "" {
			continue
		}

		linked := countLinkedAgents(target.AgentsConfig(), agentPath, agents)
		result.Targets = append(result.Targets, statusJSONAgentTarget{
			Name:     name,
			Path:     agentPath,
			Expected: len(agents),
			Linked:   linked,
			Drift:    linked != len(agents) && len(agents) > 0,
		})
	}

	return result
}

// countLinkedAgents counts healthy .md symlinks in the target agent directory,
// plus up-to-date copies made where file links are unavailable. With an
// extension, it counts the tracked converted outputs instead.
func countLinkedAgents(ac config.ResourceTargetConfig, targetDir string, agents []resource.DiscoveredResource, preserved ...*int) int {
	if ac.Extension != "" {
		return sync.SyncedExtensionOutputs(targetDir, agents)
	}
	linked, _ := countAgentLinksAndBroken(targetDir)
	return linked + sync.SyncedAgentCopies(targetDir, agents, preserved...)
}

// agentStatusLabel returns the effective agent mode and the sub-item status
// shown for a target in sync.
func agentStatusLabel(ac config.ResourceTargetConfig) (mode, status string) {
	mode = sync.EffectiveMode(ac.Mode)
	if ac.Extension == "" {
		mode = sync.EffectiveAgentMode(ac.Mode)
	}
	switch mode {
	case "copy":
		return mode, "copied"
	case "symlink":
		return mode, "linked"
	}
	return mode, "merged"
}

// agentCountLabel distinguishes healthy local copies from managed links.
func agentCountLabel(current, expected, preserved int) string {
	label := fmt.Sprintf("%d/%d linked", current-preserved, expected)
	if preserved > 0 {
		label += fmt.Sprintf(", %d local preserved", preserved)
	}
	return label
}
