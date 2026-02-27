package install

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

func getUpdatableSkillsImpl(sourceDir string) ([]string, error) {
	var skills []string

	err := filepath.Walk(sourceDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return nil
		}
		if path == sourceDir {
			return nil
		}
		// Skip .git directories
		if info.IsDir() && info.Name() == ".git" {
			return filepath.SkipDir
		}
		// Skip tracked repo directories (start with _)
		if info.IsDir() && len(info.Name()) > 0 && info.Name()[0] == '_' {
			return filepath.SkipDir
		}
		// Look for metadata files
		if !info.IsDir() && info.Name() == metaFileName {
			skillDir := filepath.Dir(path)
			relPath, relErr := filepath.Rel(sourceDir, skillDir)
			if relErr != nil || relPath == "." {
				return nil
			}
			meta, metaErr := ReadMeta(skillDir)
			if metaErr != nil || meta == nil || meta.Source == "" {
				return nil
			}
			skills = append(skills, relPath)
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return skills, nil
}

// FindRepoInstalls scans sourceDir for skills whose meta repo_url matches
// cloneURL. Returns relative paths (e.g. "feature-radar/feature-radar-archive").
// Tracked repos (_-prefixed) are skipped.
func FindRepoInstalls(sourceDir, cloneURL string) []string {
	if cloneURL == "" {
		return nil
	}
	var matches []string

	filepath.Walk(sourceDir, func(path string, info os.FileInfo, err error) error {
		if err != nil || path == sourceDir {
			return nil
		}
		if info.IsDir() && info.Name() == ".git" {
			return filepath.SkipDir
		}
		if info.IsDir() && len(info.Name()) > 0 && info.Name()[0] == '_' {
			return filepath.SkipDir
		}
		if !info.IsDir() && info.Name() == metaFileName {
			skillDir := filepath.Dir(path)
			relPath, relErr := filepath.Rel(sourceDir, skillDir)
			if relErr != nil || relPath == "." {
				return nil
			}
			meta, metaErr := ReadMeta(skillDir)
			if metaErr != nil || meta == nil {
				return nil
			}
			if repoURLsMatch(meta.RepoURL, cloneURL) {
				matches = append(matches, relPath)
			}
		}
		return nil
	})
	return matches
}

// CheckCrossPathDuplicate checks if a repo is already installed at a different
// location in sourceDir. Returns a user-facing error when duplicates are found
// outside targetPrefix, or nil if safe to proceed.
// Callers should skip this check when force is true or cloneURL is empty.
func CheckCrossPathDuplicate(sourceDir, cloneURL, targetPrefix string) error {
	existing := FindRepoInstalls(sourceDir, cloneURL)
	if len(existing) == 0 {
		return nil
	}
	var elsewhere []string
	for _, rel := range existing {
		sameLocation := false
		if targetPrefix == "" {
			sameLocation = !strings.Contains(rel, "/")
		} else {
			sameLocation = rel == targetPrefix || strings.HasPrefix(rel, targetPrefix+"/")
		}
		if !sameLocation {
			elsewhere = append(elsewhere, rel)
		}
	}
	if len(elsewhere) == 0 {
		return nil
	}
	loc := elsewhere[0]
	if len(elsewhere) > 1 {
		loc = fmt.Sprintf("%s (and %d more)", loc, len(elsewhere)-1)
	}
	return fmt.Errorf(
		"this repo is already installed at skills/%s\n"+
			"Use 'skillshare update' to refresh, or reinstall with --force to allow duplicates",
		loc)
}

// getTrackedReposImpl returns tracked repositories from the source directory.
// It walks subdirectories recursively so repos nested in organizational
// directories (e.g. category/_team-repo/) are found.

func getTrackedReposImpl(sourceDir string) ([]string, error) {
	var repos []string

	err := filepath.Walk(sourceDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return nil
		}
		if path == sourceDir {
			return nil
		}
		// Skip .git directories
		if info.IsDir() && info.Name() == ".git" {
			return filepath.SkipDir
		}
		// Look for _-prefixed directories that are git repos
		if info.IsDir() && len(info.Name()) > 0 && info.Name()[0] == '_' {
			gitDir := filepath.Join(path, ".git")
			if _, statErr := os.Stat(gitDir); statErr == nil {
				relPath, relErr := filepath.Rel(sourceDir, path)
				if relErr == nil {
					repos = append(repos, relPath)
				}
				return filepath.SkipDir // Don't recurse into tracked repos
			}
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return repos, nil
}
