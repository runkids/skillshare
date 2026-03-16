package sync

import (
	"strings"

	"skillshare/internal/config"
)

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
