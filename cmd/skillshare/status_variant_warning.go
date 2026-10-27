package main

import (
	"strings"

	"skillshare/internal/config"
	"skillshare/internal/sourcewalk"
	"skillshare/internal/sync"
	"skillshare/internal/ui"
)

// variantWarningSkills includes ignored files only for a whole-directory link.
// Keep this diagnostic inventory separate from counts and merge/copy selection.
func variantWarningSkills(source string, targets map[string]config.TargetConfig, mode string, walk sourcewalk.Options, discovered []sync.DiscoveredSkill) []sync.DiscoveredSkill {
	for _, skill := range discovered {
		if skill.VariantAgent != "" {
			return discovered
		}
	}
	for _, target := range targets {
		if target.SkillsConfig().IsEnabled() && getTargetMode(target.SkillsConfig().Mode, mode) == "symlink" {
			if all, err := sync.DiscoverSourceSkillsAll(source, walk); err == nil {
				return all
			}
			break
		}
	}
	return discovered
}

func variantSymlinkWarning(target config.TargetConfig, mode string, discovered []sync.DiscoveredSkill) string {
	if mode != "symlink" || !target.SkillsConfig().IsEnabled() {
		return ""
	}
	for _, skill := range discovered {
		if skill.VariantAgent != "" {
			return "symlink mode exposes all agent variants; automatic agent routing requires merge or copy mode"
		}
	}
	return ""
}

func skillTargetWarning(target config.TargetConfig, mode string, discovered []sync.DiscoveredSkill) string {
	var warnings []string
	if warning := namingWarning(target, mode); warning != "" {
		warnings = append(warnings, warning)
	}
	if warning := variantSymlinkWarning(target, mode, discovered); warning != "" {
		warnings = append(warnings, warning)
	}
	return strings.Join(warnings, "; ")
}

func checkVariantRouting(cfg *config.Config, result *doctorResult, discovered []sync.DiscoveredSkill) {
	discovered = variantWarningSkills(cfg.EffectiveSkillsSource(), cfg.Targets, cfg.Mode, cfg.SkillsWalk(), discovered)
	var details []string
	for _, name := range targetNamesFromConfig(cfg.Targets) {
		target := cfg.Targets[name]
		mode := getTargetMode(target.SkillsConfig().Mode, cfg.Mode)
		if warning := variantSymlinkWarning(target, mode, discovered); warning != "" {
			ui.Row(ui.MarkWarn, name, warning, doctorWidth)
			result.addWarning()
			details = append(details, name+": "+warning)
		}
	}
	if len(details) > 0 {
		result.addCheck("variant_routing", checkWarning, "Agent routing unavailable in symlink mode", details)
	}
}
