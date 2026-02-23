package install

import (
	"os"
	"path/filepath"
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

// GetTrackedRepos returns a list of tracked repositories in the source directory.
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
