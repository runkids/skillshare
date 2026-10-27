package sync

import (
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
