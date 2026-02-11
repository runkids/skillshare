package config

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"skillshare/internal/install"
	"skillshare/internal/utils"
)

// ReconcileProjectSkills scans the project source directory recursively for
// remotely-installed skills (those with install metadata or tracked repos)
// and ensures they are listed in ProjectConfig.Skills[].
// It also updates .skillshare/.gitignore for each tracked skill.
func ReconcileProjectSkills(projectRoot string, projectCfg *ProjectConfig, sourcePath string) error {
	if _, err := os.Stat(sourcePath); os.IsNotExist(err) {
		return nil // no skills dir yet
	}

	changed := false
	index := map[string]int{}
	for i, skill := range projectCfg.Skills {
		index[skill.Name] = i
	}

	err := filepath.WalkDir(sourcePath, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		if path == sourcePath {
			return nil
		}
		if !d.IsDir() {
			return nil
		}
		// Skip hidden directories
		if utils.IsHidden(d.Name()) {
			return filepath.SkipDir
		}
		// Skip .git directories
		if d.Name() == ".git" {
			return filepath.SkipDir
		}

		relPath, relErr := filepath.Rel(sourcePath, path)
		if relErr != nil {
			return nil
		}

		// Determine source and tracked status
		var source string
		tracked := isGitRepo(path)

		meta, metaErr := install.ReadMeta(path)
		if metaErr == nil && meta != nil && meta.Source != "" {
			source = meta.Source
		} else if tracked {
			// Tracked repos have no meta file; derive source from git remote
			source = gitRemoteOrigin(path)
		}
		if source == "" {
			// Not an installed skill — continue walking deeper
			return nil
		}

		// Use relative path as skill name (e.g. "frontend/pdf" or "_team-repo")
		skillName := filepath.ToSlash(relPath)

		if existingIdx, ok := index[skillName]; ok {
			if projectCfg.Skills[existingIdx].Source != source {
				projectCfg.Skills[existingIdx].Source = source
				changed = true
			}
			if projectCfg.Skills[existingIdx].Tracked != tracked {
				projectCfg.Skills[existingIdx].Tracked = tracked
				changed = true
			}
		} else {
			projectCfg.Skills = append(projectCfg.Skills, ProjectSkill{
				Name:    skillName,
				Source:  source,
				Tracked: tracked,
			})
			index[skillName] = len(projectCfg.Skills) - 1
			changed = true
		}

		if err := install.UpdateGitIgnore(filepath.Join(projectRoot, ".skillshare"), filepath.Join("skills", skillName)); err != nil {
			return fmt.Errorf("failed to update .skillshare/.gitignore: %w", err)
		}

		// If it's a tracked repo (has .git), don't recurse into it
		if tracked {
			return filepath.SkipDir
		}

		// If it has metadata, it's a leaf skill — don't recurse
		if meta != nil && meta.Source != "" {
			return filepath.SkipDir
		}

		return nil
	})
	if err != nil {
		return fmt.Errorf("failed to scan project skills: %w", err)
	}

	if changed {
		if err := projectCfg.Save(projectRoot); err != nil {
			return err
		}
	}

	return nil
}

// isGitRepo checks if the given path is a git repository (has .git/ directory or file).
func isGitRepo(path string) bool {
	_, err := os.Stat(filepath.Join(path, ".git"))
	return err == nil
}

// gitRemoteOrigin returns the "origin" remote URL for a git repo, or "" on failure.
func gitRemoteOrigin(repoPath string) string {
	cmd := exec.Command("git", "-C", repoPath, "remote", "get-url", "origin")
	out, err := cmd.Output()
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(out))
}
