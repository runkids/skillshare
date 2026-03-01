package config

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"skillshare/internal/install"
	"skillshare/internal/utils"
)

// ReconcileGlobalSkills scans the global source directory for remotely-installed
// skills (those with install metadata or tracked repos) and ensures they are
// listed in Config.Skills[]. This is the global-mode counterpart of
// ReconcileProjectSkills.
func ReconcileGlobalSkills(cfg *Config, reg *Registry) error {
	sourcePath := cfg.Source
	if _, err := os.Stat(sourcePath); os.IsNotExist(err) {
		return nil // no skills dir yet
	}

	changed := false
	index := map[string]int{}
	for i, skill := range reg.Skills {
		index[skill.FullName()] = i
	}

	// Migrate legacy entries: name "frontend/pdf" → group "frontend", name "pdf"
	for i := range reg.Skills {
		s := &reg.Skills[i]
		if s.Group == "" && strings.Contains(s.Name, "/") {
			group, bare := s.EffectiveParts()
			s.Group = group
			s.Name = bare
			changed = true
		}
	}

	walkRoot := utils.ResolveSymlink(sourcePath)
	err := filepath.WalkDir(walkRoot, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		if path == walkRoot {
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

		relPath, relErr := filepath.Rel(walkRoot, path)
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

		fullPath := filepath.ToSlash(relPath)

		if existingIdx, ok := index[fullPath]; ok {
			if reg.Skills[existingIdx].Source != source {
				reg.Skills[existingIdx].Source = source
				changed = true
			}
			if reg.Skills[existingIdx].Tracked != tracked {
				reg.Skills[existingIdx].Tracked = tracked
				changed = true
			}
		} else {
			entry := SkillEntry{
				Source:  source,
				Tracked: tracked,
			}
			if idx := strings.LastIndex(fullPath, "/"); idx >= 0 {
				entry.Group = fullPath[:idx]
				entry.Name = fullPath[idx+1:]
			} else {
				entry.Name = fullPath
			}
			reg.Skills = append(reg.Skills, entry)
			index[fullPath] = len(reg.Skills) - 1
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
		if err := reg.Save(filepath.Dir(ConfigPath())); err != nil {
			return err
		}
	}

	return nil
}
