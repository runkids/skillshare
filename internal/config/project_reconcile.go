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

// ReconcileProjectSkills scans the project source directory for remotely-installed
// skills (those with install metadata) and ensures they are listed in ProjectConfig.Skills[].
// It also updates .skillshare/.gitignore for each tracked skill.
func ReconcileProjectSkills(projectRoot string, projectCfg *ProjectConfig, sourcePath string) error {
	entries, err := os.ReadDir(sourcePath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil // no skills dir yet
		}
		return fmt.Errorf("failed to read project skills: %w", err)
	}

	changed := false
	index := map[string]int{}
	for i, skill := range projectCfg.Skills {
		index[skill.Name] = i
	}

	for _, entry := range entries {
		if !entry.IsDir() || utils.IsHidden(entry.Name()) {
			continue
		}

		skillName := entry.Name()
		skillPath := filepath.Join(sourcePath, skillName)

		// Determine source and tracked status
		var source string
		tracked := isGitRepo(skillPath)

		meta, err := install.ReadMeta(skillPath)
		if err == nil && meta != nil && meta.Source != "" {
			source = meta.Source
		} else if tracked {
			// Tracked repos have no meta file; derive source from git remote
			source = gitRemoteOrigin(skillPath)
		}
		if source == "" {
			continue
		}

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
