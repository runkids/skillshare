package config

import (
	"fmt"
	"os"
	"path/filepath"

	"skillshare/internal/install"
	"skillshare/internal/utils"
)

// ReconcileGlobalSkills scans the global source directory for remotely-installed
// skills (those with install metadata or tracked repos) and ensures they are
// listed in Config.Skills[]. This is the global-mode counterpart of
// ReconcileProjectSkills.
func ReconcileGlobalSkills(cfg *Config) error {
	sourcePath := cfg.Source
	if _, err := os.Stat(sourcePath); os.IsNotExist(err) {
		return nil // no skills dir yet
	}

	changed := false
	index := map[string]int{}
	for i, skill := range cfg.Skills {
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
		if utils.IsHidden(d.Name()) {
			return filepath.SkipDir
		}
		if d.Name() == ".git" {
			return filepath.SkipDir
		}

		relPath, relErr := filepath.Rel(sourcePath, path)
		if relErr != nil {
			return nil
		}

		var source string
		tracked := isGitRepo(path)

		meta, metaErr := install.ReadMeta(path)
		if metaErr == nil && meta != nil && meta.Source != "" {
			source = meta.Source
		} else if tracked {
			source = gitRemoteOrigin(path)
		}
		if source == "" {
			return nil
		}

		skillName := filepath.ToSlash(relPath)

		if existingIdx, ok := index[skillName]; ok {
			if cfg.Skills[existingIdx].Source != source {
				cfg.Skills[existingIdx].Source = source
				changed = true
			}
			if cfg.Skills[existingIdx].Tracked != tracked {
				cfg.Skills[existingIdx].Tracked = tracked
				changed = true
			}
		} else {
			cfg.Skills = append(cfg.Skills, SkillEntry{
				Name:    skillName,
				Source:  source,
				Tracked: tracked,
			})
			index[skillName] = len(cfg.Skills) - 1
			changed = true
		}

		if tracked {
			return filepath.SkipDir
		}
		if meta != nil && meta.Source != "" {
			return filepath.SkipDir
		}

		return nil
	})
	if err != nil {
		return fmt.Errorf("failed to scan global skills: %w", err)
	}

	if changed {
		if err := cfg.Save(); err != nil {
			return err
		}
	}

	return nil
}
