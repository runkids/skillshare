package sync

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"skillshare/internal/config"
	"skillshare/internal/sourcewalk"
)

// confirmSkillVariants requires a matching suffix under a second canonical
// agent. Check the source tree so ignoring one peer does not turn the remaining
// agent's instructions into a shared skill.
func confirmSkillVariants(skills []DiscoveredSkill, source string) {
	confirmed := make(map[string]bool)
	checked := make(map[string]bool)
	for i := range skills {
		sk := &skills[i]
		if sk.LogicalID == "" {
			continue
		}
		parts := strings.Split(sk.LogicalID, "\x00")
		agentRoot := filepath.Join(source, filepath.FromSlash(strings.TrimSuffix(sk.RelPath, "/"+parts[2])))
		parentIsSkill := hasVariantParentSkill(agentRoot)
		if !parentIsSkill && !checked[sk.LogicalID] {
			checked[sk.LogicalID] = true
			root := filepath.Join(source, parts[0], filepath.FromSlash(parts[1]))
			agents, _ := sourcewalk.ReadDir(root, sourcewalk.Options{})
			for _, entry := range agents {
				if !entry.IsDir() || hasVariantParentSkill(filepath.Join(root, entry.Name())) {
					continue
				}
				agent := config.CanonicalTargetName(entry.Name())
				if agent == "" || agent == sk.VariantAgent {
					continue
				}
				if info, err := os.Stat(filepath.Join(root, entry.Name(), filepath.FromSlash(parts[2]), "SKILL.md")); err == nil && !info.IsDir() {
					confirmed[sk.LogicalID] = true
					break
				}
			}
		}
		if parentIsSkill || !confirmed[sk.LogicalID] {
			sk.LogicalID, sk.VariantAgent = "", ""
			if sk.variantRouting {
				sk.Targets = inferSkillTargetsFromRelPath(sk.RelPath)
				sk.variantRouting = false
			}
		}
	}
}

// Agent-named skills own their nested skills; they are not variant containers.
func hasVariantParentSkill(dir string) bool {
	info, err := os.Stat(filepath.Join(dir, "SKILL.md"))
	return err == nil && !info.IsDir()
}

// skillVariant recognizes only repository-relative skills/<known agent>/<suffix>.
// The outer recognized component owns the complete suffix, including nested skills,
// unless the agent directory is itself a skill (checked by confirmSkillVariants).
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

// UsesVariantRouting reports whether agent routing was inferred from the layout.
// Explicit frontmatter targets and source-local overrides retain their own policy.
func (s DiscoveredSkill) UsesVariantRouting() bool { return s.variantRouting }

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
	selection, err := selectTargetSkills(skills, name, sc)
	if err != nil {
		return ""
	}
	selected := make(map[string]bool, len(selection.Skills))
	for _, skill := range selection.Skills {
		selected[skill.RelPath] = true
	}
	for _, skill := range skills {
		if !skill.variantRouting || selected[skill.RelPath] || skill.Disabled {
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
	return fmt.Sprintf("%s: automatic agent routing %s %d managed variant entries because their agent does not match this folder's targets or their selection is ambiguous; use a source-local target override to select the desired variant (an empty override means all targets)", name, action, len(mismatches))
}
