package main

import (
	"fmt"
	"os"
	"path/filepath"

	"skillshare/internal/config"
	"skillshare/internal/sync"
	"skillshare/internal/ui"
)

// cmdCollectAgents collects non-symlinked agent .md files from agent-capable targets
// back to the agent source directory.
func cmdCollectAgents(cfg *config.Config, dryRun, jsonOutput bool) error {
	agentsSource := cfg.EffectiveAgentsSource()

	if err := os.MkdirAll(agentsSource, 0755); err != nil {
		return fmt.Errorf("cannot create agents source directory: %w", err)
	}

	builtinAgents := config.DefaultAgentTargets()
	var allCollected []string

	if !jsonOutput {
		ui.Header(ui.WithModeLabel("Collect agents"))
	}

	for name := range cfg.Targets {
		agentPath := resolveAgentTargetPath(cfg.Targets[name], builtinAgents, name)
		if agentPath == "" {
			continue
		}

		if _, err := os.Stat(agentPath); err != nil {
			continue // target agent dir doesn't exist, skip
		}

		collected, err := sync.CollectAgents(agentPath, agentsSource, dryRun, os.Stdout)
		if err != nil {
			if !jsonOutput {
				ui.Warning("%s: collect failed: %v", name, err)
			}
			continue
		}

		if len(collected) > 0 {
			allCollected = append(allCollected, collected...)
			if !jsonOutput {
				ui.Success("%s: collected %d agent(s)", name, len(collected))
			}
		}
	}

	if !jsonOutput {
		if len(allCollected) == 0 {
			ui.Info("No local agents found to collect")
		} else {
			fmt.Println()
			ui.Info("Collected %d agent(s) to %s", len(allCollected), agentsSource)
		}
	}

	return nil
}

// cmdCollectProjectAgents collects non-symlinked agent .md files from project targets.
func cmdCollectProjectAgents(projectRoot string) error {
	agentsSource := filepath.Join(projectRoot, ".skillshare", "agents")
	if err := os.MkdirAll(agentsSource, 0755); err != nil {
		return fmt.Errorf("cannot create project agents directory: %w", err)
	}

	projCfg, err := config.LoadProject(projectRoot)
	if err != nil {
		return fmt.Errorf("cannot load project config: %w", err)
	}

	builtinAgents := config.ProjectAgentTargets()
	var allCollected []string

	ui.Header(ui.WithModeLabel("Collect agents"))

	for _, entry := range projCfg.Targets {
		agentPath := resolveProjectAgentTargetPath(entry, builtinAgents, projectRoot)
		if agentPath == "" {
			continue
		}
		if _, statErr := os.Stat(agentPath); statErr != nil {
			continue
		}

		collected, collectErr := sync.CollectAgents(agentPath, agentsSource, false, os.Stdout)
		if collectErr != nil {
			ui.Warning("%s: collect failed: %v", entry.Name, collectErr)
			continue
		}
		if len(collected) > 0 {
			allCollected = append(allCollected, collected...)
			ui.Success("%s: collected %d agent(s)", entry.Name, len(collected))
		}
	}

	if len(allCollected) == 0 {
		ui.Info("No local agents found to collect")
	} else {
		fmt.Println()
		ui.Info("Collected %d agent(s) to %s", len(allCollected), agentsSource)
	}

	return nil
}
