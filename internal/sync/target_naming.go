package sync

import (
	"fmt"
	"path/filepath"
	"slices"
	"strings"

	"skillshare/internal/config"
	"skillshare/internal/utils"
)

// ResolvedTargetSkill represents a discovered skill together with the target
// entry name that should be used for a specific target.
type ResolvedTargetSkill struct {
	Skill      DiscoveredSkill
	TargetName string
	SkillName  string
}

// TargetSkillResolution contains the resolved target-visible skills for one
// target after include/exclude filters, target filters, standard validation,
// and collision handling are applied.
type TargetSkillResolution struct {
	Naming     string
	Skills     []ResolvedTargetSkill
	Warnings   []string
	Collisions []NameCollision
}

// ResolveTargetSkillsForTarget applies a target's filters and target_naming
// policy to discovered skills and returns the effective target-visible skills.
func ResolveTargetSkillsForTarget(targetName string, sc config.ResourceTargetConfig, allSkills []DiscoveredSkill) (*TargetSkillResolution, error) {
	filtered, err := FilterSkills(allSkills, sc.Include, sc.Exclude)
	if err != nil {
		return nil, fmt.Errorf("failed to apply filters for target %s: %w", targetName, err)
	}
	filtered = FilterSkillsByTarget(filtered, targetName)

	naming := config.EffectiveTargetNaming(sc.TargetNaming)
	result := &TargetSkillResolution{Naming: naming}

	if naming == "flat" {
		result.Skills = make([]ResolvedTargetSkill, 0, len(filtered))
		for _, skill := range filtered {
			result.Skills = append(result.Skills, ResolvedTargetSkill{
				Skill:      skill,
				TargetName: skill.FlatName,
			})
		}
		return result, nil
	}

	candidates := make([]ResolvedTargetSkill, 0, len(filtered))
	collisionMap := make(map[string][]string)

	for _, skill := range filtered {
		skillName, nameErr := utils.ParseSkillName(skill.SourcePath)
		if nameErr != nil {
			result.Warnings = append(result.Warnings,
				fmt.Sprintf("Target '%s': skipped %s because SKILL.md name could not be read: %v", targetName, skill.RelPath, nameErr))
			continue
		}

		if reason := validateStandardTargetSkill(skill, skillName); reason != "" {
			result.Warnings = append(result.Warnings,
				fmt.Sprintf("Target '%s': skipped %s because %s", targetName, skill.RelPath, reason))
			continue
		}

		candidates = append(candidates, ResolvedTargetSkill{
			Skill:      skill,
			TargetName: skillName,
			SkillName:  skillName,
		})
		collisionMap[skillName] = append(collisionMap[skillName], skill.RelPath)
	}

	collisionNames := make(map[string]bool)
	for name, paths := range collisionMap {
		if len(paths) <= 1 {
			continue
		}
		slices.Sort(paths)
		collisionNames[name] = true
		result.Collisions = append(result.Collisions, NameCollision{
			Name:  name,
			Paths: paths,
		})
	}

	if len(result.Collisions) > 0 {
		slices.SortFunc(result.Collisions, func(a, b NameCollision) int {
			return strings.Compare(a.Name, b.Name)
		})
	}

	result.Skills = make([]ResolvedTargetSkill, 0, len(candidates))
	for _, candidate := range candidates {
		if collisionNames[candidate.TargetName] {
			continue
		}
		result.Skills = append(result.Skills, candidate)
	}

	return result, nil
}

// ValidTargetNames returns the target-visible names that are valid for the
// current target after naming resolution.
func (r *TargetSkillResolution) ValidTargetNames() map[string]bool {
	names := make(map[string]bool, len(r.Skills))
	for _, skill := range r.Skills {
		names[skill.TargetName] = true
	}
	return names
}

// LegacyFlatNames returns the old flat names for skills that now use a
// different target-visible name under standard naming.
func (r *TargetSkillResolution) LegacyFlatNames() map[string]ResolvedTargetSkill {
	legacy := make(map[string]ResolvedTargetSkill)
	if r == nil || r.Naming != "standard" {
		return legacy
	}
	for _, skill := range r.Skills {
		if skill.TargetName == skill.Skill.FlatName {
			continue
		}
		legacy[skill.Skill.FlatName] = skill
	}
	return legacy
}

func validateStandardTargetSkill(skill DiscoveredSkill, skillName string) string {
	if skillName == "" {
		return "SKILL.md is missing a name"
	}
	if len(skillName) > 64 {
		return fmt.Sprintf("SKILL.md name %q is longer than 64 characters", skillName)
	}
	if strings.HasPrefix(skillName, "-") || strings.HasSuffix(skillName, "-") {
		return fmt.Sprintf("SKILL.md name %q cannot start or end with '-'", skillName)
	}
	if strings.Contains(skillName, "--") {
		return fmt.Sprintf("SKILL.md name %q cannot contain consecutive hyphens", skillName)
	}
	for _, r := range skillName {
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') || r == '-' {
			continue
		}
		return fmt.Sprintf("SKILL.md name %q must use only lowercase letters, numbers, and hyphens", skillName)
	}

	dirName := filepath.Base(filepath.Clean(skill.SourcePath))
	if skillName != dirName {
		return fmt.Sprintf("SKILL.md name %q does not match directory name %q", skillName, dirName)
	}

	return ""
}
