package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"skillshare/internal/config"
	"skillshare/internal/resource"
	"skillshare/internal/sync"
	"skillshare/internal/ui"
	"skillshare/internal/utils"
)

// checkAgentTargetInline validates the agent target for a single target,
// printing as an indented sub-item under the target name in doctor output.
// It applies the target's include/exclude filters to compute the expected count.
func checkAgentTargetInline(name string, target config.TargetConfig, builtinAgents map[string]config.TargetConfig, allAgents []resource.DiscoveredResource, result *doctorResult, row func(mark, kind, text string)) {
	agentPath := resolveAgentTargetPath(target, builtinAgents, name)
	if agentPath == "" {
		return
	}

	ac := target.AgentsConfig()
	mode, _ := agentStatusLabel(ac)

	// Apply per-target include/exclude filters to get expected agent count
	filtered, filterErr := sync.FilterAgents(allAgents, ac.Include, ac.Exclude)
	if filterErr != nil {
		row(ui.MarkFail, "agents", "invalid filter: "+filterErr.Error()+ui.DimText(" · "+mode))
		result.addError()
		result.addCheck("agent_target_"+name, checkError,
			fmt.Sprintf("Agent target %s: invalid filter: %v", name, filterErr), nil)
		return
	}
	expected := sync.FilterAgentsByTarget(filtered, name)
	agentCount := len(expected)

	// Build details for JSON output
	var details []string
	details = append(details, fmt.Sprintf("path: %s", agentPath))
	details = append(details, fmt.Sprintf("mode: %s", mode))
	if len(ac.Include) > 0 {
		details = append(details, fmt.Sprintf("include: %s", strings.Join(ac.Include, ", ")))
	}
	if len(ac.Exclude) > 0 {
		details = append(details, fmt.Sprintf("exclude: %s", strings.Join(ac.Exclude, ", ")))
	}

	info, err := os.Stat(agentPath)
	if err != nil {
		if os.IsNotExist(err) {
			row(ui.MarkNone, "agents", ui.DimText("not created · "+mode))
			result.addCheck("agent_target_"+name, checkPass,
				fmt.Sprintf("Agent target %s: not created yet", name), details)
			return
		}
		row(ui.MarkFail, "agents", "error: "+err.Error()+ui.DimText(" · "+mode))
		result.addError()
		result.addCheck("agent_target_"+name, checkError,
			fmt.Sprintf("Agent target %s: %v", name, err), details)
		return
	}

	if !info.IsDir() {
		row(ui.MarkFail, "agents", "not a directory"+ui.DimText(" · "+mode))
		result.addError()
		result.addCheck("agent_target_"+name, checkError,
			fmt.Sprintf("Agent target %s: path is not a directory", name), details)
		return
	}

	preserved := 0
	linked, broken := countAgentLinksAndBroken(agentPath)
	if ac.Extension != "" {
		// Outputs are converted copies; a leftover link still counts as broken.
		linked = sync.SyncedExtensionOutputs(agentPath, expected)
	} else {
		linked += sync.SyncedAgentCopies(agentPath, expected, &preserved)
	}
	countLabel := agentCountLabel(linked, agentCount, preserved)
	if broken > 0 {
		msg := fmt.Sprintf("[%s] %s, %d broken", mode, countLabel, broken)
		row(ui.MarkWarn, "agents", fmt.Sprintf("%d broken", broken)+ui.DimText(" · "+mode+" · "+countLabel))
		result.addWarning()
		result.addCheck("agent_target_"+name, checkWarning,
			fmt.Sprintf("Agent target %s: %s", name, msg), details)
		return
	}

	if linked != agentCount && agentCount > 0 {
		row(ui.MarkWarn, "agents", "drift"+ui.DimText(" · "+mode+" · "+countLabel))
		result.suggest("skillshare sync agents", "sync the missing agents")
		result.addWarning()
		result.addCheck("agent_target_"+name, checkWarning,
			fmt.Sprintf("Agent target %s: drift (%d/%d agents linked)", name, linked, agentCount), details)
		return
	}

	row(ui.MarkOK, "agents", "synced"+ui.DimText(" · "+mode+" · "+countLabel))
	result.addCheck("agent_target_"+name, checkPass,
		fmt.Sprintf("Agent target %s: %d agents synced", name, linked), details)
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
		if !utils.IsLinkMode(filepath.Join(dir, e.Name()), e.Type()) {
			continue
		}
		// It's a symlink — check if target exists (os.Stat follows symlinks)
		if _, statErr := os.Stat(filepath.Join(dir, e.Name())); statErr != nil {
			broken++
		} else {
			linked++
		}
	}
	return linked, broken
}
