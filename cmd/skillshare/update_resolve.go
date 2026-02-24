package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"skillshare/internal/install"
)

type resolvedMatch struct {
	relPath string
	isRepo  bool
}

// resolveByBasename searches nested skills and tracked repos by their
// directory basename. Returns an error when zero or multiple matches found.
func resolveByBasename(sourceDir, name string) (resolvedMatch, error) {
	var matches []resolvedMatch

	// Search tracked repos
	repos, _ := install.GetTrackedRepos(sourceDir)
	for _, r := range repos {
		if filepath.Base(r) == "_"+name || filepath.Base(r) == name {
			matches = append(matches, resolvedMatch{relPath: r, isRepo: true})
		}
	}

	// Search updatable skills
	skills, _ := install.GetUpdatableSkills(sourceDir)
	for _, s := range skills {
		if filepath.Base(s) == name {
			matches = append(matches, resolvedMatch{relPath: s, isRepo: false})
		}
	}

	if len(matches) == 0 {
		return resolvedMatch{}, fmt.Errorf("'%s' not found as tracked repo or skill with metadata", name)
	}
	if len(matches) == 1 {
		return matches[0], nil
	}

	// Ambiguous: list all matches
	lines := []string{fmt.Sprintf("'%s' matches multiple items:", name)}
	for _, m := range matches {
		lines = append(lines, fmt.Sprintf("  - %s", m.relPath))
	}
	lines = append(lines, "Please specify the full path")
	return resolvedMatch{}, fmt.Errorf("%s", strings.Join(lines, "\n"))
}

// resolveGroupUpdatable finds all updatable items (tracked repos or skills with
// metadata) under a group directory. Local skills without metadata are skipped.
func resolveGroupUpdatable(group, sourceDir string) ([]resolvedMatch, error) {
	group = strings.TrimSuffix(group, "/")
	groupPath := filepath.Join(sourceDir, group)

	info, err := os.Stat(groupPath)
	if err != nil || !info.IsDir() {
		return nil, fmt.Errorf("group '%s' not found in source", group)
	}

	var matches []resolvedMatch
	if walkErr := filepath.Walk(groupPath, func(path string, fi os.FileInfo, err error) error {
		if err != nil {
			return nil
		}
		if path == groupPath || !fi.IsDir() {
			return nil
		}
		if fi.Name() == ".git" {
			return filepath.SkipDir
		}

		rel, relErr := filepath.Rel(sourceDir, path)
		if relErr != nil || rel == "." {
			return nil
		}

		// Tracked repo (has .git)
		if install.IsGitRepo(path) {
			matches = append(matches, resolvedMatch{relPath: rel, isRepo: true})
			return filepath.SkipDir
		}

		// Skill with metadata (has .skillshare-meta.json)
		if meta, metaErr := install.ReadMeta(path); metaErr == nil && meta != nil && meta.Source != "" {
			matches = append(matches, resolvedMatch{relPath: rel, isRepo: false})
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
