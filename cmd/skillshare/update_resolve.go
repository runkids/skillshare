package main

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"skillshare/internal/install"
	"skillshare/internal/utils"
)

type updateTarget struct {
	name   string             // relative path from source dir (display name)
	path   string             // absolute path on disk
	isRepo bool               // true for tracked repos (_-prefixed git repos)
	meta   *install.SkillMeta // cached metadata; nil for tracked repos
}

// resolveByBasename searches nested skills and tracked repos by their
// directory basename. Returns an error when zero or multiple matches found.
func resolveByBasename(sourceDir, name string) (updateTarget, error) {
	var matches []updateTarget

	// Search tracked repos
	repos, _ := install.GetTrackedRepos(sourceDir)
	for _, r := range repos {
		if filepath.Base(r) == "_"+name || filepath.Base(r) == name {
			matches = append(matches, updateTarget{name: r, path: filepath.Join(sourceDir, r), isRepo: true})
		}
	}

	// Search updatable skills
	skills, _ := install.GetUpdatableSkills(sourceDir)
	for _, s := range skills {
		if filepath.Base(s) == name {
			matches = append(matches, updateTarget{name: s, path: filepath.Join(sourceDir, s), isRepo: false})
		}
	}

	if len(matches) == 0 {
		return updateTarget{}, fmt.Errorf("'%s' not found as tracked repo or skill with metadata", name)
	}
	if len(matches) == 1 {
		return matches[0], nil
	}

	// Ambiguous: list all matches
	lines := []string{fmt.Sprintf("'%s' matches multiple items:", name)}
	for _, m := range matches {
		lines = append(lines, fmt.Sprintf("  - %s", m.name))
	}
	lines = append(lines, "Please specify the full path")
	return updateTarget{}, fmt.Errorf("%s", strings.Join(lines, "\n"))
}

// resolveByGlob searches tracked repos and updatable skills whose basenames
// match the given glob pattern (e.g. "core-*", "_team-?"). Returns all matches
// sorted by name.
func resolveByGlob(sourceDir, pattern string) ([]updateTarget, error) {
	var matches []updateTarget

	repos, _ := install.GetTrackedRepos(sourceDir)
	for _, r := range repos {
		if matchGlob(pattern, filepath.Base(r)) {
			matches = append(matches, updateTarget{name: r, path: filepath.Join(sourceDir, r), isRepo: true})
		}
	}

	skills, _ := install.GetUpdatableSkills(sourceDir)
	for _, s := range skills {
		if matchGlob(pattern, filepath.Base(s)) {
			matches = append(matches, updateTarget{name: s, path: filepath.Join(sourceDir, s), isRepo: false})
		}
	}

	sort.Slice(matches, func(i, j int) bool { return matches[i].name < matches[j].name })
	return matches, nil
}

// resolveGroupUpdatable finds all updatable items (tracked repos or skills with
// metadata) under a group directory. Local skills without metadata are skipped.
func resolveGroupUpdatable(group, sourceDir string) ([]updateTarget, error) {
	group = strings.TrimSuffix(group, "/")
	groupPath := filepath.Join(sourceDir, group)

	info, err := os.Stat(groupPath)
	if err != nil || !info.IsDir() {
		return nil, fmt.Errorf("group '%s' not found in source", group)
	}

	walkRoot := utils.ResolveSymlink(groupPath)
	resolvedSourceDir := utils.ResolveSymlink(sourceDir)

	// Guard: walkRoot must be inside resolvedSourceDir to prevent
	// symlinked groups from reaching outside the source tree.
	if srcRel, err := filepath.Rel(resolvedSourceDir, walkRoot); err != nil || strings.HasPrefix(srcRel, "..") {
		return nil, fmt.Errorf("group '%s' resolves outside source directory", group)
	}

	var matches []updateTarget
	if walkErr := filepath.Walk(walkRoot, func(path string, fi os.FileInfo, err error) error {
		if err != nil {
			return nil
		}
		if path == walkRoot || !fi.IsDir() {
			return nil
		}
		if fi.Name() == ".git" {
			return filepath.SkipDir
		}

		rel, relErr := filepath.Rel(resolvedSourceDir, path)
		if relErr != nil || rel == "." || strings.HasPrefix(rel, "..") {
			return nil
		}

		// Tracked repo (has .git)
		if install.IsGitRepo(path) {
			matches = append(matches, updateTarget{name: rel, path: path, isRepo: true})
			return filepath.SkipDir
		}

		// Skill with metadata (has .skillshare-meta.json)
		if meta, metaErr := install.ReadMeta(path); metaErr == nil && meta != nil && meta.Source != "" {
			matches = append(matches, updateTarget{name: rel, path: path, isRepo: false, meta: meta})
			return filepath.SkipDir
		}

		return nil
	}); walkErr != nil {
		return nil, fmt.Errorf("failed to walk group '%s': %w", group, walkErr)
	}

	return matches, nil
}

// isGroupDir checks if a name corresponds to a group directory (a container
// for other skills). Returns false for tracked repos, skills with metadata,
// and directories that are themselves a skill (have SKILL.md).
func isGroupDir(name, sourceDir string) bool {
	path := filepath.Join(sourceDir, name)
	info, err := os.Stat(path)
	if err != nil || !info.IsDir() {
		return false
	}
	// Not a tracked repo
	if install.IsGitRepo(path) {
		return false
	}
	// Not a skill with metadata
	if meta, metaErr := install.ReadMeta(path); metaErr == nil && meta != nil && meta.Source != "" {
		return false
	}
	// Not a skill directory (has SKILL.md)
	if _, statErr := os.Stat(filepath.Join(path, "SKILL.md")); statErr == nil {
		return false
	}
	return true
}
