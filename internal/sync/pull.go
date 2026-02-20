package sync

import (
	"fmt"
	"os"
	"path/filepath"
	"time"

	"skillshare/internal/utils"
)

// LocalSkillInfo describes a local skill in a target directory
type LocalSkillInfo struct {
	Name       string
	Path       string
	TargetName string
	Size       int64
	ModTime    time.Time
}

// PullOptions holds options for pull operation
type PullOptions struct {
	DryRun bool
	Force  bool
}

// PullResult describes the result of a pull operation
type PullResult struct {
	Pulled  []string
	Skipped []string
	Failed  map[string]error
}

// FindLocalSkills finds all local (non-symlinked) skills in a target directory
func FindLocalSkills(targetPath, sourcePath string) ([]LocalSkillInfo, error) {
	var skills []LocalSkillInfo

	// Check if target exists
	info, err := os.Lstat(targetPath)
	if err != nil {
		if os.IsNotExist(err) {
			return skills, nil // Empty list if target doesn't exist
		}
		return nil, err
	}

	// If target is a symlink pointing to source, no local skills exist
	if info.Mode()&os.ModeSymlink != 0 {
		link, err := os.Readlink(targetPath)
		if err != nil {
			return nil, err
		}
		absLink, _ := filepath.Abs(link)
		absSource, _ := filepath.Abs(sourcePath)
		if utils.PathsEqual(absLink, absSource) {
			// Symlink mode - no local skills
			return skills, nil
		}
		// Symlink to somewhere else - also no local skills
		return skills, nil
	}

	// Target is a directory (merge or copy mode) - scan for local skills

	// Read manifest to identify copy-mode managed skills
	manifest, _ := ReadManifest(targetPath)

	entries, err := os.ReadDir(targetPath)
	if err != nil {
		return nil, err
	}

	for _, entry := range entries {
		// Skip hidden files/directories
		if utils.IsHidden(entry.Name()) {
			continue
		}

		skillPath := filepath.Join(targetPath, entry.Name())
		skillInfo, err := os.Lstat(skillPath)
		if err != nil {
			continue
		}

		// Skip symlinks (these are synced from source)
		if skillInfo.Mode()&os.ModeSymlink != 0 {
			continue
		}

		// Only process directories (skills are directories)
		if !skillInfo.IsDir() {
			continue
		}

		// Skip copy-mode managed skills
		if manifest != nil {
			if _, isManaged := manifest.Managed[entry.Name()]; isManaged {
				continue
			}
		}

		// This is a local skill
		skills = append(skills, LocalSkillInfo{
			Name:    entry.Name(),
			Path:    skillPath,
			ModTime: skillInfo.ModTime(),
			Size:    calculateDirSize(skillPath),
		})
	}

	return skills, nil
}

// PullSkill copies a single skill from target to source
func PullSkill(skill LocalSkillInfo, sourcePath string, force bool) error {
	destPath := filepath.Join(sourcePath, skill.Name)

	// Check if skill already exists in source
	if _, err := os.Stat(destPath); err == nil {
		if !force {
			return fmt.Errorf("already exists in source")
		}
		// Remove existing to overwrite
		if err := os.RemoveAll(destPath); err != nil {
			return fmt.Errorf("failed to remove existing: %w", err)
		}
	}

	// Copy skill to source
	return copyDirectory(skill.Path, destPath)
}

// PullSkills pulls multiple skills to source
func PullSkills(skills []LocalSkillInfo, sourcePath string, opts PullOptions) (*PullResult, error) {
	result := &PullResult{
		Failed: make(map[string]error),
	}

	for _, skill := range skills {
		if opts.DryRun {
			result.Pulled = append(result.Pulled, skill.Name)
			continue
		}

		err := PullSkill(skill, sourcePath, opts.Force)
		if err != nil {
			if err.Error() == "already exists in source" {
				result.Skipped = append(result.Skipped, skill.Name)
			} else {
				result.Failed[skill.Name] = err
			}
			continue
		}

		result.Pulled = append(result.Pulled, skill.Name)
	}

	return result, nil
}

// calculateDirSize calculates total size of a directory
func calculateDirSize(path string) int64 {
	var size int64
	filepath.Walk(path, func(_ string, info os.FileInfo, err error) error {
		if err != nil {
			return nil
		}
		if !info.IsDir() {
			size += info.Size()
		}
		return nil
	})
	return size
}
