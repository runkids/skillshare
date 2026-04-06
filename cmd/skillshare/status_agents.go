package main

import (
	"os"
	"path/filepath"
	"strings"

	"skillshare/internal/config"
	"skillshare/internal/resource"
	"skillshare/internal/ui"
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

// printAgentStatus prints agent source and per-target agent status (text mode).
func printAgentStatus(cfg *config.Config) {
	agentsSource := cfg.EffectiveAgentsSource()

	ui.Header("Agents")

	exists := dirExists(agentsSource)
	if !exists {
		ui.Info("Source: %s (not created)", agentsSource)
		return
	}

	agents, _ := resource.AgentKind{}.Discover(agentsSource)
	ui.Info("Source: %s (%d agents)", agentsSource, len(agents))

	// Per-target agent status
	builtinAgents := config.DefaultAgentTargets()
	var targets []string
	for name := range cfg.Targets {
		targets = append(targets, name)
	}

	for _, name := range targets {
		agentPath := resolveAgentTargetPath(cfg.Targets[name], builtinAgents, name)
		if agentPath == "" {
			continue
		}

		linked := countLinkedAgents(agentPath)
		driftLabel := ""
		if linked != len(agents) && len(agents) > 0 {
			driftLabel = ui.Yellow + " (drift)" + ui.Reset
		}
		ui.Info("  %s: %s (%d/%d linked)%s", name, agentPath, linked, len(agents), driftLabel)
	}
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
		agentPath := resolveAgentTargetPath(cfg.Targets[name], builtinAgents, name)
		if agentPath == "" {
			continue
		}

		linked := countLinkedAgents(agentPath)
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

// countLinkedAgents counts .md symlinks in the target agent directory.
func countLinkedAgents(targetDir string) int {
	entries, err := os.ReadDir(targetDir)
	if err != nil {
		return 0
	}
	count := 0
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		if !strings.HasSuffix(strings.ToLower(e.Name()), ".md") {
			continue
		}
		fullPath := filepath.Join(targetDir, e.Name())
		if info, lErr := os.Lstat(fullPath); lErr == nil && info.Mode()&os.ModeSymlink != 0 {
			count++
		}
	}
	return count
}
