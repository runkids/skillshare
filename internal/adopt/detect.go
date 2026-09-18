package adopt

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"skillshare/internal/sync"
	"skillshare/internal/utils"
)

// ErrUnsafePathOverlap marks a configuration where adopting could overwrite or
// trash files inside the skills source itself.
var ErrUnsafePathOverlap = errors.New("agents target overlaps skills source")

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
	if _, err := os.Lstat(agentsPath); err != nil {
		if os.IsNotExist(err) {
			return []Candidate{}, nil
		}
		return nil, err
	}
	if err := validateSeparateRoots(agentsPath, sourcePath); err != nil {
		return nil, err
	}

	locals, err := sync.FindLocalSkills(agentsPath, sourcePath, syncMode)
	if err != nil {
		return nil, err
	}

	absAgents, _ := filepath.Abs(agentsPath)
	if canonicalAgents, err := filepath.EvalSymlinks(absAgents); err == nil {
		absAgents = canonicalAgents
	}

	candidates := make([]Candidate, 0, len(locals))
	for _, local := range locals {
		skillFile, err := os.Lstat(filepath.Join(local.Path, "SKILL.md"))
		if err != nil || !skillFile.Mode().IsRegular() {
			continue
		}
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

func validateSeparateRoots(agentsPath, sourcePath string) error {
	if strings.TrimSpace(agentsPath) == "" || strings.TrimSpace(sourcePath) == "" {
		return fmt.Errorf("%w: both paths must be configured", ErrUnsafePathOverlap)
	}
	agentsRoot := effectiveTargetPath(agentsPath)
	sourceRoot := effectiveTargetPath(sourcePath)
	if utils.PathWithin(agentsRoot, sourceRoot) || utils.PathWithin(sourceRoot, agentsRoot) {
		return fmt.Errorf("%w: agents=%s source=%s", ErrUnsafePathOverlap, agentsPath, sourcePath)
	}
	return nil
}

// findExternalLinks scans every target dir (except the agents target itself)
// for a symlink named skillName that resolves into absAgents. Missing target
// dirs are skipped.
func findExternalLinks(skillName, absAgents string, allTargets map[string]string) []string {
	var links []string
	candidateRoot := effectiveTargetPath(filepath.Join(absAgents, skillName))
	for _, targetPath := range uniqueTargetPaths(allTargets) {
		absTarget := effectiveTargetPath(targetPath)
		// Skip the agents/universal target itself (any alias mapping to it).
		if utils.PathsEqual(absTarget, absAgents) {
			continue
		}

		linkPath := filepath.Join(targetPath, skillName)
		if !utils.IsSymlinkOrJunction(linkPath) {
			continue
		}

		resolved, err := utils.ResolveLinkTarget(linkPath)
		if err != nil {
			continue
		}
		if canonicalTarget, err := filepath.EvalSymlinks(resolved); err == nil {
			resolved = canonicalTarget
		}
		// Cleanup later removes a link only when it still targets this exact
		// candidate, so preview uses the same identity rule.
		if utils.PathsEqual(resolved, candidateRoot) {
			links = append(links, linkPath)
		}
	}
	sort.Strings(links)
	return links
}

// uniqueTargetPaths returns one deterministic representative for each
// effective target directory. Config aliases can use different path spellings
// (including symlinked parents) while still addressing the same directory.
func uniqueTargetPaths(allTargets map[string]string) []string {
	names := make([]string, 0, len(allTargets))
	for name := range allTargets {
		names = append(names, name)
	}
	sort.Strings(names)

	paths := make([]string, 0, len(names))
	seen := make([]string, 0, len(names))
	for _, name := range names {
		path := allTargets[name]
		effective := effectiveTargetPath(path)
		duplicate := false
		for _, existing := range seen {
			if utils.PathsEqual(existing, effective) {
				duplicate = true
				break
			}
		}
		if duplicate {
			continue
		}
		seen = append(seen, effective)
		paths = append(paths, path)
	}
	return paths
}

func effectiveTargetPath(path string) string {
	abs, err := filepath.Abs(path)
	if err != nil {
		abs = filepath.Clean(path)
	}
	if canonical, err := filepath.EvalSymlinks(abs); err == nil {
		return filepath.Clean(canonical)
	}

	// Resolve a symlinked existing ancestor even when the final path does not
	// exist yet. Otherwise aliases such as macOS /var -> /private/var can bypass
	// overlap checks during first-run setup.
	tail := ""
	current := abs
	for {
		parent := filepath.Dir(current)
		if parent == current {
			break
		}
		tail = filepath.Join(filepath.Base(current), tail)
		if canonical, err := filepath.EvalSymlinks(parent); err == nil {
			return filepath.Clean(filepath.Join(canonical, tail))
		}
		current = parent
	}
	return filepath.Clean(abs)
}
