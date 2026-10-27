package sync

import (
	"strings"

	"skillshare/internal/config"
)

// SkillTargetClassification is a physical skill's selection result for previews.
type SkillTargetClassification struct{ Status, Reason string }

// ClassifySkillsForTarget evaluates the full source selection once so preview
// rows share exact-agent routing, account identity, and ambiguity handling with sync.
func ClassifySkillsForTarget(skills []DiscoveredSkill, name string, sc config.ResourceTargetConfig) (map[string]SkillTargetClassification, error) {
	selection, err := selectTargetSkills(skills, name, sc)
	if err != nil {
		return nil, err
	}
	selected := make(map[string]bool, len(selection.Skills))
	for _, skill := range selection.Skills {
		selected[skill.RelPath] = true
	}
	rows := make(map[string]SkillTargetClassification, len(skills))
	for _, skill := range skills {
		targets := skill.Targets
		// Inferred membership comes from the shared exact-agent selector below.
		if skill.variantRouting || selected[skill.RelPath] {
			targets = nil
		}
		status, reason := ClassifySkillForTarget(skill.FlatName, targets, name, sc.Include, sc.Exclude)
		if status == "synced" && !selected[skill.RelPath] {
			if warning, ok := selection.Skipped[skill.RelPath]; ok {
				status, reason = "skipped", warning
			} else if skill.Disabled {
				status, reason = "skipped", "disabled by .skillignore"
			} else {
				status, reason = "skill_target_mismatch", strings.Join(skill.Targets, ", ")
			}
		}
		rows[skill.RelPath] = SkillTargetClassification{Status: status, Reason: reason}
	}
	return rows, nil
}

// ClassifySkillForTarget determines whether a skill should sync to a target,
// returning a status string and an optional reason.
//
// Statuses:
//   - "synced": skill will be synced to this target
//   - "excluded": skill matched an exclude pattern (reason = the pattern)
//   - "not_included": include patterns exist but skill matched none
//   - "skill_target_mismatch": SKILL.md targets field excludes this target (reason = declared targets)
func ClassifySkillForTarget(flatName string, skillTargets []string, targetName string, include, exclude []string) (status, reason string) {
	// 1. Check SKILL.md targets field first
	if skillTargets != nil {
		matched := false
		for _, t := range skillTargets {
			if config.MatchesTargetName(t, targetName) {
				matched = true
				break
			}
		}
		if !matched {
			return "skill_target_mismatch", strings.Join(skillTargets, ", ")
		}
	}

	// Normalize patterns (trim whitespace, validate syntax) — same path as shouldSyncFlatName
	incNorm, excNorm, _ := normalizedFilterPatterns(include, exclude)

	// 2. Check include first (matches shouldSyncFlatName precedence)
	if len(incNorm) > 0 && !matchesAnyPattern(flatName, incNorm) {
		return "not_included", ""
	}

	// 3. Check exclude
	if pattern, matched := firstMatchingPattern(flatName, excNorm); matched {
		return "excluded", pattern
	}

	return "synced", ""
}
