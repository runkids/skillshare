package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"skillshare/internal/config"
	"skillshare/internal/resource"
	"skillshare/internal/ui"
)

// checkAgentTargets validates agent target paths, broken links, and drift.
func checkAgentTargets(cfg *config.Config, result *doctorResult) {
	agentsSource := cfg.EffectiveAgentsSource()
	if _, err := os.Stat(agentsSource); err != nil {
		return // no agents source → skip target checks
	}

	agents, discErr := resource.AgentKind{}.Discover(agentsSource)
	if discErr != nil || len(agents) == 0 {
		return
	}

	builtinAgents := config.DefaultAgentTargets()

	for name := range cfg.Targets {
		agentPath := resolveAgentTargetPath(cfg.Targets[name], builtinAgents, name)
		if agentPath == "" {
			continue
		}

		info, err := os.Stat(agentPath)
		if err != nil {
			if os.IsNotExist(err) {
				ui.Info("Agent target %s: %s (not created)", name, agentPath)
				result.addCheck("agent_target_"+name, checkPass,
					fmt.Sprintf("Agent target %s: not created yet", name), nil)
				continue
			}
			ui.Error("Agent target %s: %v", name, err)
			result.addError()
			result.addCheck("agent_target_"+name, checkError,
				fmt.Sprintf("Agent target %s: %v", name, err), nil)
			continue
		}

		if !info.IsDir() {
			ui.Error("Agent target %s: not a directory: %s", name, agentPath)
			result.addError()
			result.addCheck("agent_target_"+name, checkError,
				fmt.Sprintf("Agent target %s: path is not a directory", name), nil)
			continue
		}

		// Count linked agents and check for broken links
		linked, broken := countAgentLinksAndBroken(agentPath)
		if broken > 0 {
			ui.Warning("Agent target %s: %d broken link(s)", name, broken)
			result.addWarning()
			result.addCheck("agent_target_"+name, checkWarning,
				fmt.Sprintf("Agent target %s: %d linked, %d broken", name, linked, broken), nil)
			continue
		}

		if linked != len(agents) {
			ui.Warning("Agent target %s: drift (%d/%d linked)", name, linked, len(agents))
			result.addWarning()
			result.addCheck("agent_target_"+name, checkWarning,
				fmt.Sprintf("Agent target %s: drift (%d/%d agents linked)", name, linked, len(agents)), nil)
			continue
		}

		ui.Success("Agent target %s: %s (%d agents)", name, agentPath, linked)
		result.addCheck("agent_target_"+name, checkPass,
			fmt.Sprintf("Agent target %s: %d agents synced", name, linked), nil)
	}
}

// countAgentLinksAndBroken counts .md symlinks and broken symlinks in a directory.
func countAgentLinksAndBroken(dir string) (linked, broken int) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return 0, 0
	}
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		if !strings.HasSuffix(strings.ToLower(e.Name()), ".md") {
			continue
		}
		fullPath := filepath.Join(dir, e.Name())
		fi, lErr := os.Lstat(fullPath)
		if lErr != nil {
			continue
		}
		if fi.Mode()&os.ModeSymlink == 0 {
			continue
		}
		// It's a symlink — check if target exists
		if _, statErr := os.Stat(fullPath); statErr != nil {
			broken++
		} else {
			linked++
		}
	}
	return linked, broken
}
