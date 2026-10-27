package sync

import (
	"fmt"
	"strings"

	"skillshare/internal/config"
)

// skillVariant recognizes only repository-relative skills/<known agent>/<suffix>.
// The outer recognized component owns the complete suffix, including nested skills.
func skillVariant(relPath, repo string) (id, agent string) {
	if repo == "" {
		return "", ""
	}
	parts := strings.Split(strings.TrimPrefix(relPath, repo+"/"), "/")
	for i := 0; i+2 < len(parts); i++ {
		if parts[i] != "skills" {
			continue
		}
		if agent = config.CanonicalTargetName(parts[i+1]); agent != "" {
			return repo + "\x00" + strings.Join(parts[:i+1], "/") + "\x00" + strings.Join(parts[i+2:], "/"), agent
		}
	}
	return "", ""
}

// SkillCounts distinguishes enabled logical skills from physical source entries.
type SkillCounts struct {
	Skills   int
	Variants int
}

// CountSkills summarizes enabled source inventory without dropping physical entries.
func CountSkills(skills []DiscoveredSkill) SkillCounts {
	counts, _ := CountSkillsByRepo(skills)
	return counts
}

// CountSkillsByRepo summarizes source and repository inventory using discovery identity.
func CountSkillsByRepo(skills []DiscoveredSkill) (SkillCounts, map[string]SkillCounts) {
	var total SkillCounts
	repos := make(map[string]SkillCounts)
	seen := make(map[string]bool)
	for _, skill := range skills {
		if skill.Disabled {
			continue
		}
		id := skill.LogicalID
		if id == "" {
			id = skill.RelPath
		}
		r := repos[skill.RepoRelPath]
		total.Variants++
		r.Variants++
		if !seen[id] {
			total.Skills++
			r.Skills++
			seen[id] = true
		}
		repos[skill.RepoRelPath] = r
	}
	return total, repos
}

func targetAgent(name string, sc config.ResourceTargetConfig) string {
	if sc.Agent != "" {
		name = sc.Agent
	}
	return config.CanonicalTargetName(name)
}

// variantRemovalWarning explains the upgrade effect only for removed entries
// whose known physical/naming IDs belong to automatically excluded variants.
func variantRemovalWarning(name string, sc config.ResourceTargetConfig, skills []DiscoveredSkill, pruned []string, dryRun bool) string {
	if len(pruned) == 0 {
		return ""
	}
	removed := make(map[string]bool, len(pruned))
	for _, entry := range pruned {
		removed[entry] = true
	}
	mismatches := make(map[string]bool)
	agent := targetAgent(name, sc)
	for _, skill := range skills {
		if !skill.variantRouting || skill.VariantAgent == agent || skill.Disabled {
			continue
		}
		if allowed, _ := ShouldSyncFlatName(skill.FlatName, sc.Include, sc.Exclude); !allowed {
			continue
		}
		for _, candidate := range targetNameCandidates(skill) {
			if removed[candidate.name] {
				mismatches[candidate.name] = true
			}
		}
	}
	if len(mismatches) == 0 {
		return ""
	}
	action := "removed"
	if dryRun {
		action = "would remove"
	}
	return fmt.Sprintf("%s: automatic agent routing %s %d managed variant entries because their agent does not match this target; set a source-local target override for the desired variant to restore it (an empty override means all targets)", name, action, len(mismatches))
}
