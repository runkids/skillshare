package adopt

import (
	"os"
	"path/filepath"
	"sort"

	"skillshare/internal/sync"
	"skillshare/internal/utils"
)

// Candidate is a real (non-symlinked) skill living in the agents/universal
// target that can be adopted into skillshare's source of truth.
type Candidate struct {
	// Name is the skill directory name.
	Name string
	// Path is the absolute path of the skill directory in the agents target.
	Path string
	// SourceTool is the owning external tool, if recorded in the lockfile.
	SourceTool string
	// Conflict is true when a directory of the same name already exists in source.
	Conflict bool
	// ExternalLinks are symlinks in other target dirs that point into the
	// agents target for this skill (orphan symlinks the external tool created).
	ExternalLinks []string
}

// DetectAdoptable scans the agents/universal target for real skill directories
// that bypass skillshare's source-of-truth model.
//
//   - agentsPath:  the universal target skills dir (e.g. ~/.agents/skills)
//   - sourcePath:  skillshare's source skills dir
//   - syncMode:    the agents target's sync mode ("merge", "copy", "symlink")
//   - allTargets:  name -> skills dir for every configured target; used to find
//     orphan symlinks. The agents target itself (and its alias) is skipped.
//
// A missing agentsPath yields an empty slice and nil error. Conflict is marked
// when the skill name already exists in source (existence check, v1).
func DetectAdoptable(agentsPath, sourcePath, syncMode string, allTargets map[string]string) ([]Candidate, error) {
	locals, err := sync.FindLocalSkills(agentsPath, sourcePath, syncMode)
	if err != nil {
		return nil, err
	}

	absAgents, _ := filepath.Abs(agentsPath)

	candidates := make([]Candidate, 0, len(locals))
	for _, local := range locals {
		c := Candidate{
			Name: local.Name,
			Path: local.Path,
		}

		// Conflict: same-name dir already present in source (v1: existence check).
		if _, statErr := os.Stat(filepath.Join(sourcePath, local.Name)); statErr == nil {
			c.Conflict = true
		}

		c.ExternalLinks = findExternalLinks(local.Name, absAgents, allTargets)
		candidates = append(candidates, c)
	}

	sort.Slice(candidates, func(i, j int) bool {
		return candidates[i].Name < candidates[j].Name
	})

	return candidates, nil
}

// findExternalLinks scans every target dir (except the agents target itself)
// for a symlink named skillName that resolves into absAgents. Missing target
// dirs are skipped.
func findExternalLinks(skillName, absAgents string, allTargets map[string]string) []string {
	var links []string
	for _, targetPath := range allTargets {
		absTarget, _ := filepath.Abs(targetPath)
		// Skip the agents/universal target itself (any alias mapping to it).
		if utils.PathsEqual(absTarget, absAgents) {
			continue
		}

		linkPath := filepath.Join(targetPath, skillName)
		info, err := os.Lstat(linkPath)
		if err != nil || info.Mode()&os.ModeSymlink == 0 {
			continue
		}

		resolved, err := utils.ResolveLinkTarget(linkPath)
		if err != nil {
			continue
		}
		// The symlink targets this skill inside the agents dir.
		if utils.PathHasPrefix(resolved, absAgents) {
			links = append(links, linkPath)
		}
	}
	sort.Strings(links)
	return links
}
